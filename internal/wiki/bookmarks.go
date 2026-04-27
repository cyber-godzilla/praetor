// Package wiki contains curated bookmarks for the Eternal City wiki at
// http://eternal-city.wikidot.com. The bookmark list is hand-curated;
// regenerate from docs/wiki-bookmarks-draft.md if it changes.
package wiki

import (
	"sort"
	"strings"
)

// BaseURL is the wiki root.
const BaseURL = "http://eternal-city.wikidot.com"

// Bookmark is a (key, slug) pair. URL is BaseURL + "/" + Slug.
type Bookmark struct {
	Key  string
	Slug string
}

// Section is a named group of bookmarks. Order is preserved.
type Section struct {
	Name      string
	Bookmarks []Bookmark
}

// Sections returns the ordered list of bookmark sections.
func Sections() []Section {
	return sections
}

// URL returns the full wiki URL for a bookmark slug.
func URL(slug string) string {
	return BaseURL + "/" + slug
}

// Lookup finds a bookmark by key. Match is case-insensitive and treats
// underscores/spaces as hyphens. Returns the matching slug and true on
// hit; ("", false) on miss.
func Lookup(key string) (string, bool) {
	norm := normalize(key)
	for _, sec := range sections {
		for _, bm := range sec.Bookmarks {
			if normalize(bm.Key) == norm {
				return bm.Slug, true
			}
		}
	}
	return "", false
}

// Keys returns all bookmark keys, sorted alphabetically.
func Keys() []string {
	var out []string
	for _, sec := range sections {
		for _, bm := range sec.Bookmarks {
			out = append(out, bm.Key)
		}
	}
	sort.Strings(out)
	return out
}

func normalize(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, "_", "-")
	s = strings.ReplaceAll(s, " ", "-")
	return s
}

