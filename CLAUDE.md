# mirrorleague

> **Current-state reference:** see `OVERVIEW.md` for an accurate, code-derived snapshot of
> what the app does today (routes, lock system, scoring, frontend pages, build status).
> Read it before working on a feature. This file (CLAUDE.md) is the source of truth for
> *patterns and conventions*; `OVERVIEW.md` is the source of truth for *current
> capabilities* — keep it updated as features land.

## Project Overview
A fantasy football "mirror" app where users can import a celebrity or public figure's Sleeper fantasy football team and set their own lineup using the same roster — then compare scores to prove they're a better fantasy manager.

**Core concept:** Same players, same pool, different brain making the decisions.

---

## Tech Stack
- **Backend:** Go (primary focus)
- **Database:** (Postgres) — added in Step 2
- **Cache:** In-memory to start, Redis (Upstash) later
- **External APIs:** Sleeper API (free, no auth), Tank01 NFL API via RapidAPI (player stats, added in Step 3)

---

## Current Status

> Full, per-capability build status lives in `OVERVIEW.md`. High-level summary:

- [x] Step 1 — Mirror a league: rosters, league metadata, and week matchups all served
- [~] Step 2 — Reaction layer: not built (no votes/predictions yet)
- [x] Step 3 — User accounts + lineup picks: Google OAuth + JWT cookie, anonymous→account
  merge, persisted lineups, kickoff lock enforcement (`week_locks`)
- [x] Step 4 — Scoring + winner: `.../compare` scores user vs official lineup using
  Sleeper per-player points and declares a winner (Tank01/AI roast still future)
- Frontend: My Leagues, Lineups editor, Teams, Rookie Rankings done; Global Leaderboard,
  League Stats, and Best Setters are still stubs

---

## Architecture

```
/cmd
  /server
    main.go              ← calls run(), nothing else
    server.go            ← run(), NewServer(), corsMiddleware()
    routes.go            ← addRoutes() — single source of truth for all routes
    server_test.go       ← end-to-end tests via go run() + waitForReady

/internal
  /sleeper
    client.go            ← all Sleeper API calls (GetRosters, GetLeague, etc.)
    cache.go             ← player map caching (once/day, 5MB, saved to players.json)
  /handlers
    rosters.go           ← HandleGetRosters handler maker function
    league.go            ← HandleGetLeague handler maker function
    encode.go            ← generic encode[T] helper
  /provider
    provider.go          ← shared models (Player, Roster, League)

/pkg
  /config
    config.go            ← env vars and configuration
```

---

## Sleeper API — Step 1 Endpoints

All calls are read-only, no auth token required.
Base URL: `https://api.sleeper.app/v1`

| Step | Endpoint | Purpose |
|------|----------|---------|
| 1 | `GET /user/<username>` | Resolve username → user_id |
| 2 | `GET /user/<user_id>/leagues/nfl/2025` | Get all leagues for user |
| 3 | `GET /league/<league_id>/rosters` | Get rosters (player IDs) |
| 4 | `GET /league/<league_id>/users` | Get owners of each roster |
| 5 | `GET /players/nfl` | Full player ID → name map (cache, once/day) |
| 6 | `GET /league/<league_id>/matchups/<week>` | Get lineups + scores for a week |

**Rate limit:** Stay under 1,000 API calls/minute or risk IP block.
**Player map:** 5MB response — save to memory/disk, do not call repeatedly.

---

## Core Go Patterns to Follow

> These patterns follow https://grafana.com/blog/how-i-write-http-services-in-go-after-13-years/
> Do not deviate from these without a strong reason.

### Server Structure

`main()` does nothing except call `run()`:
```go
func main() {
    ctx := context.Background()
    if err := run(ctx, os.Getenv, os.Stdout, os.Stderr); err != nil {
        fmt.Fprintf(os.Stderr, "%s\n", err)
        os.Exit(1)
    }
}
```

`run()` owns startup, dependency wiring, and graceful shutdown:
```go
func run(ctx context.Context, getenv func(string) string, stdout, stderr io.Writer) error {
    ctx, cancel := signal.NotifyContext(ctx, os.Interrupt)
    defer cancel()
    // wire deps, call NewServer, start httpServer, WaitGroup shutdown
}
```

`NewServer()` takes all dependencies explicitly and returns `http.Handler`:
```go
func NewServer(dep1 SomeType, dep2 AnotherType) http.Handler {
    mux := http.NewServeMux()
    addRoutes(mux, dep1, dep2)
    var handler http.Handler = mux
    handler = someMiddleware(handler)
    return handler
}
```

### Routes

All routes live in `cmd/server/routes.go` — this is the single place to see the full API surface:
```go
func addRoutes(mux *http.ServeMux, dep1 SomeType) {
    mux.Handle("GET /api/something", handlers.HandleSomething(dep1))
    mux.HandleFunc("GET /healthz", handleHealthz())
    mux.Handle("/", spaHandler("web/dist"))
}
```

### Handler Maker Functions

Each handler is a function that takes its dependencies and returns `http.Handler`. One file per handler group in `internal/handlers/`. Each file defines its own narrow interface so it only depends on what it uses:
```go
type somethingProvider interface {
    GetSomething(id string) (Something, error)
}

func HandleGetSomething(p somethingProvider) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        id := r.PathValue("id")
        thing, err := p.GetSomething(id)
        if err != nil {
            http.Error(w, err.Error(), http.StatusInternalServerError)
            return
        }
        encode(w, r, http.StatusOK, thing)
    })
}
```

### encode Helper

Use `encode[T]` from `internal/handlers/encode.go` for all JSON responses — never write `json.NewEncoder` directly in a handler:
```go
encode(w, r, http.StatusOK, result)
```

