# HookRelay

**Durable webhook delivery, visible retries, and controlled replay.**

When a receiving application is unavailable, a one-shot HTTP callback loses important events. HookRelay accepts an event into PostgreSQL before acknowledging it, then workers deliver signed HTTP callbacks and retain the delivery history.

## What works

- Authenticated REST API for endpoint registration, event ingestion, delivery inspection, and replay.
- Persistent event/job creation in one transaction, with idempotency keys and request-content conflict detection.
- Concurrent Go workers claim jobs through `FOR UPDATE SKIP LOCKED` and expiring leases. A crashed worker's job can be reclaimed.
- Exponential backoff with jitter, five-claim attempt budget, dead-letter state, and replay with retained event identity.
- HMAC-SHA256 signing, timestamp verification example, bounded HTTP timeouts, response excerpts, and disabled redirects.
- Outbound URL/DNS restrictions outside explicit local mode, including direct dialing of validated resolved addresses.
- React/TypeScript console for endpoints, JSON events, filtering, attempt details, and replay.
- A Python receiver with SQLite deduplication and failure injection, plus an automated end-to-end demo.
- Docker Compose and GitHub Actions checks, including PostgreSQL integration tests and Go race detection.

## Stack

| Layer | Technology |
|---|---|
| Backend | Go 1.24, net/http, goroutines, pgx connection pool |
| Data/queue | PostgreSQL 16, SQL, row locks, SKIP LOCKED, lease tokens |
| Frontend | React 19, TypeScript, Vite |
| Receiver example | Python 3.12 standard library, SQLite |
| Delivery | Docker Compose, Nginx, GitHub Actions |

PostgreSQL is deliberately the queue as well as the database. This avoids a database/message-broker dual-write failure and keeps the project easy to run. RabbitMQ, Kafka, Redis, Kubernetes, and AWS are not claimed as implemented technologies.

## Fastest way to run

**Requirements:** Git; Docker Engine/Desktop with Docker Compose v2; internet access on first build. You do not need local Go, Node, Python, or PostgreSQL for the container path. Docker Desktop must be running on Windows/macOS.

```bash
git clone https://github.com/RUTH808/50-Projects-in-2026.git
cd 50-Projects-in-2026/hookrelay
cp .env.example .env
docker compose up --build -d
docker compose logs -f relay
```

PowerShell: use `Copy-Item .env.example .env` for the copy step. Open **http://localhost:3001**, then connect with the `API_TOKEN` from `.env`. The local default is `hookrelay-local-development-token-2026`.

| Service | Address |
|---|---|
| Console | http://localhost:3001 |
| API | http://localhost:8090/api |
| API health | http://localhost:8090/healthz |
| Demo receiver | http://localhost:8091 |
| PostgreSQL | localhost:5433 |

### First event through the dashboard

1. In Endpoints, create `Campus booking receiver` with URL `http://receiver:8091/webhook`.
2. Set its signing secret to `DEMO_WEBHOOK_SECRET` from `.env` (default: `hookrelay-demo-signing-secret-change-me`). The receiver must have the same secret.
3. In Send event, choose that endpoint, keep or change `booking.confirmed`, and submit the JSON payload.
4. Open Deliveries and Inspect to see the receiver response and duration.
5. Send the same request with the same idempotency key: the existing delivery is returned. A different request with that key returns HTTP 409.
6. Replay a delivered event. The delivery runs again, but the example receiver recognizes the signed event ID and returns `duplicate: true`.

`receiver` is a Docker service hostname. Use it when HookRelay itself is in Compose. A relay running directly on your laptop must use `http://localhost:8091/webhook` instead.

### Automated failure/recovery demo

With Python 3.10+ installed on your laptop:

```bash
# From hookrelay/ after Compose is running
python3 examples/demo.py
```

On Windows use `python`. The script injects two HTTP 503 failures, verifies recovery on the third attempt, checks ingestion idempotency, replays the event, and confirms receiver deduplication. It creates an endpoint and delivery in the local database. If you changed `.env`, also export `API_TOKEN` and `DEMO_WEBHOOK_SECRET` for the script; Python does not automatically load `.env`.

