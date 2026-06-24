package handlers

import (
	"context"
	"net/http"
	"sync"

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
		userID := claims.Subject

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

		var wg sync.WaitGroup
		var weekMatchups []provider.WeekMatchup
		var league provider.League
		var lineups []provider.Lineup
		var matchupErr, leagueErr, lineupErr error

		wg.Add(3)
		go func() {
			defer wg.Done()
			weekMatchups, matchupErr = p.GetWeekMatchups(r.Context(), leagueID, week)
		}()
		go func() {
			defer wg.Done()
			league, leagueErr = p.GetLeague(r.Context(), leagueID)
		}()
		go func() {
			defer wg.Done()
			lineups, lineupErr = store.ListLineups(r.Context(), userID, leagueID, week, &rosterID)
		}()
		wg.Wait()

		if matchupErr != nil {
			http.Error(w, "failed to fetch matchups", http.StatusInternalServerError)
			return
		}
		if leagueErr != nil {
			http.Error(w, "failed to fetch league", http.StatusInternalServerError)
			return
		}
		if lineupErr != nil {
			http.Error(w, "failed to fetch lineup", http.StatusInternalServerError)
			return
		}

		official := findMatchup(weekMatchups, rosterID)
		if official == nil {
			http.Error(w, "roster not found in matchups for this week", http.StatusNotFound)
			return
		}
		if len(lineups) == 0 {
			http.Error(w, "no lineup submitted for this week", http.StatusNotFound)
			return
		}
		userLineup := lineups[0]

		// Index the roster's players for display + position lookup. A user starter that
		// has left the roster (not here) simply scores 0 (D12) — never an error.
		playerByID := make(map[string]provider.Player, len(official.Players))
		playerPositions := make(map[string][]string, len(official.Players))
		rosterPlayerIDs := make([]string, 0, len(official.Players))
		for _, pl := range official.Players {
			playerByID[pl.PlayerID] = pl
			playerPositions[pl.PlayerID] = pl.FantasyPositions
			rosterPlayerIDs = append(rosterPlayerIDs, pl.PlayerID)
		}

		officialTotal := official.Points
		if official.CustomPoints != nil {
			officialTotal = *official.CustomPoints
		}

		grade := scoring.GradeWeek(scoring.GradeInput{
			RosterPositions: league.RosterPositions,
			RosterPlayers:   rosterPlayerIDs,
			UserStarters:    userLineup.Starters,
			OfficialTotal:   officialTotal,
			Points:          official.PlayerPoints,
			PlayerPositions: playerPositions,
			Week:            week,
			CurrentWeek:     currentWeek(r.Context()),
		})

		_ = encode(w, r, http.StatusOK, provider.CompareResponse{
			RosterID: rosterID,
			Week:     week,
			Official: provider.ScoredLineup{
				Starters:    scoreStarters(playerIDs(official.Starters), playerByID, official.PlayerPoints),
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
		})
	})
}

// scoreStarters builds the per-player display rows for a lineup. A player id not on the
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

func playerIDs(players []provider.Player) []string {
	ids := make([]string, len(players))
	for i, p := range players {
		ids[i] = p.PlayerID
	}
	return ids
}

func findMatchup(matchups []provider.WeekMatchup, rosterID int) *provider.WeekMatchup {
	for i := range matchups {
		if matchups[i].RosterID == rosterID {
			return &matchups[i]
		}
	}
	return nil
}
