package main

import (
	"testing"
	"time"
)

// PR 6 — the live NFL week is inferred from week_locks + the current date when no
// CURRENT_WEEK override is set (the override is what every other test uses).

func TestCurrentWeek_InferredFromWeekLocks(t *testing.T) {
	w := buildWorld()
	// No CURRENT_WEEK env → infer. Use season 2099 (no migration-seeded locks) so the
	// inference is fully controlled by the rows we seed below.
	baseURL := newTestServer(t, fakeSleeper(w), map[string]string{"CURRENT_SEASON": "2099"})
	seedWorld(t, w)

	// Weeks 1 & 2 have kicked off (past); week 3 has not → inferred current week = 2,
	// so only week-1 lineups (week < 2) are final and gradable.
	seedWeekLock(t, "2099", 1, time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC))
	seedWeekLock(t, "2099", 2, time.Date(2020, 1, 8, 0, 0, 0, 0, time.UTC))
	seedWeekLock(t, "2099", 3, time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC))

	graded := runGrading(t, baseURL)
	want := gradableCount(w, 2) // week-1 lineups only
	if graded != want {
		t.Fatalf("inferred current week (2) should grade only week-1 lineups: want %d, got %d", want, graded)
	}
}
