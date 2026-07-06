# Plex Series Scheduler - Project Specification

## Objective

Build a lightweight service that adds **Series Link** style recording functionality to Plex Live TV.

The service should automatically schedule future recordings for configured TV programmes, as Plex does not currently provide the level of recurring recording control that is required.

The application should be designed to run as a Docker container and ultimately be deployed into Kubernetes.

---

# High-Level Architecture

```text
                +------------------+
                |   Plex Server    |
                |                  |
                |  Live TV + DVR   |
                +--------+---------+
                         ^
                         |
             Query Guide | Create Recording
                         |
                +--------+---------+
                | Series Scheduler |
                |                  |
                | Rule Engine      |
                | Plex Client      |
                | Scheduler        |
                | SQLite           |
                | Metrics          |
                +--------+---------+
                         |
                  Configuration
```

Dispatcharr is **not** the primary guide source.

Although Dispatcharr supplies the EPG to Plex, the scheduler should retrieve guide information directly from Plex because:

* Plex has already mapped guide data to the configured tuners.
* Plex exposes the programme/airing identifiers required to create recordings.
* Plex is the source of truth for scheduled recordings.
* This avoids maintaining channel mappings between Dispatcharr and Plex.

Dispatcharr XMLTV may be supported in the future as a diagnostic or fallback guide source.

---

# Goals

The scheduler should:

* Automatically schedule future recordings based on configurable rules.
* Prevent duplicate recordings.
* Support recurring programmes.
* Support sports and live event recording.
* Be idempotent (safe to run repeatedly).
* Expose metrics and structured logging.
* Be easily extendable with additional rule types.

---

# Phase 1 - Reverse Engineer Plex

The first task is determining how Plex creates a recording.

Capture the HTTP request made by Plex Web when manually scheduling a recording.

Identify:

* Endpoint
* HTTP method
* Authentication
* Required headers
* Query parameters
* Request body
* DVR identifier
* Programme identifier
* Airing identifier
* Channel identifier
* Recording options
* Padding options
* Existing schedule API

The scheduler should replicate the same API requests rather than inventing its own behaviour.

---

# Phase 2 - Plex Client

Create a reusable Plex client.

Responsibilities:

* Authenticate
* Query DVRs
* Query guide data
* Query scheduled recordings
* Create recordings
* Future support:

  * Delete recordings
  * Cancel scheduled recordings
  * Refresh guide

The Plex client should be isolated behind an interface to allow mocking during testing.

The scheduler should query the Plex guide for the next 7 days, which matches the currently observed Plex TV guide horizon.

This lookahead window should be configurable, but default to 7d.

---

# Configuration

Configuration and recording rules should use YAML.

YAML is the only source of truth for recording rules. SQLite stores scheduler state, deduplication data, ignored items, and execution history only.

Example:

```yaml
plex:
  url: http://plex:32400
  token: ${PLEX_TOKEN}

scheduler:
  interval: 30m
  dryRun: true

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
      - Classic
      - Ted's Notebook
      - The F1 Show
      - Preview
      - Review
      - Documentary

    channels:
      - Sky Sports F1 HD
      - Sky Sports F1

    preferFirstMatch: true

    dedupeWindow: 72h

    paddingBefore: 10m

    paddingAfter: 45m

  - name: Taskmaster
    matchMode: exact
    title: Taskmaster

  - name: Match of the Day
    matchMode: contains
    title: Match of the Day
```

---

# Rule Engine

The rule engine should support multiple matching strategies.

Supported match modes:

* exact
* contains
* regex
* sports_event

Future match modes should be easy to add without changing scheduler logic.

The scheduler should depend only on a generic matcher interface.

---

# Sports Event Matching

Sports programmes often have many repeat broadcasts throughout the week.

Simple title matching is insufficient.

The sports matcher should evaluate:

* Title
* Subtitle
* Episode title
* Description

Rules should support:

* Required keywords
* Excluded keywords
* Channel filtering
* Regex matching
* Preferred first airing
* Deduplication window
* Recording padding

When multiple configured channels match the same event, the scheduler should prefer higher-quality variants such as HD or FHD over SD variants where this can be inferred from channel naming or configured priority.

`preferFirstMatch` means the scheduler should choose the best airing candidate within the deduplication window using the following priority:

1. Higher quality channel variant preferred over lower quality variants.
2. Earlier airing time preferred when quality is equivalent.
3. Configured channel list order used as a final tie-breaker.

Example:

Programme:

```
Formula 1: British Grand Prix - Practice 1
```

Should match.

Programme:

```
Formula 1 Highlights
```

Should not.

Programme:

```
Ted's Notebook
```

Should not.

Programme:

```
Formula 1 Replay
```

Should not.

---

# Matching Pipeline

Each programme should be evaluated in this order:

1. Channel filter
2. Title / Regex
3. Required keywords
4. Excluded keywords
5. Same-event deduplication
6. Exact duplicate / already scheduled check
7. Schedule recording

Only programmes that pass every enabled filter should be recorded.

---

# Scheduler

Every configurable interval:

1. Load and validate configuration from YAML.
2. Query Plex guide.
3. Query existing scheduled recordings.
4. Match programmes against rules.
5. Remove duplicates.
6. Schedule missing recordings.
7. Persist state.
8. Export metrics.
9. Write structured logs.

