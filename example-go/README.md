# Sucorwave Golang API

REST API built with Go, Echo v5, PostgreSQL, JWT authentication, and structured logging (zerolog).

## Documentation

- Architecture and team rules: [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md)
- Arsitektur dan aturan tim: [docs/ARCHITECTURE.id.md](docs/ARCHITECTURE.id.md)

## Requirements

- Go 1.27 or later
- PostgreSQL
- Docker with Docker Compose

## Setup with Docker

1. Create the environment file and provide at least `DB_*` and `JWT_SECRET`:

   ```bash
   cp .env.example .env
   ```

2. Build and run.

   Development with hot reload:

   ```bash
   docker compose -f docker-compose.dev.yml up --build
   ```

   Production-like verification:

   ```bash
   docker compose -f docker-compose.yml up --build
   ```

3. Verify:

   ```bash
   curl http://localhost:8090/
   ```

The dev container runs `air` with the source mounted at `/app`, so code changes trigger a rebuild. The production-like container builds and runs a binary without the source mount. Logs are written to stdout and to `./logs/app.log`.

Two values in both compose files mirror the environment file and must be changed together with it:

- `ports: "8090:8090"` follows the `HTTP_PORT` format `host:container`. For `HTTP_PORT=3000`, use `"8090:3000"`.
- The `./logs:/app/logs` volume collects only files written under `/app/logs`, so `LOG_FILE` should remain in that directory.

Since PostgreSQL runs on the host, `DB_HOST` must be `host.docker.internal`, which is the default in `.env.example`.

## Setup without Docker

```bash
cp .env.example .env
go mod download
go run ./cmd/api
```

Set `LOG_FILE=./logs/app.log`, as the default points to the container path.

With the default local storage driver, uploads land in `storage/app`, a fixed path relative to the working directory. It is git ignored, and `docker-compose.yml` mounts `./storage/app` to `/app/storage/app` so files survive a container rebuild. Files are always read back through the API, never from a public bucket URL.

## Example file handling module

`internal/examplefile` shows the full store/update/delete cycle with `platform/files` and `platform/storage`. Bytes go to the object store; the key plus metadata go to an in-memory repository, so records reset on restart while stored files stay on disk. This is an example only, not a database-backed module.

| Method | Path | Description |
| --- | --- | --- |
| `GET` | `/examples/documents` | List documents |
| `GET` | `/examples/documents/:id` | Get one document |
| `POST` | `/examples/documents` | Upload a document (JPEG, PNG, or PDF, max 1 MiB) |
| `PATCH` | `/examples/documents/:id` | Replace the file and/or rename the document |
| `DELETE` | `/examples/documents/:id` | Delete the record and its stored object |

Writes require a bearer token. `url` in the response is the `/files/<key>` path served by `files.Service`, made absolute when `APP_URL` is set.

```bash
curl -X POST http://localhost:8090/examples/documents \
  -H "Authorization: Bearer $TOKEN" \
  -F "title=Monthly report" \
  -F "file=@report.pdf"

curl -X PATCH http://localhost:8090/examples/documents/1 \
  -H "Authorization: Bearer $TOKEN" \
  -F "file=@report-v2.pdf"

curl -X DELETE http://localhost:8090/examples/documents/1 \
  -H "Authorization: Bearer $TOKEN"
```

## Environment variables

| Variable | Default | Description |
| --- | --- | --- |
| `APP_NAME` | `Sucor Go API` | Service name attached to log entries |
| `APP_VERSION` | `1.0.0` | Version attached to log entries |
| `APP_ENV` | `development` | Also determines the log level |
| `APP_URL` | - | Public origin, e.g. `http://localhost:8090`; makes file URLs absolute. Empty keeps `/files/<key>` relative |
| `HTTP_PORT` | `8090` | Listen port; keep in sync with `docker-compose.yml` |
| `LOG_LEVEL` | `info` | `trace` to `panic`, or `disabled` |
| `LOG_FILE` | `/app/logs/app.log` | Daily rotation with gzip compression |
| `DB_HOST`, `DB_PORT` | - | `host.docker.internal` when using Docker |
| `DB_DATABASE`, `DB_USERNAME`, `DB_PASSWORD` | - | PostgreSQL credentials |
| `DB_MAX_OPEN_CONNS` | `25` | Maximum open connections in the pool |
| `DB_MAX_IDLE_CONNS` | `10` | Maximum idle connections in the pool |
| `DB_CONN_MAX_LIFETIME_MINUTES` | `30` | Maximum lifetime of a pooled connection |
| `DB_CONN_MAX_IDLE_MINUTES` | `5` | Maximum idle time of a pooled connection |
| `JWT_SECRET` | - | Required, minimum 32 characters; must match the issuing service |
| `JWT_ISSUER` | - | When empty, the `iss` claim is not verified |
| `JWT_TTL_MINUTES` | `30` | Lifetime of tokens issued by this service |
| `STORAGE_DRIVER` | `local` | `local` (disk) or `s3` |
| `STORAGE_S3_BUCKET_URL` | - | Required when the driver is `s3`, e.g. `s3://my-bucket` |
| `STORAGE_SIGNING_KEY` | - | HMAC key for local signed URLs; required when signatures are required. Use a long random value |
| `STORAGE_REQUIRE_SIGNATURE` | `false` | Reject `/files` requests that carry no valid signature |

