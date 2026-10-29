package bookdata

import (
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// MaxFantLabWorkIDsPerWork caps how many FantLab work IDs of one Wikidata
// work are tried for a cover.
const MaxFantLabWorkIDsPerWork = 2

var fantLabWorkIDPattern = regexp.MustCompile(`(?i)^(?:work)?(\d+)$`)

// OrderFantLabWorkIDs returns the usable FantLab work IDs (Wikidata P7439:
// plain digits, an optional "work" prefix is tolerated) as numbers: duplicates
// removed, ascending, at most MaxFantLabWorkIDsPerWork values. Anything else,
// and zero, is dropped.
func OrderFantLabWorkIDs(ids []string) []int {
	seen := make(map[int]struct{}, len(ids))
	out := make([]int, 0, len(ids))
	for _, raw := range ids {
		m := fantLabWorkIDPattern.FindStringSubmatch(strings.TrimSpace(raw))
		if m == nil {
			continue
		}
		n, err := strconv.Atoi(m[1])
		if err != nil || n <= 0 {
			continue
		}
		if _, dup := seen[n]; dup {
			continue
		}
		seen[n] = struct{}{}
		out = append(out, n)
	}
	sort.Ints(out)
	if len(out) > MaxFantLabWorkIDsPerWork {
		out = out[:MaxFantLabWorkIDsPerWork]
	}
	return out
}
