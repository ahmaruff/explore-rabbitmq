# Example Go: RabbitMQ Publisher and Subscriber

Go module behind the Docker example in the repository root. See [`../README.md`](../README.md) for the end-to-end quick start and the architecture diagram.

- `cmd/pub`: HTTP server that publishes a message to a quorum queue.
- `cmd/sub`: HTTP server that consumes the queue and streams messages to browsers with SSE.

Both commands are separate composition roots and share `internal/rabbitmq` plus a small part of `internal/platform`.

## Layout

```text
cmd/
  pub/main.go        publisher entry point
  sub/main.go        subscriber entry point
internal/
  pub/               publisher handler and routes
    handler.go       GET /, GET /send-msg
    route.go
  sub/               subscriber handler, SSE hub, and routes
    handler.go       GET /, GET /events
    hub.go           connected clients + last 50 messages
    route.go
  rabbitmq/          AMQP 1.0 helpers
    rabbitmq.go      connection, queue, publisher, consumer
  platform/
    config/          env loading and broker validation
    http/            Echo server, response envelope, request logging
    logger/          zerolog + timberjack logging
    auth/ db/ files/ storage/ ...   inherited, not wired into pub/sub
docs/                architecture guides from the original API template
.air.pub.toml        air config for the publisher
.air.sub.toml        air config for the subscriber
Dockerfile           multi-stage build with `pub` and `sub` targets
Dockerfile.dev       development image with `air`
```

## Flow

1. `cmd/pub` loads config, opens one RabbitMQ connection, declares the quorum queue, and creates a publisher.
2. `GET /send-msg?msg=...` calls `rabbitmq.Publish`. Strings are sent as raw bytes; other values are JSON encoded. The handler returns HTTP `400` when `msg` is empty and HTTP `500` when the broker rejects the message.
3. `cmd/sub` loads config, opens its own connection, declares the same queue, and creates a consumer.
4. `rabbitmq.Consume` receives one message at a time. On success it calls `Accept`; on a handler error it calls `Requeue`.
5. The subscriber broadcasts each message through `sub.Handler`, which keeps the last 50 messages and pushes them to every connected `/events` client. Slow clients skip messages instead of blocking the consumer.

## Endpoints

### Publisher (`HTTP_PORT=8090` by default)

| Method | Path | Description |
| --- | --- | --- |
| `GET` | `/` | HTML form for sending a message |
| `GET` | `/send-msg?msg=...` | Publish `msg` and return a JSON envelope |

### Subscriber (`HTTP_PORT=8091` by default)

| Method | Path | Description |
| --- | --- | --- |
| `GET` | `/` | HTML page that connects to the SSE stream |
| `GET` | `/events` | `text/event-stream` of `{"message":"..."}` events |

## Environment files

Each command loads its own file from the working directory:

| File | Used by |
| --- | --- |
| `.env.pub` | `cmd/pub` |
| `.env.sub` | `cmd/sub` |

Both are ignored by Git. Create them from `.env.example` and set at least:

```dotenv
RABBITMQ_BROKER_URI=amqp://guest:guest@localhost:5672/
RABBITMQ_QUEUE=hello
HTTP_PORT=8090          # 8091 for .env.sub
LOG_FILE=./logs/app.log
```

Inside Docker, use the Compose service name as the host (`rabbitmq`), not `localhost`, and keep `LOG_FILE` under `/app/logs` so the mounted volume collects it.

`Config.ValidateBroker` checks only `HTTP_PORT`, `RABBITMQ_BROKER_URI`, and `RABBITMQ_QUEUE`. The `DB_*`, `JWT_*`, and `STORAGE_*` keys present in `.env.example` are inherited from the API template and are unused here.

## Run

Through Compose from the repository root (recommended):

```bash
docker compose up --build
```

Local processes (RabbitMQ must already be running on `localhost:5672`):

```bash
go run ./cmd/pub
```

```bash
go run ./cmd/sub
```

Run from this directory so `godotenv` finds `.env.pub` and `.env.sub`.

## Tests

```bash
go test ./...
```

Tests exist for the inherited platform packages. The pub, sub, and rabbitmq packages are exercised manually through the HTTP endpoints and the RabbitMQ Management UI.

## Notes

- The example uses the RabbitMQ AMQP 1.0 client (`rabbitmq-amqp-go-client`), not `streadway/amqp`.
- Broker calls are wrapped in `internal/rabbitmq`; the publisher handler only holds the `*rmq.Publisher` value that the package created.
- A queue delivers each message to a single consumer, so replicas of the subscriber split the work. Use a fanout exchange with per-instance queues when every replica must see every message.
- Delivery has no retry limit. A handler that always fails causes the message to be requeued indefinitely. Add a retry counter or a dead-letter queue for production use.
- `cmd/pub` and `cmd/sub` contain copied `openDatabase` and `setupJWTService` helpers from the API template. They are unused by the RabbitMQ flow.
