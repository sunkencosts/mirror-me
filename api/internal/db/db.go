package db

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sunkencosts/mirrorleague/internal/provider"
	"github.com/sunkencosts/mirrorleague/internal/scoring"
)

type scanner interface {
	Scan(dest ...any) error
}

func scanLineup(row scanner) (provider.Lineup, error) {
	var l provider.Lineup
	err := row.Scan(&l.ID, &l.UserID, &l.LeagueID, &l.Source, &l.RosterID, &l.WeekNumber, &l.Season, &l.Starters, &l.CreatedAt, &l.UpdatedAt)
	return l, err
}

type Store struct {
	pool *pgxpool.Pool
}

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

func (s *Store) Ping(ctx context.Context) error {
	return s.pool.Ping(ctx)
}

func (s *Store) UpsertPlayers(ctx context.Context, players []provider.Player) error {
	batch := &pgx.Batch{}
	for _, player := range players {
		if player.FantasyPositions == nil {
			player.FantasyPositions = []string{}
		}
		batch.Queue(`INSERT INTO players (player_id, first_name, last_name, team, active, fantasy_positions, number, age, rarity, updated_at)
					VALUES($1, $2, $3, $4, $5, $6, $7, $8, $9, now())
					ON CONFLICT (player_id)
					DO UPDATE SET first_name=EXCLUDED.first_name, last_name=EXCLUDED.last_name, team=EXCLUDED.team, active=EXCLUDED.active,
      				fantasy_positions=EXCLUDED.fantasy_positions, number=EXCLUDED.number, age=EXCLUDED.age, rarity=EXCLUDED.rarity, updated_at=now()
					WHERE players.first_name IS DISTINCT FROM EXCLUDED.first_name
					   OR players.last_name IS DISTINCT FROM EXCLUDED.last_name
					   OR players.team IS DISTINCT FROM EXCLUDED.team
					   OR players.active IS DISTINCT FROM EXCLUDED.active
					   OR players.fantasy_positions IS DISTINCT FROM EXCLUDED.fantasy_positions
					   OR players.number IS DISTINCT FROM EXCLUDED.number
					   OR players.age IS DISTINCT FROM EXCLUDED.age
					   OR players.rarity IS DISTINCT FROM EXCLUDED.rarity`,
			player.PlayerID, player.FirstName, player.LastName, player.Team, player.Active, player.FantasyPositions, player.Number, player.Age, player.Rarity)
	}
	results := s.pool.SendBatch(ctx, batch)
	defer func() { _ = results.Close() }()

	for _, player := range players {
		if _, err := results.Exec(); err != nil {
			return fmt.Errorf("upserting player %s: %w", player.PlayerID, err)
		}
	}
	return nil
}