var sections = []Section{
	{
		Name: "Getting Started",
		Bookmarks: []Bookmark{
			{Key: "account", Slug: "account"},
			{Key: "faq", Slug: "faq"},
			{Key: "customization-guide", Slug: "customization-guide"},
			{Key: "general-rules", Slug: "general-rules"},
			{Key: "getting-started", Slug: "getting-started"},
			{Key: "players-guide", Slug: "players-guide"},
			{Key: "returning-players", Slug: "returning-players"},
		},
	},
	{
		Name: "Newbie Guides",
		Bookmarks: []Bookmark{
			{Key: "newbie-combat", Slug: "newbie-combat-guide"},
			{Key: "newbie-language", Slug: "newbie-language-guide"},
			{Key: "newbie-mission", Slug: "newbie-mission-guide"},
			{Key: "newbie-money", Slug: "newbie-money-guide"},
			{Key: "newbie-non-combat", Slug: "newbie-non-combat-guides"},
			{Key: "newbie-office", Slug: "newbie-office"},
		},
	},
	{
		Name: "Character Generation",
		Bookmarks: []Bookmark{
			{Key: "stats", Slug: "stats"},
			{Key: "traits", Slug: "traits"},
			{Key: "national-advantages", Slug: "national-advantages"},
			{Key: "national-lores", Slug: "national-lores"},
			{Key: "veteran-characters", Slug: "veteran-characters"},
			{Key: "character-generator", Slug: "character-generator"},
		},
	},
	{
		Name: "Reference",
		Bookmarks: []Bookmark{
			{Key: "commands", Slug: "commands"},
			{Key: "advanced-commands", Slug: "advanced-commands"},
			{Key: "advanced-speech", Slug: "advanced-speech"},
			{Key: "macros", Slug: "macros"},
			{Key: "macros-and-targeting", Slug: "macros-and-targeting"},
			{Key: "skills", Slug: "skills"},
			{Key: "character-condition", Slug: "character-condition"},
			{Key: "dates-and-time", Slug: "dates-and-time"},
			{Key: "containers", Slug: "containers"},
			{Key: "reputation", Slug: "reputation"},
			{Key: "wealth", Slug: "wealth"},
			{Key: "property", Slug: "property"},
			{Key: "rp-expenditure", Slug: "rp-expenditure"},
			{Key: "languages", Slug: "languages"},
		},
	},
	{
		Name: "Combat",
		Bookmarks: []Bookmark{
			{Key: "combat-overview", Slug: "combat-overview"},
			{Key: "critical-hits", Slug: "critical-hits"},
			{Key: "blocks-and-dodges", Slug: "blocks-and-dodges"},
			{Key: "pvp", Slug: "pvp"},
			{Key: "hunting-grounds", Slug: "hunting-grounds"},
			{Key: "enemy-guide", Slug: "enemy-guide"},
			{Key: "aoden-hunting", Slug: "aoden-hunting-guide"},
			{Key: "weapons", Slug: "weapons"},
		},
	},
	{
		Name: "Combat Skills",
		Bookmarks: []Bookmark{
			{Key: "hoplite", Slug: "hoplite-combat"},
			{Key: "combat-maneuvers", Slug: "combat-maneuvers"},
			{Key: "armor", Slug: "armor"},
			{Key: "shields", Slug: "shields"},
			{Key: "one-handed-swords", Slug: "one-handed-swords"},
			{Key: "two-handed-axes", Slug: "two-handed-axes"},
			{Key: "one-handed-axes", Slug: "one-handed-axes"},
			{Key: "one-handed-crushing", Slug: "one-handed-crushing"},
			{Key: "two-handed-crushing", Slug: "two-handed-crushing"},
			{Key: "knives", Slug: "knives"},
			{Key: "spears", Slug: "spears"},
			{Key: "tridents", Slug: "tridents"},
			{Key: "staves", Slug: "staves"},
			{Key: "whips", Slug: "whips"},
			{Key: "sling", Slug: "sling"},
			{Key: "bows", Slug: "missile-weapons-bows"},
			{Key: "chainblade", Slug: "chainblade"},
			{Key: "cestus", Slug: "cestus"},
			{Key: "falcata", Slug: "falcata"},
			{Key: "falx", Slug: "falx"},
			{Key: "parcines", Slug: "parcines"},
			{Key: "brawling", Slug: "brawling"},
			{Key: "pankration", Slug: "pankration"},
			{Key: "avros", Slug: "avros-one-handed-swords"},
			{Key: "nelsor", Slug: "nelsor-one-handed-swords"},
			{Key: "pardelian", Slug: "pardelian-one-handed-swords"},
			{Key: "cineran-knife-fighting", Slug: "cineran-knife-fighting-knives"},
		},
	},
	{
		Name: "Non-Combat Skills",
		Bookmarks: []Bookmark{
			{Key: "healing", Slug: "healing"},
			{Key: "herbalism", Slug: "herbalism"},
			{Key: "tailoring", Slug: "tailoring"},
			{Key: "locksmithing", Slug: "locksmithing"},
			{Key: "pickpocketing", Slug: "pickpocketing"},
			{Key: "street-smarts", Slug: "street-smarts"},
			{Key: "outdoor-survival", Slug: "outdoor-survival"},
			{Key: "setups", Slug: "setups"},
			{Key: "hunting", Slug: "hunting"},
		},
	},
	{
		Name: "Calculators",
		Bookmarks: []Bookmark{
			{Key: "money-calculator", Slug: "money-calculator"},
			{Key: "rank-bonus-calculator", Slug: "rank-bonus-calculator"},
			{Key: "training-cost-calculator", Slug: "training-cost-calculator"},
		},
	},
	{
		Name: "Maps — World & Region",
		Bookmarks: []Bookmark{
			{Key: "maps", Slug: "maps"},
			{Key: "world-map", Slug: "unofficial-world-map"},
			{Key: "game-world", Slug: "game-world"},
			{Key: "midlight-map", Slug: "midlight-map"},
			{Key: "miscellaneous-maps", Slug: "miscellaneous-maps"},
			{Key: "historic-map-marnevel", Slug: "historic-map-marnevel"},
			{Key: "historic-map-pepaquest", Slug: "historic-map-pepaquest"},
			{Key: "illustrated-iridine", Slug: "illustrated-iridine"},
			{Key: "illustrated-iridine-and-neighbors", Slug: "illustrated-iridine-and-neighbors"},
		},
	},
	{
		Name: "Maps — Iridine",
		Bookmarks: []Bookmark{
			{Key: "iridine", Slug: "city-of-iridine"},
			{Key: "republic-of-iridine", Slug: "republic-of-iridine"},
			{Key: "the-steps", Slug: "the-steps"},
			{Key: "the-steps-central", Slug: "the-steps-central"},
			{Key: "the-steps-east", Slug: "the-steps-east"},
			{Key: "the-steps-north", Slug: "the-steps-north"},
			{Key: "the-steps-south", Slug: "the-steps-south"},
			{Key: "the-steps-sewers", Slug: "the-steps-sewers"},
			{Key: "steps-ludus-quintus", Slug: "steps-ludus-quintus"},
			{Key: "bronze-lane", Slug: "bronze-lane"},
			{Key: "campus-martius", Slug: "campus-martius"},
			{Key: "gardens-and-hospice", Slug: "gardens-and-hospice"},
			{Key: "harbor", Slug: "harbor"},
			{Key: "house-of-mercantile", Slug: "house-of-mercantile"},
			{Key: "old-city-and-moondeep", Slug: "old-city-and-moondeep"},
			{Key: "quartz-heights-area", Slug: "quartz-heights"},
			{Key: "riverside", Slug: "riverside"},
			{Key: "sandbar", Slug: "sandbar"},
			{Key: "storm-drain-system", Slug: "storm-drain-system"},
			{Key: "sewers-and-sea-caves", Slug: "sewers-and-sea-caves"},
			{Key: "library", Slug: "library"},
			{Key: "colosseum", Slug: "colosseum"},
			{Key: "the-colosseum", Slug: "the-colosseum"},
			{Key: "the-library-of-iridine", Slug: "the-library-of-iridine"},
		},
	},
	{
		Name: "Maps — Monlon",
		Bookmarks: []Bookmark{
			{Key: "monlon", Slug: "city-of-monlon"},
			{Key: "monlon-area", Slug: "monlon"},
			{Key: "monlon-master", Slug: "monlon-master"},
			{Key: "monlon-battlefield", Slug: "monlon-battlefield"},
			{Key: "monlon-catacombs", Slug: "monlon-catacombs"},
			{Key: "monlon-invasion", Slug: "monlon-invasion"},
			{Key: "monlon-kelestian-outpost", Slug: "monlon-kelestian-outpost"},
			{Key: "monlon-mines", Slug: "monlon-mines"},
			{Key: "monlon-ravines", Slug: "monlon-ravines"},
			{Key: "monlon-rockslide", Slug: "monlon-rockslide"},
		},
	},
	{
		Name: "Maps — Rock Valley",
		Bookmarks: []Bookmark{
			{Key: "rock-valley", Slug: "town-of-rock-valley"},
			{Key: "rock-valley-map", Slug: "town-of-rock-valley-map"},
			{Key: "rock-valley-area", Slug: "rock-valley"},
			{Key: "rock-valley-region", Slug: "rock-valley-region"},
			{Key: "rock-valley-dumps", Slug: "rock-valley-dumps"},
			{Key: "rock-valley-mine", Slug: "rock-valley-mine"},
			{Key: "rock-valley-well", Slug: "rock-valley-well"},
		},
	},
	{
		Name: "Maps — Vetallun",
		Bookmarks: []Bookmark{
			{Key: "vetallun", Slug: "town-of-vetallun"},
			{Key: "vetallun-area", Slug: "vetallun"},
			{Key: "vetallun-road", Slug: "vetallun-road"},
		},
	},
	{
		Name: "Maps — Other Towns & Villages",
		Bookmarks: []Bookmark{
			{Key: "franlius", Slug: "town-of-franlius"},
			{Key: "seld", Slug: "village-of-seld"},
			{Key: "blackvine-village", Slug: "village-of-blackvine"},
			{Key: "stromheim-village", Slug: "village-of-stromheim"},
		},
	},
	{
		Name: "Maps — Outlying Areas",
		Bookmarks: []Bookmark{
			{Key: "blackvine-area", Slug: "blackvine"},
			{Key: "brigand-treehouse", Slug: "brigand-treehouse"},
			{Key: "black-hand-caverns", Slug: "black-hand-caverns"},
			{Key: "black-hand-mines", Slug: "black-hand-mines"},
			{Key: "burnt-villa", Slug: "burnt-villa"},
			{Key: "cullaiden-island", Slug: "cullaiden-island"},
			{Key: "cullaiden-island-map", Slug: "cullaiden-island-map"},
			{Key: "cullaiden-island-temple", Slug: "cullaiden-island-temple"},
			{Key: "eastern-grasslands-and-woods", Slug: "eastern-grasslands-and-woods"},
			{Key: "esecarnus-caves", Slug: "esecarnus-caves"},
			{Key: "fenri-gifr-ruins", Slug: "fenri-gifr-ruins"},
			{Key: "filinius-villa", Slug: "filinius-villa"},
			{Key: "franlius-area", Slug: "franlius"},
			{Key: "grey-sands", Slug: "grey-sands"},
			{Key: "harbor-of-the-moons", Slug: "harbor-of-the-moons"},
			{Key: "hg-filinius-villa", Slug: "hg-filinius-villa"},
			{Key: "hg-undertown", Slug: "hg-undertown"},
			{Key: "kelestian-outpost", Slug: "kelestian-outpost"},
			{Key: "lighthouse", Slug: "lighthouse"},
			{Key: "rat-pits-and-aralex-pits", Slug: "rat-pits-and-aralex-pits"},
			{Key: "salt-flats", Slug: "salt-flats"},
			{Key: "shipwreck", Slug: "shipwreck"},
			{Key: "signal-tower-island", Slug: "signal-tower-island"},
			{Key: "spider-caverns", Slug: "spider-caverns"},
			{Key: "stone-toga-inn", Slug: "stone-toga-inn"},
			{Key: "stromheim-area", Slug: "stromheim"},
			{Key: "swamp-mansion", Slug: "swamp-mansion"},
			{Key: "swamp-vale", Slug: "swamp-vale"},
			{Key: "the-salinae-swamp", Slug: "the-salinae-swamp"},
			{Key: "the-west-grasslands", Slug: "the-west-grasslands"},
			{Key: "transinvexium-east", Slug: "transinvexium-east"},
			{Key: "worm-temple", Slug: "worm-temple"},
		},
	},
	{
		Name: "Places (info)",
		Bookmarks: []Bookmark{
			{Key: "altene", Slug: "altene"},
			{Key: "argosius", Slug: "argosius"},
			{Key: "astraea", Slug: "astraea"},
			{Key: "basran-fount", Slug: "basran-fount"},
			{Key: "blue-sands", Slug: "blue-sands"},
			{Key: "cenath", Slug: "cenath"},
			{Key: "cinera", Slug: "cinera"},
			{Key: "east-ravanite-tunnels", Slug: "east-ravanite-tunnels"},
			{Key: "fehcratos", Slug: "fehcratos"},
			{Key: "gadaene", Slug: "gadaene"},
			{Key: "gardens-of-sunset", Slug: "gardens-of-sunset"},
			{Key: "hutt-s-road", Slug: "hutt-s-road"},
			{Key: "kelestia", Slug: "kelestia"},
			{Key: "lantos-wall", Slug: "lantos-wall"},
			{Key: "old-wall", Slug: "old-wall"},
			{Key: "panzacor", Slug: "panzacor"},
			{Key: "remath", Slug: "remath"},
			{Key: "safelands", Slug: "safelands"},
			{Key: "seld-area", Slug: "seld"},
			{Key: "sostaera", Slug: "sostaera"},
			{Key: "tepsin", Slug: "tepsin"},
			{Key: "three-hills", Slug: "three-hills"},
			{Key: "tuchea", Slug: "tuchea"},
			{Key: "ut-jor", Slug: "ut-jor"},
			{Key: "windward", Slug: "windward"},
		},
	},
	{
		Name: "Non-Place Pages",
		Bookmarks: []Bookmark{
			{Key: "characters", Slug: "characters"},
			{Key: "contraband", Slug: "contraband"},
			{Key: "mining", Slug: "mining"},
		},
	},
	{
		Name: "Lore & Society",
		Bookmarks: []Bookmark{
			{Key: "history", Slug: "history"},
			{Key: "religion", Slug: "religion"},
			{Key: "law", Slug: "law"},
			{Key: "orgs", Slug: "orgs"},
			{Key: "shops", Slug: "shops"},
			{Key: "trainers", Slug: "trainers"},
			{Key: "fiction", Slug: "fiction"},
			{Key: "flora-fauna", Slug: "flora-fauna"},
			{Key: "metals", Slug: "metals"},
			{Key: "stones-ores", Slug: "stones-ores"},
			{Key: "magic", Slug: "magic"},
			{Key: "orchil", Slug: "orchil"},
			{Key: "tecelite", Slug: "tecelite"},
			{Key: "transinvexium", Slug: "transinvexium"},
			{Key: "aestivan-league", Slug: "aestivan-league"},
			{Key: "player-stories", Slug: "player-stories"},
			{Key: "adrian-lantos", Slug: "adrian-lantos"},
			{Key: "altene-language", Slug: "altene-language"},
			{Key: "arandes-pardelian", Slug: "arandes-pardelian"},
			{Key: "cult-of-ereal", Slug: "cult-of-ereal"},
			{Key: "darpen", Slug: "darpen"},
			{Key: "harmony", Slug: "harmony"},
			{Key: "history-of-creation", Slug: "history-of-creation"},
			{Key: "methodios-callias", Slug: "methodios-callias"},
			{Key: "old-cult-of-ereal", Slug: "old-cult-of-ereal"},
			{Key: "peace-with-the-nehal", Slug: "peace-with-the-nehal"},
			{Key: "phoenix-guard", Slug: "phoenix-guard"},
			{Key: "shrikes", Slug: "shrikes"},
			{Key: "sinistrals", Slug: "sinistrals"},
			{Key: "soldiers-of-ereal", Slug: "soldiers-of-ereal"},
			{Key: "superlatives", Slug: "superlatives"},
			{Key: "the-way-of-the-thief", Slug: "the-way-of-the-thief"},
			{Key: "tralius-allende", Slug: "tralius-allende"},
			{Key: "way-of-bright-hope", Slug: "way-of-bright-hope"},
		},
	},
	{
		Name: "Hunting Grounds",
		Bookmarks: []Bookmark{
			{Key: "hg-alleys", Slug: "hg-alleys"},
			{Key: "hg-aralex-pit", Slug: "hg-aralex-pit"},
			{Key: "hg-bandit-complex", Slug: "hg-bandit-complex"},
			{Key: "hg-bandit-forest", Slug: "hg-bandit-forest"},
			{Key: "hg-black-hand-caverns", Slug: "hg-black-hand-caverns"},
			{Key: "hg-brigand-treehouse", Slug: "hg-brigand-treehouse"},
			{Key: "hg-burnt-villa", Slug: "hg-burnt-villa"},
			{Key: "hg-coastal-alleys", Slug: "hg-coastal-alleys"},
			{Key: "hg-colosseum", Slug: "hg-colosseum"},
			{Key: "hg-franlius", Slug: "hg-franlius"},
			{Key: "hg-iridine-dumps", Slug: "hg-iridine-dumps"},
			{Key: "hg-iridine-pits", Slug: "hg-iridine-pits"},
			{Key: "hg-iridine-sewers", Slug: "hg-iridine-sewers"},
			{Key: "hg-ludus-valerius", Slug: "hg-ludus-valerius"},
			{Key: "hg-monlon-mines", Slug: "hg-monlon-mines"},
			{Key: "hg-old-city", Slug: "hg-old-city"},
			{Key: "hg-quartz-heights-boardwalk", Slug: "hg-quartz-heights-boardwalk"},
			{Key: "hg-rock-valley-alley", Slug: "hg-rock-valley-alley"},
			{Key: "hg-rock-valley-broken-tower", Slug: "hg-rock-valley-broken-tower"},
			{Key: "hg-rock-valley-burial-grounds", Slug: "hg-rock-valley-burial-grounds"},
			{Key: "hg-rock-valley-dumps", Slug: "hg-rock-valley-dumps"},
			{Key: "hg-rock-valley-fenri-gifr-ruins", Slug: "hg-rock-valley-fenri-gifr-ruins"},
			{Key: "hg-rock-valley-resting-place", Slug: "hg-rock-valley-resting-place"},
			{Key: "hg-rock-valley-well", Slug: "hg-rock-valley-well"},
			{Key: "hg-sea-caves", Slug: "hg-sea-caves"},
			{Key: "hg-shipwreck", Slug: "hg-shipwreck"},
			{Key: "hg-signal-tower-island", Slug: "hg-signal-tower-island"},
			{Key: "hg-spider-caverns", Slug: "hg-spider-caverns"},
			{Key: "hg-swamp-mansion", Slug: "hg-swamp-mansion"},
			{Key: "hg-swamp-worm-temple", Slug: "hg-swamp-worm-temple"},
			{Key: "hg-vetallun-apple-orchard", Slug: "hg-vetallun-apple-orchard"},
			{Key: "hg-aziri-caves", Slug: "hg:aziri-caves"},
			{Key: "hg-bandit-fort", Slug: "hg:bandit-fort"},
			{Key: "hg-monlon-battlefields", Slug: "hg:monlon-battlefields"},
			{Key: "hg-monlon-ravines", Slug: "hg:monlon-ravines"},
		},
	},
	{
		Name: "Combat Skill Guides",
		Bookmarks: []Bookmark{
			{Key: "archery-guide", Slug: "archery-guide"},
			{Key: "avros-guide", Slug: "avros-guide"},
			{Key: "brawling-guide", Slug: "brawling-guide"},
			{Key: "cestus-guide", Slug: "cestus-guide"},
			{Key: "ckf-guide", Slug: "ckf-guide"},
			{Key: "combat-guide", Slug: "combat-guide"},
			{Key: "falcata-guide", Slug: "falcata-guide"},
			{Key: "falx-guide", Slug: "falx-guide"},
			{Key: "healing-guide", Slug: "healing-guide"},
			{Key: "herbalism-guide", Slug: "herbalism-guide"},
			{Key: "hoplite-combat-guide", Slug: "hoplite-combat-guide"},
			{Key: "hunting-guide", Slug: "hunting-guide"},
			{Key: "knives-guide", Slug: "knives-guide"},
			{Key: "locksmithing-guide", Slug: "locksmithing-guide"},
			{Key: "nelsor-guide", Slug: "nelsor-guide"},
			{Key: "one-handed-axes-guide", Slug: "one-handed-axes-guide"},
			{Key: "one-handed-crushing-guide", Slug: "one-handed-crushing-guide"},
			{Key: "one-handed-swords-guide", Slug: "one-handed-swords-guide"},
			{Key: "outdoor-survival-guide", Slug: "outdoor-survival-guide"},
			{Key: "pankration-guide", Slug: "pankration-guide"},
			{Key: "pardelian-guide", Slug: "pardelian-guide"},
			{Key: "shields-guide", Slug: "shields-guide"},
			{Key: "signal-tower-island-guide", Slug: "signal-tower-island-guide"},
			{Key: "sling-guide", Slug: "sling-guide"},
			{Key: "spears-guide", Slug: "spears-guide"},
			{Key: "staves-guide", Slug: "staves-guide"},
			{Key: "tailoring-guide", Slug: "tailoring-guide"},
			{Key: "tridents-guide", Slug: "tridents-guide"},
			{Key: "two-handed-axes-guide", Slug: "two-handed-axes-guide"},
			{Key: "whips-guide", Slug: "whips-guide"},
		},
	},
	{
		Name: "Government & Civic",
		Bookmarks: []Bookmark{
			{Key: "assemblies-and-legislation", Slug: "assemblies-and-legislation"},
			{Key: "building-and-civic-maintenance", Slug: "building-and-civic-maintenance"},
			{Key: "census-and-citizenship", Slug: "census-and-citizenship"},
			{Key: "comitia-centuriata", Slug: "comitia-centuriata"},
			{Key: "commerce", Slug: "commerce"},
			{Key: "constables", Slug: "constables"},
			{Key: "criminal-acts", Slug: "criminal-acts"},
			{Key: "debt", Slug: "debt"},
			{Key: "divortium-auxilii", Slug: "divortium-auxilii"},
			{Key: "influence", Slug: "influence"},
			{Key: "justice-and-courts", Slug: "justice-and-courts"},
			{Key: "legio", Slug: "legio"},
			{Key: "legio-misc", Slug: "legio-misc"},
			{Key: "lex-legalis", Slug: "lex-legalis"},
			{Key: "magistracies", Slug: "magistracies"},
			{Key: "marriage-inheritance-and-funerals", Slug: "marriage-inheritance-and-funerals"},
			{Key: "military-service", Slug: "military-service"},
			{Key: "patricians", Slug: "patricians"},
			{Key: "political-factions", Slug: "political-factions"},
			{Key: "punishment", Slug: "punishment"},
			{Key: "senate", Slug: "senate"},
			{Key: "taxes-and-civic-finance", Slug: "taxes-and-civic-finance"},
			{Key: "vestis-formatae", Slug: "vestis-formatae"},
			{Key: "warrants", Slug: "warrants"},
		},
	},
	{
		Name: "Reference (additional)",
		Bookmarks: []Bookmark{
			{Key: "combat", Slug: "combat"},
			{Key: "creatures", Slug: "creatures"},
			{Key: "creatures-of-midlight", Slug: "creatures-of-midlight"},
			{Key: "furniture", Slug: "furniture"},
			{Key: "guides", Slug: "guides"},
			{Key: "humanoids", Slug: "humanoids"},
			{Key: "modules-reference", Slug: "modules-reference"},
			{Key: "missions", Slug: "missions"},
			{Key: "places", Slug: "places"},
			{Key: "roleplaying", Slug: "roleplaying"},
			{Key: "services", Slug: "services"},
			{Key: "shrines", Slug: "shrines"},
			{Key: "specialty-items", Slug: "specialty-items"},
			{Key: "weapons-overview", Slug: "weapons-overview"},
			{Key: "hair-styles-barbershop", Slug: "hair-styles-barbershop"},
			{Key: "little-black-book-of-thievery", Slug: "little-black-book-of-thievery"},
			{Key: "useful-macros-for-thieves", Slug: "useful-macros-for-thieves"},
			{Key: "org-perks", Slug: "org-perks"},
		},
	},
}
