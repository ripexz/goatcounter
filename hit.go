// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the AGPLv3,
// which can be found in the LICENSE file or at gnu.org/licenses/agpl.html

package goatcounter

import (
	"context"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"
	"github.com/teamwork/utils/jsonutil"
	"github.com/teamwork/validate"
	"zgo.at/zlog"
)

func ptr(s string) *string { return &s }

// ref_scheme column
var (
	RefSchemeHTTP      = ptr("h")
	RefSchemeOther     = ptr("o")
	RefSchemeGenerated = ptr("g")
)

type Hit struct {
	Site int64 `db:"site" json:"-"`

	Path        string    `db:"path" json:"p,omitempty"`
	Ref         string    `db:"ref" json:"r,omitempty"`
	RefParams   *string   `db:"ref_params" json:"ref_params,omitempty"`
	RefOriginal *string   `db:"ref_original" json:"ref_original,omitempty"`
	RefScheme   *string   `db:"ref_scheme" json:"ref_scheme,omitempty"`
	CreatedAt   time.Time `db:"created_at" json:"-"`

	refURL *url.URL `db:"-"`
}

func cleanURL(ref string, refURL *url.URL) (string, *string, bool, bool) {
	// Always remove protocol.
	refURL.Scheme = ""
	if p := strings.Index(ref, ":"); p > -1 && p < 7 {
		ref = ref[p+3:]
	}

	changed := false

	// Normalize hosts.
	if a, ok := hostAlias[refURL.Host]; ok {
		changed = true
		refURL.Host = a
	}

	// Group based on host.
	if g, ok := hostGroups[refURL.Host]; ok {
		// TODO: not all are "generated".
		return g.URL, nil, true, g.Generated
	}

	// Group based on host + path.
	if g, ok := pathGroups[path.Join(refURL.Host, refURL.Path)]; ok {
		return g.URL, nil, true, g.Generated
	}

	// Special groupings.
	for _, grouping := range groupings {
		g := grouping(refURL)
		if g != nil {
			return g.ref, g.params, g.store, g.generated
		}
	}

	// Reddit
	// https://www.reddit.com/r/programming/top
	// https://www.reddit.com/r/programming/.compact
	// https://www.reddit.com/r/programming.compact
	// https://www.reddit.com/r/webdev/new
	// TODO: put in grouping
	if refURL.Host == "www.reddit.com" {
		switch {
		case strings.HasSuffix(refURL.Path, "/top") || strings.HasSuffix(refURL.Path, "/new"):
			refURL.Path = refURL.Path[:len(refURL.Path)-4]
			changed = true
		case strings.HasSuffix(refURL.Path, ".compact"):
			refURL.Path = refURL.Path[:len(refURL.Path)-8]
			changed = true
		}
	}

	// Clean query parameters.
	i := strings.Index(ref, "?")
	if i == -1 {
		// No parameters so no work.
		return strings.TrimLeft(refURL.String(), "/"), nil, changed, false
	}
	eq := ref[i+1:]
	ref = ref[:i]

	// Twitter's t.co links add this.
	if refURL.Host == "t.co" && eq == "amp=1" {
		return ref, nil, false, false
	}

	q := refURL.Query()
	refURL.RawQuery = ""
	start := len(q)

	// Google analytics tracking parameters.
	q.Del("utm_source")
	q.Del("utm_medium")
	q.Del("utm_campaign")
	q.Del("utm_term")

	if len(q) == 0 {
		return refURL.String()[2:], nil, changed || len(q) != start, false
	}
	eq = q.Encode()
	return refURL.String()[2:], &eq, changed || len(q) != start, false
}

// Defaults sets fields to default values, unless they're already set.
func (h *Hit) Defaults(ctx context.Context) {
	// TODO: not doing this as it's not set from memstore.
	// site := MustGetSite(ctx)
	// h.Site = site.ID

	if h.CreatedAt.IsZero() {
		h.CreatedAt = time.Now().UTC()
	}

	if h.Ref != "" && h.refURL != nil {
		if h.refURL.Scheme == "http" || h.refURL.Scheme == "https" {
			h.RefScheme = RefSchemeHTTP
		} else {
			h.RefScheme = RefSchemeOther
		}

		var store, generated bool
		r := h.Ref
		h.Ref, h.RefParams, store, generated = cleanURL(h.Ref, h.refURL)
		if store {
			h.RefOriginal = &r
		}

		if generated {
			h.RefScheme = RefSchemeGenerated
		}
	}

	h.Ref = strings.TrimRight(h.Ref, "/")
	h.Path = "/" + strings.Trim(h.Path, "/")
}

// Validate the object.
func (h *Hit) Validate(ctx context.Context) error {
	v := validate.New()

	v.Required("site", h.Site)
	v.Required("path", h.Path)

	return v.ErrorOrNil()
}

// Insert a new row.
//
// Note: this is also in memstore.go
func (h *Hit) Insert(ctx context.Context) error {
	var err error
	h.refURL, err = url.Parse(h.Ref)
	if err != nil {
		zlog.Fields(zlog.F{"ref": h.Ref}).Errorf("could not parse ref: %s", err)
	}

	// Ignore spammers.
	// TODO: move to handler?
	if _, ok := blacklist[h.refURL.Host]; ok {
		return nil
	}

	h.Defaults(ctx)
	err = h.Validate(ctx)
	if err != nil {
		return err
	}

	_, err = MustGetDB(ctx).ExecContext(ctx,
		`insert into hits (site, path, ref, ref_params, ref_original, created_at, ref_scheme)
		values ($1, $2, $3, $4, $5, $6, $7)`,
		h.Site, h.Path, h.Ref, h.RefParams, h.RefOriginal, sqlDate(h.CreatedAt), h.RefScheme)
	return errors.Wrap(err, "Hit.Insert")
}

