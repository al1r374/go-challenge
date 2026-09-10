# How many users? — Estimation Service (ES)

Go implementation of the Estimation Service from the challenge brief: ingest `(user_id, segment)` pairs from a User Segmentation Service (USS) and answer **“how many unique users are currently in this segment?”** with a configurable sliding window (`ES_RETENTION_DAYS`, default 14).

## High-level design

```
USS ──POST /v1/events──► API ──► Estimation Service
                                      │
                     config.Provider ─┤  (exact | approximate per segment)
                                      │
                    ┌─────────────────┴─────────────────┐
                    ▼                                   ▼
            ExactStore (ZSET)                 ApproximateStore (HLL)
                    │                                   │
                    └──────────── Redis ────────────────┘
                         daily time-buckets (14 days)
```

### Package structure

| Package | Responsibility |
|---------|----------------|
| `cmd/server` | Entrypoint: env config, Redis, HTTP server, graceful shutdown |
| `internal/api` | REST handlers (`Add`, `Count`) |
| `internal/service` | Domain routing: pick Exact vs Approximate from config |
| `internal/config` | Env settings (`FromEnv`); `server.go` / `redis.go` / `logging.go` |
| `internal/store` | Redis backends behind `SegmentStore` interfaces |
| `internal/timebucket` | Daily bucket IDs, active retention window, TTL helpers |
| `examples/` | Programmatic `Add` / `Count` usage |

### Main interfaces

```go
// config.Provider
Mode(segment string) Mode

// store.SegmentStore
Add(ctx, userID, segment string) error
Count(ctx, segment string) (int64, error)

// service.Estimation
Add / Count / Mode
```

Implementations are swappable (e.g. another datastore) without changing the API layer.

## Time-buckets (Exact and Approximate)

Time is split into **UTC calendar days**. Each day gets its own Redis key:

| Mode | Key pattern | Redis type |
|------|-------------|------------|
| Exact | `es:exact:{segment}:{YYYY-MM-DD}` | Sorted Set (`ZADD` member=`user_id`) |
| Approximate | `es:approx:{segment}:{YYYY-MM-DD}` | HyperLogLog (`PFADD`) |

- **Add**: write only into **today’s** bucket; set TTL ≈ retentionDays+1 so Redis drops stale keys.
- **Count**: consider only the **last N** bucket keys (`ES_RETENTION_DAYS`, default 14):
  - **Exact**: `ZUNIONSTORE` those keys → `ZCARD` (precise unique users across days).
  - **Approximate**: `PFMERGE` those keys → `PFCOUNT` (~0.81% std. error).
- Buckets older than the retention window are **not** read and eventually expire.

A user seen on day 1 and day 10 still counts as **one** unique user in the window (union / HLL merge).

## Counting modes — trade-offs

| | Exact (Sorted Set) | Approximate (HyperLogLog) |
|--|--------------------|---------------------------|
| Accuracy | Exact | ~0.81% standard error |
| Memory | O(unique users × days) | ~12 KB per bucket (fixed) |
| Count cost | Multi-key union; slower at huge cardinality | Cheap merge of ≤N sketches |
| Best for | Billing, small/medium segments, audits | Millions of users, dashboards, alerts |

**Default**: if a segment is missing from config → **approximate** (`ES_DEFAULT_MODE`).

Modes are set only via `ES_SEGMENTS` at process start (restart to change).

## Configuration (env)

Copy `.env.example` → `.env`. The server loads `.env` automatically at startup
(process env vars already set in the shell still win). **Restart** after editing `.env`.

| Variable | Purpose | Example |
|----------|---------|---------|
| `ES_SEGMENTS` | Segment modes | `sports:approximate,premium_users:exact` |
| `ES_DEFAULT_MODE` | Mode for unknown segments | `approximate` (default) |
| `ES_RETENTION_DAYS` | Sliding window length (days) | `14` (default) |
| `ES_ADDR` | HTTP listen address | `:8080` |
| `ES_HTTP_READ_HEADER_TIMEOUT_SEC` | Read header timeout (seconds) | `5` |
| `ES_HTTP_READ_TIMEOUT_SEC` | Read timeout (seconds) | `15` |
| `ES_HTTP_WRITE_TIMEOUT_SEC` | Write timeout (seconds) | `30` |
| `ES_HTTP_IDLE_TIMEOUT_SEC` | Idle timeout (seconds) | `60` |
| `ES_LOG_LEVEL` | `debug\|info\|warn\|error` | `info` |
| `ES_LOG_FORMAT` | `text\|json` | `text` |
| `REDIS_ADDR` | Redis host:port | `127.0.0.1:6379` |
| `REDIS_PASSWORD` | Redis auth | _(empty)_ |
| `REDIS_DB` | Redis DB index | `0` |

```bash
export ES_SEGMENTS='sports:approximate,premium_users:exact,high_value_buyers:exact,casual_readers:approximate'
export ES_DEFAULT_MODE=approximate
export REDIS_ADDR=127.0.0.1:6379
make run
```