func (s *Store) ListActiveFantasyPlayers(ctx context.Context) ([]provider.SlimPlayer, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT player_id, first_name, last_name, team, fantasy_positions, rarity
		 FROM players
		 WHERE active = true AND fantasy_positions && $1`,
		[]string{"QB", "RB", "WR", "TE", "K", "DEF"})
	if err != nil {
		return nil, fmt.Errorf("listing active fantasy players: %w", err)
	}
	defer rows.Close()

	players := []provider.SlimPlayer{}
	for rows.Next() {
		var p provider.SlimPlayer
		if err := rows.Scan(&p.PlayerID, &p.FirstName, &p.LastName, &p.Team, &p.FantasyPositions, &p.Rarity); err != nil {
			return nil, fmt.Errorf("scanning slim player: %w", err)
		}
		p.ImageURL = "https://sleepercdn.com/content/nfl/players/thumb/" + p.PlayerID + ".jpg"
		players = append(players, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating slim players: %w", err)
	}
	return players, nil
}

func (s *Store) GetPlayersByIDs(ctx context.Context, ids []string) (map[string]provider.Player, error) {
	rows, err := s.pool.Query(ctx, "SELECT player_id, first_name, last_name, team, active, fantasy_positions, number, age, rarity FROM players WHERE player_id = ANY($1)", ids)

	if err != nil {
		return nil, fmt.Errorf("selecting players by id: %w", err)
	}
	defer rows.Close()

	result := map[string]provider.Player{}
	for rows.Next() {
		var p provider.Player
		if err := rows.Scan(&p.PlayerID, &p.FirstName, &p.LastName, &p.Team, &p.Active, &p.FantasyPositions, &p.Number, &p.Age, &p.Rarity); err != nil {
			return nil, fmt.Errorf("scanning player: %w", err)
		}
		result[p.PlayerID] = p
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating players: %w", err)
	}
	return result, nil
}

func (s *Store) CreateLineup(ctx context.Context, userID, leagueID, season, source string, rosterID, weekNumber int, starters []string) (provider.Lineup, error) {
	row := s.pool.QueryRow(ctx, `
		INSERT INTO lineups (user_id, league_id, source, roster_id, week_number, season, starters)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, user_id, league_id, source, roster_id, week_number, season, starters, created_at, updated_at
	`, userID, leagueID, source, rosterID, weekNumber, season, starters)
	l, err := scanLineup(row)
	if err != nil {
		return provider.Lineup{}, fmt.Errorf("creating lineup: %w", err)
	}
	return l, nil
}

func (s *Store) GetLineup(ctx context.Context, id string) (provider.Lineup, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, user_id, league_id, source, roster_id, week_number, season, starters, created_at, updated_at
		FROM lineups WHERE id = $1
	`, id)
	l, err := scanLineup(row)
	if err != nil {
		return provider.Lineup{}, fmt.Errorf("getting lineup %s: %w", id, err)
	}
	return l, nil
}

// GetWeekLock returns the lock time for an NFL (season, week). ok is false when no
// row has been seeded, which callers treat as "not locked" (fail open).
func (s *Store) GetWeekLock(ctx context.Context, season string, week int) (time.Time, bool, error) {
	var locksAt time.Time
	err := s.pool.QueryRow(ctx,
		`SELECT locks_at FROM week_locks WHERE season = $1 AND week = $2`, season, week).Scan(&locksAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return time.Time{}, false, nil
	}
	if err != nil {
		return time.Time{}, false, fmt.Errorf("getting week lock %s/%d: %w", season, week, err)
	}
	return locksAt, true, nil
}

func (s *Store) ListLineups(ctx context.Context, userID, leagueID string, weekNumber int, rosterID *int) ([]provider.Lineup, error) {
	query := `
		SELECT id, user_id, league_id, source, roster_id, week_number, season, starters, created_at, updated_at
		FROM lineups
		WHERE user_id = $1 AND league_id = $2`
	args := []any{userID, leagueID}
	// weekNumber <= 0 means "all weeks" (the discovery listing); a positive value filters.
	if weekNumber > 0 {
		args = append(args, weekNumber)
		query += fmt.Sprintf(` AND week_number = $%d`, len(args))
	}
	if rosterID != nil {
		args = append(args, *rosterID)
		query += fmt.Sprintf(` AND roster_id = $%d`, len(args))
	}
	query += ` ORDER BY week_number DESC`

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("listing lineups: %w", err)
	}
	defer rows.Close()

	lineups := []provider.Lineup{}
	for rows.Next() {
		l, err := scanLineup(rows)
		if err != nil {
			return nil, fmt.Errorf("scanning lineup: %w", err)
		}
		lineups = append(lineups, l)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating lineups: %w", err)
	}
	return lineups, nil
}

