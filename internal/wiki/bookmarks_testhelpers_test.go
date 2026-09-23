package wiki

import "sort"

func Keys() []string {
	var keys []string
	for _, section := range sections {
		for _, bookmark := range section.Bookmarks {
			keys = append(keys, bookmark.Key)
		}
	}
	sort.Strings(keys)
	return keys
}
