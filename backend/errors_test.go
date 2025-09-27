package backend

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestAppError(t *testing.T) {
	t.Run("Error method returns formatted string", func(t *testing.T) {
		err := AppError{
			Code:    ErrCodeValidation,
			Message: "Test validation error",
		}

		expected := "[VALIDATION_ERROR] Test validation error"
		if err.Error() != expected {
			t.Errorf("Expected error string '%s', got '%s'", expected, err.Error())
		}
	})

	t.Run("Error with different codes", func(t *testing.T) {
		testCases := []struct {
			code     ErrorCode
			message  string
			expected string
		}{
			{ErrCodeAuth, "Auth failed", "[AUTH_ERROR] Auth failed"},
			{ErrCodeNotFound, "Resource not found", "[NOT_FOUND] Resource not found"},
			{ErrCodeForbidden, "Access denied", "[FORBIDDEN] Access denied"},
			{ErrCodeInternal, "Internal error", "[INTERNAL_ERROR] Internal error"},
		}

		for _, tc := range testCases {
			err := AppError{Code: tc.code, Message: tc.message}
			if err.Error() != tc.expected {
				t.Errorf("Expected '%s', got '%s'", tc.expected, err.Error())
			}
		}
	})
}

func TestNewAppError(t *testing.T) {
	t.Run("Create error without details", func(t *testing.T) {
		err := NewAppError(ErrCodeValidation, "Test message")

		if err.Code != ErrCodeValidation {
			t.Errorf("Expected code %s, got %s", ErrCodeValidation, err.Code)
		}

		if err.Message != "Test message" {
			t.Errorf("Expected message 'Test message', got '%s'", err.Message)
		}

		if err.Details != "" {
			t.Errorf("Expected empty details, got '%s'", err.Details)
		}

		if err.Timestamp.IsZero() {
			t.Error("Expected timestamp to be set")
		}

		// Check timestamp is recent (within last minute)
		if time.Since(err.Timestamp) > time.Minute {
			t.Error("Expected timestamp to be recent")
		}
	})

	t.Run("Create error with details", func(t *testing.T) {
		err := NewAppError(ErrCodeInternal, "Test message", "Additional details")

		if err.Details != "Additional details" {
			t.Errorf("Expected details 'Additional details', got '%s'", err.Details)
		}
	})

	t.Run("Create error with multiple details (only first used)", func(t *testing.T) {
		err := NewAppError(ErrCodeBadRequest, "Test message", "First detail", "Second detail")

		if err.Details != "First detail" {
			t.Errorf("Expected details 'First detail', got '%s'", err.Details)
		}
	})
}

func TestRespondWithError(t *testing.T) {
	t.Run("Standard error response", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/test-path", nil)

		err := NewAppError(ErrCodeValidation, "Test validation error", "Field is required")
		RespondWithError(recorder, req, err, http.StatusBadRequest)

		// Check status code
		if recorder.Code != http.StatusBadRequest {
			t.Errorf("Expected status %d, got %d", http.StatusBadRequest, recorder.Code)
		}

		// Check content type
		contentType := recorder.Header().Get("Content-Type")
		if contentType != "application/json" {
			t.Errorf("Expected Content-Type 'application/json', got '%s'", contentType)
		}

		// Parse response
		var response ErrorResponse
		if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		// Check response structure
		if response.Success != false {
			t.Error("Expected success to be false")
		}

		if response.Error.Code != ErrCodeValidation {
			t.Errorf("Expected error code %s, got %s", ErrCodeValidation, response.Error.Code)
		}

		if response.Error.Message != "Test validation error" {
			t.Errorf("Expected message 'Test validation error', got '%s'", response.Error.Message)
		}

		if response.Error.Details != "Field is required" {
			t.Errorf("Expected details 'Field is required', got '%s'", response.Error.Details)
		}

		if response.Error.RequestPath != "/test-path" {
			t.Errorf("Expected request path '/test-path', got '%s'", response.Error.RequestPath)
		}

		if response.Error.Timestamp.IsZero() {
			t.Error("Expected timestamp to be set")
		}
	})

	t.Run("Error response with nil request", func(t *testing.T) {
		recorder := httptest.NewRecorder()

		err := NewAppError(ErrCodeInternal, "Internal error")
		RespondWithError(recorder, nil, err, http.StatusInternalServerError)

		if recorder.Code != http.StatusInternalServerError {
			t.Errorf("Expected status %d, got %d", http.StatusInternalServerError, recorder.Code)
		}

		var response ErrorResponse
		if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		// Request path should be empty when request is nil
		if response.Error.RequestPath != "" {
			t.Errorf("Expected empty request path, got '%s'", response.Error.RequestPath)
		}
	})
}

func TestRespondAuthError(t *testing.T) {
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/protected", nil)

	RespondAuthError(recorder, req, "Authentication required")

	if recorder.Code != http.StatusUnauthorized {
		t.Errorf("Expected status %d, got %d", http.StatusUnauthorized, recorder.Code)
	}

	var response ErrorResponse
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response.Error.Code != ErrCodeAuth {
		t.Errorf("Expected error code %s, got %s", ErrCodeAuth, response.Error.Code)
	}

	if response.Error.Message != "Authentication required" {
		t.Errorf("Expected message 'Authentication required', got '%s'", response.Error.Message)
	}
}

