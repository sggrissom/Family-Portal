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
	ErrCodeConflict    ErrorCode = "CONFLICT"
	ErrCodeRateLimited ErrorCode = "RATE_LIMITED"
	ErrCodeUnavailable ErrorCode = "SERVICE_UNAVAILABLE"
)

const unexpectedErrorMessage = "Something went wrong on our end."

const ReferencePrefix = "Reference: "

type AppError struct {
	Code        ErrorCode `json:"code"`
	Message     string    `json:"message"`
	Details     string    `json:"-"`
	RequestId   string    `json:"requestId,omitempty"`
	Timestamp   time.Time `json:"timestamp"`
	RequestPath string    `json:"request_path,omitempty"`
}

func (e AppError) Error() string {
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

type ErrorResponse struct {
	Error   AppError `json:"error"`
	Success bool     `json:"success"`
}

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

func RespondWithError(w http.ResponseWriter, r *http.Request, err *AppError, statusCode int) {
	if r != nil {
		err.RequestPath = r.URL.Path
		err.RequestId = RequestID(r)
	}

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

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	response := ErrorResponse{
		Error:   *err,
		Success: false,
	}

	if encodeErr := json.NewEncoder(w).Encode(response); encodeErr != nil {
		log.Printf("Failed to encode error response: %v", encodeErr)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

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

func RespondUnexpectedError(w http.ResponseWriter, r *http.Request, cause error) {
	details := ""
	if cause != nil {
		details = cause.Error()
	}
	appErr := NewAppError(ErrCodeInternal, unexpectedErrorMessage, details)
	RespondWithError(w, r, appErr, http.StatusInternalServerError)
}

func RespondConflictError(w http.ResponseWriter, r *http.Request, message string) {
	err := NewAppError(ErrCodeConflict, message)
	RespondWithError(w, r, err, http.StatusConflict)
}

func RespondUnavailableError(w http.ResponseWriter, r *http.Request, message string, details ...string) {
	err := NewAppError(ErrCodeUnavailable, message, details...)
	RespondWithError(w, r, err, http.StatusServiceUnavailable)
}

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

var publicErrors = []error{
	ErrAuthFailure,
	ErrFamilyAccessDenied,
	ErrNoFamily,
	ErrFaceAnalysisUnavailable,
	ErrPhotoWorkerUnavailable,
	ErrAdminRequired,
	ErrUserNotFound,
	ErrSeedPasswordRequired,
	ErrSeedDomainInvalid,
	ErrSeedEmailsExist,
	ErrSeedRunNotFound,
	ErrSeedConfirmationMismatch,
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
		return http.StatusBadRequest
	}
}

var ErrFaceAnalysisUnavailable = errors.New("Face analysis is not available on this server")

var ErrPhotoWorkerUnavailable = errors.New("Photo processing is not running on this server")

var ErrAdminRequired = errors.New("Unauthorized: Admin access required")

var ErrUserNotFound = errors.New("No such user")

var ErrSeedPasswordRequired = errors.New("A password for the seeded accounts is required")

var ErrSeedDomainInvalid = errors.New("Email domain must look like example.test")

var ErrSeedEmailsExist = errors.New("Accounts already exist at that email domain")

var ErrSeedRunNotFound = errors.New("No such seed run")

var ErrSeedConfirmationMismatch = errors.New("Type the email domain exactly to confirm")
