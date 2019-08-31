package goatcounter

import (
	"net/url"
	"strings"

	"zgo.at/zlog"
)

type (
	group struct {
		URL       string
		Generated bool
	}

	grouping struct {
		ref       string
		params    *string
		store     bool
		generated bool
	}
)

var (
	hackernews = group{"Hacker News", true}
	email      = group{"Email", true}
	google     = group{"Google", true}
	reddit     = group{"www.reddit.com", false}
	facebook   = group{"www.facebook.com", false}
)

var (
	// Normalize hosts; mostly for special domains for mobile and the like.V
	hostAlias = map[string]string{
		"en.m.wikipedia.org": "en.wikipedia.org",
		"m.facebook.com":     "www.facebook.com",
		"m.habr.com":         "habr.com",
		"old.reddit.com":     "www.reddit.com",
		"i.reddit.com":       "www.reddit.com",
		"np.reddit.com":      "www.reddit.com",
		"fr.reddit.com":      "www.reddit.com",
	}

	// Group based on host.
	hostGroups = map[string]group{
		// HN has <meta name="referrer" content="origin"> so we only get the domain.
		"news.ycombinator.com":               hackernews,
		"hn.algolia.com":                     hackernews,
		"hckrnews.com":                       hackernews,
		"hn.premii.com":                      hackernews,
		"com.stefandekanski.hackernews.free": hackernews,
		"io.github.hidroh.materialistic":     hackernews,
		"hackerweb.app":                      hackernews,
		"www.daemonology.net/hn-daily":       hackernews,
		"quiethn.com":                        hackernews,
		// http://www.elegantreader.com/item/17358103
		// https://www.daemonology.net/hn-daily/2019-05.html

		"mail.google.com":       email,
		"com.google.android.gm": email,
		"mail.yahoo.com":        email,
		//  https://mailchi.mp

		"com.google.android.googlequicksearchbox":                      google,
		"com.google.android.googlequicksearchbox/https/www.google.com": google,

		"com.andrewshu.android.reddit":       reddit,
		"com.laurencedawson.reddit_sync":     reddit,
		"com.laurencedawson.reddit_sync.dev": reddit,
		"com.laurencedawson.reddit_sync.pro": reddit,

		"m.facebook.com":  facebook,
		"l.facebook.com":  facebook,
		"lm.facebook.com": facebook,

		"com.Slack":              group{"Slack Chat", true},
		"com.linkedin.android":   group{"www.linkedin.com", false},
		"org.fox.ttrss":          group{"RSS", true},
		"org.telegram.messenger": group{"Telegram Messenger", true},
	}

	// Group based on host+path.
	pathGroups = map[string]group{
		"www.daemonology.net/hn-daily": group{"Hacker News", true},
		"www.linkedin.com/feed":        group{"www.linkedin.com", false},
		"getpocket.com/redirect":       group{"getpocket.com", false},
	}

	groupings = []func(*url.URL) *grouping{
		// I'm not sure where these links are generated, but there are *a lot*
		// of them all with different paths.
		func(ref *url.URL) *grouping {
			if ref.Host != "link.oreilly.com" {
				return nil
			}
			return &grouping{ref: "link.oreilly.com", params: nil, store: true, generated: false}
		},

		// Group all "google.co.nz", "google.nl", etc. as "Google".
		func(ref *url.URL) *grouping {
			if !strings.HasPrefix(ref.Host, "www.google.") {
				return nil
			}
			return &grouping{ref: "Google", params: nil, store: true, generated: true}
		},

		// Useful: https://lobste.rs/s/tslw6k/why_i_m_still_using_jquery_2019
		// Not really: https://lobste.rs/newest/page/8, https://lobste.rs/page/7
		//             https://lobste.rs/search, https://lobste.rs/t/javascript
		func(ref *url.URL) *grouping {
			if refURL.Host == "lobste.rs" && !strings.HasPrefix(refURL.Path, "/s/") {
				return &grouping{ref: "lobste.rs", params: nil, store: true, generated: false}
			}
			return nil
		},

		// Special-fu for Feedly.
		func(ref *url.URL) *grouping {
			if !strings.HasPrefix(ref.Host, "feedly.com") {
				return nil
			}

			// These URLs are all private, and we can't get any informatio from
			// them. Just list as "Feedly".
			//
			// https://feedly.com/i/collection/content/user/e5b84827-c85e-47db-81e6-15edd38e48f6/category/os-news
			// https://feedly.com/i/tag/user/34270c99-ef32-4b69-9e66-91f647b26247/tag/Test
			// https://feedly.com/i/category/programming
			if ref.Path == "/i/latest" ||
				ref.Path == "/i/my" ||
				ref.Path == "/i/saved" ||
				strings.HasPrefix(ref.Path, "/i/collection/") ||
				strings.HasPrefix(ref.Path, "/i/tag/") ||
				strings.HasPrefix(ref.Path, "/i/category/") {
				return &grouping{ref: "feedly.com", params: nil, store: true, generated: false}
			}

			// Subscriptions:
			// https://feedly.com/i/subscription/feed%2Fhttp%3A%2F%2Fafreshcup.com%2Ffeed%2F
			// https://feedly.com/i/subscription/feed%2Fhttp%3A%2F%2Fafreshcup.com%2Fhome%2Frss.xml
			// https://feedly.com/i/subscription/feed%2Fhttp%3A%2F%2Fb.hatena.ne.jp%2FRockridge%2Finterest.rss%3Fword%3Djavascript%26key%3Df919e91e6d5a8c39f710390e3f2703d2e0fee557
			// https://feedly.com/i/subscription/feed%2Fhttp%3A%2F%2Ffeeds.feedburner.com%2FCodrops
			// https://feedly.com/i/subscription/feed%2Fhttp%3A%2F%2Ffeeds.feedburner.com%2Fcodrops
			// https://feedly.com/i/subscription/feed%2Fhttp%3A%2F%2Ffeeds.feedburner.com%2Fthechangelog
			// https://feedly.com/i/subscription/feed%2Fhttp%3A%2F%2Ffeeds.feedburner.com%2Ftympanus
			// https://feedly.com/i/subscription/feed%2Fhttp%3A%2F%2Ffeeds2.feedburner.com%2Ftympanus
			// https://feedly.com/i/subscription/feed%2Fhttp%3A%2F%2Ffeeds2.feedburner.com%2Fvnf
			// https://feedly.com/i/subscription/feed%2Fhttp%3A%2F%2Ffrontendfront.com%2Ffeed%2Fstories
			// https://feedly.com/i/subscription/feed%2Fhttp%3A%2F%2Fhnbest.heroku.com%2Frss
			// https://feedly.com/i/subscription/feed%2Fhttp%3A%2F%2Fhnrss.org%2Fnewest%3Fpoints%3D300
			// https://feedly.com/i/subscription/feed%2Fhttp%3A%2F%2Fhnrss.org%2Fnewest%3Fpoints%3D400
			// https://feedly.com/i/subscription/feed%2Fhttp%3A%2F%2Fnews.ycombinator.com%2Frss
			// https://feedly.com/i/subscription/feed%2Fhttp%3A%2F%2Fprogramming.reddit.com%2F.rss
			// https://feedly.com/i/subscription/feed%2Fhttp%3A%2F%2Fthechangelog.com%2Frss
			// https://feedly.com/i/subscription/feed%2Fhttp%3A%2F%2Ftympanus.net%2Fcodrops%2Fcollective%2Ffeed%2F
			// https://feedly.com/i/subscription/feed%2Fhttp%3A%2F%2Ftympanus.net%2Fcodrops%2Ffeed
			// https://feedly.com/i/subscription/feed%2Fhttp%3A%2F%2Ftympanus.net%2Fcodrops%2Ffeed%2F
			// https://feedly.com/i/subscription/feed%2Fhttp%3A%2F%2Fwww.daemonology.net%2Fhn-daily%2Findex.rss
			// https://feedly.com/i/subscription/feed%2Fhttps%3A%2F%2Fjavascriptweekly.com%2Frss%2F1a537ef6
			// https://feedly.com/i/subscription/feed%2Fhttps%3A%2F%2Flobste.rs%2Frss
			// https://feedly.com/i/subscription/feed%2Fhttps%3A%2F%2Fnews.ycombinator.com%2Frss
			if strings.HasPrefix(ref.Path, "/i/subscription/feed%2F") {
				p, err := url.PathUnescape(ref.Path[23:])
				if err != nil {
					zlog.Error(err)
					return nil
				}
				return &grouping{ref: p, params: nil, store: false, generated: false}
			}

			// TODO: get feed from this too.
			// https://feedly.com/i/entry/+XHjch7MQtkDE3jVoUKNd7EXkxgLP+qd5d/qDPKdWEI=_16b1e5448ca:a8305:2a7e54a4
			// https://feedly.com/i/entry/1gOA8sgsyIN6Fa4oaXZX0qh2K2SOUMLVRi6qwkvVFZQ=_16a9fa31a3c:ac380:2a7e54a4
			// https://feedly.com/i/entry/5Td+U2A0pKfHcMqAZWYZgKWgpIItLeNiq7cfP1bAozw=_16b0df5c298:11e19b3:fe3711f1
			// https://feedly.com/i/entry/Adyh05yyS6P2dEGA70P6RZTpm+9fcVBbj3rdOPnTg2U=_16afdbe053e:1463b62:5de7e37
			// https://feedly.com/i/entry/BbTAX4LtCddgo1dM9OS8qYvn5PrOso8rKvu1tqTRuaI=_16b270db86b:1bd6c1e:5de7e37
			// https://feedly.com/i/entry/BgxeOpEdUOr+F1shMt4oTZESvhLX3biNkfeafPoI1ls=_16b014c2c26:15e0ec:2a7e54a4
			// https://feedly.com/i/entry/GKe86Rj3pD5b6EaS4Zyzok+G7xsA1CH+GpIvK65W36o=_16b21ca342a:14cffaa:247b6d24
			// https://feedly.com/i/entry/LpvBBqJY4++R44Zq5/58hQix7jj+lojUroKrFpT5mXQ=_16afd36ca5b:1441cdb:d02fd57c
			// https://feedly.com/i/entry/MOQoYVKSGzmBHETaDeZW0XIL4IDlBoxFVszHLV+Buf4=_16b1e540fec:a8283:2a7e54a4
			// https://feedly.com/i/entry/OBiicYSFN1mEqpoRsG2xFp1XPbzTZoRkMFmH5jF1S1Q=_16b32bfe265:451b38:ccd3afb0
			// https://feedly.com/i/entry/OWncAkp3cxHDLDRO6zssSYezi4/LdolIIrvrPVfGH+A=_16b46bcf7f9:44cace1:247b6d24
			// https://feedly.com/i/entry/QTcFpL1t+TisCjuFx2gTugZzLFIZRgyolz1HqkxJ1LQ=_16afa6fb243:110de5b:d02fd57c
			// https://feedly.com/i/entry/SEWCTMlbfcpbJP4Zfymks5Trfv5LMfLHy3ysLbAuIYw=_16a7e2277d8:17975ff:5de7e37
			// https://feedly.com/i/entry/SWtoDyS4ef0/KFFSC5sNzFYqFz1ETKQ9S0n54MHvVj8=_16b19eab304:8968a6:f9e594d2
			// https://feedly.com/i/entry/TDtO97MoJ4N+0nIO0Z8/bclS7UZEXW/ViF2oDlfAx98=_16b16c6e172:737f03:5de7e37
			// https://feedly.com/i/entry/TkvV5X4IW/zSWWpa3DpW5z38rd/Z27cqYQHckSpHn5M=_16b26f7acb1:1bb2527:5de7e37
			// https://feedly.com/i/entry/VLAzliamc330wxof9ziLWEvyHNu4J3VoQ9DJdnKPLVw=_16b1e544897:a82ff:2a7e54a4
			// https://feedly.com/i/entry/YaoPgL8nzZ+zS2v6RnVHrvn0uak6PfMnjwN5FLr831A=_16b11a1bfeb:21c8b9:5de7e37
			// https://feedly.com/i/entry/aqJa42RfrEUev8ScN2ZST7jB0w3pbk5UVyXRhFJywqY=_16b41523115:3cd9607:d02fd57c
			// https://feedly.com/i/entry/bP6fhtnktJjMnJete3SsX4RROHezPMw0Xyp0+sHhQik=_16b32d71bc2:2c2be4d:5de7e37
			// https://feedly.com/i/entry/boy6SFLkVrCyAh1eWapnAnWnZYkgaqPKxviKcFW5h20=_16af91f74cf:f2dc42:247b6d24
			// https://feedly.com/i/entry/d7/1YuAkdmL4BHkhjZ6Y4gbgRmrrRabY+Tv1pdGdNG0=_16b1e541065:a8289:2a7e54a4
			// https://feedly.com/i/entry/dr5pNMWznZrq0ZL2xn6uJHVq1sjh5WqNEfV0sxmuuFk=_16a9fad6a61:108a9ad:c67e73a
			// https://feedly.com/i/entry/g14T3bW1LujmJt8KOQKIRZoELZcdJQfF/izb+rjqI+g=_16b32aaec1e:2ce4986:247b6d24
			// https://feedly.com/i/entry/hFygvKgYkpMGeUvtvnY+JL7+nt6/iLQIrzrP/Jkgv5U=_16b7946d9e0:ec2edd:5e307cc6
			// https://feedly.com/i/entry/kgfeR2Ig/Cnt8U/wi+f4OM6pmg78zjlG+144gk4PnMs=_16b32be5755:2d050e5:d02fd57c
			// https://feedly.com/i/entry/nCR2RBYuO17VUiXWDrmJID4Ggyw0xANAetg/QelkBsk=_16b1e5813f7:101e854:d02fd57c
			// https://feedly.com/i/entry/oNWXIUFEq3deZ9p31Bzsn84rUoNNfVyF0iFTFhkkP/M=_16afa891b71:10a01cf:5de7e37
			// https://feedly.com/i/entry/rUbv9J05YglevoMf/+srwnFVKf4NlmylIWprW57lUxk=_16b1e54489f:a8302:2a7e54a4
			// https://feedly.com/i/entry/uEmeUNrQsHJpft8vs62AZfb2Vf1BFJ3jW+p2WlOf7VU=_16afbbf0655:11f7925:5de7e37
			// https://feedly.com/i/entry/uW6OVyMOU/Wf09ueJdtxBuVG3zPAAxiGRBuAOlsem8k=_16afd016619:135b309:5de7e37
			// https://feedly.com/i/entry/v9DLEPBnAH/mTf8LoFpM0IJgfpr6xCLso52Uas9kWOs=_16b16c91e74:77fb62:247b6d24
			// https://feedly.com/i/entry/vquR+QmrNVZwz4vl4xIsDCov4KdPo2zT4jJlCEpbzCc=_16b1e5410af:a828a:2a7e54a4
			// https://feedly.com/i/entry/wc5eRRoWELg/euZnSyLRevs1/md3IP+kwEFqGblYO1g=_16b0edee4e5:136ec52:d02fd57c
			return nil
		},
	}
)

type groupFunc func(url.URL) (string, *string, bool, bool)
