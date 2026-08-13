# Writing a Worker

A **worker** is any application that connects to Scheduler, receives tasks, executes business logic, and reports the result.

Scheduler does not require a specific worker implementation or programming language. Workers communicate with Scheduler over **gRPC** and can be written in any language with gRPC support.

This guide explains the worker lifecycle and provides a minimal Python example.

> [!NOTE]
> The Python code in this document is an example only. Your production worker can use any language, framework, or execution model that fits your application.

---

## Worker Lifecycle

At a high level, a worker does four things:

1. Connects to Scheduler.
2. Waits for tasks with `PollTask`.
3. Executes the task.
4. Reports the result with `CompleteTask` or `FailTask`.

For long-running tasks, the worker also sends periodic `Heartbeat` calls while the task is being processed.

```text
PollTask
   ↓
Receive task
   ↓
Execute business logic
   ↓
CompleteTask / FailTask
   ↓
PollTask again
```

---

## gRPC API

Workers use the following methods from `tasks.v1.TaskService`:

| Method         | Purpose                               |
| -------------- | ------------------------------------- |
| `PollTask`     | Wait for an available task            |
| `CompleteTask` | Mark a task as successfully completed |
| `FailTask`     | Report a task failure                 |
| `Heartbeat`    | Extend the lease of a running task    |

No Scheduler-specific SDK is required. The gRPC contract is the interface between Scheduler and the worker.

---

## Python Example

A complete runnable example is available in [`examples/python-worker/worker.py`](../examples/python-worker/worker.py).

The examples below use Python for clarity.

### Requirements

Install the required packages:

```bash
pip install grpcio grpcio-tools protobuf
```

Generate the Python gRPC client from `task.proto`:

```bash
python -m grpc_tools.protoc \
  -I. \
  --python_out=. \
  --grpc_python_out=. \
  task.proto
```

This generates:

* `task_pb2.py`
* `task_pb2_grpc.py`

---

## Connect to Scheduler

Workers communicate with Scheduler over gRPC.

```python
import grpc
import task_pb2
import task_pb2_grpc

channel = grpc.insecure_channel("scheduler:50051")
stub = task_pb2_grpc.TaskServiceStub(channel)
```

Replace the address with the address of your Scheduler instance.

For example, in Docker Compose:

```text
scheduler:50051
```

---

## Poll for Tasks

Use `PollTask` to wait for work.

`PollTask` is a **long-polling request**: the call can remain open until a task becomes available or the request timeout expires.

```python
worker_id = "unique-worker-id"

response = stub.PollTask(
    task_pb2.PollTaskRequest(
        queue_name="default",
        worker_id=worker_id,
    ),
    timeout=30,
)

if response.id:
    print(f"Received task: {response.id}")
    print(f"Payload: {response.payload}")
```

The worker should normally keep calling `PollTask` while it is running.

### Task payload

`response.payload` is an opaque byte string.

Scheduler does not interpret its contents. The worker is responsible for decoding and validating the payload according to the application's own format.

For example, a worker may use JSON:

```python
import json

payload = json.loads(response.payload.decode())
```

---

## Complete a Task

After successful execution, call `CompleteTask`:

```python
stub.CompleteTask(
    task_pb2.CompleteTaskRequest(
        id=response.id,
        worker_id=worker_id,
    )
)
```

A successfully completed task is removed from active processing.

For recurring cron tasks, Scheduler determines the next scheduled execution according to the task configuration.

---

## Fail a Task

When task execution fails, report the failure with `FailTask`:

```python
stub.FailTask(
    task_pb2.FailTaskRequest(
        id=response.id,
        worker_id=worker_id,
        error_message="Something went wrong",
    )
)
```

Scheduler uses the task's retry configuration to determine what happens next.

For example, a task with `max_retries > 0` can be scheduled for another attempt according to the configured retry policy.

> [!IMPORTANT]
> A worker should call `FailTask` when it has actually attempted the task and the execution failed. Errors unrelated to task execution, such as a temporary connection failure to Scheduler itself, should be handled separately by the worker.

---

## Heartbeats for Long-Running Tasks

Tasks that can run longer than their lease duration should send periodic heartbeats.

A heartbeat tells Scheduler that the worker is still actively processing the task and extends its lease.

```python
import threading
import time

def send_heartbeats(task_id, worker_id, interval=5):
    while not stop_event.is_set():
        stub.Heartbeat(
            task_pb2.HeartbeatRequest(
                id=task_id,
                worker_id=worker_id,
                extend_seconds=interval * 2,
            )
        )
        time.sleep(interval)
```

Start the heartbeat loop when task processing begins and stop it after the task finishes.

> [!WARNING]
> If a worker stops sending heartbeats and the task lease expires, Scheduler may consider the worker unavailable and make the task eligible for recovery.

For production workers, heartbeat handling should run independently from the main task execution logic so that a long-running operation does not block heartbeats.

---

## Minimal Worker Loop

The basic worker can be reduced to the following loop:

```python
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
        # Execute your business logic here.
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

This is intentionally minimal. A production worker will typically also need graceful shutdown, connection error handling, structured logging, task-level timeouts, and concurrency control appropriate for the application.

---

## Multiple Workers

Multiple worker instances can consume from the same queue:

```text
                 Scheduler
                /    |    \
               /     |     \
          Worker  Worker  Worker
             1       2       3
```

Each worker should use its own `worker_id`.

Scheduler coordinates task ownership, so multiple workers can safely compete for tasks from the same queue.

For higher throughput, increase the number of worker processes or instances rather than changing the worker protocol.

---

## Queues

Workers poll a specific queue by providing `queue_name`:

```python
task_pb2.PollTaskRequest(
    queue_name="default",
    worker_id=worker_id,
)
```

This allows applications to separate different categories of work.

For example:

```text
default
email
notifications
video-processing
billing
```

Workers should normally be configured to consume only the queues they are responsible for.

---

## Other Languages

The worker protocol is language independent.

To implement a worker in another language:

1. Generate gRPC client code from the Scheduler protobuf definitions.
2. Connect to the Scheduler gRPC endpoint.
3. Implement `PollTask`.
4. Execute the task in your application.
5. Call `CompleteTask` or `FailTask`.
6. Send `Heartbeat` for long-running tasks.

No Scheduler-specific SDK is required.

The protocol can therefore be integrated into an existing service without introducing another application framework or runtime.

---

## Production Considerations

The minimal example is intended to demonstrate the protocol, not to be a complete production worker.

A production implementation should generally consider:

* graceful shutdown;
* connection and RPC error handling;
* structured logging;
* task execution timeouts;
* heartbeat management for long-running jobs;
* worker concurrency limits;
* retry-safe business logic;
* idempotency where duplicate execution is possible;
* monitoring and health checks.

The exact implementation depends on the worker's language and the requirements of the application using Scheduler.