// CurrentWeekFromLocks infers the live NFL week for a season from week_locks: the latest
// week whose first kickoff has already passed. Returns 0 when no week has kicked off yet
// (pre-season), which the caller maps to week 1. This auto-advances at each week's kickoff,
// so no manual CURRENT_WEEK bump is needed. (Unseeded weeks — e.g. 2026 wk 12/18 — are gaps
// that can briefly hold the inferred week back until the next seeded kickoff passes.)
func (s *Store) CurrentWeekFromLocks(ctx context.Context, season string, at time.Time) (int, error) {
	var week int
	err := s.pool.QueryRow(ctx,
		`SELECT COALESCE(MAX(week), 0) FROM week_locks WHERE season = $1 AND locks_at <= $2`,
		season, at).Scan(&week)
	if err != nil {
		return 0, fmt.Errorf("inferring current week for %s: %w", season, err)
	}
	return week, nil
}

// ListGradableLineups returns every lineup for a past week (week_number < currentWeek)
// that does NOT yet have a week_results row. This drives both first-time grading and
// backfill: a lineup that failed to grade once (e.g. transient Sleeper outage) keeps
// showing up until a result is written, and an already-graded lineup is excluded
// (idempotent).
func (s *Store) ListGradableLineups(ctx context.Context, currentWeek int) ([]provider.Lineup, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT l.id, l.user_id, l.league_id, l.source, l.roster_id, l.week_number, l.season, l.starters, l.created_at, l.updated_at
		FROM lineups l
		WHERE l.week_number < $1
		  AND NOT EXISTS (
		      SELECT 1 FROM week_results wr
		      WHERE wr.user_id = l.user_id AND wr.league_id = l.league_id
		        AND wr.roster_id = l.roster_id AND wr.week = l.week_number AND wr.season = l.season
		  )
	`, currentWeek)
	if err != nil {
		return nil, fmt.Errorf("listing gradable lineups: %w", err)
	}
	defer rows.Close()

	lineups := []provider.Lineup{}
	for rows.Next() {
		l, err := scanLineup(rows)
		if err != nil {
			return nil, fmt.Errorf("scanning gradable lineup: %w", err)
		}
		lineups = append(lineups, l)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating gradable lineups: %w", err)
	}
	return lineups, nil
}

// Leaderboard aggregates week_results into per-user standings for a season, sorted by
// mean lineup efficiency (D3/D24) then weeks played then user id (D20). leagueID == ""
// gives the global board (pooled across all of a user's mirrors, D14); a non-empty
// leagueID scopes to one league. The JOIN to users means only authenticated users with a
// username appear (D15) — anonymous user_ids are excluded. Rank/Provisional are policy
// applied by the caller (the min-weeks gate differs global vs per-league).
func (s *Store) Leaderboard(ctx context.Context, season, leagueID string) ([]provider.LeaderboardRow, error) {
	args := []any{season}
	leagueFilter := ""
	if leagueID != "" {
		args = append(args, leagueID)
		leagueFilter = "AND wr.league_id = $2"
	}

	// edge mirrors scoring.GradeWeek's per-week Edge exactly: the mean of
	// clamp01(user/optimal) - clamp01(official/optimal), so the season figure reconciles
	// with the per-week edges the compare endpoint reports. weeks_played counts only
	// SCORED weeks (optimal_total > 0); weeks excluded from mean_eff via NULLIF must not
	// pad the count or the provisional-rank gate (they would otherwise describe a larger
	// sample than the efficiency they sit next to).
	query := `
		SELECT wr.user_id, u.username,
		       COALESCE(AVG(wr.user_total / NULLIF(wr.optimal_total, 0)), 0) AS mean_eff,
		       COALESCE(AVG(
		           LEAST(GREATEST(wr.user_total / NULLIF(wr.optimal_total, 0), 0), 1)
		           - LEAST(GREATEST(wr.official_total / NULLIF(wr.optimal_total, 0), 0), 1)
		       ), 0) AS edge,
		       COALESCE(
		           COUNT(*) FILTER (WHERE wr.result = 'user')::float8
		           / NULLIF(COUNT(*) FILTER (WHERE wr.result IN ('user', 'official')), 0),
		       0) AS win_rate,
		       COUNT(*) FILTER (WHERE wr.optimal_total > 0) AS weeks_played
		FROM week_results wr
		JOIN users u ON u.id::text = wr.user_id
		WHERE wr.season = $1 ` + leagueFilter + `
		GROUP BY wr.user_id, u.username
		ORDER BY mean_eff DESC, weeks_played DESC, wr.user_id ASC`

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("querying leaderboard: %w", err)
	}
	defer rows.Close()

	out := []provider.LeaderboardRow{}
	for rows.Next() {
		var r provider.LeaderboardRow
		if err := rows.Scan(&r.UserID, &r.Username, &r.MeanEfficiency, &r.Edge, &r.WinRate, &r.WeeksPlayed); err != nil {
			return nil, fmt.Errorf("scanning leaderboard row: %w", err)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating leaderboard: %w", err)
	}
	return out, nil
}

// WeeklyResults returns the per-setter standings for one (season, league, week, roster):
// the official/optimal baseline (identical across the roster's rows, so MAX picks it) plus
// every setter ranked by efficiency desc. Rank is computed over ALL setters (a window), then
// the slice is optionally filtered by a username substring (q) and paginated (limit/offset) —
// so a searched/paged row still carries its true standing. SetterCount is the unfiltered
// total. Powers the weekly results browser; reads only the cached week_results (no Sleeper).
func (s *Store) WeeklyResults(ctx context.Context, season, leagueID string, rosterID, week int, q string, limit, offset int) (provider.WeeklyRosterResults, error) {
	out := provider.WeeklyRosterResults{RosterID: rosterID, Setters: []provider.WeeklySetterResult{}}

	err := s.pool.QueryRow(ctx, `
		SELECT COALESCE(MAX(official_total), 0), COALESCE(MAX(optimal_total), 0), COUNT(*),
		       COUNT(*) FILTER (WHERE user_total > official_total)
		FROM week_results
		WHERE season = $1 AND league_id = $2 AND roster_id = $3 AND week = $4`,
		season, leagueID, rosterID, week).Scan(&out.OfficialTotal, &out.OptimalTotal, &out.SetterCount, &out.BeatOfficialCount)
	if err != nil {
		return provider.WeeklyRosterResults{}, fmt.Errorf("weekly results baseline: %w", err)
	}
	if out.OptimalTotal > 0 {
		out.OfficialEfficiency = scoring.Clamp01(out.OfficialTotal / out.OptimalTotal)
	}
	if out.SetterCount == 0 {
		return out, nil
	}

	rows, err := s.pool.Query(ctx, `
		WITH ranked AS (
			SELECT wr.user_id, u.username, wr.user_total, wr.result,
			       LEAST(GREATEST(wr.user_total / NULLIF(wr.optimal_total, 0), 0), 1) AS efficiency,
			       LEAST(GREATEST(wr.user_total / NULLIF(wr.optimal_total, 0), 0), 1)
			         - LEAST(GREATEST(wr.official_total / NULLIF(wr.optimal_total, 0), 0), 1) AS edge,
			       ROW_NUMBER() OVER (
			           ORDER BY LEAST(GREATEST(wr.user_total / NULLIF(wr.optimal_total, 0), 0), 1) DESC,
			                    wr.user_id ASC
			       ) AS rank
			FROM week_results wr
			JOIN users u ON u.id::text = wr.user_id
			WHERE wr.season = $1 AND wr.league_id = $2 AND wr.roster_id = $3 AND wr.week = $4
		)
		SELECT user_id, username, user_total, efficiency, edge, result, rank
		FROM ranked
		WHERE ($5 = '' OR username ILIKE '%' || $5 || '%')
		ORDER BY rank
		LIMIT $6 OFFSET $7`,
		season, leagueID, rosterID, week, q, limit, offset)
	if err != nil {
		return provider.WeeklyRosterResults{}, fmt.Errorf("weekly results setters: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var r provider.WeeklySetterResult
		if err := rows.Scan(&r.UserID, &r.Username, &r.UserTotal, &r.Efficiency, &r.Edge, &r.Result, &r.Rank); err != nil {
			return provider.WeeklyRosterResults{}, fmt.Errorf("scanning weekly setter: %w", err)
		}
		out.Setters = append(out.Setters, r)
	}
	if err := rows.Err(); err != nil {
		return provider.WeeklyRosterResults{}, fmt.Errorf("iterating weekly setters: %w", err)
	}
	return out, nil
}

// UpsertWeekResult writes (or overwrites) the graded outcome of one head-to-head week.
func (s *Store) UpsertWeekResult(ctx context.Context, r provider.WeekResult) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO week_results (user_id, league_id, roster_id, week, season, user_total, official_total, optimal_total, result)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (user_id, league_id, roster_id, week, season)
		DO UPDATE SET user_total = EXCLUDED.user_total, official_total = EXCLUDED.official_total,
		              optimal_total = EXCLUDED.optimal_total, result = EXCLUDED.result
	`, r.UserID, r.LeagueID, r.RosterID, r.Week, r.Season, r.UserTotal, r.OfficialTotal, r.OptimalTotal, r.Result)
	if err != nil {
		return fmt.Errorf("upserting week result: %w", err)
	}
	return nil
}

