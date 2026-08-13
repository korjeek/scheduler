# API Reference

Scheduler exposes two APIs:

* **gRPC** — primary API for workers and programmatic integrations.
* **HTTP/REST** — management and simple integrations.

The gRPC contract is defined by [`api/proto/tasks/v1/task.proto`](../api/proto/tasks/v1/task.proto).

This file is the source of truth for the gRPC service, messages, enums, and wire contract.

## gRPC

### Service

```text
tasks.v1.TaskService
```

| Method         | Purpose                     |
| -------------- | --------------------------- |
| `CreateTask`   | Create a task               |
| `GetTask`      | Get a task by ID            |
| `ListTasks`    | List tasks                  |
| `PollTask`     | Wait for an available task  |
| `CompleteTask` | Mark a task as completed    |
| `FailTask`     | Report a task failure       |
| `Heartbeat`    | Extend a running task lease |

Workers primarily use `PollTask`, `CompleteTask`, `FailTask`, and `Heartbeat`.

See [Workers](docs/workers.md) for the worker lifecycle and examples.

### Client Generation

Clients should be generated from the protobuf definition using the standard protobuf/gRPC tooling for the target language.

For example, Python:

```bash
python -m grpc_tools.protoc \
  -I api/proto \
  --python_out=. \
  --grpc_python_out=. \
  api/proto/tasks/v1/task.proto
```

This generates the Python message and gRPC client code required to communicate with Scheduler.

> [!NOTE]
> Generated client files do not need to be copied from the Scheduler repository. Generate them from the version of `task.proto` used by your integration.

## HTTP / REST

The HTTP API provides the management operations and uses JSON.

Base URL:

```text
http://localhost:8080
```

### Create Task

```http
POST /api/tasks
Content-Type: application/json
```

Example:

```json
{
  "name": "example",
  "queue_name": "default",
  "payload": "eyJhY3Rpb24iOiJzdWNjZXNzIn0=",
  "max_retries": 3
}
```

`payload` is an opaque byte sequence and is base64-encoded in JSON.

Delayed and recurring tasks can use `run_at` and `cron`:

```json
{
  "name": "scheduled-task",
  "queue_name": "default",
  "payload": "eyJhY3Rpb24iOiJzdWNjZXNzIn0=",
  "run_at": "2026-08-13T18:00:00Z"
}
```

```json
{
  "name": "recurring-task",
  "queue_name": "default",
  "payload": "eyJhY3Rpb24iOiJzdWNjZXNzIn0=",
  "cron": "* * * * *"
}
```

### Get Task

```http
GET /api/tasks/{id}
```

### List Tasks

```http
GET /api/tasks?limit=10&offset=0
```

The HTTP API does not expose the worker operations `PollTask`, `CompleteTask`, `FailTask`, or `Heartbeat`.

Use gRPC for workers.

## Examples

### Create a Task with curl

```bash
curl -X POST http://localhost:8080/api/tasks \
  -H "Content-Type: application/json" \
  -d '{
    "name": "hello",
    "queue_name": "default",
    "payload": "eyJhY3Rpb24iOiJzdWNjZXNzIn0="
  }'
```

### Poll a Task with grpcurl

```bash
grpcurl -plaintext \
  -d '{"queue_name":"default","worker_id":"550e8400-e29b-41d4-a716-446655440000"}' \
  localhost:50051 \
  tasks.v1.TaskService/PollTask
```

## Local Development

When `SERVER_DEV_MODE=true`, Scheduler enables Swagger/OpenAPI for the HTTP API and gRPC reflection.

Swagger can be used to explore the HTTP API interactively.

gRPC reflection allows tools such as `grpcurl` to discover the service without manually providing the protobuf definitions.

For the exact Swagger endpoint, use the route exposed by the running Scheduler instance.
