package task

import "fmt"

type ServiceError struct {
	Message   string
	Operation string
	Err       error
}

func (e *ServiceError) Error() string {
	return fmt.Sprintf("[TASK SERVICE] %s during %s: %v", e.Message, e.Operation, e.Err)
}

func (e *ServiceError) Unwrap() error {
	return e.Err
}

func NewServiceError(msg, op string, err error) *ServiceError {
	return &ServiceError{
		Message:   msg,
		Operation: op,
		Err:       err,
	}
}
