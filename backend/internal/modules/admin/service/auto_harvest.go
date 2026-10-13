package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// AutoHarvestMinInterval is the minimum time between two scheduled
// harvests of the same author (fixed, not configurable; see openspec change
// add-scheduled-harvest design.md Decision 3).
const AutoHarvestMinInterval = 30 * 24 * time.Hour

// AutoHarvestCandidate is one author picked for a scheduled harvest run.
type AutoHarvestCandidate struct {
	ID  uuid.UUID
	QID string
}

// ListAutoHarvestCandidates returns up to limit authors eligible for the
// scheduled harvest at now: auto_harvest set, published, with a Wikidata
// QID, and not auto-harvested within AutoHarvestMinInterval. Never
// auto-harvested authors come first, then the least recently harvested
// (ties by name, then id). limit <= 0 returns nothing.
func (s *Service) ListAutoHarvestCandidates(ctx context.Context, limit int, now time.Time) ([]AutoHarvestCandidate, error) {
	if limit <= 0 {
		return []AutoHarvestCandidate{}, nil
	}
	authors, err := s.repo.ListAutoHarvestCandidates(ctx, limit, now.Add(-AutoHarvestMinInterval))
	if err != nil {
		return nil, fmt.Errorf("list auto-harvest candidates: %w", err)
	}
	out := make([]AutoHarvestCandidate, 0, len(authors))
	for _, a := range authors {
		if !hasWikidataQID(a) {
			continue
		}
		out = append(out, AutoHarvestCandidate{ID: a.ID, QID: strings.TrimSpace(*a.ExternalID)})
	}
	return out, nil
}

// MarkAuthorsAutoHarvested records at as the last automatic harvest time of
// the given authors. Called by the schedule tick only after the scheduled
// harvest job was queued.
func (s *Service) MarkAuthorsAutoHarvested(ctx context.Context, ids []uuid.UUID, at time.Time) error {
	if len(ids) == 0 {
		return nil
	}
	if err := s.repo.MarkAuthorsAutoHarvested(ctx, ids, at); err != nil {
		return fmt.Errorf("mark authors auto-harvested: %w", err)
	}
	return nil
}
