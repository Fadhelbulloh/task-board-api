# Task Management API

Multi-user task API with JWT auth, teams, idempotent task creation, transactional
assignment and structured logging. Go 1.27, Gin, PostgreSQL (pgx).

## Running

### Docker

```sh
docker compose up --build
```

API on `http://localhost:8080`, Postgres on host port `5433`.

### Local

```sh
docker compose up -d db          # or any Postgres 14+
cp .env.example .env
set -a; source .env; set +a
make run
```

### Migrations

SQL files live in `migrations/` and are embedded into the binary. The app
applies pending migrations on startup, so normally there is nothing to do.
To run them by hand (needs the [golang-migrate CLI](https://github.com/golang-migrate/migrate)):

```sh
make migrate-up                  # apply all
make migrate-down                # roll back one
make migrate-new name=add_index  # create a new pair of up/down files
```

### Tests

```sh
make test-race
```

Unit tests need no database. Repositories are replaced by in-memory versions
in `internal/mock`.

## Configuration

| Var | Default | Notes |
|---|---|---|
| `DATABASE_URL` | required | `postgres://user:pass@host:5432/db?sslmode=disable` |
| `JWT_SECRET` | required | min 32 chars |
| `JWT_TTL` | `24h` | Go duration |
| `IDEMPOTENCY_TTL` | `24h` | how long an `Idempotency-Key` is remembered |
| `PORT` | `8080` | |
| `APP_ENV` | `development` | `production` puts Gin in release mode |

## Endpoints

Authenticated routes need `Authorization: Bearer <access_token>`.

### Auth

| Method | Path | Body |
|---|---|---|
| POST | `/auth/register` | `{email, name, password}` |
| POST | `/auth/login` | `{email, password}` → `{access_token, token_type, expires_at, user}` |

### Tasks

| Method | Path | Notes |
|---|---|---|
| POST | `/tasks` | Header `Idempotency-Key: <uuid>` required. Body `{title, description?, status?, team_id?}` |
| GET | `/tasks` | `?status=todo\|in_progress\|done&search=<title prefix>&page=1&limit=20` (limit max 100) |
| GET | `/tasks/:id` | |
| PUT | `/tasks/:id` | owner only |
| DELETE | `/tasks/:id` | owner only |
| POST | `/tasks/:id/assign` | `{assignee_id}`. Task needs a `team_id`; caller and assignee must both be members |

A task is visible to its owner and its assignee. Everyone else gets 404.

```json
{"data": [...], "pagination": {"page": 1, "limit": 20, "total": 42}}
```

### Teams

| Method | Path | Notes |
|---|---|---|
| POST | `/teams` | `{name}`; creator becomes owner |
| GET | `/teams` | teams the caller is in |
| GET | `/teams/:id` | members only |
| PUT | `/teams/:id` | owner only |
| DELETE | `/teams/:id` | owner only; tasks stay, with `team_id = null` |
| GET | `/teams/:id/members` | members only |
| POST | `/teams/:id/members` | `{email}`; owner only |
| DELETE | `/teams/:id/members/:userId` | owner only |

### Error format

```json
{
  "status": 404,
  "code": "NOT_FOUND",
  "message": "task not found",
  "timestamp": "2026-10-12T08:00:00Z",
  "request_id": "3f0c..."
}
```

Codes: `BAD_REQUEST`, `VALIDATION_ERROR`, `PAYLOAD_TOO_LARGE`,
`UNAUTHORIZED`, `FORBIDDEN`, `NOT_FOUND`, `EMAIL_ALREADY_REGISTERED`,
`ALREADY_MEMBER`, `IDEMPOTENCY_KEY_REUSED`, `NOTIFICATION_FAILED`,
`INTERNAL_ERROR`. Validation messages use the JSON field names, e.g.
`"title is required; limit must be at most 100"`. Request bodies are capped
at 1 MiB. 5xx bodies never contain the underlying error; look it up
in the logs by `request_id`.

## Project structure

Code is grouped by layer. A request flows top to bottom:

```
cmd/api/main.go           wiring: config, db, migrations, repo -> service -> controller -> route
internal/
  route/                  every endpoint and the middleware in front of it
  middleware/             request id, logging, panic recovery, body size limit, JWT auth
  controller/             bind + validate request into dto, call service, write response
  service/                business rules; works with dto (in/out) and model (to repository)
  repository/             SQL against Postgres, returns model
  dto/                    request/response structs between controller and service
  model/                  structs that map to database rows
  helper/                 AppError, error response writer, JWT, password hashing, context getters
  integration/            third-party clients (notification; currently logs only)
  database/               pgx pool + migration runner
  config/                 env vars
  mock/                   in-memory repositories for unit tests
migrations/               SQL migrations (embedded)
```

Services depend on repository interfaces declared in `service/interfaces.go`,
not on the Postgres structs, which is what lets the tests swap in `mock`.

### Idempotency

Handled in `TaskService.Create`, with storage in
`repository/idempotency_repository.go`. The controller only checks that the
`Idempotency-Key` header is a UUID.

One transaction covers the whole request:

1. `Claim` inserts `(user_id, key, request_hash)` into `idempotency_keys`.
   The primary key is the lock: if another request inserted the same key
   and hasn't committed yet, Postgres makes this INSERT wait.
2. Inserted → create the task, store the JSON response next to the key,
   commit. Client gets `201`.
3. Conflict → the other request has committed by now, so its response is
   there. Same `request_hash` → return it with `Idempotent-Replayed: true`.
   Different hash → `422 IDEMPOTENCY_KEY_REUSED`.

`request_hash` is a SHA-256 of the parsed body, so whitespace or key order
doesn't make it a different request.

Because the key, the task and the stored response commit or roll back
together:

- A task never exists without its key, so a retry can't create a duplicate.
- A key never exists without its response, so a crash mid-request doesn't
  block the key. The transaction rolls back and the retry starts fresh.
- Failed requests (validation, 403, DB error) leave no key behind and can
  be retried with the same key.

Concurrent duplicates all get the same `201`; the later ones just wait a
few milliseconds for the first to commit. Keys expire after
`IDEMPOTENCY_TTL` (24h). After that the same key is treated as new.

### Assign transaction

`TaskService.Assign` runs inside `Transactor.WithinTx`. The transaction
travels in `context.Context`, so repositories pick it up without extra
parameters. Steps: lock the task row (`SELECT ... FOR UPDATE`), check both
users are in the team, update the assignee, insert into `task_logs`, send the
notification. Any error rolls the whole thing back, including a failed
notification.

### Query notes

- `GET /tasks` builds its `WHERE` from the filters that are actually set,
  so Postgres gets a plain query it can plan with the indexes in
  `000003_create_tasks.up.sql`.
- `search` is a case-insensitive prefix match backed by a `lower(title)
  text_pattern_ops` index. `%` and `_` in user input are escaped.

### Logging

Each request writes one JSON line: `request_id`, `method`, `path`, `status`,
`latency_ms`, `client_ip`, and `user_id` when logged in. INFO for < 400, WARN
for 4xx, ERROR for 5xx. The request id is returned in `X-Request-ID`.

## Not included

- Refresh tokens / logout.
- Cleanup job for expired idempotency keys (they are recycled on reuse; a
  periodic `DELETE ... WHERE expires_at < now()` would keep the table small).
- Rate limiting on `/auth/login`.
