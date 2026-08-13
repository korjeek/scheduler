# Writing a Worker

A **worker** is an application that connects to Scheduler, receives tasks, executes them, and reports the result.

Workers communicate with Scheduler over **gRPC**. They can be written in any language with gRPC support.

> [!NOTE]
> Scheduler does not provide or require a worker SDK. The gRPC service definition is the worker interface.

## Worker Lifecycle

The worker loop is simple:

```text
PollTask
   ↓
Execute task
   ↓
CompleteTask / FailTask
   ↓
PollTask again
```

For long-running tasks, the worker also sends `Heartbeat` calls while the task is running.

The worker-facing API consists of:

| Method         | Purpose                         |
| -------------- | ------------------------------- |
| `PollTask`     | Wait for a task                 |
| `CompleteTask` | Report successful execution     |
| `FailTask`     | Report failed execution         |
| `Heartbeat`    | Keep a running task lease alive |

See [API Reference](api.md) for the full gRPC contract.

## Python Example

A complete runnable example is available in [`examples/python-worker/worker.py`](../examples/python-worker/worker.py).

The following shows the core worker loop:

```python
import grpc
import task_pb2
import task_pb2_grpc

worker_id = "worker-1"

channel = grpc.insecure_channel("scheduler:50051")
stub = task_pb2_grpc.TaskServiceStub(channel)

while True:
    response = stub.PollTask(
        task_pb2.PollTaskRequest(
            queue_name="default",
            worker_id=worker_id,
        ),
        timeout=30,
    )

    if not response.id:
        continue

    try:
        # Decode and execute the task payload.
        process_task(response)

        stub.CompleteTask(
            task_pb2.CompleteTaskRequest(
                id=response.id,
                worker_id=worker_id,
            )
        )

    except Exception as exc:
        stub.FailTask(
            task_pb2.FailTaskRequest(
                id=response.id,
                worker_id=worker_id,
                error_message=str(exc),
            )
        )
```

`PollTask` is a long-polling request, so a worker can wait for work without continuously querying the database.

## Task Payload

The payload is opaque to Scheduler.

The worker is responsible for decoding and validating it according to the application's format.

For example:

```python
import json

payload = json.loads(response.payload.decode())
```

Scheduler does not assume JSON, protobuf, or any other payload format.

## Long-Running Tasks

A task that can outlive its lease must send periodic heartbeats:

```python
stub.Heartbeat(
    task_pb2.HeartbeatRequest(
        id=task_id,
        worker_id=worker_id,
        extend_seconds=30,
    )
)
```

> [!WARNING]
> If heartbeats stop and the task lease expires, Scheduler may make the task available for recovery by another worker.

Heartbeat handling should run independently from the task execution path so that long-running work cannot prevent heartbeats from being sent.

## Queues

Workers select a queue when polling:

```python
task_pb2.PollTaskRequest(
    queue_name="default",
    worker_id=worker_id,
)
```

Different workers can consume different queues, allowing applications to separate types of work.

## Multiple Workers

Multiple workers can consume from the same queue:

```text
                 Scheduler
                /    |    \
          Worker  Worker  Worker
             1       2       3
```

Each worker should use its own `worker_id`.

Scheduler coordinates task ownership, so workers can safely compete for available tasks.

## Production

The example above only demonstrates the protocol.

A production worker should also handle the concerns appropriate to its environment, especially graceful shutdown, RPC failures, concurrency, task timeouts, heartbeat management, logging, and idempotent task execution.

> [!TIP]
> Tasks may be executed again after worker failure and lease expiration. Business logic should therefore be safe to retry where possible.
