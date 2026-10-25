// Package apperr defines the application error type and the stable error codes
// used by every endpoint. Handlers map these codes to HTTP statuses.
package apperr

import (
	"errors"
	"fmt"
	"net/http"
)

// Error codes from PRD Section 6.4. Clients switch on these strings.
const (
	CodeValidation        = "VALIDATION_ERROR"
	CodeUnauthorized      = "UNAUTHORIZED"
	CodeForbidden         = "FORBIDDEN"
	CodeNotFound          = "NOT_FOUND"
	CodeConflict          = "CONFLICT"
	CodeDuplicateRequest  = "DUPLICATE_REQUEST"
	CodeUnprocessable     = "UNPROCESSABLE_ENTITY"
	CodeAccountLocked     = "ACCOUNT_LOCKED"
	CodeRateLimited       = "RATE_LIMITED"
	CodeInternal          = "INTERNAL_ERROR"
	CodeInvalidToken      = "INVALID_TOKEN"
	CodeExpiredToken      = "TOKEN_EXPIRED"
	CodeInviteExpired     = "INVITE_EXPIRED"
	CodeInviteUsed        = "INVITE_ALREADY_USED"
	CodeConsentMissing    = "CONSENT_MISSING"
	CodeSummaryNotApprove = "SUMMARY_NOT_APPROVED"
	CodeEmptyPresentation = "EMPTY_PRESENTATION"
	CodeNoEligibleRecords = "NO_ELIGIBLE_RECORDS"
	CodeAIProviderTimeout = "AI_PROVIDER_TIMEOUT"
	CodeAIUnavailable     = "AI_PROVIDER_UNAVAILABLE"
	CodeShiftOverbooked   = "SHIFT_OVERBOOKED"
	CodeCapacityExceeded  = "CAPACITY_EXCEEDED"
	CodeInvalidCredentials = "INVALID_CREDENTIALS"
	CodeAccountInactive   = "ACCOUNT_INACTIVE"
)

// AppError is a safe-to-return error carrying a stable code, an HTTP status,
// a client-facing message and an optional internal cause that is only logged.
type AppError struct {
	Code    string
	Status  int
	Message string
	Fields  map[string][]string
	Err     error
}

// Error implements the error interface. Only the safe message is exposed.
func (e *AppError) Error() string { return e.Message }

// Unwrap exposes the internal cause to errors.Is/As without leaking it.
func (e *AppError) Unwrap() error { return e.Err }

// WithFields attaches per-field validation messages.
func (e *AppError) WithFields(fields map[string][]string) *AppError {
	e.Fields = fields
	return e
}

// WithCause records an internal cause for logging.
func (e *AppError) WithCause(err error) *AppError {
	e.Err = err
	return e
}

func newErr(code string, status int, msg string) *AppError {
	return &AppError{Code: code, Status: status, Message: msg}
}

// Validation builds a 400 validation error.
func Validation(msg string) *AppError { return newErr(CodeValidation, http.StatusBadRequest, msg) }

// ValidationFields builds a 400 validation error with field details.
func ValidationFields(msg string, fields map[string][]string) *AppError {
	return newErr(CodeValidation, http.StatusBadRequest, msg).WithFields(fields)
}

// Unauthorized builds a 401 error.
func Unauthorized(msg string) *AppError {
	return newErr(CodeUnauthorized, http.StatusUnauthorized, msg)
}

// InvalidCredentials builds a 401 error for failed logins.
func InvalidCredentials() *AppError {
	return newErr(CodeInvalidCredentials, http.StatusUnauthorized, "Email or password is incorrect")
}

// Forbidden builds a 403 error.
func Forbidden(msg string) *AppError { return newErr(CodeForbidden, http.StatusForbidden, msg) }

// NotFound builds a 404 error.
func NotFound(entity string) *AppError {
	return newErr(CodeNotFound, http.StatusNotFound, fmt.Sprintf("%s not found", entity))
}

// Conflict builds a 409 error.
func Conflict(msg string) *AppError { return newErr(CodeConflict, http.StatusConflict, msg) }

// DuplicateRequest builds the 409 duplicate warning for prayer requests.
func DuplicateRequest(msg string) *AppError {
	return newErr(CodeDuplicateRequest, http.StatusConflict, msg)
}

// Unprocessable builds a 422 error with a business-rule code.
func Unprocessable(code, msg string) *AppError {
	return newErr(code, http.StatusUnprocessableEntity, msg)
}

// Locked builds a 423 account-locked error.
func Locked(msg string) *AppError { return newErr(CodeAccountLocked, http.StatusLocked, msg) }

// RateLimited builds a 429 error.
func RateLimited(msg string) *AppError {
	return newErr(CodeRateLimited, http.StatusTooManyRequests, msg)
}

// Internal builds a 500 error. The cause is logged, never returned.
func Internal(cause error) *AppError {
	return newErr(CodeInternal, http.StatusInternalServerError, "Something went wrong. Please try again.").WithCause(cause)
}

// From converts any error into an *AppError, defaulting to 500.
func From(err error) *AppError {
	if err == nil {
		return nil
	}
	var ae *AppError
	if errors.As(err, &ae) {
		return ae
	}
	return Internal(err)
}

// Is reports whether err carries the given code.
func Is(err error, code string) bool {
	var ae *AppError
	if errors.As(err, &ae) {
		return ae.Code == code
	}
	return false
}

// Sentinel errors used by repositories.
var (
	ErrNotFound      = errors.New("record not found")
	ErrDuplicate     = errors.New("duplicate record")
	ErrConflict      = errors.New("conflicting record")
	ErrUnauthorized  = errors.New("unauthorized")
	ErrCapacityFull  = errors.New("capacity full")
	ErrInvalidInput  = errors.New("invalid input")
	ErrNotImplemented = errors.New("not implemented")
)
