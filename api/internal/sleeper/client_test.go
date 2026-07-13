package sleeper

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sunkencosts/mirrorleague/internal/provider"
)

type mapPlayerLookup struct {
	players map[string]provider.Player
}

func (m *mapPlayerLookup) GetPlayersByIDs(_ context.Context, ids []string) (map[string]provider.Player, error) {
	result := map[string]provider.Player{}
	for _, id := range ids {
		if p, ok := m.players[id]; ok {
			result[id] = p
		}
	}
	return result, nil
}

func TestGetRosters_teamName(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/league/abc/rosters", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]roster{
			{RosterID: 1, OwnerID: "user1", Players: []string{"111"}, Starters: []string{"111"}},
		})
	})
	mux.HandleFunc("/league/abc/users", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]leagueUser{
			{UserID: "user1", Metadata: leagueUserMetadata{TeamName: "Mahomes Enjoyers"}},
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	lookup := &mapPlayerLookup{players: map[string]provider.Player{"111": {PlayerID: "111"}}}
	c := New(srv.URL, lookup, func(context.Context) int { return 1 })

	rosters, err := c.GetRosters(context.Background(), "abc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rosters) != 1 {
		t.Fatalf("expected 1 roster, got %d", len(rosters))
	}
	if rosters[0].TeamName != "Mahomes Enjoyers" {
		t.Errorf("expected team name %q, got %q", "Mahomes Enjoyers", rosters[0].TeamName)
	}
}

func TestGetRosters_RostersNonOKStatus(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/league/abc/rosters", func(w http.ResponseWriter, r *http.Request) {
		// Sleeper outages/rate-limits commonly return a non-200 with a null or
		// empty body, not an unparseable one — decoding it succeeds, so only an
		// explicit status check catches this.
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode([]roster{})
	})
	mux.HandleFunc("/league/abc/users", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]leagueUser{})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	lookup := &mapPlayerLookup{}
	c := New(srv.URL, lookup, func(context.Context) int { return 1 })

	rosters, err := c.GetRosters(context.Background(), "abc")
	if err == nil {
		t.Fatal("expected an error for a non-200 rosters response, got nil")
	}
	if rosters != nil {
		t.Errorf("expected nil rosters on error, got %v", rosters)
	}
	if _, cached := c.rosterCache["abc"]; cached {
		t.Error("expected a failed roster fetch to NOT be cached")
	}
}

func TestGetRosters_UsersNonOKStatus(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/league/abc/rosters", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]roster{
			{RosterID: 1, OwnerID: "user1", Players: []string{"111"}, Starters: []string{"111"}},
		})
	})
	mux.HandleFunc("/league/abc/users", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		json.NewEncoder(w).Encode([]leagueUser{})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	lookup := &mapPlayerLookup{players: map[string]provider.Player{"111": {PlayerID: "111"}}}
	c := New(srv.URL, lookup, func(context.Context) int { return 1 })

	rosters, err := c.GetRosters(context.Background(), "abc")
	if err == nil {
		t.Fatal("expected an error for a non-200 users response, got nil")
	}
	if rosters != nil {
		t.Errorf("expected nil rosters on error, got %v", rosters)
	}
	if _, cached := c.rosterCache["abc"]; cached {
		t.Error("expected a failed roster fetch to NOT be cached")
	}
}

func TestGetRosters_ZeroRowsNotCached(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/league/abc/rosters", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]roster{})
	})
	mux.HandleFunc("/league/abc/users", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]leagueUser{})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	lookup := &mapPlayerLookup{}
	c := New(srv.URL, lookup, func(context.Context) int { return 1 })

	rosters, err := c.GetRosters(context.Background(), "abc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rosters) != 0 {
		t.Errorf("expected 0 rosters, got %d", len(rosters))
	}
	if _, cached := c.rosterCache["abc"]; cached {
		t.Error("expected a zero-row roster fetch to NOT be cached")
	}
}

func TestGetWeekMatchups_NonOKStatus(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/league/abc/matchups/8", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode([]matchupEntry{})
	})
	mux.HandleFunc("/league/abc/rosters", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]roster{})
	})
	mux.HandleFunc("/league/abc/users", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]leagueUser{})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	lookup := &mapPlayerLookup{}
	c := New(srv.URL, lookup, func(context.Context) int { return 1 })

	matchups, err := c.GetWeekMatchups(context.Background(), "abc", 8)
	if err == nil {
		t.Fatal("expected an error for a non-200 matchups response, got nil")
	}
	if matchups != nil {
		t.Errorf("expected nil matchups on error, got %v", matchups)
	}
	if _, cached := c.matchupCache["abc/8"]; cached {
		t.Error("expected a failed matchup fetch to NOT be cached")
	}
}

func TestGetWeekMatchups_ZeroRowsNotCached(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/league/abc/matchups/8", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]matchupEntry{})
	})
	mux.HandleFunc("/league/abc/rosters", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]roster{})
	})
	mux.HandleFunc("/league/abc/users", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]leagueUser{})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	lookup := &mapPlayerLookup{}
	c := New(srv.URL, lookup, func(context.Context) int { return 1 })

	matchups, err := c.GetWeekMatchups(context.Background(), "abc", 8)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(matchups) != 0 {
		t.Errorf("expected 0 matchups, got %d", len(matchups))
	}
	if _, cached := c.matchupCache["abc/8"]; cached {
		t.Error("expected a zero-row matchup fetch to NOT be cached")
	}
}

func TestNew_SetsHTTPClientTimeout(t *testing.T) {
	c := New("http://example.invalid", &mapPlayerLookup{}, func(context.Context) int { return 1 })
	if c.httpClient.Timeout <= 0 {
		t.Fatalf("expected a positive http client timeout, got %v", c.httpClient.Timeout)
	}
}

func TestResolvePlayers(t *testing.T) {
	playerMap := map[string]provider.Player{
		"111": {PlayerID: "111", FirstName: "Patrick", LastName: "Mahomes"},
		"222": {PlayerID: "222", FirstName: "Travis", LastName: "Kelce"},
	}
	c := &Client{players: &mapPlayerLookup{players: playerMap}}

	got := c.resolvePlayers(playerMap, []string{"111", "222", "999"})

	if len(got) != 2 {
		t.Fatalf("expected 2 players, got %d", len(got))
	}
	if got[0].PlayerID != "111" {
		t.Errorf("expected player 111, got %s", got[0].PlayerID)
	}
}