// GetCachedWeekMatchups returns the persisted matchup rows for a (league, week), resolving
// stored player IDs back into full Player objects (the same shape the Sleeper client returns).
// ok is false when nothing is cached. Keyed (league, week) like the in-memory cache.
func (s *Store) GetCachedWeekMatchups(ctx context.Context, leagueID string, week int) ([]provider.WeekMatchup, bool, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT roster_id, matchup_id, owner_id, team_name, points, custom_points, players, starters, player_points
		FROM week_matchups
		WHERE league_id = $1 AND week = $2
		ORDER BY roster_id`, leagueID, week)
	if err != nil {
		return nil, false, fmt.Errorf("reading cached week matchups: %w", err)
	}
	defer rows.Close()

	type rawMatchup struct {
		rosterID, matchupID int
		ownerID, teamName   string
		points              float64
		customPoints        *float64
		players, starters   []string
		playerPoints        map[string]float64
	}
	var raws []rawMatchup
	allIDs := []string{}
	for rows.Next() {
		var m rawMatchup
		if err := rows.Scan(&m.rosterID, &m.matchupID, &m.ownerID, &m.teamName, &m.points,
			&m.customPoints, &m.players, &m.starters, &m.playerPoints); err != nil {
			return nil, false, fmt.Errorf("scanning cached week matchup: %w", err)
		}
		allIDs = append(allIDs, m.players...)
		allIDs = append(allIDs, m.starters...)
		raws = append(raws, m)
	}
	if err := rows.Err(); err != nil {
		return nil, false, fmt.Errorf("iterating cached week matchups: %w", err)
	}
	if len(raws) == 0 {
		return nil, false, nil
	}

	playerMap, err := s.GetPlayersByIDs(ctx, allIDs)
	if err != nil {
		return nil, false, fmt.Errorf("resolving cached matchup players: %w", err)
	}

	out := make([]provider.WeekMatchup, 0, len(raws))
	for _, m := range raws {
		out = append(out, provider.WeekMatchup{
			RosterID:     m.rosterID,
			MatchupID:    m.matchupID,
			OwnerID:      m.ownerID,
			TeamName:     m.teamName,
			Points:       m.points,
			CustomPoints: m.customPoints,
			Players:      resolvePlayers(playerMap, m.players),
			Starters:     resolvePlayers(playerMap, m.starters),
			PlayerPoints: m.playerPoints,
		})
	}
	return out, true, nil
}

// SaveWeekMatchups upserts the matchup rows for a (league, week) into the persistent cache.
// Player objects are stored as their IDs; player_points as jsonb. Best-effort write-through
// for FINAL weeks so later reads skip Sleeper.
func (s *Store) SaveWeekMatchups(ctx context.Context, leagueID string, week int, matchups []provider.WeekMatchup) error {
	for _, m := range matchups {
		_, err := s.pool.Exec(ctx, `
			INSERT INTO week_matchups
			  (league_id, week, roster_id, matchup_id, owner_id, team_name, points, custom_points, players, starters, player_points)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
			ON CONFLICT (league_id, week, roster_id) DO UPDATE SET
			  matchup_id = EXCLUDED.matchup_id, owner_id = EXCLUDED.owner_id, team_name = EXCLUDED.team_name,
			  points = EXCLUDED.points, custom_points = EXCLUDED.custom_points, players = EXCLUDED.players,
			  starters = EXCLUDED.starters, player_points = EXCLUDED.player_points, fetched_at = now()`,
			leagueID, week, m.RosterID, m.MatchupID, m.OwnerID, m.TeamName, m.Points, m.CustomPoints,
			provider.PlayerIDs(m.Players), provider.PlayerIDs(m.Starters), m.PlayerPoints)
		if err != nil {
			return fmt.Errorf("saving week matchup %s/%d/r%d: %w", leagueID, week, m.RosterID, err)
		}
	}
	return nil
}

// GetCachedLeague returns the full cached provider.League, decoded from the jsonb blob.
// ok is false when the league isn't cached, or when the row predates the jsonb column
// (NULL data) — in which case it's treated as a miss so the caller refetches live.
func (s *Store) GetCachedLeague(ctx context.Context, leagueID string) (provider.League, bool, error) {
	var data []byte
	err := s.pool.QueryRow(ctx, `SELECT data FROM leagues WHERE league_id = $1`, leagueID).Scan(&data)
	if errors.Is(err, pgx.ErrNoRows) {
		return provider.League{}, false, nil
	}
	if err != nil {
		return provider.League{}, false, fmt.Errorf("reading cached league: %w", err)
	}
	if len(data) == 0 {
		// Row predates the jsonb cache (NULL data); treat as a miss so it refetches live.
		return provider.League{}, false, nil
	}
	var league provider.League
	if err := json.Unmarshal(data, &league); err != nil {
		return provider.League{}, false, fmt.Errorf("unmarshaling cached league %s: %w", leagueID, err)
	}
	return league, true, nil
}

// SaveLeague upserts a league's full cached shape as a jsonb blob. Best-effort write-through so
// later reads (grading, the per-setter compare, the bookmark list) skip Sleeper's /league
// endpoint. Storing the whole provider.League means new Sleeper fields need no schema change.
func (s *Store) SaveLeague(ctx context.Context, league provider.League) error {
	data, err := json.Marshal(league)
	if err != nil {
		return fmt.Errorf("marshaling league %s: %w", league.LeagueID, err)
	}
	_, err = s.pool.Exec(ctx, `
		INSERT INTO leagues (league_id, data, fetched_at)
		VALUES ($1, $2, now())
		ON CONFLICT (league_id) DO UPDATE SET data = EXCLUDED.data, fetched_at = now()`,
		league.LeagueID, data)
	if err != nil {
		return fmt.Errorf("saving league %s: %w", league.LeagueID, err)
	}
	return nil
}

// resolvePlayers turns stored player IDs back into Player objects, reconstructing the CDN
// image URL exactly as the Sleeper client does. Unknown IDs are dropped (matching the client).
func resolvePlayers(playerMap map[string]provider.Player, ids []string) []provider.Player {
	players := []provider.Player{}
	for _, id := range ids {
		if player, ok := playerMap[id]; ok {
			player.ImageURL = fmt.Sprintf("https://sleepercdn.com/content/nfl/players/thumb/%s.jpg", player.PlayerID)
			players = append(players, player)
		}
	}
	return players
}

func (s *Store) UpdateLineup(ctx context.Context, id string, starters []string) (provider.Lineup, error) {
	row := s.pool.QueryRow(ctx, `
		UPDATE lineups
		SET starters = $2, updated_at = now()
		WHERE id = $1
		RETURNING id, user_id, league_id, source, roster_id, week_number, season, starters, created_at, updated_at
	`, id, starters)
	l, err := scanLineup(row)
	if err != nil {
		return provider.Lineup{}, fmt.Errorf("updating lineup %s: %w", id, err)
	}
	return l, nil
}

func scanUserLeague(row scanner) (provider.UserLeague, error) {
	var ul provider.UserLeague
	err := row.Scan(&ul.UserID, &ul.LeagueID, &ul.Label, &ul.Source, &ul.CreatedAt, &ul.UpdatedAt)
	return ul, err
}

func (s *Store) SaveUserLeague(ctx context.Context, userID, leagueID, source, label string) (provider.UserLeague, error) {
	row := s.pool.QueryRow(ctx, `
		INSERT INTO league_bookmarks (user_id, league_id, source, label)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (user_id, league_id, source) DO UPDATE SET label = EXCLUDED.label, updated_at = now()
		RETURNING user_id, league_id, label, source, created_at, updated_at
	`, userID, leagueID, source, label)
	ul, err := scanUserLeague(row)
	if err != nil {
		return provider.UserLeague{}, fmt.Errorf("saving user league: %w", err)
	}
	return ul, nil
}

func (s *Store) ListUserLeagues(ctx context.Context, userID string) ([]provider.UserLeague, error) {
	// LEFT JOIN the cached league so we can serve the real league avatar (when known) as the
	// bookmark icon. IconURL stays empty for leagues not yet in the cache; the handler then
	// falls back to the generic per-source icon.
	rows, err := s.pool.Query(ctx, `
		SELECT b.user_id, b.league_id, b.label, b.source, b.created_at, b.updated_at,
		       COALESCE(l.data->>'avatar_url', '')
		FROM league_bookmarks b
		LEFT JOIN leagues l ON l.league_id = b.league_id
		WHERE b.user_id = $1
		ORDER BY b.created_at DESC
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("listing user leagues: %w", err)
	}
	defer rows.Close()

	leagues := []provider.UserLeague{}
	for rows.Next() {
		var ul provider.UserLeague
		if err := rows.Scan(&ul.UserID, &ul.LeagueID, &ul.Label, &ul.Source,
			&ul.CreatedAt, &ul.UpdatedAt, &ul.IconURL); err != nil {
			return nil, fmt.Errorf("scanning user league: %w", err)
		}
		leagues = append(leagues, ul)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating user leagues: %w", err)
	}
	return leagues, nil
}

