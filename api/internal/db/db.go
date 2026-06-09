package db

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sunkencosts/mirrorleague/internal/provider"
)

type scanner interface {
	Scan(dest ...any) error
}

func scanLineup(row scanner) (provider.Lineup, error) {
	var l provider.Lineup
	err := row.Scan(&l.ID, &l.UserID, &l.LeagueID, &l.Source, &l.RosterID, &l.WeekNumber, &l.Starters, &l.CreatedAt, &l.UpdatedAt)
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

func (s *Store) CreateLineup(ctx context.Context, userID, leagueID, source string, rosterID, weekNumber int, starters []string) (provider.Lineup, error) {
	row := s.pool.QueryRow(ctx, `
		INSERT INTO lineups (user_id, league_id, source, roster_id, week_number, starters)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, user_id, league_id, source, roster_id, week_number, starters, created_at, updated_at
	`, userID, leagueID, source, rosterID, weekNumber, starters)
	l, err := scanLineup(row)
	if err != nil {
		return provider.Lineup{}, fmt.Errorf("creating lineup: %w", err)
	}
	return l, nil
}

func (s *Store) GetLineup(ctx context.Context, id string) (provider.Lineup, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, user_id, league_id, source, roster_id, week_number, starters, created_at, updated_at
		FROM lineups WHERE id = $1
	`, id)
	l, err := scanLineup(row)
	if err != nil {
		return provider.Lineup{}, fmt.Errorf("getting lineup %s: %w", id, err)
	}
	return l, nil
}
func (s *Store) ListLineups(ctx context.Context, userID, leagueID string, weekNumber int, rosterID *int) ([]provider.Lineup, error) {
	query := `
		SELECT id, user_id, league_id, source, roster_id, week_number, starters, created_at, updated_at
		FROM lineups
		WHERE user_id = $1 AND league_id = $2 AND week_number = $3`
	args := []any{userID, leagueID, weekNumber}
	if rosterID != nil {
		query += ` AND roster_id = $4`
		args = append(args, *rosterID)
	}

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

func (s *Store) UpdateLineup(ctx context.Context, id string, starters []string) (provider.Lineup, error) {
	row := s.pool.QueryRow(ctx, `
		UPDATE lineups
		SET starters = $2, updated_at = now()
		WHERE id = $1
		RETURNING id, user_id, league_id, source, roster_id, week_number, starters, created_at, updated_at
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
	rows, err := s.pool.Query(ctx, `
		SELECT user_id, league_id, label, source, created_at, updated_at
		FROM league_bookmarks
		WHERE user_id = $1
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("listing user leagues: %w", err)
	}
	defer rows.Close()

	leagues := []provider.UserLeague{}
	for rows.Next() {
		ul, err := scanUserLeague(rows)
		if err != nil {
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
	err := row.Scan(&user.ID, &user.Email, &user.Username)
	return user, err
}

func (s *Store) CreateOrGetOAuthUser(ctx context.Context, oauthProvider, providerID, email, username string) (provider.AuthUser, error) {
	row := s.pool.QueryRow(ctx, `
		INSERT INTO users (oauth_provider, oauth_id, email, username)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (oauth_provider, oauth_id) DO UPDATE
			SET email = EXCLUDED.email
		RETURNING id, email, username
	`, oauthProvider, providerID, email, username)
	u, err := scanAuthUser(row)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "users_username_key" {
			return provider.AuthUser{}, provider.ErrUsernameConflict
		}
		return provider.AuthUser{}, fmt.Errorf("creating or getting oauth user %s/%s: %w", oauthProvider, providerID, err)
	}
	return u, nil
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