func TestRespondValidationError(t *testing.T) {
	t.Run("Validation error without details", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest("POST", "/api/data", nil)

		RespondValidationError(recorder, req, "Invalid input")

		if recorder.Code != http.StatusBadRequest {
			t.Errorf("Expected status %d, got %d", http.StatusBadRequest, recorder.Code)
		}

		var response ErrorResponse
		if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if response.Error.Code != ErrCodeValidation {
			t.Errorf("Expected error code %s, got %s", ErrCodeValidation, response.Error.Code)
		}

		if response.Error.Message != "Invalid input" {
			t.Errorf("Expected message 'Invalid input', got '%s'", response.Error.Message)
		}

		if response.Error.Details != "" {
			t.Errorf("Expected empty details, got '%s'", response.Error.Details)
		}
	})

	t.Run("Validation error with details", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest("POST", "/api/data", nil)

		RespondValidationError(recorder, req, "Invalid input", "Email field is required")

		var response ErrorResponse
		if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if response.Error.Details != "Email field is required" {
			t.Errorf("Expected details 'Email field is required', got '%s'", response.Error.Details)
		}
	})
}

func TestRespondNotFoundError(t *testing.T) {
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/user/999", nil)

	RespondNotFoundError(recorder, req, "User not found")

	if recorder.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, recorder.Code)
	}

	var response ErrorResponse
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response.Error.Code != ErrCodeNotFound {
		t.Errorf("Expected error code %s, got %s", ErrCodeNotFound, response.Error.Code)
	}

	if response.Error.Message != "User not found" {
		t.Errorf("Expected message 'User not found', got '%s'", response.Error.Message)
	}
}

func TestRespondForbiddenError(t *testing.T) {
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest("DELETE", "/api/admin/users", nil)

	RespondForbiddenError(recorder, req, "Admin access required")

	if recorder.Code != http.StatusForbidden {
		t.Errorf("Expected status %d, got %d", http.StatusForbidden, recorder.Code)
	}

	var response ErrorResponse
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response.Error.Code != ErrCodeForbidden {
		t.Errorf("Expected error code %s, got %s", ErrCodeForbidden, response.Error.Code)
	}

	if response.Error.Message != "Admin access required" {
		t.Errorf("Expected message 'Admin access required', got '%s'", response.Error.Message)
	}
}

func TestRespondInternalError(t *testing.T) {
	t.Run("Internal error without details", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest("POST", "/api/process", nil)

		RespondInternalError(recorder, req, "Database connection failed")

		if recorder.Code != http.StatusInternalServerError {
			t.Errorf("Expected status %d, got %d", http.StatusInternalServerError, recorder.Code)
		}

		var response ErrorResponse
		if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if response.Error.Code != ErrCodeInternal {
			t.Errorf("Expected error code %s, got %s", ErrCodeInternal, response.Error.Code)
		}

		if response.Error.Message != "Database connection failed" {
			t.Errorf("Expected message 'Database connection failed', got '%s'", response.Error.Message)
		}
	})

	t.Run("Internal error with details", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest("POST", "/api/process", nil)

		RespondInternalError(recorder, req, "Database connection failed", "Connection timeout after 30s")

		var response ErrorResponse
		if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if response.Error.Details != "Connection timeout after 30s" {
			t.Errorf("Expected details 'Connection timeout after 30s', got '%s'", response.Error.Details)
		}
	})
}

func TestRespondFileTooLargeError(t *testing.T) {
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/upload", nil)

	RespondFileTooLargeError(recorder, req, "10MB")

	if recorder.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, recorder.Code)
	}

	var response ErrorResponse
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response.Error.Code != ErrCodeTooLarge {
		t.Errorf("Expected error code %s, got %s", ErrCodeTooLarge, response.Error.Code)
	}

	expectedMessage := "File too large. Maximum size is 10MB"
	if response.Error.Message != expectedMessage {
		t.Errorf("Expected message '%s', got '%s'", expectedMessage, response.Error.Message)
	}
}

func TestRespondInvalidFileTypeError(t *testing.T) {
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/upload", nil)

	RespondInvalidFileTypeError(recorder, req, "JPEG, PNG, GIF")

	if recorder.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, recorder.Code)
	}

	var response ErrorResponse
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response.Error.Code != ErrCodeInvalidType {
		t.Errorf("Expected error code %s, got %s", ErrCodeInvalidType, response.Error.Code)
	}

	expectedMessage := "Invalid file type. Allowed types: JPEG, PNG, GIF"
	if response.Error.Message != expectedMessage {
		t.Errorf("Expected message '%s', got '%s'", expectedMessage, response.Error.Message)
	}
}

func TestErrorCodes(t *testing.T) {
	// Verify all error codes are properly defined
	testCases := []struct {
		code     ErrorCode
		expected string
	}{
		{ErrCodeAuth, "AUTH_ERROR"},
		{ErrCodeValidation, "VALIDATION_ERROR"},
		{ErrCodeNotFound, "NOT_FOUND"},
		{ErrCodeForbidden, "FORBIDDEN"},
		{ErrCodeInternal, "INTERNAL_ERROR"},
		{ErrCodeBadRequest, "BAD_REQUEST"},
		{ErrCodeTooLarge, "FILE_TOO_LARGE"},
		{ErrCodeInvalidType, "INVALID_FILE_TYPE"},
	}

	for _, tc := range testCases {
		if string(tc.code) != tc.expected {
			t.Errorf("Expected error code '%s', got '%s'", tc.expected, string(tc.code))
		}
	}
}

func TestErrorResponseJSONFormat(t *testing.T) {
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)

	RespondValidationError(recorder, req, "Test error", "Test details")

	// Check the raw JSON structure
	responseBody := recorder.Body.String()

	// Verify it contains expected JSON fields
	expectedFields := []string{
		`"success":false`,
		`"error":{`,
		`"code":"VALIDATION_ERROR"`,
		`"message":"Test error"`,
		`"details":"Test details"`,
		`"timestamp"`,
		`"request_path":"/test"`,
	}

	for _, field := range expectedFields {
		if !strings.Contains(responseBody, field) {
			t.Errorf("Expected JSON to contain '%s', got: %s", field, responseBody)
		}
	}
}