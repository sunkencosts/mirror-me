package handlers

import (
	"context"
	"net/http"
	"sync"

	"github.com/sunkencosts/mirrorleague/internal/grading"
	"github.com/sunkencosts/mirrorleague/internal/provider"
	"github.com/sunkencosts/mirrorleague/internal/scoring"
)

type compareProvider interface {
	GetWeekMatchups(ctx context.Context, leagueID string, week int) ([]provider.WeekMatchup, error)
	GetLeague(ctx context.Context, leagueID string) (provider.League, error)
}

type compareLineupStore interface {
	ListLineups(ctx context.Context, userID, leagueID string, weekNumber int, rosterID *int) ([]provider.Lineup, error)
}

// HandleGetCompare scores the authenticated user's submitted lineup against the real
// manager's official lineup AND the roster's optimal lineup, for one week. It is behind
// requireAuth; the user is taken from the JWT (not a query param). It also powers the
// per-week results screen, so it returns live scores for the current week with final=false
// (the leaderboard grader ignores non-final results). See scenario set A.
func HandleGetCompare(p compareProvider, store compareLineupStore, currentWeek func(context.Context) int) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, ok := ClaimsFromContext(r.Context())
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		leagueID := r.PathValue("leagueId")
		week, ok := parseWeek(r.PathValue("week"))
		if !ok {
			http.Error(w, "invalid week", http.StatusBadRequest)
			return
		}
		rosterID, ok := parsePositiveInt(r.PathValue("rosterId"))
		if !ok {
			http.Error(w, "invalid roster_id", http.StatusBadRequest)
			return
		}

		resp, status, msg := scoreUserWeek(r.Context(), p, store, currentWeek, claims.Subject, leagueID, week, rosterID)
		if status != http.StatusOK {
			http.Error(w, msg, status)
			return
		}
		_ = encode(w, r, http.StatusOK, resp)
	})
}

// HandleSetterLineup is the PUBLIC per-setter view powering the weekly results browser's
// drill-down: given ?user_id=, it scores that user's lineup for (league, week, roster) vs the
// official + optimal lineup. Same shape as compare; lineups are public within the app. 404 if
// that user set no lineup for the week.
func HandleSetterLineup(p compareProvider, store compareLineupStore, currentWeek func(context.Context) int) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID := r.URL.Query().Get("user_id")
		if userID == "" {
			http.Error(w, "user_id is required", http.StatusBadRequest)
			return
		}
		leagueID := r.PathValue("leagueId")
		week, ok := parseWeek(r.PathValue("week"))
		if !ok {
			http.Error(w, "invalid week", http.StatusBadRequest)
			return
		}
		rosterID, ok := parsePositiveInt(r.PathValue("rosterId"))
		if !ok {
			http.Error(w, "invalid roster_id", http.StatusBadRequest)
			return
		}

		resp, status, msg := scoreUserWeek(r.Context(), p, store, currentWeek, userID, leagueID, week, rosterID)
		if status != http.StatusOK {
			http.Error(w, msg, status)
			return
		}
		_ = encode(w, r, http.StatusOK, resp)
	})
}

// scoreUserWeek scores userID's submitted lineup for (league, week, roster) against the real
// manager's official lineup AND the roster's optimal lineup. It returns the response plus an
// HTTP status; when status != 200 the response is zero and msg is the error body. Shared by
// the auth'd self-compare (HandleGetCompare) and the public per-setter view (HandleSetterLineup)
// so the two never drift. A started player off the roster scores 0 (D12), not an error.
func scoreUserWeek(ctx context.Context, p compareProvider, store compareLineupStore, currentWeek func(context.Context) int, userID, leagueID string, week, rosterID int) (provider.CompareResponse, int, string) {
	var wg sync.WaitGroup
	var weekMatchups []provider.WeekMatchup
	var league provider.League
	var lineups []provider.Lineup
	var matchupErr, leagueErr, lineupErr error

	wg.Add(3)
	go func() {
		defer wg.Done()
		weekMatchups, matchupErr = p.GetWeekMatchups(ctx, leagueID, week)
	}()
	go func() {
		defer wg.Done()
		league, leagueErr = p.GetLeague(ctx, leagueID)
	}()
	go func() {
		defer wg.Done()
		lineups, lineupErr = store.ListLineups(ctx, userID, leagueID, week, &rosterID)
	}()
	wg.Wait()

	if matchupErr != nil {
		return provider.CompareResponse{}, http.StatusInternalServerError, "failed to fetch matchups"
	}
	if leagueErr != nil {
		return provider.CompareResponse{}, http.StatusInternalServerError, "failed to fetch league"
	}
	if lineupErr != nil {
		return provider.CompareResponse{}, http.StatusInternalServerError, "failed to fetch lineup"
	}

	official := provider.FindMatchup(weekMatchups, rosterID)
	if official == nil {
		return provider.CompareResponse{}, http.StatusNotFound, "roster not found in matchups for this week"
	}
	if len(lineups) == 0 {
		return provider.CompareResponse{}, http.StatusNotFound, "no lineup submitted for this week"
	}
	userLineup := lineups[0]

	// Index the roster's players for the user lineup's display. A user starter that
	// has left the roster (not here) simply scores 0 (D12) — never an error.
	playerByID := make(map[string]provider.Player, len(official.Players))
	for _, pl := range official.Players {
		playerByID[pl.PlayerID] = pl
	}

	grade := scoring.GradeWeek(grading.BuildGradeInput(league, official, userLineup, currentWeek(ctx)))

	return provider.CompareResponse{
		RosterID: rosterID,
		Week:     week,
		Official: provider.ScoredLineup{
			Starters:    scoredPlayers(official.Starters, official.PlayerPoints),
			TotalPoints: grade.OfficialTotal,
		},
		User: provider.ScoredLineup{
			LineupID:    userLineup.ID,
			Starters:    scoreStarters(userLineup.Starters, playerByID, official.PlayerPoints),
			TotalPoints: grade.UserTotal,
		},
		Winner:             grade.Result,
		OptimalPoints:      grade.OptimalTotal,
		UserEfficiency:     grade.UserEfficiency,
		OfficialEfficiency: grade.OfficialEfficiency,
		Edge:               grade.Edge,
		Final:              grade.Final,
	}, http.StatusOK, ""
}

// scoreStarters builds the per-player display rows from starter IDs. A player id not on the
// roster (departed) still renders, carrying its id and 0 points.
func scoreStarters(ids []string, playerByID map[string]provider.Player, points map[string]float64) []provider.ScoredPlayer {
	out := make([]provider.ScoredPlayer, len(ids))
	for i, id := range ids {
		player, ok := playerByID[id]
		if !ok {
			player = provider.Player{PlayerID: id}
		}
		out[i] = provider.ScoredPlayer{Player: player, Points: points[id]}
	}
	return out
}

// scoredPlayers builds the per-player display rows from full player objects (the official
// starters already carry their player data, so no roster lookup is needed).
func scoredPlayers(players []provider.Player, points map[string]float64) []provider.ScoredPlayer {
	out := make([]provider.ScoredPlayer, len(players))
	for i, p := range players {
		out[i] = provider.ScoredPlayer{Player: p, Points: points[p.PlayerID]}
	}
	return out
}
