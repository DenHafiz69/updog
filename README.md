# Updog

Ultra-lightweight Go + SQLite uptime and telemetry monitor (single binary, embedded web UI, zero Docker/Postgres/Node required).

## Features

- **Single Binary + Pure-Go SQLite (WAL mode):** Zero external database or runtime dependencies (`CGO_ENABLED=0`).
- **Active URL Ping (Pull):** Scheduled HTTP/HTTPS checks with latency tracking, status validation, and optional keyword matching.
- **Push Heartbeat + Telemetry (Push):** Drop-in scripts in your app report liveness plus CPU %, RAM %, Disk %, Load, Uptime, and App Version. Missed heartbeats automatically trigger `DOWN` status.
- **Telemetry Graphs & Threshold Alerts:** Built-in SVG charts and pre-crash threshold alerts (CPU / RAM / Disk).
- **Copy-Paste Tracker Generator:** Generates pre-filled Bash/Cron, Node.js, Python, and Go tracker scripts directly from the UI.
- **Smart Alerts:** Discord, Slack, Telegram, and generic Webhooks with per-channel cooldowns and automatic recovery alerts.
- **Real-Time Dashboard:** Embedded HTML/CSS/JS UI powered by Server-Sent Events (SSE).

## Quick Start

```bash
make build
./bin/updog
# Open http://localhost:8080
```

### Configuration (flags or env vars)

| Flag | Env Var | Default | Description |
|---|---|---|---|
| `-addr` | `UPDOG_ADDR` | `:8080` | HTTP listen address (use `127.0.0.1:8080` behind Cloudflare Tunnel) |
| `-db` | `UPDOG_DB` | `updog.db` | SQLite database file path |
| `-retention` | `UPDOG_RETENTION_DAYS` | `30` | Days of check history to retain |
| `-password` | `UPDOG_PASSWORD` | *(empty)* | Admin password (any length; enables login gate & API auth) |

### Hosting Behind Cloudflare Tunnel

```bash
UPDOG_ADDR=127.0.0.1:8080 UPDOG_PASSWORD="your-very-long-password-here" ./bin/updog
```

- Dashboard & management APIs require password login (`HttpOnly` cookie, or `Authorization: Bearer <password>`).
- `/api/v1/heartbeat/<PUSH_TOKEN>` stays open for your remote apps/scripts (authenticated by each monitor's 128-bit push token and rate-limited).
- Alert webhook URLs / bot tokens are masked in API responses.

## Push Heartbeat API

```bash
curl -X POST http://localhost:8080/api/v1/heartbeat/<PUSH_TOKEN> \
  -H "Content-Type: application/json" \
  -d '{"cpu_pct": 24.5, "ram_pct": 61.2, "disk_pct": 48.0, "version": "1.2.0"}'
```

A simple `GET /api/v1/heartbeat/<PUSH_TOKEN>` also works for minimal cron pings.