## Project layout

```
cmd/api/             entry point
internal/auth/       JWT, middleware, handler, repository
internal/community/  community membership lookup
internal/examplefile/ store/update/delete example using platform/files
internal/health/     liveness/readiness endpoint with a database ping
internal/hello/      sample endpoint
internal/platform/   config, db, logger, http, identity, contentcleaner, textutil, storage, files
internal/post/       posts, comments and likes
internal/profanity/  profanity filter used by posts and comments
```

Most feature modules use the same set of files: `domain.go` (types and interfaces), `repository.go` (SQL), `service.go` (rules), `handler.go` (HTTP), `errors.go` (sentinel errors and their status) and `route.go` (URLs). Modules without HTTP routes, such as `community`, keep only the files they need.

## Files

Uploads go through `internal/platform/storage` (local disk by default, `s3` when configured) and are always read back through the API, so no bucket has to be public.

- `files.Service.Upload(ctx, policy, input)` validates the bytes, writes `{folder}/{yyyy}/{mm}/{prefix}_{random}{ext}` and returns the key plus metadata. The policy is passed per call, not configured.
- `files.Service.URL(key)` returns the client path `/files/<key>`. It is relative on purpose; the host belongs to the reverse proxy or the client.
- `files.Service.SignedURL(ctx, key, ttl)` returns a presigned link when the driver can sign (S3). Otherwise, with `STORAGE_SIGNING_KEY` set, it appends `?expires=…&signature=…` (HMAC-SHA256 over key and expiry) to `/files/<key>`; the handler verifies it and answers `403` when it is missing, expired or tampered with. Without a key it returns `files.ErrSigningUnavailable`.
- `files.Service` implements `http.Handler` and `platform/http` mounts it at `GET /files/*`, so the package never imports a router. Responses go through `http.ServeContent`, which gives `Range`, `HEAD` and `If-Modified-Since` support.

## Responses

Every reply uses the envelope from `internal/platform/http`: `success`, `code`, `message`, and `data` on success. `errors` appears only on a validation failure. Framework errors (404, 405, 413) and recovered panics go through the same handler, so they use the same envelope too.

`GET /health` pings the database and answers `200` when it responds, `503` otherwise.

Validation failures answer `422` with a fixed `message` and an `errors` map holding one message array per json field:

```json
{
  "success": false,
  "code": 422,
  "message": "The given data was invalid.",
  "errors": {
    "content": ["The content field is required."],
    "post_comment_id": ["The post comment id field must be at least 1."]
  }
}
```

Rules are `validate` tags on the request struct. `internal/platform/http/validator.go` is the only validator in the project: `NewServer` registers it on echo, it reports the json field name instead of the Go one, and it collects every failing field instead of stopping at the first:

```go
type CreateCommentRequest struct {
    Content         string `json:"content" validate:"required,notblank"`
    ParentCommentID *int   `json:"post_comment_id" validate:"omitempty,min=1"`
}

if err := echo.BindBody(c, &req); err != nil {
    return platformhttp.Error(c, http.StatusBadRequest, "Invalid request body")
}

if err := c.Validate(&req); err != nil {
    return platformhttp.ValidationError(c, err, "The given data was invalid.")
}
```

Custom tags are registered once in `NewRequestValidator`, and `validationMessage` holds one case per tag, so a tag never falls back to a generic sentence. Rules that need the database or another service (mute state, membership, parent comment) are business rules and stay in the service as sentinel errors. A body that is not json at all stays `400`.

## Conventions

- **Layering**: the handler maps HTTP and owns no rules, the service holds the rules and returns sentinel errors, and the repository is the only layer that speaks SQL. Nothing below the handler knows about `http.Status*`.
- **Feature errors**: the sentinels live in `internal/<feature>/errors.go` next to `MapError`, which turns them into a status and a message. A handler calls one helper for every error it gets back; anything `MapError` does not recognise is logged once and answered `500`.
- **Routes**: each module registers its own URLs in `route.go`, grouped under its resource prefix. The auth middleware is passed in by the composition root, so features never import each other.
- **Logging**: use `logger.L()` outside a request and `logger.Ctx(ctx)` inside one so `request_id` is attached. Never log tokens, secrets, or passwords.
- **Errors**: attach the stack once where the error originates (`logger.WithStack`), add context with `fmt.Errorf("...: %w", err)` in callers, and log once at the boundary that decides the outcome.


