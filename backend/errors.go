package backend

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"runtime"
	"time"
)

// ErrorCode represents standardized error codes
type ErrorCode string

const (
	ErrCodeAuth        ErrorCode = "AUTH_ERROR"
	ErrCodeValidation  ErrorCode = "VALIDATION_ERROR"
	ErrCodeNotFound    ErrorCode = "NOT_FOUND"
	ErrCodeForbidden   ErrorCode = "FORBIDDEN"
	ErrCodeInternal    ErrorCode = "INTERNAL_ERROR"
	ErrCodeBadRequest  ErrorCode = "BAD_REQUEST"
	ErrCodeTooLarge    ErrorCode = "FILE_TOO_LARGE"
	ErrCodeInvalidType ErrorCode = "INVALID_FILE_TYPE"
	// ErrCodeConflict is for a request that is well formed and permitted but
	// contradicts the current state — a name already taken, a member already
	// removed. It is not the client's fault and not a server fault.
	ErrCodeConflict ErrorCode = "CONFLICT"
	// ErrCodeRateLimited mirrors what the rate limiter returns, so a client can
	// tell "slow down" apart from "you did something wrong".
	ErrCodeRateLimited ErrorCode = "RATE_LIMITED"
	// ErrCodeUnavailable is a dependency being down — face analysis, AI import,
	// push. The request was fine and retrying later may work.
	ErrCodeUnavailable ErrorCode = "SERVICE_UNAVAILABLE"
)

// unexpectedErrorMessage is what a user sees when something failed in a way
// nobody anticipated. It says nothing about what — the reference code is how the
// specifics get from the server log to me without going through the browser.
const unexpectedErrorMessage = "Something went wrong on our end."

// ReferencePrefix introduces the correlation id in a user-facing message. The
// frontend matches on it to pull the code out for its copy button, so the two
// have to agree; it is exported for the test that pins that.
const ReferencePrefix = "Reference: "

// AppError represents a structured application error.
//
// Details is deliberately not serialized. It is where call sites put the
// underlying error — decode failures, filesystem paths, database messages — and
// none of that belongs in a response. It goes to the log instead, joined to the
// response by RequestId.
type AppError struct {
	Code        ErrorCode `json:"code"`
	Message     string    `json:"message"`
	Details     string    `json:"-"`
	RequestId   string    `json:"requestId,omitempty"`
	Timestamp   time.Time `json:"timestamp"`
	RequestPath string    `json:"request_path,omitempty"`
}

