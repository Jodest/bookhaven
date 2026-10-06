package repository

import (
	"strings"
	"testing"
)

// The calendar query is Postgres-only (EXTRACT ... ::int), so its
// published-only filter is asserted on the SQL text: both UNION branches
// must exclude non-published authors.
func TestCalendarEventsQuery_FiltersPublishedAuthorsInBothBranches(t *testing.T) {
	if n := strings.Count(calendarEventsQuery, "a.status = 'published'"); n != 2 {
		t.Fatalf("expected published filter in both UNION branches, found %d", n)
	}
}
