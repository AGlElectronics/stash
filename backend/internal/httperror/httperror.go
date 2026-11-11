// Package httperror defines typed service errors and an HTTP renderer that
// turns them into a consistent JSON shape: {"error":{"code","message","details"}}.
package httperror

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/rs/zerolog/log"
)

// Code is a stable string code returned to API clients.
type Code string

const (
	CodeBadRequest   Code = "bad_request"
	CodeValidation   Code = "validation_failed"
	CodeUnauthorized Code = "unauthorized"
	CodeForbidden    Code = "forbidden"
	CodeNotFound     Code = "not_found"
	CodeConflict     Code = "conflict"
	CodeInternal     Code = "internal"
	// The codes of the shared authbase package (decoder, rate limiter),
	// returned unchanged.
	CodeRequestTooLarge      Code = "request_too_large"
	CodeUnsupportedMediaType Code = "unsupported_media_type"
	CodeRateLimited          Code = "rate_limited"
)

// Error is the canonical service error type. Services return *Error or wrap a
// well-known sentinel via Wrap.
type Error struct {
	Code    Code   `json:"code"`
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
	cause   error
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if e.cause != nil {
		return string(e.Code) + ": " + e.Message + ": " + e.cause.Error()
	}
	return string(e.Code) + ": " + e.Message
}

func (e *Error) Unwrap() error { return e.cause }

// New constructs a typed error.
func New(code Code, message string) *Error {
	return &Error{Code: code, Message: message}
}

// WithDetails attaches arbitrary details (e.g. validation issues) to an Error.
func (e *Error) WithDetails(d any) *Error {
	e.Details = d
	return e
}

// WithCause keeps the original wrapped error for logging while exposing only
// Message to the client.
func (e *Error) WithCause(cause error) *Error {
	e.cause = cause
	return e
}

// Common helpers.
var (
	ErrNotFound     = New(CodeNotFound, "resource not found")
	ErrUnauthorized = New(CodeUnauthorized, "authentication required")
	ErrForbidden    = New(CodeForbidden, "permission denied")
)

// NotFound returns an ErrNotFound with a custom resource name.
func NotFound(resource string) *Error {
	return New(CodeNotFound, resource+" not found")
}

// Validation builds a validation-failed error with details.
func Validation(message string, details any) *Error {
	return New(CodeValidation, message).WithDetails(details)
}

// Conflict builds a conflict error.
func Conflict(message string) *Error {
	return New(CodeConflict, message)
}

// BadRequest builds a bad request error.
func BadRequest(message string) *Error {
	return New(CodeBadRequest, message)
}

// Internal wraps an internal error; the client only sees a generic message.
func Internal(cause error) *Error {
	return New(CodeInternal, "internal server error").WithCause(cause)
}

type response struct {
	Error *Error `json:"error"`
}

// Render writes err as JSON. If err is not an *Error, it is treated as internal.
func Render(w http.ResponseWriter, r *http.Request, err error) {
	var e *Error
	if !errors.As(err, &e) {
		e = Internal(err)
	}

	status := statusFor(e.Code)
	if status >= 500 {
		log.Ctx(r.Context()).Error().Err(err).
			Str("path", r.URL.Path).
			Str("method", r.Method).
			Msg("http handler error")
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(response{Error: e})
}

func statusFor(code Code) int {
	switch code {
	case CodeBadRequest:
		return http.StatusBadRequest
	case CodeValidation:
		return http.StatusUnprocessableEntity
	case CodeUnauthorized:
		return http.StatusUnauthorized
	case CodeForbidden:
		return http.StatusForbidden
	case CodeNotFound:
		return http.StatusNotFound
	case CodeConflict:
		return http.StatusConflict
	case CodeRequestTooLarge:
		return http.StatusRequestEntityTooLarge
	case CodeUnsupportedMediaType:
		return http.StatusUnsupportedMediaType
	case CodeRateLimited:
		return http.StatusTooManyRequests
	default:
		return http.StatusInternalServerError
	}
}
