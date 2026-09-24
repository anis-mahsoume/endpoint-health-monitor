# endpoint-health-monitor

A lightweight Go service that monitors HTTP(S) endpoints and reports their status, response time and TLS certificate expiry.

> **Status: work in progress.** Checking, scheduling, down detection and the REST API work; the web dashboard is next (see [Roadmap](#roadmap)).

## Features (so far)

- Checks endpoints concurrently with a configurable number of workers
- Runs a check round on a fixed interval until the process is stopped
- Applies a per-request timeout, so a hanging server can't block a round
- Measures response latency
- Reads the TLS certificate expiry date for HTTPS endpoints
- Classifies each check as `up`, `timeout`, `error` (DNS failure, connection refused, TLS problem) or `bad_status` (status code outside 2xx/3xx)
- Keeps the last 20 results per endpoint in memory
- Marks an endpoint `down` after 3 consecutive failures, and records when the failure streak started
- REST API to list statuses, read history, and add or remove endpoints
- Optional API key protection for the write routes
- Graceful shutdown on Ctrl+C

## Quick start

Requires Go 1.27 or newer.

```bash
git clone https://github.com/anis-mahsoume/endpoint-health-monitor.git
cd endpoint-health-monitor
go run ./cmd/monitor
```

The service checks a few built-in endpoints (hardcoded in `cmd/monitor/main.go`) right away, then once per interval, and serves the API on `:8080`:

```bash
curl http://localhost:8080/api/v1/status
```

### Flags

| Flag        | Default | Description                 |
|-------------|---------|-----------------------------|
| `-addr`     | `:8080` | HTTP listen address         |
| `-workers`  | `10`    | number of concurrent checks |
| `-timeout`  | `5s`    | per-check timeout           |
| `-interval` | `30s`   | time between check rounds   |

### Environment variables

| Variable  | Description |
|-----------|-------------|
| `API_KEY` | If set, `POST` and `DELETE` requests must send it in the `X-API-Key` header. If not set, those routes are open and a warning is logged at startup. |

## API

All routes are under `/api/v1`. Errors are returned as `{"error": "..."}`.

| Method   | Route                    | Auth    | Description |
|----------|--------------------------|---------|-------------|
| `GET`    | `/status`                | none    | Current status of every endpoint, sorted by ID |
| `GET`    | `/endpoints/:id/history` | none    | Recent check results for one endpoint, newest first |
| `POST`   | `/endpoints`             | API key | Add an endpoint. Body: `{"id": "...", "url": "https://..."}` |
| `DELETE` | `/endpoints/:id`         | API key | Stop watching an endpoint and drop its history |

Each endpoint in `/status` has one of these statuses:

| Status    | Meaning |
|-----------|---------|
| `pending` | not checked yet |
| `up`      | the latest check succeeded |
| `failing` | failed recently, but fewer than 3 times in a row |
| `down`    | failed 3 or more times in a row; `down_since` is when the streak started |

Example `/status` response (trimmed):

```json
{
  "endpoints": [
    {
      "id": "broken",
      "url": "https://nothing.invalid",
      "status": "down",
      "consecutive_failures": 3,
      "down_since": "2026-09-24T10:00:00Z",
      "last_check": {
        "checked_at": "2026-09-24T10:01:00Z",
        "outcome": "error",
        "latency_ms": 5,
        "error": "..."
      }
    }
  ]
}
```

Adding an endpoint:

```bash
curl -X POST http://localhost:8080/api/v1/endpoints \
  -H "Content-Type: application/json" \
  -H "X-API-Key: $API_KEY" \
  -d '{"id": "example", "url": "https://example.com"}'
```

## Testing

```bash
go test ./...
```

## Project structure

```
cmd/monitor/         entry point: flags, scheduler and HTTP server wiring
internal/api/        Gin router, handlers, middleware and response types
internal/checker/    single-endpoint checks and the concurrent worker pool
internal/scheduler/  runs a check round on a fixed interval
internal/store/      in-memory endpoint list, result history and down detection
```

## Roadmap

- Web dashboard