func (s *Store) UpdateUserLeague(ctx context.Context, userID, leagueID, source, label string) (provider.UserLeague, error) {
	row := s.pool.QueryRow(ctx, `
		UPDATE league_bookmarks
		SET label = $4, updated_at = now()
		WHERE user_id = $1 AND league_id = $2 AND source = $3
		RETURNING user_id, league_id, label, source, created_at, updated_at
	`, userID, leagueID, source, label)
	ul, err := scanUserLeague(row)
	if err != nil {
		return provider.UserLeague{}, fmt.Errorf("updating user league: %w", err)
	}
	return ul, nil
}

func (s *Store) DeleteUserLeague(ctx context.Context, userID, leagueID, source string) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM league_bookmarks WHERE user_id = $1 AND league_id = $2 AND source = $3`, userID, leagueID, source)
	if err != nil {
		return fmt.Errorf("deleting user league: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func scanAuthUser(row scanner) (provider.AuthUser, error) {
	var user provider.AuthUser
	err := row.Scan(&user.ID, &user.Email, &user.Username, &user.DisplayName)
	return user, err
}

func (s *Store) CreateOrGetOAuthUser(ctx context.Context, oauthProvider, providerID, email, username, displayName string) (provider.AuthUser, error) {
	row := s.pool.QueryRow(ctx, `
		INSERT INTO users (oauth_provider, oauth_id, email, username, display_name)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (oauth_provider, oauth_id) DO UPDATE
			SET email = EXCLUDED.email
		RETURNING id, email, username, display_name
	`, oauthProvider, providerID, email, username, displayName)
	u, err := scanAuthUser(row)
	if err != nil {
		if isUsernameConflict(err) {
			return provider.AuthUser{}, provider.ErrUsernameConflict
		}
		return provider.AuthUser{}, fmt.Errorf("creating or getting oauth user %s/%s: %w", oauthProvider, providerID, err)
	}
	return u, nil
}

// UpdateProfile sets the username (unique) and display name (free-form) for a user, returning the
// updated record. Callers are expected to have validated/normalized both fields first.
func (s *Store) UpdateProfile(ctx context.Context, userID, username, displayName string) (provider.AuthUser, error) {
	row := s.pool.QueryRow(ctx, `
		UPDATE users SET username = $2, display_name = $3
		WHERE id = $1
		RETURNING id, email, username, display_name
	`, userID, username, displayName)
	u, err := scanAuthUser(row)
	if err != nil {
		if isUsernameConflict(err) {
			return provider.AuthUser{}, provider.ErrUsernameConflict
		}
		if errors.Is(err, pgx.ErrNoRows) {
			return provider.AuthUser{}, fmt.Errorf("updating profile: user %s: %w", userID, err)
		}
		return provider.AuthUser{}, fmt.Errorf("updating profile for user %s: %w", userID, err)
	}
	return u, nil
}

// isUsernameConflict reports whether err is a Postgres unique violation on the case-insensitive
// username index (users_username_lower_key).
func isUsernameConflict(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "users_username_lower_key"
}

func (s *Store) MergeAnonymousData(ctx context.Context, anonymousID, userID string) error {
	if anonymousID == userID {
		return nil
	}

	var isRealUser bool
	if err := s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE id = $1)`, anonymousID).Scan(&isRealUser); err != nil {
		return fmt.Errorf("checking anonymous ID: %w", err)
	}
	if isRealUser {
		return nil
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("beginning merge transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	_, err = tx.Exec(ctx, `
		WITH moved AS (
			INSERT INTO league_bookmarks (user_id, league_id, source, label, created_at, updated_at)
			SELECT $1, league_id, source, label, created_at, updated_at
			FROM league_bookmarks WHERE user_id = $2
			ON CONFLICT (user_id, league_id, source) DO NOTHING
		)
		DELETE FROM league_bookmarks WHERE user_id = $2
	`, userID, anonymousID)
	if err != nil {
		return fmt.Errorf("merging bookmarks: %w", err)
	}

	_, err = tx.Exec(ctx, `
		WITH moved AS (
			INSERT INTO lineups (user_id, league_id, source, roster_id, week_number, starters, created_at, updated_at)
			SELECT $1, league_id, source, roster_id, week_number, starters, created_at, updated_at
			FROM lineups WHERE user_id = $2
			ON CONFLICT (user_id, league_id, roster_id, week_number, source) DO NOTHING
		)
		DELETE FROM lineups WHERE user_id = $2
	`, userID, anonymousID)
	if err != nil {
		return fmt.Errorf("merging lineups: %w", err)
	}

	return tx.Commit(ctx)
}

// InsertVisit records one first-party page-view event. Empty UserID, Referrer, and
// Country are stored as NULL so read-time queries can distinguish anonymous visits and
// unknown fields from real values.
func (s *Store) InsertVisit(ctx context.Context, v provider.Visit) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO visits (visitor_id, user_id, path, referrer, country, is_bot)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, v.VisitorID, nullIfEmpty(v.UserID), v.Path, nullIfEmpty(v.Referrer), nullIfEmpty(v.Country), v.IsBot)
	if err != nil {
		return fmt.Errorf("inserting visit: %w", err)
	}
	return nil
}

// nullIfEmpty maps "" to a SQL NULL and any other string to itself.
func nullIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
