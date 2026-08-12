package server

import (
	"context"
	"errors"
	"scheduler/internal/domain/entities"
	"time"

	"scheduler/internal/usecases/task"
	"scheduler/pkg/pb"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type TaskHandler struct {
	pb.UnimplementedTaskServiceServer
	taskService TaskService
	cronParser  CronParser
}

func NewTaskHandler(svc TaskService, parser CronParser) *TaskHandler {
	return &TaskHandler{
		taskService: svc,
		cronParser:  parser,
	}
}

func taskStatusEntityToProto(status entities.TaskStatus) pb.TaskStatus {
	switch status {
	case entities.StatusPending:
		return pb.TaskStatus_TASK_STATUS_PENDING
	case entities.StatusRunning:
		return pb.TaskStatus_TASK_STATUS_RUNNING
	case entities.StatusSuccess:
		return pb.TaskStatus_TASK_STATUS_SUCCESS
	case entities.StatusFailed:
		return pb.TaskStatus_TASK_STATUS_FAILED
	default:
		return pb.TaskStatus_TASK_STATUS_UNSPECIFIED
	}
}

func taskToPbResponse(t *entities.Task) *pb.GetTaskResponse {
	resp := &pb.GetTaskResponse{
		Id:         t.ID.String(),
		Name:       t.Name,
		Cron:       t.Cron,
		Status:     taskStatusEntityToProto(t.Status),
		MaxRetries: int32(t.MaxRetries),
	}
	if t.NextRunAt != nil {
		resp.NextRunAt = timestamppb.New(*t.NextRunAt)
	}
	if t.CreatedAt != nil {
		resp.CreatedAt = timestamppb.New(*t.CreatedAt)
	}
	return resp
}

func (h *TaskHandler) CreateTask(ctx context.Context, req *pb.CreateTaskRequest) (*pb.CreateTaskResponse, error) {
	if req.GetCron() != "" {
		if _, err := h.cronParser.Parse(req.GetCron()); err != nil {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
	}

	var runAt *time.Time
	if req.GetRunAt() != nil {
		runAt = new(req.GetRunAt().AsTime().UTC())
	}

	d := task.CreateDto{
		Name:       req.GetName(),
		QueueName:  req.GetQueueName(),
		MaxRetries: int(req.GetMaxRetries()),
		Payload:    req.GetPayload(),
		Cron:       req.GetCron(),
		RunAt:      runAt,
	}

	output, err := h.taskService.Create(ctx, d)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &pb.CreateTaskResponse{
		Id:        output.ID.String(),
		Name:      output.Name,
		Status:    taskStatusEntityToProto(output.Status),
		NextRunAt: timestamppb.New(*output.NextRunAt),
	}, nil
}

func (h *TaskHandler) GetTask(ctx context.Context, req *pb.GetTaskRequest) (*pb.GetTaskResponse, error) {
	t, err := h.taskService.Get(ctx, req.GetId())
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return taskToPbResponse(t), nil
}

func (h *TaskHandler) ListTasks(ctx context.Context, req *pb.ListTasksRequest) (*pb.ListTasksResponse, error) {
	limit := int(req.GetLimit())
	offset := int(req.GetOffset())

	tasks, err := h.taskService.List(ctx, limit, offset)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	pbTasks := make([]*pb.GetTaskResponse, 0, len(tasks))
	for _, t := range tasks {
		pbTasks = append(pbTasks, taskToPbResponse(&t))
	}

	return &pb.ListTasksResponse{
		Tasks: pbTasks,
	}, nil
}

func (h *TaskHandler) PollTask(ctx context.Context, req *pb.PollTaskRequest) (*pb.PollTaskResponse, error) {
	t, err := h.taskService.Poll(ctx, req.GetQueueName(), req.GetWorkerId())
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return &pb.PollTaskResponse{}, nil
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	if t == nil {
		return &pb.PollTaskResponse{}, nil
	}

	return &pb.PollTaskResponse{
		Id:         t.ID.String(),
		QueueName:  t.QueueName,
		Payload:    t.Payload,
		Attempt:    int32(t.RetryCount + 1),
		MaxRetries: int32(t.MaxRetries),
	}, nil
}

func (h *TaskHandler) CompleteTask(ctx context.Context, req *pb.CompleteTaskRequest) (*pb.CompleteTaskResponse, error) {
	err := h.taskService.Complete(ctx, req.GetId(), req.GetWorkerId())
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &pb.CompleteTaskResponse{}, nil
}

func (h *TaskHandler) FailTask(ctx context.Context, req *pb.FailTaskRequest) (*pb.FailTaskResponse, error) {
	err := h.taskService.Fail(ctx, req.GetId(), req.GetWorkerId(), req.GetErrorMessage())
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &pb.FailTaskResponse{}, nil
}

func (h *TaskHandler) Heartbeat(ctx context.Context, req *pb.HeartbeatRequest) (*pb.HeartbeatResponse, error) {
	err := h.taskService.Heartbeat(ctx, req.GetId(), req.GetWorkerId(), time.Duration(req.GetExtendSeconds())*time.Second)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &pb.HeartbeatResponse{}, nil
}