To force dead-letter handling, configure `POST http://localhost:8091/control` with `{"fail_next":5}` using bearer authentication with the API token, then enqueue a new event. Watch it move to `dead` after retries. Reset `fail_next` to zero and use Replay. Avoid injecting failures while other demo deliveries are in flight, since the receiver failure counter applies to all incoming callbacks.

Stop with `docker compose down`; both databases survive in named volumes. `docker compose down -v` **deletes the relay and receiver databases**.

## Run without application containers

**Requirements:** Go 1.24+, Node.js 22+, npm, PostgreSQL 16, and Python 3.10+ if running the demo receiver. Python has no third-party requirements.

```bash
# From hookrelay/
docker compose up -d db

# Terminal 1: backend (bash)
export API_TOKEN=hookrelay-local-development-token-2026
export ALLOW_PRIVATE_ENDPOINTS=true
go run ./cmd/hookrelay

# Terminal 2: demo receiver
python3 examples/receiver.py

# Terminal 3: frontend
cd frontend
npm ci
npm run dev
```

Open **http://localhost:5174**. Register the receiver as `http://localhost:8091/webhook`. For the automated demo in this mode:

```bash
ENDPOINT_URL=http://localhost:8091/webhook python3 examples/demo.py
```

PowerShell uses `$env:API_TOKEN='hookrelay-local-development-token-2026'` and `$env:ALLOW_PRIVATE_ENDPOINTS='true'` instead of `export`. For your own database, set `DATABASE_URL` to a PostgreSQL connection string for an existing database/user. The default is `postgres://hookrelay:hookrelay-local@localhost:5433/hookrelay?sslmode=disable`. `.env` is for Compose; `go run` does not load it.

## Configuration

| Variable | Default / purpose |
|---|---|
| API_TOKEN | Required, at least 24 characters; shared single-workspace bearer credential |
| DATABASE_URL | Local PostgreSQL URL shown above |
| LISTEN_ADDR | :8090 |
| MODE | all; alternatives api or worker |
| WORKERS | 2; valid range 1–32 per process |
| ALLOW_PRIVATE_ENDPOINTS | false in the binary; explicitly true in local Compose for the demo receiver |
| WEBHOOK_SECRET | Receiver signing secret, supplied through DEMO_WEBHOOK_SECRET in Compose |
| CONTROL_TOKEN | Receiver admin credential, supplied through API_TOKEN in Compose |
| RECEIVER_DB | SQLite path for the example receiver |

Run separate API and worker processes against the same database with `MODE=api` and `MODE=worker`. API_TOKEN remains required for both modes by the startup configuration. Startup schema creation is serialized with a PostgreSQL advisory transaction lock.

## Tests

```bash
go vet ./...
go test -race -v ./...

# Full suite; use a pre-created disposable PostgreSQL test database.
TEST_DATABASE_URL='postgres://hookrelay:hookrelay-local@localhost:5433/hookrelay_test?sslmode=disable' \
  go test -race -v ./...

python3 -m unittest discover -s examples -p 'test_*.py' -v

# From frontend/
npm ci
npm run build
```

Without TEST_DATABASE_URL, the three database integration tests are explicitly skipped. They cover idempotency, precision preservation for large JSON integers, competing claims, lease recovery, stale-worker fencing, dead letters, and replay identity. They truncate project tables: **never point them at a database you want to keep**. The repository-level `HookRelay checks` workflow uses real PostgreSQL and runs the Compose recovery demo.

## API examples

All `/api` requests require `Authorization: Bearer <API_TOKEN>`. Secrets are returned only in the endpoint-creation response and are omitted from endpoint lists.

```bash
curl -X POST http://localhost:8090/api/endpoints \
  -H 'Authorization: Bearer hookrelay-local-development-token-2026' \
  -H 'Content-Type: application/json' \
  -d '{"name":"Demo receiver","url":"http://receiver:8091/webhook","secret":"hookrelay-demo-signing-secret-change-me"}'

# Substitute the id returned by endpoint creation.
curl -X POST http://localhost:8090/api/events \
  -H 'Authorization: Bearer hookrelay-local-development-token-2026' \
  -H 'Idempotency-Key: booking-42-confirmed' \
  -H 'Content-Type: application/json' \
  -d '{"endpoint_id":"<endpoint-id>","type":"booking.confirmed","payload":{"booking_id":42}}'
```

