# Deploy Handoff

## Decisions Made

### Architecture
- **Frontend:** Cloudflare Pages → `mirrorleague.com` (auto-deploys from `main`)
- **Backend:** Go binary on Raspberry Pi → `api.mirrorleague.com` via Cloudflare Tunnel
- **Database:** Postgres 15 on the Pi (same machine), tuned with pgtune
- **CI/CD:** Self-hosted GitHub Actions runner on the Pi (polls GitHub, no inbound ports needed)
- **Local dev:** Unchanged — `localhost:8080` + `localhost:5173`. Google OAuth allows `http://localhost` callbacks.

### Why these choices
- Pi is free hardware already owned; Bell residential blocks inbound ports so Cloudflare Tunnel is required
- Cloudflare Tunnel is outbound-only from the Pi — no port forwarding, no static IP needed
- Self-hosted runner solves the CI/CD access problem the same way: Pi reaches out to GitHub, not the other way around
- Bare binary + systemd (not Docker) — Pi 3B has 1GB RAM, Docker overhead not worth it
- Postgres on-Pi is fine with tuned config; 1GB is enough with pgtune settings applied

---

## Pi Details

- **Model:** Raspberry Pi 3B, 1GB RAM, 4-core ARM Cortex-A53
- **Architecture:** aarch64 (64-bit) — compile Go with `GOARCH=arm64`
- **OS:** Raspberry Pi OS 64-bit (Debian 12)
- **Network:** Bell residential, no open inbound ports
- **Go version:** 1.25.1, installed at `/usr/local/go`

---

## Phase 1 — Pi Setup (COMPLETE)

### What was done
1. Postgres 15 installed and running
2. pgtune config applied — see `/etc/postgresql/15/main/conf.d/pgtune.conf`
3. Database `mirrorleague` and user `mirrorleague` created
4. Migrations applied (and will auto-run on server startup — see `api/cmd/server/server.go:71-77`)
5. Go 1.25.1 installed
6. Repo cloned to `/home/bpalmer/apps/mirrorleague/`
7. Binary built at `/home/bpalmer/apps/mirrorleague/server`
8. systemd service enabled and running

### Key file paths on Pi
| File | Path |
|---|---|
| systemd unit | `/etc/systemd/system/mirrorleague.service` |
| binary | `/home/bpalmer/apps/mirrorleague/server` |
| migrate binary | `/home/bpalmer/apps/mirrorleague/migrate` |
| env file | `/home/bpalmer/apps/mirrorleague/.env` |
| repo root | `/home/bpalmer/apps/mirrorleague/` |
| pgtune config | `/etc/postgresql/15/main/conf.d/pgtune.conf` |

### .env template
```
APP_ENV=production
PORT=8080
DATABASE_URL=postgres://mirrorleague:<password>@localhost:5432/mirrorleague
JWT_SECRET=<openssl rand -base64 32>
ADMIN_SECRET=<openssl rand -base64 32>
GOOGLE_CLIENT_ID=<from Google Cloud Console>
GOOGLE_CLIENT_SECRET=<from Google Cloud Console>
GOOGLE_REDIRECT_URL=https://api.mirrorleague.com/auth/google/callback
FRONTEND_URL=https://mirrorleague.com
SLEEPER_BASE_URL=https://api.sleeper.app/v1
MIGRATIONS_URL=file:///home/bpalmer/apps/mirrorleague/api/migrations
```

### Useful commands
```bash
# Restart the server
sudo systemctl restart mirrorleague

# Check service status (all at once)
sudo systemctl status mirrorleague cloudflared actions.runner*

# Tail logs live (stays open, Ctrl+C to exit)
sudo journalctl -u mirrorleague -f

# View last N lines without following
sudo journalctl -u mirrorleague -n 100

# Resource usage (interactive, F4 to filter by name)
htop

# Quick memory/CPU snapshot
ps aux --sort=-%mem | head -20

# Source .env manually (for running binaries outside systemd)
set -a && source ~/apps/mirrorleague/.env && set +a

# Sync players (run after first deploy or if players table is empty)
curl -X POST -H "Authorization: Bearer <ADMIN_SECRET>" https://api.mirrorleague.com/admin/sync-players
```

### Gotchas discovered
- Pi Postgres runs on port **5432**. Local Docker dev uses **5433**. Don't mix these up in DATABASE_URL.
- The `migrate` binary reads `MIGRATIONS_URL` from env — it doesn't find migrations automatically. Always source `.env` before running it manually.
- `server.go` runs migrations on startup automatically, so CI/CD just needs to restart the service — no separate migrate step needed.
- The default credentials in `config.go` (`mirrorleague/mirrorleague`) are only fallbacks when `DATABASE_URL` isn't set. Production always uses the `.env` value.
- `.env` values must have **no leading whitespace** — even a single space before the value breaks parsing.

---

## Phase 2 — Cloudflare Tunnel (COMPLETE)

Remotely-managed tunnel — routes are configured in the Cloudflare dashboard, not a local config file. Routes can be changed without SSH-ing into the Pi.

`api.mirrorleague.com` → `http://localhost:8080` via `cloudflared.service` on the Pi.

---

## Phase 3 — Cloudflare Pages (COMPLETE)

Frontend auto-deploys from `main`. Build settings:
- **Build command:** `cd web && npm ci && npm run build`
- **Build output directory:** `web/dist`
- **Env var:** `VITE_API_URL=https://api.mirrorleague.com`
- **Custom domain:** `mirrorleague.com`

Preview deployments are created for every PR automatically.

---

## Phase 4 — Auth Wiring (COMPLETE)

In **Google Cloud Console → APIs & Services → Credentials → OAuth 2.0 Client**:
- `https://api.mirrorleague.com/auth/google/callback` — production
- `http://localhost:8080/auth/google/callback` — local dev

---

## Phase 5 — CI/CD (COMPLETE)

Self-hosted GitHub Actions runner installed on the Pi as a systemd service. On every push to `main`:
1. Checks out repo
2. Builds the binary to `~/apps/mirrorleague/server`
3. Restarts the systemd service

No test step in CI — Pi 3B (1GB RAM) runs out of memory compiling and running tests. Run `make test` locally before pushing.

`.github/workflows/deploy.yml`:
```yaml
on:
  push:
    branches: [main]

jobs:
  deploy:
    runs-on: self-hosted
    steps:
      - uses: actions/checkout@v4
      - name: Build
        run: cd api && go build -o ~/apps/mirrorleague/server ./cmd/server
      - name: Restart service
        run: sudo systemctl restart mirrorleague
```

sudoers entry required for the runner to restart the service:
```
bpalmer ALL=(ALL) NOPASSWD: /bin/systemctl restart mirrorleague
```

---

## Phase 6 — Smoke Test (COMPLETE)

- `https://mirrorleague.com` — React app loads
- `https://api.mirrorleague.com/healthz` — returns 200
- Google OAuth login works end-to-end
