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
GOOGLE_CLIENT_ID=<from Google Cloud Console>
GOOGLE_CLIENT_SECRET=<from Google Cloud Console>
GOOGLE_REDIRECT_URL=https://api.mirrorleague.com/auth/google/callback
FRONTEND_URL=https://mirrorleague.com
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
curl -X POST https://api.mirrorleague.com/admin/sync-players
```

### Gotchas discovered
- Pi Postgres runs on port **5432**. Local Docker dev uses **5433**. Don't mix these up in DATABASE_URL.
- The `migrate` binary reads `MIGRATIONS_URL` from env — it doesn't find migrations automatically. Always source `.env` before running it manually.
- `server.go` runs migrations on startup automatically (`m.Up()` lines 71-77), so CI/CD just needs to restart the service — no separate migrate step needed.
- The default credentials in `config.go` (`mirrorleague/mirrorleague`) are only fallbacks when `DATABASE_URL` isn't set. Production always uses the `.env` value.

---

## Phase 2 — Cloudflare Tunnel (TODO)

This makes `api.mirrorleague.com` route to the Go binary on the Pi. Uses a **remotely-managed tunnel** — routes are configured in the Cloudflare dashboard, not a local config file. This means routes can be changed without SSH-ing into the Pi.

### Install cloudflared on the Pi
```bash
curl -L https://pkg.cloudflare.com/cloudflare-main.gpg | sudo tee /usr/share/keyrings/cloudflare-main.gpg >/dev/null
echo 'deb [signed-by=/usr/share/keyrings/cloudflare-main.gpg] https://pkg.cloudflare.com/cloudflared bookworm main' | sudo tee /etc/apt/sources.list.d/cloudflared.list
sudo apt update && sudo apt install cloudflared
```

### Create the tunnel in the Cloudflare dashboard
1. Go to **Cloudflare Dashboard → Zero Trust → Networks → Tunnels**
2. Click **Create a tunnel** → choose **Cloudflared**
3. Name it `mirrorleague`
4. Follow the install instructions shown — Cloudflare will give you a `cloudflared service install <token>` command to run on the Pi. This installs the daemon as a systemd service automatically.
5. Under **Public Hostnames**, add:
   - **Subdomain:** `api`
   - **Domain:** `mirrorleague.com`
   - **Service:** `http://localhost:8080`
6. Save

The tunnel daemon runs as a systemd service (`cloudflared.service`) — starts on boot, restarts on crash, same as `mirrorleague.service`.

### Verify
```bash
curl https://api.mirrorleague.com/healthz
```
Should return `200`.

---

## Phase 3 — Cloudflare Pages (TODO)

1. Go to **Cloudflare Dashboard → Pages → Create project → Connect to GitHub**
2. Select the `mirrorleague` repo
3. Build settings:
   - **Framework preset:** None
   - **Build command:** `cd web && npm ci && npm run build`
   - **Build output directory:** `web/dist`
4. Add environment variable: `VITE_API_URL=https://api.mirrorleague.com`
5. Set custom domain: `mirrorleague.com`

Cloudflare Pages auto-deploys on every push to `main`. Preview deployments are created for every PR automatically.

---

## Phase 4 — Auth Wiring (TODO)

In **Google Cloud Console → APIs & Services → Credentials → OAuth 2.0 Client**:
- Add `https://api.mirrorleague.com/api/auth/google/callback` as an authorized redirect URI
- `http://localhost:8080/api/auth/google/callback` should already be there for local dev

---

## Phase 5 — CI/CD (TODO)

Install the self-hosted GitHub Actions runner on the Pi:
1. Go to the repo on GitHub → **Settings → Actions → Runners → New self-hosted runner**
2. Select **Linux / ARM64**
3. Follow the install instructions shown (downloads a tarball, runs a config script)
4. Install as a service: `sudo ./svc.sh install && sudo ./svc.sh start`

Allow the runner to restart the service without a password prompt. Add to sudoers:
```bash
sudo visudo
```
Add this line:
```
bpalmer ALL=(ALL) NOPASSWD: /bin/systemctl restart mirrorleague
```

Write `.github/workflows/deploy.yml`:
```yaml
on:
  push:
    branches: [main]

jobs:
  deploy:
    runs-on: self-hosted
    steps:
      - uses: actions/checkout@v4
      - name: Test
        run: cd api && go test ./...
      - name: Build
        run: cd api && go build -o ../server ./cmd/server
      - name: Restart service
        run: sudo systemctl restart mirrorleague
```

Note: the server runs migrations automatically on startup, so no explicit migrate step is needed in the pipeline.

---

## Phase 6 — Smoke Test (TODO)

1. Push a change to `main`, watch the GitHub Actions runner pick it up
2. Confirm the binary restarts: `sudo journalctl -u mirrorleague -f`
3. Hit `https://mirrorleague.com` — React app should load
4. Hit `https://api.mirrorleague.com/healthz` — should return 200
5. Test Google OAuth login end-to-end
