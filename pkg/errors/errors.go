package errors

import (
	"context"
	"errors"
	"fmt"
)

type StreamError struct {
	Code     string
	Message  string
	Cause    error
	StreamID string
}

func (e *StreamError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *StreamError) Unwrap() error { return e.Cause }

const (
	ErrCodeStreamNotFound      = "STREAM_NOT_FOUND"
	ErrCodeStreamAlreadyExists = "STREAM_EXISTS"
	ErrCodeRateLimitExceeded   = "RATE_LIMIT_EXCEEDED"
	ErrCodeInvalidConfig       = "INVALID_CONFIG"
	ErrCodeConnectionFailed    = "CONNECTION_FAILED"
	ErrCodeResourceExhausted   = "RESOURCE_EXHAUSTED"
	ErrCodeTimeout             = "TIMEOUT"
	ErrCodeCancelled           = "CANCELLED"
)

func ErrStreamNotFound(streamID string) *StreamError {
	return &StreamError{
		Code:     ErrCodeStreamNotFound,
		Message:  "stream not found",
		StreamID: streamID,
	}
}

func ErrInvalidConfig(msg string, cause error) *StreamError {
	return &StreamError{
		Code:    ErrCodeInvalidConfig,
		Message: msg,
		Cause:   cause,
	}
}

func ErrConnectionFailed(msg string, cause error) *StreamError {
	return &StreamError{
		Code:    ErrCodeConnectionFailed,
		Message: msg,
		Cause:   cause,
	}
}

func ErrResourceExhausted(msg string) *StreamError {
	return &StreamError{
		Code:    ErrCodeResourceExhausted,
		Message: msg,
	}
}

func ErrStreamAlreadyExists(streamID string) *StreamError {
	return &StreamError{
		Code:     ErrCodeStreamAlreadyExists,
		Message:  "stream already exists",
		StreamID: streamID,
	}
}

func IsContextError(err error) bool {
	return errors.Is(err, context.Canceled) ||
		errors.Is(err, context.DeadlineExceeded)
}
