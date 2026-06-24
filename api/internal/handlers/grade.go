package handlers

import (
	"context"
	"net/http"

	"github.com/sunkencosts/mirrorleague/internal/grading"
)

type gradeStore interface {
	grading.LineupLister
	grading.ResultWriter
}

// HandleGradeSeason runs the grading step: it grades every past-week lineup lacking a
// week_results row and backfills anything still ungraded (D30). Idempotent. Behind the
// admin secret; safe to run on a schedule (the live week is inferred from week_locks, so
// there's no manual week to advance) and useful to run by hand.
func HandleGradeSeason(store gradeStore, prov grading.MatchupProvider, currentWeek func(context.Context) int) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		graded, err := grading.GradeSeason(r.Context(), store, store, prov, currentWeek(r.Context()))
		if err != nil {
			http.Error(w, "grading failed", http.StatusInternalServerError)
			return
		}
		_ = encode(w, r, http.StatusOK, map[string]int{"graded": graded})
	})
}