// Error implements the error interface
func (e AppError) Error() string {
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// ErrorResponse represents the JSON response for errors
type ErrorResponse struct {
	Error   AppError `json:"error"`
	Success bool     `json:"success"`
}

// NewAppError creates a new application error
func NewAppError(code ErrorCode, message string, details ...string) *AppError {
	err := &AppError{
		Code:      code,
		Message:   message,
		Timestamp: time.Now(),
	}

	if len(details) > 0 {
		err.Details = details[0]
	}

	return err
}

// RespondWithError sends a standardized error response
func RespondWithError(w http.ResponseWriter, r *http.Request, err *AppError, statusCode int) {
	// Add request path to error for context
	if r != nil {
		err.RequestPath = r.URL.Path
		err.RequestId = RequestID(r)
	}

	// Log the error with context
	_, file, line, ok := runtime.Caller(1)
	caller := "unknown"
	if ok {
		caller = fmt.Sprintf("%s:%d", file, line)
	}

	data := map[string]interface{}{
		"caller": caller,
		"error":  err.Error(),
		"code":   err.Code,
	}
	if err.RequestId != "" {
		data["requestId"] = err.RequestId
	}
	// The one place the underlying cause is recorded. It never reaches the
	// response; this log line and the reference code are how it gets to me.
	if err.Details != "" {
		data["details"] = err.Details
	}

	if r != nil {
		data["method"] = r.Method
		data["path"] = r.URL.Path
		LogErrorWithRequest(r, LogCategorySystem, err.Error(), data)
	} else {
		LogErrorSimple(LogCategorySystem, err.Error(), data)
	}

	// Set response headers
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	// Send JSON response
	response := ErrorResponse{
		Error:   *err,
		Success: false,
	}

	if encodeErr := json.NewEncoder(w).Encode(response); encodeErr != nil {
		log.Printf("Failed to encode error response: %v", encodeErr)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// Helper functions for common error responses

func RespondAuthError(w http.ResponseWriter, r *http.Request, message string) {
	err := NewAppError(ErrCodeAuth, message)
	RespondWithError(w, r, err, http.StatusUnauthorized)
}

func RespondValidationError(w http.ResponseWriter, r *http.Request, message string, details ...string) {
	err := NewAppError(ErrCodeValidation, message, details...)
	RespondWithError(w, r, err, http.StatusBadRequest)
}

func RespondNotFoundError(w http.ResponseWriter, r *http.Request, message string) {
	err := NewAppError(ErrCodeNotFound, message)
	RespondWithError(w, r, err, http.StatusNotFound)
}

func RespondForbiddenError(w http.ResponseWriter, r *http.Request, message string) {
	err := NewAppError(ErrCodeForbidden, message)
	RespondWithError(w, r, err, http.StatusForbidden)
}

func RespondInternalError(w http.ResponseWriter, r *http.Request, message string, details ...string) {
	err := NewAppError(ErrCodeInternal, message, details...)
	RespondWithError(w, r, err, http.StatusInternalServerError)
}

// RespondUnexpectedError is the handler-side counterpart to a panic: the
// request failed for a reason the caller cannot act on and should not see. The
// underlying error goes to the log with the request's correlation id; the
// response carries a fixed message and that same id.
//
// Prefer this over RespondInternalError with a bespoke message. A user cannot
// do anything differently on "failed to marshal export data" than on "something
// went wrong", and the first one tells a stranger what the server is made of.
func RespondUnexpectedError(w http.ResponseWriter, r *http.Request, cause error) {
	details := ""
	if cause != nil {
		details = cause.Error()
	}
	appErr := NewAppError(ErrCodeInternal, unexpectedErrorMessage, details)
	RespondWithError(w, r, appErr, http.StatusInternalServerError)
}

// RespondConflictError reports a request that cannot be applied to the current
// state.
func RespondConflictError(w http.ResponseWriter, r *http.Request, message string) {
	err := NewAppError(ErrCodeConflict, message)
	RespondWithError(w, r, err, http.StatusConflict)
}

// RespondUnavailableError reports a dependency being down. Retrying later is
// the honest advice, so the message should say so.
func RespondUnavailableError(w http.ResponseWriter, r *http.Request, message string, details ...string) {
	err := NewAppError(ErrCodeUnavailable, message, details...)
	RespondWithError(w, r, err, http.StatusServiceUnavailable)
}

// ProcError converts an error a procedure hit into one that can safely cross
// the wire. vbeam writes a proc's error straight into the response body, so an
// unwrapped filesystem or database error is shown to whoever triggered it.
//
// Errors declared in this package are the vocabulary the frontend matches on
// (see the Err* constants in frontend/server.ts); those pass through unchanged.
// Anything else is logged with a correlation id and replaced by a message
// naming only that id.
//
// It mints its own id rather than reusing the request's, because a vbeam.Context
// carries the transaction and the token and no request. For a procedure the
// reference in the message is the authoritative one; the X-Request-Id header
// still identifies the HTTP request that carried it.
func ProcError(cause error) error {
	if cause == nil {
		return nil
	}
	if isPublicError(cause) {
		return cause
	}

	id := NewRequestID()
	LogErrorSimple(LogCategorySystem, "Unexpected procedure error", map[string]interface{}{
		"requestId": id,
		"error":     cause.Error(),
	})
	return errors.New(unexpectedErrorMessage + " " + ReferencePrefix + id)
}

// publicErrors are the errors whose text is part of the API: the frontend
// compares against these exact strings, and they describe the caller's own
// request rather than the server's insides.
var publicErrors = []error{
	ErrAuthFailure,
	ErrFamilyAccessDenied,
	ErrNoFamily,
	ErrFaceAnalysisUnavailable,
	ErrPhotoWorkerUnavailable,
	ErrAdminRequired,
}

func isPublicError(cause error) bool {
	for _, known := range publicErrors {
		if errors.Is(cause, known) {
			return true
		}
	}
	return false
}

func RespondFileTooLargeError(w http.ResponseWriter, r *http.Request, maxSize string) {
	message := fmt.Sprintf("File too large. Maximum size is %s", maxSize)
	err := NewAppError(ErrCodeTooLarge, message)
	RespondWithError(w, r, err, http.StatusBadRequest)
}

func RespondInvalidFileTypeError(w http.ResponseWriter, r *http.Request, allowedTypes string) {
	message := fmt.Sprintf("Invalid file type. Allowed types: %s", allowedTypes)
	err := NewAppError(ErrCodeInvalidType, message)
	RespondWithError(w, r, err, http.StatusBadRequest)
}

// statusForErrorCode maps a code to the status that goes with it, for the
// handlers that build an AppError before they know which responder to use.
func statusForErrorCode(code ErrorCode) int {
	switch code {
	case ErrCodeAuth:
		return http.StatusUnauthorized
	case ErrCodeForbidden:
		return http.StatusForbidden
	case ErrCodeNotFound:
		return http.StatusNotFound
	case ErrCodeConflict:
		return http.StatusConflict
	case ErrCodeRateLimited:
		return http.StatusTooManyRequests
	case ErrCodeUnavailable:
		return http.StatusServiceUnavailable
	case ErrCodeInternal:
		return http.StatusInternalServerError
	default:
		// VALIDATION_ERROR, BAD_REQUEST, FILE_TOO_LARGE, INVALID_FILE_TYPE.
		return http.StatusBadRequest
	}
}

// ErrFaceAnalysisUnavailable is returned when an action needs the face
// recognition daemon and it is not running: a local build, or a release build
// whose socket was unreachable at startup. It is a declared error so the
// frontend can say something specific rather than showing a generic failure.
var ErrFaceAnalysisUnavailable = errors.New("Face analysis is not available on this server")

// ErrPhotoWorkerUnavailable is returned when an action needs the photo
// processing worker and it is not running. Queueing into a stopped worker would
// report a count of photos that nothing will ever pick up.
var ErrPhotoWorkerUnavailable = errors.New("Photo processing is not running on this server")

// ErrAdminRequired is the single instance of the admin gate's error. The panel
// is gated on user 1 — fine for one operator, but the check was written out
// sixteen times with nineteen copies of this string, and one of those copies
// getting edited was a matter of time.
var ErrAdminRequired = errors.New("Unauthorized: Admin access required")
