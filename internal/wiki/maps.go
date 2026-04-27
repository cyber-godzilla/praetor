package wiki

import "sort"

// MapSections returns the ordered list of curated map page sections.
// These point at wiki pages too (same BaseURL); the data is separate
// from Sections() because /maps is a focused location-only browser.
func MapSections() []Section {
	return mapSections
}

// LookupMap finds a map bookmark by key. Match is case-insensitive and
// treats underscores/spaces as hyphens. Returns the matching slug and
// true on hit; ("", false) on miss.
func LookupMap(key string) (string, bool) {
	norm := normalize(key)
	for _, sec := range mapSections {
		for _, bm := range sec.Bookmarks {
			if normalize(bm.Key) == norm {
				return bm.Slug, true
			}
		}
	}
	return "", false
}

// MapKeys returns all map bookmark keys, sorted alphabetically.
func MapKeys() []string {
	var out []string
	for _, sec := range mapSections {
		for _, bm := range sec.Bookmarks {
			out = append(out, bm.Key)
		}
	}
	sort.Strings(out)
	return out
}

var mapSections = []Section{
	{
		Name: "Iridine",
		Bookmarks: []Bookmark{
			{Key: "iridine", Slug: "iridine"},
			{Key: "bronze-lane", Slug: "bronze-lane"},
			{Key: "campus-martius", Slug: "campus-martius"},
			{Key: "colosseum", Slug: "colosseum"},
			{Key: "ravanite-tunnels", Slug: "east-ravanite-tunnels"},
			{Key: "forum", Slug: "forum"},
			{Key: "gardens", Slug: "gardens-and-hospice"},
			{Key: "hospice", Slug: "gardens-and-hospice"},
			{Key: "harbor", Slug: "harbor"},
			{Key: "old-city", Slug: "old-city-and-moondeep"},
			{Key: "riverside", Slug: "riverside"},
			{Key: "sandbar", Slug: "sandbar"},
			{Key: "sewers", Slug: "sewers-and-sea-caves"},
			{Key: "sea-caves", Slug: "sewers-and-sea-caves"},
			{Key: "storm-drains", Slug: "storm-drain-system"},
			{Key: "signaltower", Slug: "signal-tower-island"},
			{Key: "transinvexium", Slug: "transinvexium"},
			{Key: "vetallun-road", Slug: "vetallun-road"},
			{Key: "quartz-heights", Slug: "quartz-heights"},
			{Key: "shipwreck", Slug: "shipwreck"},
			{Key: "rat-pits", Slug: "rat-pits-and-aralex-pits"},
			{Key: "aralex-pits", Slug: "rat-pits-and-aralex-pits"},
			{Key: "old-city-ustrinum", Slug: "hg-old-city"},
		},
	},
	{
		Name: "The Steps",
		Bookmarks: []Bookmark{
			{Key: "the-steps", Slug: "the-steps"},
			{Key: "the-steps-north", Slug: "the-steps-north"},
			{Key: "the-steps-central", Slug: "the-steps-central"},
			{Key: "the-steps-south", Slug: "the-steps-south"},
			{Key: "the-steps-east", Slug: "the-steps-east"},
			{Key: "the-steps-sewers", Slug: "the-steps-sewers"},
			{Key: "steps-ludus-quintus", Slug: "steps-ludus-quintus"},
		},
	},
	{
		Name: "Invex River Delta",
		Bookmarks: []Bookmark{
			{Key: "west-grasslands", Slug: "the-west-grasslands"},
			{Key: "burnt-villa", Slug: "burnt-villa"},
			{Key: "spider-caverns", Slug: "spider-caverns"},
			{Key: "vetallun", Slug: "vetallun"},
		},
	},
	{
		Name: "Salinae Swamp",
		Bookmarks: []Bookmark{
			{Key: "swamp", Slug: "the-salinae-swamp"},
			{Key: "salt-flats", Slug: "salt-flats"},
			{Key: "swamp-mansion", Slug: "swamp-mansion"},
			{Key: "swamp-vale", Slug: "swamp-vale"},
			{Key: "worm-temple", Slug: "worm-temple"},
			{Key: "bandit-complex", Slug: "hg-bandit-complex"},
		},
	},
	{
		Name: "Eastern Grasslands",
		Bookmarks: []Bookmark{
			{Key: "eastern-grasslands", Slug: "eastern-grasslands-and-woods"},
			{Key: "black-hand-caverns", Slug: "black-hand-caverns"},
			{Key: "black-hand-mines", Slug: "black-hand-mines"},
			{Key: "blackvine", Slug: "blackvine"},
			{Key: "brigand-treehouse", Slug: "brigand-treehouse"},
			{Key: "esecarnus-caves", Slug: "esecarnus-caves"},
			{Key: "grey-sands", Slug: "grey-sands"},
			{Key: "filinius-villa", Slug: "hg-filinius-villa"},
		},
	},
	{
		Name: "Rock Valley",
		Bookmarks: []Bookmark{
			{Key: "rock-valley", Slug: "rock-valley"},
			{Key: "town-of-rock-valley", Slug: "town-of-rock-valley-map"},
			{Key: "fenri-gifr-ruins", Slug: "fenri-gifr-ruins"},
			{Key: "rock-valley-dumps", Slug: "rock-valley-dumps"},
			{Key: "rock-valley-mine", Slug: "rock-valley-mine"},
			{Key: "stromheim", Slug: "stromheim"},
			{Key: "rock-valley-well", Slug: "rock-valley-well"},
			{Key: "undertown", Slug: "hg-undertown"},
			{Key: "burial-grounds", Slug: "hg-rock-valley-burial-grounds"},
			{Key: "resting-place", Slug: "hg-rock-valley-resting-place"},
			{Key: "broken-tower", Slug: "hg-rock-valley-broken-tower"},
		},
	},
	{
		Name: "Franlius",
		Bookmarks: []Bookmark{
			{Key: "franlius", Slug: "franlius"},
		},
	},
	{
		Name: "Monlon",
		Bookmarks: []Bookmark{
			{Key: "monlon", Slug: "monlon"},
			{Key: "monlon-mines", Slug: "hg-monlon-mines"},
			{Key: "kelestian-outpost", Slug: "monlon-kelestian-outpost"},
			{Key: "monlon-ravines", Slug: "hg:monlon-ravines"},
			{Key: "monlon-battlefields", Slug: "hg:monlon-battlefields"},
		},
	},
	{
		Name: "Seld",
		Bookmarks: []Bookmark{
			{Key: "seld", Slug: "seld"},
		},
	},
	{
		Name: "Cullaiden Island",
		Bookmarks: []Bookmark{
			{Key: "cullaiden-island", Slug: "cullaiden-island-map"},
			{Key: "cullaiden-island-temple", Slug: "cullaiden-island-temple"},
		},
	},
}
