package handlers

import (
	"context"
	"fmt"
	"net/http"
	"sync"

	"github.com/sunkencosts/mirrorleague/internal/provider"
)

const (
	winnerOfficial = "official"
	winnerUser     = "user"
	winnerTie      = "tie"
)

type compareMatchupProvider interface {
	GetWeekMatchups(ctx context.Context, leagueID string, week int) ([]provider.WeekMatchup, error)
}

type compareLineupStore interface {
	ListLineups(ctx context.Context, userID, leagueID string, weekNumber int, rosterID *int) ([]provider.Lineup, error)
}

func HandleGetCompare(matchups compareMatchupProvider, store compareLineupStore) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		leagueID := r.PathValue("leagueId")
		userID := r.URL.Query().Get("user_id")

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
		if userID == "" {
			http.Error(w, "user_id is required", http.StatusBadRequest)
			return
		}

		var wg sync.WaitGroup
		var weekMatchups []provider.WeekMatchup
		var lineups []provider.Lineup
		var matchupErr, lineupErr error

		wg.Add(2)
		go func() {
			defer wg.Done()
			weekMatchups, matchupErr = matchups.GetWeekMatchups(r.Context(), leagueID, week)
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

		playerByID := make(map[string]provider.Player, len(official.Players))
		for _, p := range official.Players {
			playerByID[p.PlayerID] = p
		}

		officialStarters := make([]provider.ScoredPlayer, len(official.Starters))
		for i, p := range official.Starters {
			officialStarters[i] = provider.ScoredPlayer{
				Player: p,
				Points: official.PlayerPoints[p.PlayerID],
			}
		}

		userStarters := make([]provider.ScoredPlayer, len(userLineup.Starters))
		var userTotal float64
		for i, playerID := range userLineup.Starters {
			p, ok := playerByID[playerID]
			if !ok {
				http.Error(w, fmt.Sprintf("starter %s not found in roster", playerID), http.StatusInternalServerError)
				return
			}
			pts := official.PlayerPoints[playerID]
			userStarters[i] = provider.ScoredPlayer{Player: p, Points: pts}
			userTotal += pts
		}

		officialTotal := official.Points
		if official.CustomPoints != nil {
			officialTotal = *official.CustomPoints
		}

		winner := winnerTie
		if officialTotal > userTotal {
			winner = winnerOfficial
		} else if userTotal > officialTotal {
			winner = winnerUser
		}

		_ = encode(w, r, http.StatusOK, provider.CompareResponse{
			RosterID: rosterID,
			Week:     week,
			Official: provider.ScoredLineup{
				Starters:    officialStarters,
				TotalPoints: officialTotal,
			},
			User: provider.ScoredLineup{
				LineupID:    userLineup.ID,
				Starters:    userStarters,
				TotalPoints: userTotal,
			},
			Winner: winner,
		})
	})
}

func findMatchup(matchups []provider.WeekMatchup, rosterID int) *provider.WeekMatchup {
	for i := range matchups {
		if matchups[i].RosterID == rosterID {
			return &matchups[i]
		}
	}
	return nil
}