type Hits []Hit

func (h *Hits) List(ctx context.Context) error {
	return errors.Wrap(MustGetDB(ctx).SelectContext(ctx, h,
		`select * from hits where site=$1`, MustGetSite(ctx).ID),
		"Hits.List")
}

type HitStat struct {
	Day  string
	Days [][]int
}

type hs struct {
	Count     int     `db:"count"`
	Max       int     `db:"-"`
	Path      string  `db:"path"`
	RefScheme *string `db:"ref_scheme"`
	Stats     []HitStat
}

type HitStats []hs

func (h *HitStats) List(ctx context.Context, start, end time.Time, exclude []string) (int, int, bool, error) {
	db := MustGetDB(ctx)
	site := MustGetSite(ctx)

	limit := site.Settings.Limits.Page
	if limit == 0 {
		limit = 20
	}
	more := false
	if len(exclude) > 0 {
		// Get one page more so we can detect if there are more pages after
		// this.
		more = true
		limit++
	}

	query := `
		select
			path, count(path) as count
		from hits
		where
			site=? and
			created_at >= ? and
			created_at <= ?`
	args := []interface{}{site.ID, dayStart(start), dayEnd(end)}

	// Quite a bit faster to not check path.
	if len(exclude) > 0 {
		args = append(args, exclude)
		query += ` and path not in (?) `
	}

	query, args, err := sqlx.In(query+`
		group by path
		order by count desc
		limit ?`, append(args, limit)...)
	if err != nil {
		return 0, 0, false, errors.Wrap(err, "HitStats.List")
	}

	l := zlog.Module("HitStats.List")

	err = db.SelectContext(ctx, h, db.Rebind(query), args...)
	if err != nil {
		return 0, 0, false, errors.Wrap(err, "HitStats.List")
	}
	l = l.Since("select hits")

	if more {
		if len(*h) == limit {
			x := *h
			x = x[:len(x)-1]
			*h = x
		} else {
			more = false
		}
	}

	// Add stats
	type stats struct {
		Path  string    `json:"path"`
		Day   time.Time `json:"day"`
		Stats []byte    `json:"stats"`
	}
	var st []stats
	err = db.SelectContext(ctx, &st, `
		select path, day, stats
		from hit_stats
		where
			site=$1 and
			day >= $2 and
			day <= $3
		order by day asc`,
		site.ID, start.Format("2006-01-02"), end.Format("2006-01-02"))
	if err != nil {
		return 0, 0, false, errors.Wrap(err, "HitStats.List")
	}
	l = l.Since("select hits_stats")

	// TODO: meh...
	hh := *h
	totalDisplay := 0
	for i := range hh {
		for _, s := range st {
			if s.Path == hh[i].Path {
				var x [][]int
				jsonutil.MustUnmarshal(s.Stats, &x)
				hh[i].Stats = append(hh[i].Stats, HitStat{Day: s.Day.Format("2006-01-02"), Days: x})

				// Get max.
				for j := range x {
					totalDisplay += x[j][1]
					if x[j][1] > hh[i].Max {
						hh[i].Max = x[j][1]
					}
				}
			}
		}

		if hh[i].Max < 10 {
			hh[i].Max = 10
		}
	}

	l = l.Since("reorder data")

	// Get total.
	total := 0
	err = db.GetContext(ctx, &total, `
		select count(path)
		from hits
		where
			site=$1 and
			created_at >= $2 and
			created_at <= $3`,
		site.ID, dayStart(start), dayEnd(end))

	l = l.Since("get total")
	return total, totalDisplay, more, errors.Wrap(err, "HitStats.List")
}

// ListRefs lists all references for a path.
func (h *HitStats) ListRefs(ctx context.Context, path string, start, end time.Time, offset int) (bool, error) {
	site := MustGetSite(ctx)

	limit := site.Settings.Limits.Ref
	if limit == 0 {
		limit = 10
	}

	// TODO: using offset for pagination is not ideal:
	// data can change in the meanwhile, and it still gets the first N rows,
	// which is more expensive than it needs to be.
	// It's "good enough" for now, though.
	err := MustGetDB(ctx).SelectContext(ctx, h, `
		select
			ref as path,
			count(ref) as count,
			ref_scheme
		from hits
		where
			site=$1 and
			lower(path)=lower($2) and
			created_at >= $3 and
			created_at <= $4
		group by ref, ref_scheme
		order by count(*) desc, path desc
		limit $5 offset $6`,
		site.ID, path, dayStart(start), dayEnd(end), limit+1, offset)

	more := false
	if len(*h) > limit {
		more = true
		x := *h
		x = x[:len(x)-1]
		*h = x
	}

	return more, errors.Wrap(err, "RefStats.ListRefs")
}

// ListPaths lists all paths we have statistics for.
func (h *HitStats) ListPaths(ctx context.Context) ([]string, error) {
	var paths []string
	err := MustGetDB(ctx).SelectContext(ctx, &paths,
		`select path from hit_stats where site=$1`, MustGetSite(ctx).ID)
	return paths, errors.Wrap(err, "Hits.ListPaths")
}
