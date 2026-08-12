package server

import "fmt"

type SrvError struct {
	Message string
	Err     error
}

func (e *SrvError) Error() string {
	return fmt.Sprintf("[SERVER] %s: %v", e.Message, e.Err)
}

func (e *SrvError) Unwrap() error {
	return e.Err
}

func NewSrvError(msg string, err error) *SrvError {
	return &SrvError{
		Message: msg,
		Err:     err,
	}
}