| Method | Path | Purpose |
|---|---|---|
| GET / POST | /api/endpoints | List / register destinations |
| POST | /api/events | Atomically enqueue an event and job |
| GET | /api/deliveries | Latest 200 jobs |
| GET | /api/deliveries/{id}/attempts | Completed attempt history |
| POST | /api/deliveries/{id}/replay | Requeue a delivered/dead job |
| GET | /healthz | Database readiness |

Event ingestion returns HTTP 202 with `delivery_id` and `duplicate`. The idempotency key is global to this single workspace; it covers endpoint ID, event type, and JSON payload. Object-key order and whitespace are canonicalized; numeric representations such as `1` and `1.0` remain distinct. JSON bodies are limited to 256 KiB.

## Delivery contract

```json
{"id":"stable-event-id","type":"booking.confirmed","data":{"booking_id":42}}
```

- `X-HookRelay-Id`: delivery ID for tracing.
- `X-HookRelay-Timestamp`: Unix timestamp in seconds.
- `X-HookRelay-Signature`: `v1=<hex HMAC-SHA256(secret, timestamp + "." + raw_body)>`.

Verify the signature against the exact received bytes, compare in constant time, enforce a timestamp tolerance, then deduplicate using the **event ID inside the signed body**. The Python example allows a five-minute tolerance and makes SQLite insertion/deduplication one transaction. A real receiver should commit its business effect and deduplication record together.

## Reliability model and limitations

- Delivery is **at least once**. A worker can crash after the receiver commits but before the sender records success. Replay can also resend an accepted event. Receiver idempotency is essential; exactly-once network delivery is not claimed.
- Claiming sets a 30-second lease and a unique lease token. Completion updates only while the same token owns the job, fencing stale workers after reclamation. HTTP requests time out after ten seconds.
- Failed requests retry at approximately 2, 4, 8, and 16 seconds plus up to 50% jitter. After five claims, a completed failure enters `dead`. A crash consumes a claim and may leave no attempt-history row; recovery can claim again even after the nominal budget to avoid silently abandoning a job.
- Replay preserves event and delivery IDs, increments `run`, resets its attempt counter, and retains historical attempt rows. Replaying an in-flight job is rejected.
- No per-endpoint ordering, per-tenant isolation, rate limiting, broker integration, or infinite-scale claims are made. Lists are bounded rather than paginated. All non-2xx responses are retried, including 4xx; Retry-After is not yet honored.
- Webhook secrets are stored in PostgreSQL and returned once after creation; envelope encryption/rotation is future work. Attempt excerpts may contain receiver data, so use synthetic payloads for this demo.
- The binary rejects non-HTTPS destinations unless private-endpoint mode is enabled, blocks private/reserved resolved addresses during dialing, and does not follow redirects. Production deployments should also enforce network egress policy.

For a public deployment, use TLS, unique generated credentials, `ALLOW_PRIVATE_ENDPOINTS=false`, a managed database with TLS/backups, encrypted secrets, request throttling, and retention policies. The included receiver controls and known default credentials are intended for local demonstrations. Compose binds published ports to localhost.

## Troubleshooting

- `API_TOKEN must contain at least 24 characters`: export a token before `go run`, or check `.env` for Compose.
- Receiver HTTP 401: its WEBHOOK_SECRET must match the endpoint's signing secret.
- Name-resolution error for `receiver`: that hostname works inside Compose; local binaries use localhost.
- Database authentication errors after editing DB_PASSWORD: an existing PostgreSQL volume retains its original password. Restore it or deliberately reset your disposable volume.
- Port in use: change the host side of the Compose port mapping. Both projects can run together with the default distinct ports.

## Resume description

See [RESUME.md](RESUME.md) for feature-backed bullets and interview talking points.
