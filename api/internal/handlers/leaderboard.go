package handlers

import (
	"context"
	"net/http"

	"github.com/sunkencosts/mirrorleague/internal/provider"
)

// globalMinWeeks is the number of graded weeks a user needs to appear in the ranked global
// standings (D7). Below it they are "provisional" — shown, flagged, but never above a
// ranked user. The per-league board uses 0 (ranked from week 1, for league-mate immediacy).
const globalMinWeeks = 3

type leaderboardStore interface {
	Leaderboard(ctx context.Context, season, leagueID string) ([]provider.LeaderboardRow, error)
}

// HandleGlobalLeaderboard ranks every authenticated user across all their mirrors for a
// season by mean lineup efficiency, with a 3-week provisional gate.
func HandleGlobalLeaderboard(store leaderboardStore, defaultSeason string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		season := seasonParam(r, defaultSeason)
		rows, err := store.Leaderboard(r.Context(), season, "")
		if err != nil {
			http.Error(w, "failed to load leaderboard", http.StatusInternalServerError)
			return
		}
		_ = encode(w, r, http.StatusOK, applyRanking(rows, globalMinWeeks))
	})
}

// HandleLeagueLeaderboard ranks users within one league, with no minimum (instant — F4/F7).
func HandleLeagueLeaderboard(store leaderboardStore, defaultSeason string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		leagueID := r.PathValue("leagueId")
		season := seasonParam(r, defaultSeason)
		rows, err := store.Leaderboard(r.Context(), season, leagueID)
		if err != nil {
			http.Error(w, "failed to load leaderboard", http.StatusInternalServerError)
			return
		}
		_ = encode(w, r, http.StatusOK, applyRanking(rows, 0))
	})
}

func seasonParam(r *http.Request, defaultSeason string) string {
	if s := r.URL.Query().Get("season"); s != "" {
		return s
	}
	return defaultSeason
}

// applyRanking partitions efficiency-sorted rows into ranked (>= minWeeks) and provisional
// (< minWeeks). Ranked rows come first with 1-based Rank; provisional rows follow with
// Rank 0 and Provisional=true, so a high-efficiency small-sample user never outranks a
// proven one (D7/E5). Input must already be sorted by mean efficiency (the DB does this).
func applyRanking(rows []provider.LeaderboardRow, minWeeks int) []provider.LeaderboardRow {
	ranked := make([]provider.LeaderboardRow, 0, len(rows))
	provisional := make([]provider.LeaderboardRow, 0)
	for _, row := range rows {
		if row.WeeksPlayed >= minWeeks {
			ranked = append(ranked, row)
		} else {
			row.Provisional = true
			provisional = append(provisional, row)
		}
	}
	for i := range ranked {
		ranked[i].Rank = i + 1
	}
	return append(ranked, provisional...)
}
