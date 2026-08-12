package poller

import "fmt"

type Error struct {
	Message string
	Err     error
}

func (e *Error) Error() string {
	return fmt.Sprintf("[POLLER] %s: %v", e.Message, e.Err)
}

func (e *Error) Unwrap() error {
	return e.Err
}

func NewPollerError(msg string, err error) *Error {
	return &Error{
		Message: msg,
		Err:     err,
	}
}
