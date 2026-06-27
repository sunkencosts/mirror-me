-include api/.env
export

.PHONY: db db-stop db-reset migrate-up migrate-down migrate-version migrate-create seed-players seed-dev dump-players test lint dev

db:
	docker compose up -d

db-stop:
	docker compose down

db-reset:
	docker compose down -v
	docker compose up -d
	until docker compose exec db pg_isready -U mirrorleague; do sleep 1; done
	$(MAKE) migrate-up
	$(MAKE) seed-players
	$(MAKE) seed-dev

seed-players:
	psql $(DATABASE_URL) -c "\COPY players FROM STDIN" < api/seeds/players.tsv

# Dev test data: 12 accounts, a real league mirrored over 8 weeks, submitted lineups, and
# graded results derived by the REAL grader (so the weekly-results list and the per-setter
# lineup drill-down are consistent and clickable). Fetches the league/rosters from Sleeper
# once, then grades offline. Run automatically by db-reset; idempotent. Needs seed-players first.
seed-dev:
	cd api && go run ./cmd/seeddev

dump-players:
	psql $(DATABASE_URL) -c "\COPY players TO STDOUT" > api/seeds/players.tsv

migrate-up:
	cd api && go run ./cmd/migrate up

migrate-down:
	cd api && go run ./cmd/migrate down

migrate-version:
	cd api && go run ./cmd/migrate version

migrate-create:
	@if [ -z "$(name)" ]; then echo "usage: make migrate-create name=<migration_name>"; exit 1; fi
	@next=$$(printf '%06d' $$(( $$(ls api/migrations/*.sql 2>/dev/null | wc -l) / 2 + 1 ))); \
	touch api/migrations/$${next}_$(name).up.sql; \
	touch api/migrations/$${next}_$(name).down.sql; \
	echo "created api/migrations/$${next}_$(name).{up,down}.sql"

test:
	cd api && go test ./...

lint:
	cd api && golangci-lint run
	cd web && npm run lint

dev:
	./dev.sh
