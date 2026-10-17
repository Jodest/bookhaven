package service

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"

	adminservice "github.com/bookhaven/backend/internal/modules/admin/service"
	"github.com/bookhaven/backend/internal/modules/aigenres/domain"
	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	"github.com/google/uuid"
)

type candidate = adminservice.GenreAnalysisCandidate

// Plan is the split of the candidate genres into model requests: the anchors
// go into every request, the other candidates are shared out in Batches.
type Plan struct {
	Anchors []candidate
	Batches [][]candidate
}

// planBatches splits the candidates. Anchors are the up to domain.AnchorCount
// published genres with the most books (ties by name). The other candidates
// are ordered pending first (oldest first), then published by the fewest
// books, and cut into batches of about size: a group of genres linked by
// similar-name hints is moved whole into the batch where its first member
// lands, so a batch may exceed size by one group (a group is capped at
// domain.MaxClusterSize). At most maxBatches batches are returned.
func planBatches(cands []candidate, pairs []adminservice.GenreHintPair, size, maxBatches int) Plan {
	if size < 1 {
		size = 1
	}
	published := make([]candidate, 0, len(cands))
	for _, c := range cands {
		if c.Genre.Status == catalogDomain.GenreStatusPublished {
			published = append(published, c)
		}
	}
	sort.SliceStable(published, func(i, j int) bool {
		a, b := published[i], published[j]
		if a.BooksTotal != b.BooksTotal {
			return a.BooksTotal > b.BooksTotal
		}
		if a.Genre.Name != b.Genre.Name {
			return a.Genre.Name < b.Genre.Name
		}
		return a.Genre.ID.String() < b.Genre.ID.String()
	})
	if len(published) > domain.AnchorCount {
		published = published[:domain.AnchorCount]
	}
	anchorIDs := make(map[uuid.UUID]struct{}, len(published))
	for _, a := range published {
		anchorIDs[a.Genre.ID] = struct{}{}
	}

	rest := make([]candidate, 0, len(cands))
	for _, c := range cands {
		if _, isAnchor := anchorIDs[c.Genre.ID]; !isAnchor {
			rest = append(rest, c)
		}
	}
	sort.SliceStable(rest, func(i, j int) bool {
		a, b := rest[i], rest[j]
		ap, bp := a.Genre.Status == catalogDomain.GenreStatusPending, b.Genre.Status == catalogDomain.GenreStatusPending
		if ap != bp {
			return ap
		}
		if ap {
			if !a.Genre.CreatedAt.Equal(b.Genre.CreatedAt) {
				return a.Genre.CreatedAt.Before(b.Genre.CreatedAt)
			}
		} else if a.BooksTotal != b.BooksTotal {
			return a.BooksTotal < b.BooksTotal
		}
		if a.Genre.Name != b.Genre.Name {
			return a.Genre.Name < b.Genre.Name
		}
		return a.Genre.ID.String() < b.Genre.ID.String()
	})

	// Union-find over the hint pairs between non-anchor candidates.
	index := make(map[uuid.UUID]int, len(rest))
	for i, c := range rest {
		index[c.Genre.ID] = i
	}
	parent := make([]int, len(rest))
	for i := range parent {
		parent[i] = i
	}
	var find func(int) int
	find = func(x int) int {
		for parent[x] != x {
			parent[x] = parent[parent[x]]
			x = parent[x]
		}
		return x
	}
	for _, p := range pairs {
		a, okA := index[p.A]
		b, okB := index[p.B]
		if !okA || !okB {
			continue
		}
		ra, rb := find(a), find(b)
		if ra != rb {
			// The earlier member stays the root, so the group is anchored at
			// the position of its first member.
			if ra < rb {
				parent[rb] = ra
			} else {
				parent[ra] = rb
			}
		}
	}
	groups := make(map[int][]int, len(rest))
	for i := range rest {
		root := find(i)
		groups[root] = append(groups[root], i)
	}

	plan := Plan{Anchors: published}
	var current []candidate
	placed := make([]bool, len(rest))
	for i := range rest {
		if placed[i] {
			continue
		}
		var members []int
		for _, m := range groups[find(i)] {
			if !placed[m] {
				members = append(members, m)
			}
		}
		if len(members) > domain.MaxClusterSize {
			// Oversized group: the first MaxClusterSize members now, the
			// rest as later groups of their own.
			members = members[:domain.MaxClusterSize]
		}
		for _, m := range members {
			placed[m] = true
			current = append(current, rest[m])
		}
		if len(current) >= size {
			plan.Batches = append(plan.Batches, current)
			current = nil
			if len(plan.Batches) >= maxBatches {
				return plan
			}
		}
	}
	if len(current) > 0 && len(plan.Batches) < maxBatches {
		plan.Batches = append(plan.Batches, current)
	}
	if len(plan.Batches) == 0 && len(plan.Anchors) > 1 {
		// A small catalog of published genres only: one request with just the
		// anchors, so duplicates among them are still found.
		plan.Batches = [][]candidate{{}}
	}
	return plan
}

// Fingerprint is a stable hash of the candidate genres (id, name, status,
// QID) and the prompt version. A scheduled run is skipped when it equals the
// fingerprint of the last clean run.
func Fingerprint(cands []candidate) string {
	lines := make([]string, 0, len(cands))
	for _, c := range cands {
		qid := ""
		if c.Genre.WikidataQID != nil {
			qid = *c.Genre.WikidataQID
		}
		lines = append(lines, fmt.Sprintf("%s\x1f%s\x1f%s\x1f%s", c.Genre.ID, c.Genre.Name, c.Genre.Status, qid))
	}
	sort.Strings(lines)
	h := sha256.New()
	fmt.Fprintf(h, "v%d\n", PromptVersion)
	for _, l := range lines {
		h.Write([]byte(l))
		h.Write([]byte{'\n'})
	}
	return hex.EncodeToString(h.Sum(nil))
}
