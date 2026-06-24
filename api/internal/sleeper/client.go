package sleeper

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/sunkencosts/mirrorleague/internal/provider"
)

const IconURL = "https://sleepercdn.com/images/sleeperbot_icon_sm.png"

const (
	rosterCacheTTL            = 5 * time.Minute
	matchupCacheTTLHistorical = 24 * time.Hour
	matchupCacheTTLCurrent    = 2 * time.Minute
)

type roster struct {
	RosterID int      `json:"roster_id"`
	OwnerID  string   `json:"owner_id"`
	Players  []string `json:"players"`
	Starters []string `json:"starters"`
	Reserve  []string `json:"reserve"`
	Taxi     []string `json:"taxi"`
}

type leagueUserMetadata struct {
	TeamName string `json:"team_name"`
}

type leagueUser struct {
	UserID   string             `json:"user_id"`
	Metadata leagueUserMetadata `json:"metadata"`
}

type rosterCacheEntry struct {
	rosters   []provider.Roster
	fetchedAt time.Time
}

type matchupCacheEntry struct {
	matchups  []provider.WeekMatchup
	fetchedAt time.Time
}

type matchupEntry struct {
	RosterID      int                `json:"roster_id"`
	MatchupID     int                `json:"matchup_id"`
	Players       []string           `json:"players"`
	Starters      []string           `json:"starters"`
	Points        float64            `json:"points"`
	CustomPoints  *float64           `json:"custom_points"`
	PlayersPoints map[string]float64 `json:"players_points"`
}

type Client struct {
	baseURL    string
	httpClient *http.Client
	players    playerLookup
	// currentWeek resolves the live NFL week at call time (inferred from week_locks, or an
	// override). Used to pick matchup cache TTL: past weeks cache longer than the live week.
	currentWeek func(context.Context) int

	rosterMu    sync.RWMutex
	rosterCache map[string]rosterCacheEntry

	matchupMu    sync.RWMutex
	matchupCache map[string]matchupCacheEntry
}

type playerLookup interface {
	GetPlayersByIDs(ctx context.Context, ids []string) (map[string]provider.Player, error)
}

func New(baseURL string, players playerLookup, currentWeek func(context.Context) int) *Client {
	return &Client{
		baseURL:      baseURL,
		httpClient:   &http.Client{},
		players:      players,
		currentWeek:  currentWeek,
		rosterCache:  make(map[string]rosterCacheEntry),
		matchupCache: make(map[string]matchupCacheEntry),
	}
}

func (c *Client) InvalidateRosters() {
	c.rosterMu.Lock()
	clear(c.rosterCache)
	c.rosterMu.Unlock()
}

func (c *Client) resolvePlayers(playerMap map[string]provider.Player, ids []string) []provider.Player {
	players := []provider.Player{}
	for _, id := range ids {
		if player, ok := playerMap[id]; ok {
			player.ImageURL = fmt.Sprintf("https://sleepercdn.com/content/nfl/players/thumb/%s.jpg", player.PlayerID)
			players = append(players, player)
		}
	}
	return players
}

func collectPlayerIDs(slices ...[]string) []string {
	seen := map[string]struct{}{}
	var ids []string
	for _, s := range slices {
		for _, id := range s {
			if _, ok := seen[id]; !ok {
				seen[id] = struct{}{}
				ids = append(ids, id)
			}
		}
	}
	return ids
}

func (c *Client) getLeagueUsers(ctx context.Context, leagueID string) (map[string]leagueUser, error) {
	url := c.baseURL + "/league/" + leagueID + "/users"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating users request: %w", err)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("getting users for league %s: %w", leagueID, err)
	}
	defer resp.Body.Close()

	var users []leagueUser
	if err := json.NewDecoder(resp.Body).Decode(&users); err != nil {
		return nil, fmt.Errorf("decoding league users: %w", err)
	}

	byID := make(map[string]leagueUser, len(users))
	for _, u := range users {
		byID[u.UserID] = u
	}
	return byID, nil
}
func (c *Client) GetLeague(ctx context.Context, leagueID string) (provider.League, error) {
	url := c.baseURL + "/league/" + leagueID

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return provider.League{}, fmt.Errorf("creating league request: %w", err)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return provider.League{}, fmt.Errorf("getting league for leagueID %s: %w", leagueID, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return provider.League{}, provider.ErrLeagueNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return provider.League{}, fmt.Errorf("sleeper returned status %d for league %s", resp.StatusCode, leagueID)
	}

	var leagueSettings *provider.League
	if err := json.NewDecoder(resp.Body).Decode(&leagueSettings); err != nil {
		return provider.League{}, fmt.Errorf("decoding league: %w", err)
	}
	if leagueSettings == nil || leagueSettings.LeagueID == "" {
		return provider.League{}, provider.ErrLeagueNotFound
	}
	return *leagueSettings, nil
}

func (c *Client) GetRosters(ctx context.Context, leagueID string) ([]provider.Roster, error) {
	c.rosterMu.RLock()
	entry, ok := c.rosterCache[leagueID]
	c.rosterMu.RUnlock()
	if ok && time.Since(entry.fetchedAt) < rosterCacheTTL {
		return entry.rosters, nil
	}

	rosters, err := c.fetchRosters(ctx, leagueID)
	if err != nil {
		return nil, err
	}

	c.rosterMu.Lock()
	c.rosterCache[leagueID] = rosterCacheEntry{rosters: rosters, fetchedAt: time.Now()}
	c.rosterMu.Unlock()

	return rosters, nil
}

