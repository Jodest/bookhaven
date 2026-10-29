package bookdata

import (
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// MaxOpenLibraryIDsPerWork caps how many Open Library IDs of one Wikidata
// work are tried for a cover.
const MaxOpenLibraryIDsPerWork = 3

var openLibraryIDPattern = regexp.MustCompile(`^OL(\d+)([WM])$`)

type orderedOLID struct {
	id      string
	isWork  bool
	numeric uint64
}

// OrderOpenLibraryIDs returns the usable Open Library IDs (OL<digits>W work or
// OL<digits>M edition) in a fixed order: duplicates removed, works before
// editions, each group in ascending numeric order, at most
// MaxOpenLibraryIDsPerWork values. Anything else is dropped.
func OrderOpenLibraryIDs(ids []string) []string {
	seen := make(map[string]struct{}, len(ids))
	parsed := make([]orderedOLID, 0, len(ids))
	for _, raw := range ids {
		id := strings.TrimSpace(raw)
		m := openLibraryIDPattern.FindStringSubmatch(id)
		if m == nil {
			continue
		}
		if _, dup := seen[id]; dup {
			continue
		}
		n, err := strconv.ParseUint(m[1], 10, 64)
		if err != nil {
			continue
		}
		seen[id] = struct{}{}
		parsed = append(parsed, orderedOLID{id: id, isWork: m[2] == "W", numeric: n})
	}
	sort.SliceStable(parsed, func(i, j int) bool {
		if parsed[i].isWork != parsed[j].isWork {
			return parsed[i].isWork
		}
		return parsed[i].numeric < parsed[j].numeric
	})
	if len(parsed) > MaxOpenLibraryIDsPerWork {
		parsed = parsed[:MaxOpenLibraryIDsPerWork]
	}
	out := make([]string, len(parsed))
	for i, p := range parsed {
		out[i] = p.id
	}
	return out
}
