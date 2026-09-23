# endpoint-health-monitor

A lightweight Go service that monitors HTTP(S) endpoints and reports their status, response time and TLS certificate expiry.

> **Status: work in progress.** Checking, concurrent rounds and in-memory history work; the scheduler, REST API and dashboard are next (see [Roadmap](#roadmap)).

## Features (so far)

- Checks endpoints concurrently with a configurable number of workers
- Applies a per-request timeout, so a hanging server can't block a round
- Measures response latency
- Reads the TLS certificate expiry date for HTTPS endpoints
- Classifies each check as `up`, `timeout`, `error` (DNS failure, connection refused, TLS problem) or `bad_status` (status code outside 2xx/3xx)
- Keeps the last 20 results per endpoint in memory

## Quick start

Requires Go 1.27 or newer.

```bash
git clone https://github.com/anis-mahsoume/endpoint-health-monitor.git
cd endpoint-health-monitor
go run ./cmd/monitor
```

Example output (`-rounds 2`):

```
round 1 done in 5.001s
round 2 done in 5s

broken       latest=error      latency=5ms      history=[error error]
closed-port  latest=error      latency=1ms      history=[error error]
github       latest=up         latency=22ms     history=[up up]
google       latest=up         latency=58ms     history=[up up]
hang-1       latest=timeout    latency=5s       history=[timeout timeout]
hang-2       latest=timeout    latency=5s       history=[timeout timeout]
notfound     latest=bad_status latency=23ms     history=[bad_status bad_status]
```

The endpoints are currently hardcoded in `cmd/monitor/main.go`.

### Flags

| Flag       | Default | Description                  |
|------------|---------|------------------------------|
| `-workers` | `10`    | number of concurrent checks  |
| `-timeout` | `5s`    | per-check timeout            |
| `-rounds`  | `3`     | number of check rounds to run |

## Testing

```bash
go test ./...
```

## Project structure

```
cmd/monitor/        entry point
internal/checker/   single-endpoint checks and the concurrent worker pool
internal/store/     in-memory endpoint list and result history
```

## Roadmap

- Scheduler to run rounds on a fixed interval
- REST API to add, remove and query endpoints
- Web dashboard
