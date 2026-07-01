# mirrorleague

[![tests](https://github.com/sunkencosts/mirrorleague/actions/workflows/test.yml/badge.svg)](https://github.com/sunkencosts/mirrorleague/actions/workflows/test.yml)
![Go](https://img.shields.io/badge/Go-1.25-00ADD8?logo=go&logoColor=white&style=flat-square)
![React](https://img.shields.io/badge/React-19-61DAFB?logo=react&logoColor=black&style=flat-square)
![TypeScript](https://img.shields.io/badge/TypeScript-6-3178C6?logo=typescript&logoColor=white&style=flat-square)
![Postgres](https://img.shields.io/badge/Postgres-15-4169E1?logo=postgresql&logoColor=white&style=flat-square)
![Vite](https://img.shields.io/badge/Vite-8-646CFF?logo=vite&logoColor=white&style=flat-square)

**Think you're a better fantasy football manager than the person who actually drafted the team? Prove it.**

mirrorleague lets you pull up any public [Sleeper](https://sleeper.com) fantasy football
league, "mirror" a team in it, and set your *own* starting lineup from that same roster.
Same players, same pool — different brain making the calls. Once the games are scored,
the app compares your lineup against the real manager's and declares a winner.

**🔗 Live:** [mirrorleague.com](https://mirrorleague.com)

---

## What it does

- **Mirror any Sleeper league** — paste a public league, browse its teams and rosters.
- **Set your own lineup** — pick starters from a mirrored roster for a given week.
- **Kickoff lock** — lineups freeze at the first game's kickoff, just like a real league,
  so you can't edit with hindsight.
- **Scored head-to-head** — after games finish, your lineup is scored against the real
  manager's official lineup using Sleeper's per-player points, and a winner is declared.
- **Player rarity + roster power score** — every player gets a rarity tier derived from
  dynasty rankings, which feeds a roster-strength score and S/A/B/C/D tiers.
- **Google sign-in with anonymous-first UX** — you can use the app anonymously, and your
  picks merge into your account the moment you log in.

---

## Tech stack

| Layer | Tech |
|---|---|
| **Backend** | Go 1.25 (net/http standard-library router, no framework) |
| **Frontend** | React 19 · Vite · React Router 7 · TanStack Query · TypeScript |
| **Database** | Postgres 15 |
| **Auth** | Google OAuth → JWT in an httpOnly cookie |
| **External data** | Sleeper API (leagues/rosters/matchups/players), DynastyProcess rankings |
| **Deploy** | Frontend on Cloudflare Pages; Go binary + Postgres self-hosted on a Raspberry Pi behind a Cloudflare Tunnel; CI/CD via a self-hosted GitHub Actions runner |

---

## Engineering highlights

A few things I'd point out in a code review of this repo:

- **Idiomatic Go HTTP service.** Follows the Mat Ryer / Grafana
  [*"How I write HTTP services after 13 years"*](https://grafana.com/blog/2024/02/09/how-i-write-http-services-in-go-after-13-years/)
  pattern end to end: `main()` only calls `run()`; `run()` owns dependency wiring and
  graceful shutdown; `NewServer()` takes deps explicitly; every route lives in one
  `routes.go`; handlers are maker-functions that depend on narrow, per-file interfaces.
  No framework, no global state, no `init()` magic.
- **Real end-to-end tests, no handler mocks.** The test suite boots the *actual* server
  with `go run(...)`, waits on `/healthz`, and fakes the Sleeper API with
  `httptest.NewServer` — so tests exercise real routing, encoding, and DB access. New
  endpoints are built test-first (TDD).
- **Authoritative kickoff lock.** Lineups lock atomically at the week's first-kickoff
  timestamp. The backend write-gate is the source of truth (rejects late edits with
  `409 lineup locked`), and the design fails *open* on missing data rather than silently
  freezing users out.
- **Shared scoring path.** The authenticated `/compare` endpoint and the public `/score`
  endpoint run through the same scoring code, so the two can never drift.
- **Aggressive caching of a read-only upstream.** Sleeper's player map is a ~5MB payload;
  it's synced on an admin endpoint and cached rather than fetched per request, with an
  explicit cache-invalidation hook.
- **Anonymous → account merge.** A client-generated UUID tracks anonymous work, which is
  reassigned to the real user id on first Google login.

For a full, code-derived snapshot of current capabilities and API surface, see
[`OVERVIEW.md`](./OVERVIEW.md).

---

## Running it locally

### Prerequisites

- [Go 1.25+](https://go.dev/dl/)
- [Node 20+](https://nodejs.org/)
- [Docker](https://docs.docker.com/get-docker/) (for local Postgres)

### First-time setup

```bash
cp api/.env.example api/.env   # edit if needed
make db                        # start postgres (creates mirrorleague + mirrorleague_test)
make migrate-up                # apply schema
```

Then sync the player reference data from Sleeper:

```bash
curl -X POST http://localhost:8080/api/admin/sync-players
```

### Daily dev

```bash
./dev.sh   # opens tmux: server (air), web (vite), terminal, claude
```

Or start pieces individually:

```bash
make db                # postgres only
cd api && air          # Go server with hot reload (localhost:8080)
cd web && npm run dev  # Vite frontend (localhost:5173)
```

### Testing

```bash
make test
```

Tests run against `mirrorleague_test` — your dev database is never touched. The suite
seeds reference data (players) once in `TestMain`. Tests that write transactional data
(lineups, picks) truncate their own tables before running.

### Migrations

```bash
make migrate-up                     # apply all pending migrations
make migrate-down                   # roll back one migration
make migrate-version                # show current version
make migrate-create name=add_users  # create new up/down migration files
```

Migration files live in `api/migrations/`. After creating new files, fill in the SQL
then run `make migrate-up`.

### Make targets

| Target | Description |
|--------|-------------|
| `make db` | Start postgres in Docker |
| `make db-stop` | Stop postgres |
| `make db-reset` | Wipe volume and restart (re-runs init.sql) |
| `make migrate-up` | Apply pending migrations |
| `make migrate-down` | Roll back one migration |
| `make migrate-version` | Show current migration version |
| `make migrate-create name=x` | Scaffold new migration files |
| `make test` | Run Go test suite |
| `make lint` | Run `go vet` + `eslint` |
| `make dev` | Start tmux dev session |

---

## Project layout

```
api/            Go backend
  cmd/server/     main → run → NewServer → addRoutes; e2e tests
  internal/       sleeper client, handlers, db, auth, provider models
  migrations/     numbered SQL migrations
web/            React + Vite frontend
OVERVIEW.md     Code-derived snapshot of current capabilities
DEPLOY.md       Production deployment (Raspberry Pi + Cloudflare)
```

---

## Status

Core loop is live: mirror a league → set a lineup → lock at kickoff → compare and declare
a winner, with Google auth and league bookmarks. In progress: a global leaderboard,
per-league stats, and season-long "best setter" manager rankings. See
[`OVERVIEW.md`](./OVERVIEW.md) for the detailed build status.
