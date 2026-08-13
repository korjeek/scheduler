# Scheduler

[![CI](https://github.com/korjeek/scheduler/actions/workflows/workflow.yml/badge.svg)](https://github.com/korjeek/scheduler/actions)
[![Coverage](https://codecov.io/gh/korjeek/scheduler/graph/badge.svg)](https://codecov.io/gh/korjeek/scheduler)
[![Go Version](https://img.shields.io/badge/Go-1.26-00ADD8?logo=go\&logoColor=white)](https://go.dev/)
[![PostgreSQL](https://img.shields.io/badge/PostgreSQL-17-336791?logo=postgresql\&logoColor=white)](https://www.postgresql.org/)
[![Latest Release](https://img.shields.io/github/v/release/korjeek/scheduler)](https://github.com/korjeek/scheduler/releases)

**A distributed task scheduler for reliable background jobs.**

Scheduler is a standalone service for creating, scheduling, dispatching, retrying, and recovering background tasks.

Applications create tasks. Workers execute them. Scheduler coordinates the work between them.

[Getting Started](#quick-start) · [API](docs/api.md) · [Workers](docs/workers.md) · [Configuration](#configuration)

## Architecture

```text
Application
    │
    │ Create / Schedule Task
    ▼
Scheduler
    │
    │ Deliver Task
    ▼
Worker
```

Scheduler stores task state in PostgreSQL and coordinates task ownership between workers.

Workers communicate with Scheduler over gRPC and can be written in any language with gRPC support.

## Features

* Immediate and delayed tasks
* Recurring cron jobs
* Automatic retries with backoff
* Long-running tasks with heartbeats
* Recovery of tasks from failed workers
* Multiple workers and Scheduler instances
* Automatic task retention and cleanup
* HTTP/REST and gRPC APIs

## Quick Start

The easiest way to run Scheduler locally is Docker Compose.

### `docker-compose.yml`

```yaml
services:
  db:
    image: postgres:17-alpine
    environment:
      POSTGRES_DB: postgres
      POSTGRES_USER: postgres
      POSTGRES_PASSWORD: postgres
    volumes:
      - pg_data:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U postgres"]
      interval: 10s
      timeout: 5s
      retries: 5
    networks:
      - scheduler-net

  scheduler:
    image: ghcr.io/korjeek/scheduler:main
    environment:
      DATABASE_CONNECTION_STRING: "postgres://postgres:postgres@db:5432/postgres?sslmode=disable"
      SERVER_HTTP_ADDRESS: "0.0.0.0:8080"
      SERVER_GRPC_ADDRESS: "0.0.0.0:50051"
      HEALTH_ADDRESS: "0.0.0.0:9090"
      SERVER_DEV_MODE: "true"
    ports:
      - "8080:8080"
      - "50051:50051"
      - "9090:9090"
    depends_on:
      db:
        condition: service_healthy
    networks:
      - scheduler-net

volumes:
  pg_data:

networks:
  scheduler-net:
    driver: bridge
```

Start the service:

```bash
docker compose up -d
```

| Service   | Address                         |
| --------- | ------------------------------- |
| HTTP API  | `http://localhost:8080`         |
| gRPC API  | `localhost:50051`               |
| Liveness  | `http://localhost:9090/healthz` |
| Readiness | `http://localhost:9090/readyz`  |

> [!TIP]
> `SERVER_DEV_MODE=true` enables Swagger/OpenAPI and gRPC reflection for local development. Disable it in production.

## Creating Tasks

Tasks can be created through the HTTP API or gRPC.

### Immediate Task

```bash
curl -X POST http://localhost:8080/api/tasks \
  -H "Content-Type: application/json" \
  -d '{
    "name": "quick-job",
    "max_retries": 1,
    "payload": "eyJhY3Rpb24iOiJzdWNjZXNzIn0=",
    "queue_name": "default"
  }'
```

The payload is opaque to Scheduler and is interpreted by the worker.

### Delayed Task

Set `run_at` to an RFC 3339 timestamp:

```json
{
  "name": "delayed-job",
  "run_at": "2026-08-13T18:00:00Z",
  "queue_name": "default"
}
```

### Recurring Task

Set `cron` to a cron expression:

```json
{
  "name": "cron-job",
  "cron": "* * * * *",
  "queue_name": "default"
}
```

See [API Reference](docs/api.md) for the full task model and request formats.

## Workers

Workers consume tasks over gRPC and report completion, failure, or heartbeats.

See [Workers](docs/workers.md) for the worker lifecycle, protocol, and examples.

> [!NOTE]
> Workers are independent services and can be written in any language with gRPC support.

## API

Scheduler exposes both HTTP/REST and gRPC interfaces.

See [API Reference](docs/api.md) for endpoints, gRPC methods, and request/response formats.

## Configuration

Configuration can be provided through `config.yaml` or environment variables.

Environment variables override values loaded from YAML.

See [`config/config.yaml`](config/config.yaml) for the complete list of options.

## Health Checks

```text
GET /healthz
GET /readyz
```

These endpoints can be used by containers, load balancers, and orchestration systems.

## Development

### Requirements

* Go 1.26+
* Docker
* PostgreSQL 17+ when running PostgreSQL outside Docker

### Tests

Run unit tests:

```bash
go test ./... -tags=unit
```

Run integration tests:

```bash
go test ./... -tags=integration
```

Run the end-to-end smoke test:

```bash
./scripts/smoke-test.sh
```

## License

This project is licensed under the [MIT License](LICENSE).

---

**Maintained by [korjeek](https://github.com/korjeek).**
