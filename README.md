# RabbitMQ Explore

End-to-end RabbitMQ example in Go. A publisher receives messages over HTTP and sends them to a quorum queue; a subscriber consumes the queue and streams every message to browsers with Server-Sent Events (SSE).

The services use the official RabbitMQ AMQP 1.0 Go client (`github.com/rabbitmq/rabbitmq-amqp-go-client`), Echo v5, and zerolog.

## Demo

https://github.com/user-attachments/assets/1ea0b712-8685-4359-8c74-d8f3bf1802c2

_Publishing from the form on port 8090, received live over SSE on port 8091._

## Architecture

```mermaid
flowchart LR
    PB["Browser<br/>localhost:8090"] -->|"GET /send-msg?msg=..."| PUB["publisher<br/>cmd/pub"]
    PUB -->|"AMQP 1.0 publish"| Q[("quorum queue<br/>hello")]
    Q -->|"consume"| SUB["subscriber<br/>cmd/sub"]
    SUB -->|"SSE /events"| SB["Browser<br/>localhost:8091"]
    MGMT["Management UI<br/>localhost:15672"]
```

Publisher and subscriber each declare the same quorum queue (`RABBITMQ_QUEUE`, default `hello`). The publisher sends to that queue through an AMQP 1.0 queue address (`/queues/hello`), so the broker delivers the message straight to the queue. No exchange, routing key, or binding is involved; see [Exchange and routing](#exchange-and-routing).

## Repository layout

| Path | Description |
| --- | --- |
| `compose.yml` | Production-like stack: RabbitMQ 4 management plus publisher and subscriber binaries |
| `compose.dev.yml` | Development stack with `air` hot reload and bind-mounted source |
| `example-go/` | Go module with both commands and the shared packages |
| `example-go/cmd/pub/` | Publisher entry point (composition root) |
| `example-go/cmd/sub/` | Subscriber entry point (composition root) |
| `example-go/internal/rabbitmq/` | Connection, queue, publisher, consumer, publish/consume helpers |
| `example-go/internal/pub/` | Publisher HTTP handler and routes |
| `example-go/internal/sub/` | Subscriber HTTP handler, SSE hub, and routes |
| `example-go/internal/platform/` | Config, HTTP server, and logger used by both commands |
| `logs/pub/`, `logs/sub/` | Host directories mounted at `/app/logs` |
| `example-go/docs/` | Architecture guide inherited from the Go API template |

`example-go/internal/platform` also contains database, auth, storage, and file packages from the original template. They are not wired into the publisher or subscriber.

## Quick start (Docker)

Requirements: Docker with Compose v2. Ports `5672`, `15672`, `8090`, and `8091` must be free.

1. Create the Compose environment file and the two service environment files:

   ```powershell
   Copy-Item .env.example .env
   Copy-Item example-go/.env.example example-go/.env.pub
   Copy-Item example-go/.env.example example-go/.env.sub
   ```

   On macOS/Linux, use `cp` instead of `Copy-Item`.

2. In `example-go/.env.sub`, set the subscriber port. The publisher keeps `8090`:

   ```dotenv
   APP_NAME="SUBSCRIBER"
   APP_URL=http://localhost:8091
   HTTP_PORT=8091
   ```

   Optionally set `APP_NAME="PUBLISHER"` in `example-go/.env.pub`.

3. Make sure the broker credentials match. `compose.yml` creates RabbitMQ with `RABBITMQ_USER` and `RABBITMQ_PASSWORD` from the root `.env`, while `RABBITMQ_BROKER_URI` in `.env.pub` and `.env.sub` contains the credentials used by the clients. With the defaults from `.env.example` both sides are `guest` / `guest`:

   ```dotenv
   RABBITMQ_BROKER_URI=amqp://guest:guest@rabbitmq:5672/
   ```

4. Start the stack:

   ```bash
   docker compose up --build
   ```

5. Open:

   - Publisher form: <http://localhost:8090/>
   - Subscriber stream: <http://localhost:8091/>
   - RabbitMQ Management: <http://localhost:15672/> (`guest` / `guest` by default)

Send a message from the publisher form and it appears on the subscriber page. The subscriber keeps the last 50 messages in memory and replays them to newly connected browsers.

## Sending and receiving without a browser

```bash
curl "http://localhost:8090/send-msg?msg=hello%20rabbit"
```

```json
{
  "success": true,
  "code": 200,
  "message": "message sent",
  "data": { "message": "hello rabbit" }
}
```

Watch the stream:

```bash
curl -N "http://localhost:8091/events"
```

```text
data: {"message":"hello rabbit"}
```

Every publish is confirmed by the broker. `internal/rabbitmq.Publish` checks the outcome and reports `accepted` or `rejected`; a rejected message returns HTTP `500` to the caller. Any other outcome, such as `released` for an unroutable message, also returns HTTP `500`.

## Development with hot reload

```bash
docker compose -f compose.dev.yml up --build
```

`compose.dev.yml` mounts `example-go/` into the container and runs `air` with `.air.pub.toml` or `.air.sub.toml`. The same `.env.pub` and `.env.sub` files are required.

`compose.dev.yml` does not pass `RABBITMQ_USER` / `RABBITMQ_PASSWORD` to the broker, so the dev broker uses the image defaults (`guest` / `guest`). The dev services also have no restart policy. If a service starts before the broker is ready, start it again after RabbitMQ is up.

## Running locally without Docker

1. Start only RabbitMQ. This uses the root `.env` created in the quick start; if it does not exist yet, copy `.env.example` to `.env` first:

   ```bash
   docker compose up -d rabbitmq
   ```

2. In both service environment files, point the broker at the host and write logs to a local path:

   ```dotenv
   RABBITMQ_BROKER_URI=amqp://guest:guest@localhost:5672/
   LOG_FILE=./logs/app.log
   ```

3. From `example-go/`, run each command in its own terminal:

   ```bash
   go run ./cmd/pub
   ```

   ```bash
   go run ./cmd/sub
   ```

`godotenv` loads `.env.pub` and `.env.sub` from the current working directory, so run both commands from `example-go/`. Keep `HTTP_PORT=8091` in `.env.sub` so the two services do not conflict.

## Configuration

Only these variables are read and validated by `pub` and `sub` (`Config.ValidateBroker`):

| Variable | Default | Description |
| --- | --- | --- |
| `RABBITMQ_BROKER_URI` | - | Required. AMQP URI, e.g. `amqp://guest:guest@rabbitmq:5672/` |
| `RABBITMQ_QUEUE` | `hello` | Quorum queue declared by both services; the publisher sends to it directly |
| `HTTP_PORT` | `8090` | HTTP listen port |
| `APP_NAME`, `APP_VERSION`, `APP_ENV` | template values | Attached to structured log entries |
| `APP_URL` | - | Public origin reported by the template config; not used by the publisher or SSE pages |
| `LOG_LEVEL` | `info` | `trace` through `panic`, or `disabled`; the templates use `debug` |
| `LOG_FILE` | `/app/logs/app.log` | JSON log file, rotated daily and compressed with gzip |
| `LOG_MAX_SIZE_MB`, `LOG_MAX_BACKUPS`, `LOG_MAX_AGE_DAYS` | `100`, `5`, `28` | Log rotation limits |

The `.env.example` template also contains `DB_*`, `JWT_*`, and `STORAGE_*` entries. Those come from the original API template; the publisher and subscriber do not use or validate them.

## Logs and volumes

- `./logs/pub` is mounted at `/app/logs` in the publisher container.
- `./logs/sub` is mounted at `/app/logs` in the subscriber container.
- `rabbitmq_data` keeps broker state between restarts.
- `uploads` is declared in `compose.yml` and mounted at `/app/storage/uploads`, but no publisher or subscriber code uses it.

Logs are written both to stdout (`docker compose logs -f`) and to the mounted directory.

## Exchange and routing

This demo does not use an exchange. Nothing in the code declares an exchange, sets a routing key, or creates a binding.

| Piece | Status |
| --- | --- |
| Custom exchange (fanout, topic, direct) | Not implemented |
| Publishing through `ExchangeAddress` | Not implemented |
| Binding a queue to an exchange | Not implemented |
| `RABBITMQ_EXCHANGE` configuration | Not implemented |
| Routing key | Not implemented |
| `StateReleased` (unroutable message) handling | Not implemented |

`internal/rabbitmq.NewPublisher` publishes with `rmq.QueueAddress`, which the AMQP 1.0 client converts into the queue node `/queues/<name>`. The AMQP 0-9-1 "default exchange" does not apply here: the message never reaches an exchange, so there is no exchange routing to configure or inspect in the Management UI.

Both commands declare `RABBITMQ_QUEUE` themselves (`cmd/pub/main.go`, `cmd/sub/main.go`), so the publisher knows the queue name even though it does not know how many consumers read from it.

Consequences:

- A single publish cannot fan out to several queues. That requires a fanout exchange bound to one queue per subscriber.
- One queue means subscriber replicas compete for messages. Because the SSE hub is in memory, browsers attached to one replica never see messages consumed by another.
- `rabbitmq.Publish` handles `StateAccepted` and `StateRejected`. An unroutable publish comes back as `StateReleased`, which currently falls into the generic `unexpected publish outcome` error.

Adding exchange routing requires these changes:

1. Declare a `FanOutExchangeSpecification` (or `TopicExchangeSpecification`) in `internal/rabbitmq`.
2. Change `NewPublisher` from `QueueAddress` to `ExchangeAddress`.
3. Declare one queue per subscriber and bind it with `ExchangeToQueueBindingSpecification`.
4. Add `RABBITMQ_EXCHANGE` to `Config` and drop the queue from the publisher.
5. Handle `StateReleased` explicitly in `Publish`.

The publisher and subscriber routes, handlers, and SSE hub do not change.

## Behaviour and limitations

- Messages go to one queue, so multiple subscriber instances compete for messages instead of each receiving a copy. Fan-out needs an exchange; see [Exchange and routing](#exchange-and-routing).
- The SSE hub and its 50-message history are per subscriber process and reset on restart.
- The subscriber accepts a message after the handler succeeds and requeues it when the handler returns an error. There is no retry limit or dead-letter queue, so a handler that always fails creates a redelivery loop. The current handler always returns `nil`.
- `depends_on` only controls startup order; it does not wait for RabbitMQ to be ready. `restart: unless-stopped` lets the production-like services recover after an early connection failure.
- `.env.pub` and `.env.sub` are ignored by Git; only `.env.example` is committed.
- The architecture documents under `example-go/docs` describe the original Go API template (`cmd/api`, database, auth, files). They are useful for layering conventions, but the RabbitMQ commands do not use those parts.

## Further reading

- [`example-go/README.md`](example-go/README.md): module layout and implementation details.
- [`example-go/docs/ARCHITECTURE.md`](example-go/docs/ARCHITECTURE.md): architecture guide (English).
- [`example-go/docs/ARCHITECTURE.id.md`](example-go/docs/ARCHITECTURE.id.md): architecture guide (Indonesian).
