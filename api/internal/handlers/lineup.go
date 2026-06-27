package handlers

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/sunkencosts/mirrorleague/internal/provider"
	"github.com/sunkencosts/mirrorleague/internal/scoring"
)

type lineupStore interface {
	CreateLineup(ctx context.Context, userID, leagueID, season, source string, rosterID, weekNumber int, starters []string) (provider.Lineup, error)
	GetLineup(ctx context.Context, id string) (provider.Lineup, error)
	UpdateLineup(ctx context.Context, id string, starters []string) (provider.Lineup, error)
	ListLineups(ctx context.Context, userID, leagueID string, weekNumber int, rosterID *int) ([]provider.Lineup, error)
	weekLockStore
}

// weekLockStore is the lock lookup shared by the lineup gate and the week-matchups
// envelope. A missing row means "not locked" (fail open).
type weekLockStore interface {
	GetWeekLock(ctx context.Context, season string, week int) (time.Time, bool, error)
}

type lineupMatchupProvider interface {
	GetWeekMatchups(ctx context.Context, leagueID string, week int) ([]provider.WeekMatchup, error)
}

// lineupCreateProvider additionally resolves the league so create can derive and
// store the season the lineup belongs to.
type lineupCreateProvider interface {
	lineupMatchupProvider
	GetLeague(ctx context.Context, leagueID string) (provider.League, error)
}

// weekLocked reports whether (season, week) is locked as of now. A missing
// week_locks row is treated as unlocked (fail open) and warned. LocksAt is nil
// only when no row exists.
func weekLocked(ctx context.Context, store weekLockStore, season string, week int) (bool, *time.Time, error) {
	locksAt, ok, err := store.GetWeekLock(ctx, season, week)
	if err != nil {
		return false, nil, err
	}
	if !ok {
		slog.WarnContext(ctx, "no week_locks row; treating week as unlocked (fail open)",
			slog.String("season", season), slog.Int("week", week))
		return false, nil, nil
	}
	return !time.Now().Before(locksAt), &locksAt, nil
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

func HandleCreateLineup(store lineupStore, p lineupCreateProvider) http.Handler {
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

		// Derive the season from the league (immutable per league_id) so the lock
		// lookup is a pure DB check and the season is stored on the lineup.
		league, err := p.GetLeague(r.Context(), req.LeagueID)
		if err != nil {
			http.Error(w, "failed to resolve league", http.StatusInternalServerError)
			return
		}

		locked, _, err := weekLocked(r.Context(), store, league.Season, req.WeekNumber)
		if err != nil {
			http.Error(w, "failed to check lock", http.StatusInternalServerError)
			return
		}
		if locked {
			http.Error(w, "lineup locked", http.StatusConflict)
			return
		}

		if err := validateStarters(r.Context(), p, req.LeagueID, req.RosterID, req.WeekNumber, league.RosterPositions, req.Starters); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		lineup, err := store.CreateLineup(r.Context(), claims.Subject, req.LeagueID, league.Season, req.Source, req.RosterID, req.WeekNumber, req.Starters)
		if err != nil {
			http.Error(w, "failed to create lineup", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Location", "/lineups/"+lineup.ID)
		_ = encode(w, r, http.StatusCreated, lineup)
	})
}

func HandleUpdateLineup(store lineupStore, p lineupCreateProvider) http.Handler {
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

		locked, _, err := weekLocked(r.Context(), store, existing.Season, existing.WeekNumber)
		if err != nil {
			http.Error(w, "failed to check lock", http.StatusInternalServerError)
			return
		}
		if locked {
			http.Error(w, "lineup locked", http.StatusConflict)
			return
		}

		league, err := p.GetLeague(r.Context(), existing.LeagueID)
		if err != nil {
			http.Error(w, "failed to resolve league", http.StatusInternalServerError)
			return
		}

		if err := validateStarters(r.Context(), p, existing.LeagueID, existing.RosterID, existing.WeekNumber, league.RosterPositions, req.Starters); err != nil {
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
		// week_number is optional: omit it to list the user's lineups across all weeks (used to
		// discover which weeks/rosters they've played). When present it must be a valid int.
		weekNumber := 0
		if raw := r.URL.Query().Get("week_number"); raw != "" {
			n, err := strconv.Atoi(raw)
			if err != nil {
				http.Error(w, "invalid week_number", http.StatusBadRequest)
				return
			}
			weekNumber = n
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
		// When a specific week was requested, all lineups share one (season, week), so one lock
		// lookup annotates them all. The all-weeks listing is for discovery only — skip the
		// per-week lock annotation (it isn't meaningful across mixed weeks).
		if weekNumber > 0 && len(lineups) > 0 {
			locked, locksAt, lockErr := weekLocked(r.Context(), store, lineups[0].Season, weekNumber)
			if lockErr == nil {
				for i := range lineups {
					lineups[i].Locked = locked
					lineups[i].LocksAt = locksAt
				}
			}
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
		if locked, locksAt, lockErr := weekLocked(r.Context(), store, lineup.Season, lineup.WeekNumber); lockErr == nil {
			lineup.Locked = locked
			lineup.LocksAt = locksAt
		}
		_ = encode(w, r, http.StatusOK, lineup)
	})
}
func validateStarters(ctx context.Context, p lineupMatchupProvider, leagueID string, rosterID, week int, rosterPositions, starters []string) error {
	matchups, err := p.GetWeekMatchups(ctx, leagueID, week)
	if err != nil {
		return fmt.Errorf("fetching matchups: %w", err)
	}

	if len(matchups) == 0 {
		// No matchup data published for this week yet — skip validation (D17).
		return nil
	}
	matchup := provider.FindMatchup(matchups, rosterID)
	if matchup == nil {
		return fmt.Errorf("roster %d not found in league for week %d", rosterID, week)
	}

	// Membership: every starter must be on this roster.
	playerSet := make(map[string]struct{}, len(matchup.Players))
	playerPositions := make(map[string][]string, len(matchup.Players))
	for _, player := range matchup.Players {
		playerSet[player.PlayerID] = struct{}{}
		playerPositions[player.PlayerID] = player.FantasyPositions
	}
	for _, id := range starters {
		if _, ok := playerSet[id]; !ok {
			return fmt.Errorf("player %s was not available for week %d", id, week)
		}
	}

	// Legality: exact slot count + FLEX/SUPER_FLEX-aware position assignment + no dupes.
	return scoring.ValidateLineup(ctx, rosterPositions, starters, playerPositions)
}