func (c *Client) fetchRosters(ctx context.Context, leagueID string) ([]provider.Roster, error) {
	var wg sync.WaitGroup
	var rawRosters []roster
	var usersByID map[string]leagueUser
	var rosterErr, userErr error

	wg.Add(2)
	go func() {
		defer wg.Done()
		url := c.baseURL + "/league/" + leagueID + "/rosters"
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			rosterErr = fmt.Errorf("creating rosters request: %w", err)
			return
		}
		resp, err := c.httpClient.Do(req)
		if err != nil {
			rosterErr = fmt.Errorf("getting rosters for league %s: %w", leagueID, err)
			return
		}
		defer resp.Body.Close()
		if err := json.NewDecoder(resp.Body).Decode(&rawRosters); err != nil {
			rosterErr = fmt.Errorf("decoding rosters: %w", err)
		}
	}()
	go func() {
		defer wg.Done()
		usersByID, userErr = c.getLeagueUsers(ctx, leagueID)
	}()
	wg.Wait()

	if rosterErr != nil {
		return nil, rosterErr
	}
	if userErr != nil {
		return nil, userErr
	}

	var playerSlices [][]string
	for _, r := range rawRosters {
		playerSlices = append(playerSlices, r.Players, r.Starters, r.Reserve, r.Taxi)
	}
	allIDs := collectPlayerIDs(playerSlices...)

	playerMap, err := c.players.GetPlayersByIDs(ctx, allIDs)
	if err != nil {
		return nil, fmt.Errorf("fetching players: %w", err)
	}

	var result []provider.Roster
	for _, r := range rawRosters {
		teamName := ""
		if u, ok := usersByID[r.OwnerID]; ok {
			teamName = u.Metadata.TeamName
		}
		result = append(result, provider.Roster{
			RosterID: r.RosterID,
			OwnerID:  r.OwnerID,
			TeamName: teamName,
			Players:  c.resolvePlayers(playerMap, r.Players),
			Starters: c.resolvePlayers(playerMap, r.Starters),
			Reserve:  c.resolvePlayers(playerMap, r.Reserve),
			Taxi:     c.resolvePlayers(playerMap, r.Taxi),
		})
	}

	return result, nil
}

func (c *Client) GetWeekMatchups(ctx context.Context, leagueID string, week int) ([]provider.WeekMatchup, error) {
	cacheKey := leagueID + "/" + strconv.Itoa(week)
	ttl := matchupCacheTTLCurrent
	if week < c.currentWeek(ctx) {
		ttl = matchupCacheTTLHistorical
	}

	c.matchupMu.RLock()
	entry, ok := c.matchupCache[cacheKey]
	c.matchupMu.RUnlock()
	if ok && time.Since(entry.fetchedAt) < ttl {
		return entry.matchups, nil
	}

	matchups, err := c.fetchWeekMatchups(ctx, leagueID, week)
	if err != nil {
		return nil, err
	}

	c.matchupMu.Lock()
	c.matchupCache[cacheKey] = matchupCacheEntry{matchups: matchups, fetchedAt: time.Now()}
	c.matchupMu.Unlock()

	return matchups, nil
}

func (c *Client) fetchWeekMatchups(ctx context.Context, leagueID string, week int) ([]provider.WeekMatchup, error) {
	var wg sync.WaitGroup
	var rawMatchups []matchupEntry
	var rosters []provider.Roster
	var matchupErr, rosterErr error

	wg.Add(2)
	go func() {
		defer wg.Done()
		url := c.baseURL + "/league/" + leagueID + "/matchups/" + strconv.Itoa(week)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			matchupErr = fmt.Errorf("creating matchups request: %w", err)
			return
		}
		resp, err := c.httpClient.Do(req)
		if err != nil {
			matchupErr = fmt.Errorf("getting matchups for league %s week %d: %w", leagueID, week, err)
			return
		}
		defer resp.Body.Close()
		if err := json.NewDecoder(resp.Body).Decode(&rawMatchups); err != nil {
			matchupErr = fmt.Errorf("decoding matchups: %w", err)
		}
	}()
	go func() {
		defer wg.Done()
		rosters, rosterErr = c.GetRosters(ctx, leagueID)
	}()
	wg.Wait()

	if matchupErr != nil {
		return nil, matchupErr
	}
	if rosterErr != nil {
		return nil, rosterErr
	}

	ownerByRosterID := make(map[int]string, len(rosters))
	teamNameByRosterID := make(map[int]string, len(rosters))
	for _, r := range rosters {
		ownerByRosterID[r.RosterID] = r.OwnerID
		teamNameByRosterID[r.RosterID] = r.TeamName
	}

	var playerSlices [][]string
	for _, m := range rawMatchups {
		playerSlices = append(playerSlices, m.Players, m.Starters)
	}
	allIDs := collectPlayerIDs(playerSlices...)

	playerMap, err := c.players.GetPlayersByIDs(ctx, allIDs)
	if err != nil {
		return nil, fmt.Errorf("fetching players: %w", err)
	}

	var result []provider.WeekMatchup
	for _, m := range rawMatchups {
		result = append(result, provider.WeekMatchup{
			RosterID:     m.RosterID,
			MatchupID:    m.MatchupID,
			OwnerID:      ownerByRosterID[m.RosterID],
			TeamName:     teamNameByRosterID[m.RosterID],
			Points:       m.Points,
			CustomPoints: m.CustomPoints,
			Players:      c.resolvePlayers(playerMap, m.Players),
			Starters:     c.resolvePlayers(playerMap, m.Starters),
			PlayerPoints: m.PlayersPoints,
		})
	}

	return result, nil
}