The scheduler must be idempotent.

Running it every 30 minutes should never create duplicate recordings.

Phase 1 assumes a single active scheduler instance.

The service should run as a singleton workload. If deployed to Kubernetes, it should use a single replica and persistent storage for the SQLite database. Running multiple scheduler instances concurrently is out of scope for the initial milestone.

---

# Duplicate Detection

The scheduler should prevent duplicate scheduling.

The scheduler should prevent duplicate scheduling using two distinct checks:

1. Exact recording deduplication.
2. Same-event deduplication for repeat airings, especially sports.

Fingerprint should include:

* Programme title
* Subtitle
* Episode title
* Channel
* Start time

Before scheduling:

* Check Plex scheduled recordings.
* Check local database.
* Future: check completed recordings.

## Same-Event Deduplication

Some programmes, especially sports events, may air multiple times within a short period as replays or repeats.

Same-event deduplication should identify multiple airings of the same event even when the start time differs.

This comparison should use normalized event metadata rather than start time. Depending on available Plex metadata, this may include:

* Programme title
* Subtitle
* Episode title
* Description-derived identifiers
* Channel-independent event identity

---

# Sports Deduplication

Many sports events are replayed repeatedly.

Example:

```
Friday 13:30
Formula 1: British Grand Prix - Practice 1

Friday 18:00
Formula 1: British Grand Prix - Practice 1 (Replay)

Saturday 03:00
Formula 1: British Grand Prix - Practice 1 (Replay)
```

Expected behaviour:

```
✓ Record Friday 13:30

✗ Ignore Friday 18:00

✗ Ignore Saturday 03:00
```

The scheduler should prefer the best matching airing within the configured deduplication window.

Selection priority:

1. Preferred higher-quality channel variant.
2. Earliest airing time among equally preferred channel options.
3. Configured channel list order as a final tie-breaker.

---

# Local Database

Use SQLite.

Suggested tables:

## scheduled_airings

Every recording created by the scheduler.

## ignored_airings

Items intentionally skipped, including same-event deduplication decisions where useful for diagnostics.

## history

Execution history.

---

# Dry Run Mode

Support:

```yaml
scheduler:
  dryRun: true
```

Output example:

```
Would schedule:

Formula 1
Sunday 14:00
Sky Sports F1 HD

Taskmaster
Friday 21:00
Channel 4 HD
```

No recordings should be created while enabled.

---

# Debug Mode

Provide a debug mode specifically for rule development.

Debug mode is intended for local development and rule authoring.

When enabled, log the complete programme metadata returned by Plex.

This should include every available field, for example:

* Title
* Subtitle
* Description
* Channel
* Channel ID
* Airing ID
* Programme ID
* Genre
* Episode information
* Original air date
* Start time
* End time
* Recording options

This mode exists to make it easy to build accurate matching rules without reverse engineering Plex repeatedly.

This mode should be opt-in only and should not be enabled by default in production due to log volume and potential exposure of internal identifiers.

---

# Time Handling

All timestamps should be normalized and stored in UTC internally.

Guide data returned by Plex should be interpreted using the timezone information provided by Plex or the server environment where necessary. Logging may include both UTC and local time for readability, but scheduling and deduplication logic should use UTC consistently.

---

# Logging

Use structured logging.

Every scheduler run should log:

* Start time
* End time
* Duration
* Programmes processed
* Rules loaded
* Matches found
* Recordings created
* Recordings skipped
* Duplicate detections
* Plex API failures

---

# Metrics

Expose Prometheus metrics.

Suggested metrics:

* scheduler_runs_total
* scheduler_run_duration_seconds
* plex_api_requests_total
* plex_api_failures_total
* recordings_created_total
* recordings_skipped_total
* duplicate_recordings_total
* guide_programmes_processed
* rules_loaded

---

# Notifications (Future)

Support:

* Home Assistant
* Slack
* Discord
* Email

Example notification:

```
Scheduled 2 recordings

Formula 1
Taskmaster
```

---

# REST API (Future)

```
GET    /health
GET    /metrics

GET    /rules
POST   /rules
PUT    /rules/{id}
DELETE /rules/{id}

GET    /scheduled
GET    /history
```

---

# Project Structure

```text
cmd/
    scheduler/

internal/
    plex/
        client.go

    scheduler/
        scheduler.go

    matcher/
        matcher.go

    rules/
        parser.go

    database/
        sqlite.go

    config/

    metrics/

    logging/

pkg/
```

---

# Technology Choices

Language:

* Go

Database:

* SQLite

Configuration:

* YAML

Logging:

* slog

Metrics:

* Prometheus

Deployment:

* Docker
* Kubernetes

---

# Initial Milestone

Success is defined as:

* Successfully authenticating with Plex.
* Reading the Plex TV guide.
* Reading the existing recording schedule.
* Matching a configured rule.
* Successfully scheduling a recording using the same API calls as Plex Web.
* Preventing duplicate scheduling.
* Running repeatedly without creating duplicate recordings.

Once this milestone is complete, future work can include:

* Web UI
* REST API
* Rule editing
* Notifications
* XMLTV fallback support
* Additional matcher types
* Machine-learning-assisted rule suggestions based on guide metadata
