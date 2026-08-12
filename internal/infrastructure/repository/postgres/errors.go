package postgres

import "fmt"

type DbError struct {
	Message   string
	Operation string
	Err       error
}

func (e *DbError) Error() string {
	return fmt.Sprintf("[DB] %s during %s: %v", e.Message, e.Operation, e.Err)
}

func (e *DbError) Unwrap() error {
	return e.Err
}

func NewDbError(msg, op string, err error) *DbError {
	return &DbError{
		Message:   msg,
		Operation: op,
		Err:       err,
	}
}
