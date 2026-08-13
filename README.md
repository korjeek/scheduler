# Scheduler

[![CI](https://github.com/korjeek/scheduler/actions/workflows/workflow.yml/badge.svg)](https://github.com/korjeek/scheduler/actions)
[![Go Version](https://img.shields.io/badge/Go-1.26-00ADD8?logo=go\&logoColor=white)](https://go.dev/)
[![PostgreSQL](https://img.shields.io/badge/PostgreSQL-17-336791?logo=postgresql\&logoColor=white)](https://www.postgresql.org/)

**A distributed task scheduler for reliable background jobs.**

Scheduler is a standalone service for creating, scheduling, dispatching, retrying, and recovering background tasks.

Applications create tasks. Workers execute them. Scheduler coordinates the work between them.

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

volumes:
  pg_data:
```

```bash
docker compose up -d
```

| Service   | Address                         |
| --------- | ------------------------------- |
| HTTP API  | `http://localhost:8080`         |
| gRPC API  | `localhost:50051`               |
| Health    | `http://localhost:9090/healthz` |
| Readiness | `http://localhost:9090/readyz`  |

`SERVER_DEV_MODE=true` enables Swagger/OpenAPI and gRPC reflection for local development.

## Creating Tasks

### Immediate

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

### Delayed

Set `run_at` to schedule a task for a specific time:

```json
{
  "name": "delayed-job",
  "run_at": "2026-08-13T18:00:00Z",
  "queue_name": "default"
}
```

### Recurring

Set `cron` to run a task on a schedule:

```json
{
  "name": "cron-job",
  "cron": "* * * * *",
  "queue_name": "default"
}
```

The task payload is opaque to Scheduler and is interpreted by the worker.

## Worker Integration

Workers consume tasks over gRPC and report completion, failure, or heartbeats.

See [Workers](workers.md) for the worker protocol and examples.

## API

Scheduler exposes both HTTP/REST and gRPC interfaces.

See [API Reference](api.md) for endpoints, gRPC methods, and request/response formats.

## Configuration

Configuration can be provided through `config.yaml` or environment variables.

See [`config/config.example.yaml`](config/config.example.yaml) for the available options.

## Development

Requirements:

* Go 1.26+
* Docker
* PostgreSQL 17+

Run unit tests:

```bash
go test ./... -tags=unit
```

Run integration tests:

```bash
go test ./... -tags=integration
```

Run the smoke test:

```bash
./scripts/smoke-test.sh
```

## License

[MIT License](LICENSE)

---

**Maintained by [korjeek](https://github.com/korjeek).**
