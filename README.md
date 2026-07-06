# Plex Series Scheduler

Lightweight Go service that watches the Plex Live TV guide and schedules future recordings from YAML-defined rules.

Current state:
- Guide discovery is implemented against Plex.
- Scheduled recording discovery is implemented against Plex.
- Recording creation uses Plex's subscription template flow.
- Matching, dedupe, dry-run, metrics, and structured logging are implemented.
- SQLite persistence, Docker packaging, and Kubernetes deployment are not wired in yet.

## Requirements

- Go `1.23.x`
- A Plex Media Server with:
  - Live TV / DVR configured
  - a valid Plex token
  - guide data available

## Configuration

The service reads YAML config from `config.yaml` by default.

Start from `config.example.yaml` and create your own `config.yaml` in the repo root.

Example:

```yaml
plex:
  url: http://your-plex-host:32400
  token: ${PLEX_TOKEN}

scheduler:
  interval: 30m
  guideLookahead: 168h
  maxRecordings: 1
  dryRun: true
  debug: false

rules:
  - name: Formula 1
    enabled: true
    matchMode: sports_event
    titleRegex: "^Formula 1"
    includeKeywords:
      - Grand Prix
      - Practice
      - Qualifying
      - Sprint
      - Sprint Shootout
      - Race
    excludeKeywords:
      - Replay
      - Repeat
      - Highlights
    channels:
      - Sky Sports F1 HD
      - Sky Sports F1
    preferFirstMatch: true
    dedupeWindow: 72h
    paddingBefore: 10m
    paddingAfter: 45m
```

Notes:
- `${PLEX_TOKEN}` is expanded from your shell environment.
- `dryRun: true` is the safest first run.
- `guideLookahead: 168h` means 7 days.
- Channel order is used as a final tie-breaker after quality and airtime.
- `scheduler.maxRecordings` limits how many recordings may overlap globally at the same time. `0` means unlimited.

## Getting a Plex Token

If you do not already have a Plex token, get one from an authenticated Plex Web session or a trusted Plex token workflow you already use.

Then export it before running the service:

```bash
export PLEX_TOKEN="your-token-here"
```

## Run Locally

Run a single scheduler pass:

```bash
go run ./cmd/scheduler -config config.yaml -once
```

Run continuously on the configured interval:

```bash
go run ./cmd/scheduler -config config.yaml
```

Change the metrics bind address:

```bash
go run ./cmd/scheduler -config config.yaml -metrics-addr :9464
```

Disable the metrics server entirely:

```bash
go run ./cmd/scheduler -config config.yaml -metrics-addr ""
```

## Build

```bash
go build ./cmd/scheduler
```

This produces a `scheduler` binary in the current directory.

Run it directly:

```bash
./scheduler -config config.yaml -once
```

## What Happens On Startup

Each scheduler run currently does this:

1. Loads and validates `config.yaml`.
2. Discovers the Plex Live TV provider.
3. Reads the configured guide window from Plex.
4. Reads existing scheduled recordings from Plex.
5. Matches programmes against enabled rules.
6. Removes same-event and exact duplicates.
7. Creates missing recordings unless `dryRun: true`.

## Metrics And Health

When metrics are enabled, the service exposes:

- `GET /metrics`
- `GET /health`

Default address:

```text
http://localhost:9464
```

Example:

```bash
curl http://localhost:9464/health
curl http://localhost:9464/metrics
```

## Logs

Logs are written to stdout as JSON using `slog`.

Useful fields include:
- scheduler start/end time
- run duration
- rules loaded
- matches found
- recordings created
- duplicate skips
- Plex API failures

## First Safe Test

Recommended first run:

1. Set `dryRun: true` in `config.yaml`.
2. Export `PLEX_TOKEN`.
3. Run `go run ./cmd/scheduler -config config.yaml -once`.
4. Inspect the JSON logs.
5. Confirm the matched guide entries are the ones you expect.
6. Change `dryRun` to `false` only after that looks correct.

## Current Limitations

- No SQLite persistence is connected yet.
  - Duplicate prevention currently relies on Plex scheduled recordings plus in-run memory.
- No Dockerfile yet.
- No Kubernetes manifests yet.
- No rule CRUD API or UI yet.
- No integration test against a real Plex server yet.

The biggest unknown still to validate on a live server is the exact `POST /media/subscriptions` payload shape for your Plex version and DVR setup. The code uses the safest known pattern: fetch Plex's subscription template first, then replay it with the selected airing and padding.

## Troubleshooting

If the scheduler cannot see the guide:
- confirm Plex Live TV works in Plex Web
- confirm the token belongs to a user with DVR access
- confirm the Plex URL is reachable from where you run the service

If recording creation fails:
- run once with a narrow rule
- inspect the logged error
- compare the request behavior with a manual schedule action in Plex Web

If no programmes match:
- loosen the rule temporarily
- start with `matchMode: exact` or `contains`
- verify channel names exactly as Plex returns them

## Test

```bash
go test ./...
```
