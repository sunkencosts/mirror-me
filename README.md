# mirrorleague

Fantasy football mirror app — import a celebrity's Sleeper team and prove you manage it better.

## Prerequisites

- [Go 1.22+](https://go.dev/dl/)
- [Node 20+](https://nodejs.org/)
- [Docker](https://docs.docker.com/get-docker/)

## First-time setup

```bash
cp api/.env.example api/.env   # edit if needed
make db                        # start postgres (creates mirrorleague + mirrorleague_test)
make migrate-up                # apply schema
```

Then sync the player reference data from Sleeper:

```bash
curl -X POST http://localhost:8080/api/admin/sync-players
```

## Daily dev

```bash
./dev.sh   # opens tmux: server (air), web (vite), terminal, claude
```

Or start pieces individually:

```bash
make db              # postgres only
cd api && air        # Go server with hot reload (localhost:8080)
cd web && npm run dev  # Vite frontend (localhost:5173)
```

## Testing

```bash
make test
```

Tests run against `mirrorleague_test` — your dev database is never touched. The test suite seeds reference data (players) once in `TestMain`. Tests that write transactional data (lineups, picks) truncate their own tables before running.

## Migrations

```bash
make migrate-up                    # apply all pending migrations
make migrate-down                  # roll back one migration
make migrate-version               # show current version
make migrate-create name=add_users # create new up/down migration files
```

Migration files live in `api/migrations/`. After creating new files, fill in the SQL then run `make migrate-up`.

## Make targets

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
