# Scheduler

[![CI](https://github.com/korjeek/scheduler/actions/workflows/workflow.yml/badge.svg)](https://github.com/korjeek/scheduler/actions)
[![Go Version](https://img.shields.io/badge/Go-1.26-00ADD8?logo=go\&logoColor=white)](https://go.dev/)
[![PostgreSQL](https://img.shields.io/badge/PostgreSQL-17-336791?logo=postgresql\&logoColor=white)](https://www.postgresql.org/)

**A distributed task scheduler for reliable background jobs.**

Scheduler is a standalone service that lets applications **schedule, dispatch, retry, and recover background tasks** without having to build their own job scheduling and delivery infrastructure.

It supports **immediate and delayed execution, recurring cron jobs, automatic retries, long-running tasks, heartbeats, and recovery of tasks from failed workers**.

Workers are external to Scheduler and can be written in **any language that supports gRPC**.

---

## Why Scheduler?

Background jobs become difficult to manage as soon as reliability and scheduling matter.

You may need to:

* execute work in the background without blocking your application;
* schedule tasks for a specific time;
* run recurring jobs;
* retry failed tasks;
* handle workers that crash or disconnect;
* support jobs that run for a long time;
* distribute work across multiple workers;
* scale task processing independently from the application.

Scheduler provides these capabilities as a single service.

Your application creates and schedules tasks.
Your workers execute them.
**Scheduler coordinates the work between them.**

---

## How It Works

The architecture is intentionally simple:

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
    │
    ├── Complete
    ├── Fail
    └── Heartbeat
```

Scheduler stores task state in PostgreSQL and coordinates task delivery between workers.

Workers can use long polling to wait for available tasks instead of continuously querying the database.

If a worker fails while processing a task, Scheduler can detect the expired task lease and make the task available again.

---

## Features

### Flexible Scheduling

Run tasks when you need them:

* **Immediate** — execute as soon as a worker is available.
* **Delayed** — execute at a specific time using `run_at`.
* **Recurring** — execute repeatedly using a cron expression.

### Automatic Retries

Failed tasks can be retried automatically.

Configure the maximum number of retries and Scheduler handles the retry cycle with exponential backoff.

### Long-Running Tasks

Workers can send periodic heartbeats while processing a task.

This allows Scheduler to keep the task active for as long as the worker is making progress.

### Automatic Recovery

If a worker crashes, disconnects, or stops sending heartbeats, its task does not remain stuck indefinitely.

After the task lease expires, Scheduler can make the task available for another worker.

> [!NOTE]
> Worker failures are handled by Scheduler rather than requiring the application to track abandoned tasks itself.

### Distributed Task Processing

Multiple workers can consume tasks from the same queue concurrently.

Scheduler coordinates task ownership using PostgreSQL, so workers can safely compete for available work.

### Horizontal Scaling

Scheduler can run as multiple instances against the same PostgreSQL database.

This allows the scheduling service and task-processing workers to scale independently.

### Automatic Cleanup

Completed tasks can be removed automatically after a configured retention period.

### Language Agnostic

Scheduler communicates with workers through gRPC.

A worker can be written in Go, Python, Java, Rust, Node.js, or any other language with gRPC support.

**Scheduler does not depend on any particular worker implementation.**

---

## Quick Start

The easiest way to run Scheduler locally is Docker Compose.

The following example starts Scheduler and PostgreSQL. A worker is an external component and is shown separately below.

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

> [!TIP]
> `SERVER_DEV_MODE=true` enables Swagger/OpenAPI and gRPC reflection for local development. Disable it in production.

### Endpoints

| Service   | Address                         |
| --------- | ------------------------------- |
| HTTP API  | `http://localhost:8080`         |
| gRPC API  | `localhost:50051`               |
| Liveness  | `http://localhost:9090/healthz` |
| Readiness | `http://localhost:9090/readyz`  |

---

## Creating Tasks

Tasks can be created through HTTP or gRPC.

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

The `payload` is an opaque value from Scheduler's point of view. Your worker is responsible for interpreting it.

The example above uses a base64-encoded JSON payload:

```json
{
  "action": "success"
}
```

---

### Retryable Task

```bash
curl -X POST http://localhost:8080/api/tasks \
  -H "Content-Type: application/json" \
  -d '{
    "name": "retryable-job",
    "max_retries": 3,
    "payload": "eyJhY3Rpb24iOiJmYWlsIn0=",
    "queue_name": "default"
  }'
```

If the worker reports a failure, Scheduler retries the task according to its retry policy.

---

### Delayed Task

Use `run_at` to schedule a task for a specific time:

```bash
curl -X POST http://localhost:8080/api/tasks \
  -H "Content-Type: application/json" \
  -d '{
    "name": "delayed-job",
    "max_retries": 0,
    "payload": "eyJhY3Rpb24iOiJzdWNjZXNzIn0=",
    "queue_name": "default",
    "run_at": "2026-08-13T18:00:00Z"
  }'
```

`run_at` should be an RFC 3339 / ISO 8601 timestamp.

---

### Cron Task

Use `cron` for recurring jobs:

```bash
curl -X POST http://localhost:8080/api/tasks \
  -H "Content-Type: application/json" \
  -d '{
    "name": "cron-every-minute",
    "max_retries": 0,
    "payload": "eyJhY3Rpb24iOiJzdWNjZXNzIn0=",
    "cron": "* * * * *",
    "queue_name": "default"
  }'
```

---

## Worker Integration

A worker is simply a service that consumes tasks from Scheduler and executes the associated business logic.

A typical worker:

```text
PollTask
    │
    ▼
Receive task
    │
    ▼
Execute business logic
    │
    ├──────────────► CompleteTask
    │
    └──────────────► FailTask

For long-running tasks:
    └──────────────► Heartbeat
```

The worker implementation is entirely up to you.

It can be:

* a Go service;
* a Python process;
* a Java application;
* a Rust service;
* a Node.js worker;
* or any other gRPC-compatible process.

The repository includes a small Python worker under `examples/python-worker/` purely as a reference implementation.

---

## API

Scheduler provides both **gRPC** and **HTTP/REST** interfaces.

### gRPC

Service:

```text
tasks.v1.TaskService
```

| Method         | Purpose                    |
| -------------- | -------------------------- |
| `CreateTask`   | Create a task              |
| `GetTask`      | Get a task                 |
| `ListTasks`    | List tasks                 |
| `PollTask`     | Wait for an available task |
| `CompleteTask` | Mark a task as completed   |
| `FailTask`     | Report a failed task       |
| `Heartbeat`    | Keep a running task active |

gRPC is the primary interface for workers.

### HTTP / REST

| Method | Endpoint                       | Purpose       |
| ------ | ------------------------------ | ------------- |
| `POST` | `/api/tasks`                   | Create a task |
| `GET`  | `/api/tasks/{id}`              | Get a task    |
| `GET`  | `/api/tasks?limit=10&offset=0` | List tasks    |

> [!TIP]
> When `SERVER_DEV_MODE=true`, Swagger/OpenAPI and gRPC reflection are enabled for local development.

---

## Configuration

Scheduler can be configured with a YAML file or environment variables.

By default, it looks for:

```text
config.yaml
```

A custom configuration path can be provided with:

```bash
CONFIG_PATH=/path/to/config.yaml
```

Environment variables override values loaded from YAML.

### Common settings

| Variable                     | Description                         | Default                                                                |
| ---------------------------- | ----------------------------------- | ---------------------------------------------------------------------- |
| `DATABASE_CONNECTION_STRING` | PostgreSQL connection string        | `postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable` |
| `SERVER_HTTP_ADDRESS`        | HTTP listen address                 | `0.0.0.0:8080`                                                         |
| `SERVER_GRPC_ADDRESS`        | gRPC listen address                 | `0.0.0.0:50051`                                                        |
| `HEALTH_ADDRESS`             | Health server listen address        | `0.0.0.0:9090`                                                         |
| `SERVER_DEV_MODE`            | Enable Swagger and gRPC reflection  | `false`                                                                |
| `LOGGER_LEVEL`               | `debug`, `info`, `warn`, or `error` | `info`                                                                 |

See [`config/config.example.yaml`](config/config.example.yaml) for the complete list of configuration options.

---

## Scaling

Scheduler is designed to scale horizontally.

You can run multiple Scheduler instances against the same PostgreSQL database and multiple workers against the same queue.

```text
                 PostgreSQL
                     │
          ┌──────────┴──────────┐
          │                     │
      Scheduler            Scheduler
          │                     │
      Workers               Workers
```

Workers compete for tasks safely, while Scheduler instances share the same task state.

This allows you to scale scheduling capacity and task-processing capacity independently.

---

## Health Checks

Scheduler exposes separate liveness and readiness endpoints.

### Liveness

```text
GET /healthz
```

### Readiness

```text
GET /readyz
```

These endpoints can be used by Docker, Kubernetes, load balancers, and other orchestration systems.

---

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

---

## License

This project is licensed under the [MIT License](LICENSE).

---

**Maintained by [korjeek](https://github.com/korjeek).**