### sync.Once for Lazy Initialisation

Use `sync.Once` inside `NewServer` for any expensive initialisation (e.g. the player cache) that should defer until the first request rather than blocking startup:
```go
var (
    once    sync.Once
    initErr error
)
core := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    once.Do(func() { initErr = expensiveInit() })
    if initErr != nil {
        http.Error(w, "initialisation failed", http.StatusServiceUnavailable)
        return
    }
    mux.ServeHTTP(w, r)
})
```

### Testing

Tests call `go run(ctx, getenv, ...)` to spin up the real server, then use `waitForReady` to poll `/healthz` before making assertions. No mocking handler interfaces — the fake external API is an `httptest.NewServer`:
```go
func newTestServer(t *testing.T, externalAPIHandler http.Handler) string {
    fakeExternal := httptest.NewServer(externalAPIHandler)
    t.Cleanup(fakeExternal.Close)

    port := freePort(t)
    getenv := func(key string) string {
        switch key {
        case "PORT": return port
        case "EXTERNAL_BASE_URL": return fakeExternal.URL
        }
        return ""
    }
    ctx, cancel := context.WithCancel(context.Background())
    t.Cleanup(cancel)
    go run(ctx, getenv, io.Discard, io.Discard)

    baseURL := "http://localhost:" + port
    if err := waitForReady(ctx, 5*time.Second, baseURL+"/healthz"); err != nil {
        t.Fatalf("server never became ready: %v", err)
    }
    return baseURL
}
```

**TDD rule:** All new endpoints require TDD — write the failing tests first, then implement until they pass.

**`TestMain` / `newTestServer` must TRUNCATE every table** that tests write to. Current list: `lineups`, `players`, `league_bookmarks`, `week_locks`. Add new tables here when you add them.

**New-endpoint checklist** — follow this sequence every time:
1. Migration (`api/migrations/000001_init.up.sql`)
2. Provider model (`internal/provider/provider.go`)
3. DB methods (`internal/db/db.go`)
4. Handler file (`internal/handlers/<name>.go`)
5. Route (`cmd/server/routes.go`)
6. Tests (`cmd/server/server_test.go`) — written first, before step 3

---

### Key Models (internal/provider/provider.go)
```go
type Player struct {
    PlayerID         string   `json:"player_id"`
    FirstName        string   `json:"first_name"`
    LastName         string   `json:"last_name"`
    Number           int      `json:"number"`
    Age              int      `json:"age"`
    Team             string   `json:"team"`
    Active           bool     `json:"active"`
    FantasyPositions []string `json:"fantasy_positions"`
    ImageURL         string   `json:"image_url"`  // constructed from sleepercdn.com
}

type Roster struct {
    RosterID int      `json:"roster_id"`
    OwnerID  string   `json:"owner_id"`
    TeamName string   `json:"team_name"`  // from /league/:id/users metadata
    Players  []Player `json:"players"`
    Starters []Player `json:"starters"`
}
```

### Player Image URLs
Sleeper doesn't return image URLs directly. Construct them from the CDN:
```
https://sleepercdn.com/content/nfl/players/thumb/<player_id>.jpg
```

### Parallel API Calls Pattern
```go
var wg sync.WaitGroup
var rosters []Roster
var users []LeagueUser
var rosterErr, userErr error

wg.Add(2)
go func() {
    defer wg.Done()
    rosters, rosterErr = client.GetRosters(leagueID)
}()
go func() {
    defer wg.Done()
    users, userErr = client.GetUsers(leagueID)
}()
wg.Wait()
```

### Error Wrapping Pattern
```go
user, err := client.GetUser(username)
if err != nil {
    return nil, fmt.Errorf("fetching user %s: %w", username, err)
}
```

---

## API Endpoints to Build (Step 1)

```
GET /user/:username          → returns user object + their leagues
GET /league/:leagueId        → returns league info + all rosters with owner names
GET /league/:leagueId/week/:week → returns each team's lineup for that week
```

**Done when:** `curl localhost:8080/user/someSleeperUsername` returns clean JSON with their leagues and roster.

---

## Key Constraints and Notes

- **Sleeper is read-only** — you cannot modify any league data through the API
- **No official terms of service** for the Sleeper API — be a good citizen, cache aggressively, attribute Sleeper in the UI
- **Player stats are NOT in Sleeper API** — Sleeper licenses that from Genius Sports (official NFL data partner). Tank01 on RapidAPI is the indie developer solution for Step 3.
- **Scoring settings live in the League object** — pull them alongside the roster so you can apply correct PPR/half-PPR/standard scoring later
- **Sleeper username vs user_id** — usernames can change, always store and reference by user_id

---

## Future Steps (Not Building Yet)

**Step 2 — Reaction layer**
- Users vote on trades, sit/start decisions
- Anonymous sessions first, no auth required yet
- Store reactions in Supabase

**Step 3 — User accounts + lineup picks**
- Supabase Auth
- Users set their own lineup from the celebrity's roster before Sunday 1pm ET
- Picks locked at kickoff

**Step 4 — Scoring + winner declaration**
- Integrate Tank01 NFL API for individual player stats
- Apply league scoring settings to both lineups
- Generate shareable result card

**Future — Celebrity challenge layer**
- Celebrity shares their Sleeper username publicly
- Fans mirror their team
- Season-long leaderboard of who manages the roster best
- AI-generated weekly roast of bad decisions (Claude API, Haiku model)

---

## Environment Variables
```
PORT=8080
SLEEPER_BASE_URL=https://api.sleeper.app/v1
# Later:
# DATABASE_URL=
# DATABASE_KEY=
# RAPIDAPI_KEY=       (Tank01)
# ANTHROPIC_API_KEY=  (Claude AI features)
```
