package handlers

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/sunkencosts/mirrorleague/internal/provider"
)

type lineupStore interface {
	CreateLineup(ctx context.Context, userID, leagueID, source string, rosterID, weekNumber int, starters []string) (provider.Lineup, error)
	GetLineup(ctx context.Context, id string) (provider.Lineup, error)
	UpdateLineup(ctx context.Context, id string, starters []string) (provider.Lineup, error)
	ListLineups(ctx context.Context, userID, leagueID string, weekNumber int, rosterID *int) ([]provider.Lineup, error)
}

type lineupMatchupProvider interface {
	GetWeekMatchups(ctx context.Context, leagueID string, week int) ([]provider.WeekMatchup, error)
}

type createLineupRequest struct {
	LeagueID   string   `json:"league_id"`
	Source     string   `json:"source"`
	RosterID   int      `json:"roster_id"`
	WeekNumber int      `json:"week_number"`
	Starters   []string `json:"starters"`
}

type updateLineupRequest struct {
	Starters []string `json:"starters"`
}

func HandleCreateLineup(store lineupStore, p lineupMatchupProvider) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, ok := ClaimsFromContext(r.Context())
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		req, err := decode[createLineupRequest](r)
		if err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		if req.Source == "" {
			http.Error(w, "missing source", http.StatusBadRequest)
			return
		}

		if err := validateStarters(r.Context(), p, req.LeagueID, req.RosterID, req.WeekNumber, req.Starters); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		lineup, err := store.CreateLineup(r.Context(), claims.Subject, req.LeagueID, req.Source, req.RosterID, req.WeekNumber, req.Starters)
		if err != nil {
			http.Error(w, "failed to create lineup", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Location", "/lineups/"+lineup.ID)
		_ = encode(w, r, http.StatusCreated, lineup)
	})
}

func HandleUpdateLineup(store lineupStore, p lineupMatchupProvider) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, ok := ClaimsFromContext(r.Context())
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		id := r.PathValue("id")
		if _, err := uuid.Parse(id); err != nil {
			http.Error(w, "invalid lineup id", http.StatusBadRequest)
			return
		}

		req, err := decode[updateLineupRequest](r)
		if err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		existing, err := store.GetLineup(r.Context(), id)
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "lineup not found", http.StatusNotFound)
			return
		}
		if err != nil {
			http.Error(w, "failed to get lineup", http.StatusInternalServerError)
			return
		}
		if existing.UserID != claims.Subject {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}

		if err := validateStarters(r.Context(), p, existing.LeagueID, existing.RosterID, existing.WeekNumber, req.Starters); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		lineup, err := store.UpdateLineup(r.Context(), id, req.Starters)
		if err != nil {
			http.Error(w, "failed to update lineup", http.StatusInternalServerError)
			return
		}
		_ = encode(w, r, http.StatusOK, lineup)
	})
}

func HandleListLineups(store lineupStore) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID := r.URL.Query().Get("user_id")
		if _, err := uuid.Parse(userID); err != nil {
			http.Error(w, "invalid user_id", http.StatusBadRequest)
			return
		}
		leagueID := r.URL.Query().Get("league_id")
		if leagueID == "" {
			http.Error(w, "missing league_id", http.StatusBadRequest)
			return
		}
		weekNumber, err := strconv.Atoi(r.URL.Query().Get("week_number"))
		if err != nil {
			http.Error(w, "invalid week_number", http.StatusBadRequest)
			return
		}

		var rosterID *int
		if raw := r.URL.Query().Get("roster_id"); raw != "" {
			id, err := strconv.Atoi(raw)
			if err != nil {
				http.Error(w, "invalid roster_id", http.StatusBadRequest)
				return
			}
			rosterID = &id
		}

		lineups, err := store.ListLineups(r.Context(), userID, leagueID, weekNumber, rosterID)
		if err != nil {
			http.Error(w, "failed to list lineups", http.StatusInternalServerError)
			return
		}
		_ = encode(w, r, http.StatusOK, lineups)
	})
}
func HandleGetLineupByID(store lineupStore) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if _, err := uuid.Parse(id); err != nil {
			http.Error(w, "invalid lineup id", http.StatusBadRequest)
			return
		}
		lineup, err := store.GetLineup(r.Context(), id)
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "lineup not found", http.StatusNotFound)
			return
		}
		if err != nil {
			http.Error(w, "failed to get lineup", http.StatusInternalServerError)
			return
		}
		_ = encode(w, r, http.StatusOK, lineup)
	})
}
func validateStarters(ctx context.Context, p lineupMatchupProvider, leagueID string, rosterID, week int, starters []string) error {
	matchups, err := p.GetWeekMatchups(ctx, leagueID, week)
	if err != nil {
		return fmt.Errorf("fetching matchups: %w", err)
	}

	if len(matchups) == 0 {
		// No matchup data published for this week yet — skip validation.
		return nil
	}
	matchup := findMatchup(matchups, rosterID)
	if matchup == nil {
		return fmt.Errorf("roster %d not found in league for week %d", rosterID, week)
	}

	playerSet := make(map[string]struct{}, len(matchup.Players))
	for _, p := range matchup.Players {
		playerSet[p.PlayerID] = struct{}{}
	}
	for _, id := range starters {
		if _, ok := playerSet[id]; !ok {
			return fmt.Errorf("player %s was not available for week %d", id, week)
		}
	}
	return nil
}
