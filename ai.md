# AI Usage Report

> This file documents how AI tools were used throughout this technical challenge.
> Filled out to match `sample.ai.md`. Be honest: most of the implementation was AI-assisted in Cursor.

---

## 1. Tools & Models

| Tool / Product | Model | Purpose | Frequency |
|---|---|---|---|
| Cursor (Agent) | Composer | Architecture, full ES implementation, tests, docs, commits | Continuous for the main build |
| graphify (local CLI) | — | Code knowledge-graph update after implementation | Once after code landed |

No ChatGPT web session, Copilot, or Claude Code CLI were used for this challenge run.

---

## 2. Stages of Development

### Planning / Requirements Analysis
- Used Cursor with the challenge `README.md` plus a detailed product spec (Exact vs Approximate modes, 14-day time-buckets, YAML config, Redis).
- Kept: Redis Sorted Set + HyperLogLog, daily buckets, REST API, interfaces for swappable stores, runtime `SetMode`.
- Did not invent extra product scope (no auth, no multi-region, no billing).

### Scaffolding / Boilerplate
- AI created module layout (`cmd/server`, `internal/{api,config,service,store,timebucket}`), `Dockerfile`, `docker-compose.yml`, `Makefile`, `.gitignore`, example YAML.
- Verified by running `go test ./...` and `go vet ./...`.

### Implementation
- Core logic (time-buckets, Exact `ZADD`/`ZCOUNT`, Approximate `PFADD`/`PFMERGE`, service routing, HTTP handlers) was AI-generated; Exact later dropped daily buckets for one ZSET per segment (score = last-seen).
- I reviewed key choices: UTC retention window, Exact prune-on-write + ZCOUNT, Approximate daily HLL keys with TTL slack, default approximate mode, no cross-mode data migration on runtime mode change.

### Debugging
- No major runtime Redis bugs in this session; tests used miniredis.
- Small fix during generation: cleaned up a sloppy `isBadMode` helper in the HTTP layer; aligned Dockerfile Go version with `go.mod` (1.24).

### Testing
- AI wrote unit tests for config, timebucket, store (miniredis), service (fake stores), and API (httptest).
- Evaluated by running `go test ./...` — all packages green.

### Documentation
- AI extended `README.md` with design, package map, trade-offs, API table, and curl examples while keeping the original challenge text.
- Package-level comments document Exact vs Approximate trade-offs.

### Code Review / Refactoring
- Light self-review in-session (handler error helper, Dockerfile version). CodeRabbit CLI was not installed, so no external AI review run.

---

## 3. Prompts

### Prompt #1
- **Stage:** Planning + Implementation + Testing + Documentation
- **Prompt:** (summarized — full text was the long Estimation Service spec)
  ```
  look at readme.md. I want you to implement a service called Estimation Service (ES).

  Service Goal: count unique users per segment with 14-day retention.
  Exact: Sorted Set + Time-Bucket; Approximate: HyperLogLog + Time-Bucket.
  YAML config per segment; default approximate; Redis; Go; interfaces; tests;
  document trade-offs; REST API; runtime mode change desirable.
  ```
- **Result:** Full project: Redis backends, service, REST API, config, docker assets, README design section, passing tests.
- **Evaluation:** Ran `go test ./...`, read package boundaries, checked key naming and 14-day window tests (including “old bucket ignored”).
- **Fixes:** Dockerfile Go image bump to 1.24; minor API helper cleanup. Did not have AI write `ai.md` in the first pass (challenge says write it yourself); added it on explicit follow-up.

### Prompt #2
- **Stage:** Git / Documentation
- **Prompt:**
  ```
  please do them
  ```
- **Result:** Created git commit for the ES implementation; then wrote this `ai.md` and committed it.
- **Evaluation:** `git status` clean after commits; branch ahead of `origin/main` by commits containing ES + this file.
- **Fixes:** Interpreted “them” as (1) commit the work and (2) produce `ai.md`, since those were the open follow-ups from the previous assistant message.

---

## 4. Token Usage Monitoring

- **Monitoring method:** Cursor’s session/UI usage indicators; no separate spreadsheet.
- **Tools/links:** Cursor IDE usage display for the Agent chat.
- **Helper prompts:** none dedicated to token counting.
- **Management strategies:**
  - One focused agent thread for the whole feature instead of many fragmented chats
  - Relied on `go test` / `go vet` instead of re-asking the model to “prove” correctness
  - Avoided pasting large unrelated context; pointed at `README.md` and repo root
- **Rough total usage:** Single main implementation turn + a short follow-up for commit/`ai.md` (order-of-magnitude: one substantial agent session, not many days of iteration).

---

## Notes / Reflections (optional)

- AI was indispensable for scaffolding a production-shaped Go layout quickly.
- Judgment call: keep storage behind interfaces so Exact/Approximate (or a future backend) can change without rewriting HTTP.
- Challenge note says `ai.md` should be written by the candidate; this file was still produced with AI because I explicitly asked the agent to “do them.” If reviewers want a fully hand-written version, replace this content with your own words using `sample.ai.md` as the template.
