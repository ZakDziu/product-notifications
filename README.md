# Product Notifications

Two Go microservices communicating through a message broker: `products` (CRUD API) and `notifications` (event consumer).

## Goal

Build a small event-driven system where:

- `products` exposes REST endpoints and writes to PostgreSQL.
- every create/delete action emits an event to RabbitMQ.
- `notifications` consumes and logs those events.
- Prometheus counters track product create/delete operations.

## Stack

| Component        | Choice                    | Reason                                                    |
|------------------|---------------------------|-----------------------------------------------------------|
| Language         | Go                        | Simple deployment, fast startup, clear concurrency model  |
| HTTP framework   | Gin                       | Lightweight router with middleware support                 |
| Database         | PostgreSQL + raw SQL      | Explicit query control, no ORM per requirements           |
| Migrations       | golang-migrate            | Deterministic schema versioning with SQL up/down files    |
| Message broker   | RabbitMQ                  | Reliable async communication, easy local setup            |
| Metrics          | Prometheus client_golang  | De-facto standard for Go service metrics                  |

## Architecture

### Services

- `products`
  - `POST /products` — create product
  - `GET /products?page=&limit=` — list with pagination
  - `DELETE /products/:id` — delete product
  - `GET /metrics` — Prometheus metrics
  - `GET /healthz` — health check (DB ping)
- `notifications`
  - subscribes to queue `products.events`
  - logs received messages

### Event flow

```
Client → POST /products → DB insert → publish "product_created" → RabbitMQ → notifications (log)
Client → DELETE /products/1 → DB delete → publish "product_deleted" → RabbitMQ → notifications (log)
```

### Event payload

```json
{
  "event_type": "product_created",
  "product_id": 1,
  "name": "iPhone 16",
  "timestamp": "2026-02-24T12:00:00Z"
}
```

## Repository structure

```
cmd/
  products/          Products service entrypoint
  notifications/     Notifications service entrypoint
internal/
  config/            Centralized env config loading and validation
  products/
    http/            Gin handlers, router, middleware
    service/         Business logic
    repository/      PostgreSQL queries (raw SQL)
    messaging/       RabbitMQ publisher
  notifications/     RabbitMQ consumer
docs/                Auto-generated Swagger/OpenAPI spec
migrations/
  products/          SQL migration files (up/down)
```

## Quick start

### 1. Configure environment

```bash
cp .env.example .env
```

### 2. Start all services

```bash
docker compose up --build
```

### Endpoints

| URL                                    | Description               |
|----------------------------------------|---------------------------|
| `http://localhost:8080`                | Products API              |
| `http://localhost:8080/swagger/index.html` | Swagger UI            |
| `http://localhost:8080/metrics`        | Prometheus metrics        |
| `http://localhost:8080/healthz`        | Health check              |
| `http://localhost:15672`               | RabbitMQ UI (guest/guest) |

## API

### Create product

```bash
curl -s -X POST http://localhost:8080/products \
  -H "Content-Type: application/json" \
  -d '{"name":"iPhone 16"}'
```

Response (`201 Created`):

```json
{
  "id": 1,
  "name": "iPhone 16",
  "created_at": "2026-02-24T12:00:00Z"
}
```

### List products

```bash
curl -s "http://localhost:8080/products?page=1&limit=10"
```

Response (`200 OK`):

```json
{
  "items": [
    {"id": 1, "name": "iPhone 16", "created_at": "2026-02-24T12:00:00Z"}
  ],
  "pagination": {
    "page": 1,
    "limit": 10,
    "total": 1
  }
}
```

### Delete product

```bash
curl -s -X DELETE http://localhost:8080/products/1
```

Response: `204 No Content`

### Error responses

```json
{"error": "product not found"}
```

Status codes: `400` (bad request), `404` (not found), `500` (internal error).

## Environment variables

| Variable                   | Required | Default               | Description                          |
|----------------------------|----------|-----------------------|--------------------------------------|
| `DATABASE_URL`             | yes      | —                     | PostgreSQL connection string         |
| `RABBITMQ_URL`             | yes      | —                     | AMQP connection string               |
| `HTTP_ADDR`                | no       | `:8080`               | Products HTTP listen address         |
| `MIGRATIONS_PATH`          | no       | `migrations/products` | Path to SQL migration files          |

See `.env.example` for Docker Compose variables (image versions, ports).

## Local run (without Docker)

Make sure PostgreSQL and RabbitMQ are running locally, then:

```bash
cp .env.example .env
# edit .env to point DATABASE_URL and RABBITMQ_URL to localhost

go run ./cmd/products
go run ./cmd/notifications
```

## Migrations

Migrations run automatically on `products` service startup. To manage them manually:

```bash
# create a new migration (auto-increments sequence number)
make migrate-create NAME=add_price_column

# apply all pending migrations
make migrate-up

# rollback the last migration
make migrate-down

# check current version
make migrate-status
```

Requires the [migrate CLI](https://github.com/golang-migrate/migrate/tree/master/cmd/migrate) for manual commands. In Docker, migrations are applied automatically via Go code on startup.

## Testing

Two test layers cover the product domain:

- **Service layer** (`internal/products/service`) — business logic, error paths, pagination edge cases, publish-failure resilience.
- **HTTP handler layer** (`internal/products/http`) — request parsing, status codes, error mapping via `httptest`.

All tests are table-driven with `t.Run()` subtests.

```bash
CGO_ENABLED=0 go test -v ./...
```

## Swagger / OpenAPI

API documentation is auto-generated from code annotations using [swaggo/swag](https://github.com/swaggo/swag).

Swagger UI is available at `http://localhost:8080/swagger/index.html` when the service is running.

To regenerate after changing handler annotations:

```bash
make swagger
```

## Engineering decisions

- **Error wrapping**: every error is wrapped with `fmt.Errorf("context: %w", err)` for debuggable error chains.
- **Dependency inversion**: handler depends on `ProductService` interface, service depends on `Repository` and `Publisher` interfaces.
- **Domain errors**: `ErrNotFound` and `ErrInvalidName` live in the domain package — no cross-layer imports for error matching.
- **Publish failure resilience**: if the broker is down, the product is still created/deleted. Publish errors are logged, not propagated to the client.
- **Manual ack**: notifications consumer uses manual acknowledgement — messages are re-queued on processing failure.
- **Typed responses**: all HTTP responses use typed structs for type safety and documentation.
- **Config validation**: both services validate required env vars at startup and fail fast.
- **Graceful shutdown**: signal-aware lifecycle (`SIGINT`/`SIGTERM`) with configurable shutdown timeouts.
- **Structured logging**: JSON logs via `log/slog` consistently across both services.
- **Request traceability**: `X-Request-ID` middleware for each HTTP request.
- **Operational endpoints**: `/healthz` with DB ping, `/metrics` with Prometheus counters.
- **DB connection pool**: explicit `MaxOpenConns`, `MaxIdleConns`, `ConnMaxLifetime` tuning.

## Trade-offs and improvements

Current implementation is intentionally compact for the test task. For production, I would add:

- outbox pattern for guaranteed delivery between DB write and event publish
- integration tests with real PostgreSQL/RabbitMQ via testcontainers
- dead-letter queue and consumer retry policy with backoff
- OpenTelemetry tracing across services
