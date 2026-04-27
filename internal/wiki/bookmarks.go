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
		Name: "Character",
		Bookmarks: []Bookmark{
			{Key: "traits", Slug: "traits"},
			{Key: "stats", Slug: "stats"},
			{Key: "skills", Slug: "skills"},
			{Key: "veteran-characters", Slug: "veteran-characters"},
			{Key: "reputation", Slug: "reputation"},
			{Key: "national-lores", Slug: "national-lores"},
			{Key: "national-advantages", Slug: "national-advantages"},
		},
	},
	{
		Name: "Combat",
		Bookmarks: []Bookmark{
			{Key: "combat-overview", Slug: "combat-overview"},
			{Key: "hunting-grounds", Slug: "hunting-grounds"},
			{Key: "armor", Slug: "armor"},
		},
	},
	{
		Name: "Skill Guides",
		Bookmarks: []Bookmark{
			{Key: "healing-guide", Slug: "healing-guide"},
			{Key: "locksmithing-guide", Slug: "locksmithing-guide"},
			{Key: "herbalism-guide", Slug: "herbalism-guide"},
			{Key: "tailoring-guide", Slug: "tailoring-guide"},
		},
	},
	{
		Name: "Maps",
		Bookmarks: []Bookmark{
			{Key: "maps", Slug: "maps"},
		},
	},
	{
		Name: "Calculators",
		Bookmarks: []Bookmark{
			{Key: "rank-bonus-calculator", Slug: "rank-bonus-calculator"},
			{Key: "training-cost-calculator", Slug: "training-cost-calculator"},
		},
	},
}