See `.env.example` for a full template.

## REST API

| Method | Path | Purpose |
|--------|------|---------|
| `POST` | `/v1/events` | Ingest `{ "user_id", "segment" }` from USS |
| `GET` | `/v1/segments/{segment}/count` | Unique active users in retention window; response includes `mode` |
| `GET` | `/healthz` | Liveness |

### Usage examples

```bash
# Start Redis + (optionally) the service
docker compose up -d redis

go run ./cmd/server
# uses ES_SEGMENTS / REDIS_ADDR / … from the environment (see .env.example)

# Add users
curl -s -X POST 127.0.0.1:8080/v1/events \
  -H 'Content-Type: application/json' \
  -d '{"user_id":"u104010","segment":"sports"}'

curl -s -X POST 127.0.0.1:8080/v1/events \
  -H 'Content-Type: application/json' \
  -d '{"user_id":"u104011","segment":"sports"}'

# Count
curl -s 127.0.0.1:8080/v1/segments/sports/count
# {"segment":"sports","count":2,"mode":"approximate"}
```

Programmatic usage: `go run ./examples/usage.go` (requires Redis).

### Why REST?

USS can POST JSON with any HTTP client; operators can probe counts with `curl`; no protobuf/codegen gate for a simple ingest + query surface. The domain lives behind `service.Estimation`, so gRPC can be added later without rewriting storage.

## Logging

Logs use stable `event=` names (filterable in Loki/Datadog). Examples:

```text
event=config.loaded segments=4 default_mode=approximate retention_days=14
event=redis.connected addr=127.0.0.1:6379
event=server.listening addr=:8080
event=http.request method=POST path=/v1/events status=202 duration_ms=4 request_id=...
event=es.add component=estimation user_id=u104010 segment=sports mode=approximate
event=redis.pfadd component=store backend=approximate key=es:approx:sports:2026-09-11
event=es.count component=estimation segment=sports count=2
event=es.add_rejected code=invalid_input reason="missing user_id or segment"
event=es.add_failed code=storage_error ...
event=redis.zunionstore_failed segment=premium_users keys=3 ...
```

Constants live in `internal/logging/events.go`. Use `make run LOG_LEVEL=debug` to see Redis key-level events.

## Run tests

```bash
go test ./...
```

Store tests use [miniredis](https://github.com/alicebob/miniredis) (no real Redis required).

## Challenge notes

Original problem statement is preserved below for reference. Document your own AI usage in `ai.md` (see `sample.ai.md`); that file is expected to be written by you, not generated.

---

# How many users? (Go Challenge)

Suppose there is a "User Segmentation Service" (USS) that segments users based on their activities.
For example, if a user visits sports news, USS classifies and tags "sport" to him.
So we have a pair: (the user_id, and the segment) for example, (u104010, "sports").

We want to develop an Estimation Service (ES) that interacts with USS directly.
ES receives the pair from USS as input and stores it.
The responsibility of ES is to answer a simple query: "How many users exist on a specific segment?".
For example, "how many users are in the sports segment?".

![](https://raw.githubusercontent.com/ArmanCreativeSolutions/go-challenge/main/Untitled%20Diagram.drawio.png?raw=true)

The query is simple, but two assumptions may make it a little challenging:
- A specific user remains just two weeks on a segment. After that,
we should not count "u104010" on the sports segment.
- There are millions of users and hundreds of segments. So your solution(s) must be scalable


## Requirements

- Implement a (REST API, RESTful API, soap, Graphql, RPC, gRPC, or whatever protocol you prefer)
interface to receive data (user_id, segment pair) from USS. 
- Implement a method to estimate the number of users in a specific segment. ( `func estimate(segment) -> number of users`)

## Implementation details

Try to write your code as reusable and readable as possible.
Also, don't forget to document your code and clear the reasons for all your decisions in the code.

If your solution is not simple enough for implementing fast, you can just describe it in your documents.

Use any tools that you prefer just explain the reason of choices in your documents.
For example explain why you choose REST API for receiving data.

It is more valuable to us that the project comes with unit tests.

Please fork this repository and add your code to that.
Don't forget that your commits are so important.
So be sure that you're committing your code often with a proper commit message.


## We all use AI. Don't be ashamed

Everyone uses AI for everything. And we expect you to do the same. AI is a big part of our development process, and we care a lot about how you use AI tools during this task.

Add a file named "ai.md" to your project. This file should contain the following information (the more explicit, the better):

1. What AI tools and models did you use?
2. Explain the different stages of software development where you used AI help. Describe how you used your tools at each stage.
3. We want to know how you prompt. So add your prompts to this file too. For each prompt, explain which stage you used it in, how you evaluated the result, and what you did to fix any misbehavior.
4 Explain how you monitor your token usage and what you do to manage it. Link your tools or add your helper prompts.

*There's a sample.ai.md file in the project representing the expected template. Don't forget: "ai.md" is the only file we expect you to write entirely by yourself.*
