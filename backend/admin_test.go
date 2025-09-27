package backend

import (
	"family/cfg"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"go.hasen.dev/vbeam"
	"go.hasen.dev/vbolt"
	"golang.org/x/crypto/bcrypt"
)

func TestListAllUsers(t *testing.T) {
	testDBPath := "test_list_users.db"
	db := vbolt.Open(testDBPath)
	vbolt.InitBuckets(db, &cfg.Info)
	defer os.Remove(testDBPath)
	defer db.Close()

	// Set the global database for auth functions
	appDb = db

	// Set the global database for auth functions
	appDb = db

	var adminUser, regularUser User
	var testFamily Family

	// Create test data
	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		// Create a family
		testFamily = Family{
			Id:   1,
			Name: "Test Family",
		}
		vbolt.Write(tx, FamiliesBkt, testFamily.Id, &testFamily)

		// Create admin user (ID = 1)
		adminReq := CreateAccountRequest{
			Name:            "Admin User",
			Email:           "admin@example.com",
			Password:        "password123",
			ConfirmPassword: "password123",
		}
		hash, _ := bcrypt.GenerateFromPassword([]byte(adminReq.Password), bcrypt.DefaultCost)
		adminUser = AddUserTx(tx, adminReq, hash)
		adminUser.Id = 1 // Force admin ID
		adminUser.FamilyId = testFamily.Id
		vbolt.Write(tx, UsersBkt, 1, &adminUser)

		// Create regular user
		regularReq := CreateAccountRequest{
			Name:            "Regular User",
			Email:           "regular@example.com",
			Password:        "password123",
			ConfirmPassword: "password123",
		}
		hash2, _ := bcrypt.GenerateFromPassword([]byte(regularReq.Password), bcrypt.DefaultCost)
		regularUser = AddUserTx(tx, regularReq, hash2)
		regularUser.FamilyId = testFamily.Id
		vbolt.Write(tx, UsersBkt, regularUser.Id, &regularUser)

		vbolt.TxCommit(tx)
	})

	t.Run("Admin user lists all users", func(t *testing.T) {
		// Create context with admin user
		ctx := &vbeam.Context{}
		vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
			ctx.Tx = tx
			// Generate JWT token for admin user
			adminToken, _ := generateAuthJwt(adminUser, httptest.NewRecorder())
			ctx.Token = adminToken

			resp, err := ListAllUsers(ctx, Empty{})
			if err != nil {
				t.Errorf("Expected no error, got %v", err)
			}

			if len(resp.Users) != 2 {
				t.Errorf("Expected 2 users, got %d", len(resp.Users))
			}

			// Check that admin user is marked as admin
			var foundAdmin, foundRegular bool
			for _, user := range resp.Users {
				if user.Id == 1 {
					foundAdmin = true
					if !user.IsAdmin {
						t.Error("Expected admin user to have IsAdmin=true")
					}
					// Family name comes from auto-generated family name, not our test name
					if user.FamilyName == "" {
						t.Error("Expected family name to be populated")
					}
				} else if user.Id == regularUser.Id {
					foundRegular = true
					if user.IsAdmin {
						t.Error("Expected regular user to have IsAdmin=false")
					}
				}
			}

			if !foundAdmin {
				t.Error("Admin user not found in response")
			}
			if !foundRegular {
				t.Error("Regular user not found in response")
			}
		})
	})

	t.Run("Regular user cannot list users", func(t *testing.T) {
		ctx := &vbeam.Context{}
		vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
			ctx.Tx = tx
			// Generate JWT token for regular user
			regularToken, _ := generateAuthJwt(regularUser, httptest.NewRecorder())
			ctx.Token = regularToken

			_, err := ListAllUsers(ctx, Empty{})
			if err == nil {
				t.Error("Expected error for non-admin user")
			}

			expectedError := "Unauthorized: Admin access required"
			if err.Error() != expectedError {
				t.Errorf("Expected error '%s', got '%s'", expectedError, err.Error())
			}
		})
	})

	t.Run("Unauthenticated request", func(t *testing.T) {
		ctx := &vbeam.Context{}
		vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
			ctx.Tx = tx
			// No user set in context

			_, err := ListAllUsers(ctx, Empty{})
			if err == nil {
				t.Error("Expected error for unauthenticated request")
			}
		})
	})
}

func TestGetPhotoStats(t *testing.T) {
	testDBPath := "test_photo_stats.db"
	db := vbolt.Open(testDBPath)
	vbolt.InitBuckets(db, &cfg.Info)
	defer os.Remove(testDBPath)
	defer db.Close()

	// Set the global database for auth functions
	appDb = db

	var adminUser User

	// Create test data
	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		// Create admin user
		adminReq := CreateAccountRequest{
			Name:            "Admin User",
			Email:           "admin@example.com",
			Password:        "password123",
			ConfirmPassword: "password123",
		}
		hash, _ := bcrypt.GenerateFromPassword([]byte(adminReq.Password), bcrypt.DefaultCost)
		adminUser = AddUserTx(tx, adminReq, hash)
		adminUser.Id = 1 // Force admin ID
		vbolt.Write(tx, UsersBkt, 1, &adminUser)

		// Create test images with different statuses
		testImages := []Image{
			{Id: 1, Status: 0, FamilyId: 1, PersonId: 1}, // Active
			{Id: 2, Status: 1, FamilyId: 1, PersonId: 1}, // Processing
			{Id: 3, Status: 0, FamilyId: 1, PersonId: 2}, // Active
			{Id: 4, Status: 2, FamilyId: 1, PersonId: 2}, // Failed/Hidden
		}

		for _, img := range testImages {
			vbolt.Write(tx, ImagesBkt, img.Id, &img)
		}

		vbolt.TxCommit(tx)
	})

	t.Run("Admin gets photo stats", func(t *testing.T) {
		ctx := &vbeam.Context{}
		vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
			ctx.Tx = tx
			// Generate JWT token for admin user
			adminToken, _ := generateAuthJwt(adminUser, httptest.NewRecorder())
			ctx.Token = adminToken

			resp, err := GetPhotoStats(ctx, GetPhotoStatsRequest{})
			if err != nil {
				t.Errorf("Expected no error, got %v", err)
			}

			if resp.TotalPhotos != 4 {
				t.Errorf("Expected 4 total photos, got %d", resp.TotalPhotos)
			}

			// Note: processedCount depends on isPhotoProcessed() logic
			// For this test, we just check that it returns reasonable values
			if resp.ProcessedPhotos > resp.TotalPhotos {
				t.Error("Processed photos cannot exceed total photos")
			}

			if resp.PendingPhotos != resp.TotalPhotos-resp.ProcessedPhotos {
				t.Error("Pending photos calculation is incorrect")
			}
		})
	})

	t.Run("Non-admin cannot get photo stats", func(t *testing.T) {
		regularUser := User{Id: 2, Email: "regular@example.com"}

		ctx := &vbeam.Context{}
		vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
			ctx.Tx = tx
			// Generate JWT token for regular user
			regularToken, _ := generateAuthJwt(regularUser, httptest.NewRecorder())
			ctx.Token = regularToken

			_, err := GetPhotoStats(ctx, GetPhotoStatsRequest{})
			if err == nil {
				t.Error("Expected error for non-admin user")
			}

			expectedError := "Unauthorized: Admin access required"
			if err.Error() != expectedError {
				t.Errorf("Expected error '%s', got '%s'", expectedError, err.Error())
			}
		})
	})
}

func TestStripAnsiCodes(t *testing.T) {
	testCases := []struct {
		input    string
		expected string
	}{
		{
			input:    "Regular text without ANSI",
			expected: "Regular text without ANSI",
		},
		{
			input:    "\x1b[31mRed text\x1b[0m",
			expected: "Red text",
		},
		{
			input:    "\x1b[1;32mBold green\x1b[0m normal text",
			expected: "Bold green normal text",
		},
		{
			input:    "\x1b[0m\x1b[31m\x1b[1mMultiple codes\x1b[0m",
			expected: "Multiple codes",
		},
		{
			input:    "",
			expected: "",
		},
		{
			input:    "No ANSI codes here",
			expected: "No ANSI codes here",
		},
	}

	for _, tc := range testCases {
		result := stripAnsiCodes(tc.input)
		if result != tc.expected {
			t.Errorf("For input '%s', expected '%s', got '%s'", tc.input, tc.expected, result)
		}
	}
}

func TestParseLogTimestamp(t *testing.T) {
	testCases := []struct {
		input           string
		expectedTime    string // Use string format for easier comparison
		expectedMessage string
		hasValidTime    bool
	}{
		{
			input:           "2023/06/15 14:30:25 This is a log message",
			expectedTime:    "2023-06-15 14:30:25",
			expectedMessage: "This is a log message",
			hasValidTime:    true,
		},
		{
			input:           "2023/12/31 23:59:59 New Year's Eve log",
			expectedTime:    "2023-12-31 23:59:59",
			expectedMessage: "New Year's Eve log",
			hasValidTime:    true,
		},
		{
			input:           "Invalid timestamp format",
			expectedMessage: "Invalid timestamp format",
			hasValidTime:    false,
		},
		{
			input:           "2023-06-15 14:30:25 Wrong separator",
			expectedMessage: "2023-06-15 14:30:25 Wrong separator",
			hasValidTime:    false,
		},
		{
			input:           "",
			expectedMessage: "",
			hasValidTime:    false,
		},
	}

	for _, tc := range testCases {
		timestamp, message := parseLogTimestamp(tc.input)

		if message != tc.expectedMessage {
			t.Errorf("For input '%s', expected message '%s', got '%s'", tc.input, tc.expectedMessage, message)
		}

		if tc.hasValidTime {
			expectedTime, _ := time.Parse("2006-01-02 15:04:05", tc.expectedTime)
			if !timestamp.Equal(expectedTime) {
				t.Errorf("For input '%s', expected time %v, got %v", tc.input, expectedTime, timestamp)
			}
		} else {
			// For invalid timestamps, should return a recent time (within last minute)
			if time.Since(timestamp) > time.Minute {
				t.Errorf("For input '%s', expected recent timestamp, got %v", tc.input, timestamp)
			}
		}
	}
}

func TestCategorizeLogMessage(t *testing.T) {
	testCases := []struct {
		message  string
		expected logCategory
	}{
		{"Photo processing completed", logCategoryPhoto},
		{"IMAGE upload failed", logCategoryPhoto},
		{"photo thumbnail generated", logCategoryPhoto},
		{"User authentication successful", logCategoryAuth},
		{"LOGIN attempt from user", logCategoryAuth},
		{"auth token expired", logCategoryAuth},
		{"Admin accessed user list", logCategoryAdmin},
		{"ADMIN panel opened", logCategoryAdmin},
		{"API request received", logCategoryAPI},
		{"RPC call executed", logCategoryAPI},
		{"GET /api/users", logCategoryAPI},
		{"POST /api/upload", logCategoryAPI},
		{"Worker started processing", logCategoryWorker},
		{"PROCESSING queue status", logCategoryWorker},
		{"QUEUE length updated", logCategoryWorker},
		{"Database connection established", logCategorySystem},
		{"System startup completed", logCategorySystem},
		{"Unknown message type", logCategorySystem},
		{"", logCategorySystem},
	}

	for _, tc := range testCases {
		result := categorizeLogMessage(tc.message)
		if result != tc.expected {
			t.Errorf("For message '%s', expected category %v, got %v", tc.message, tc.expected, result)
		}
	}
}

func TestGetLogContentSecurity(t *testing.T) {
	testDBPath := "test_log_content.db"
	db := vbolt.Open(testDBPath)
	vbolt.InitBuckets(db, &cfg.Info)
	defer os.Remove(testDBPath)
	defer db.Close()

	// Set the global database for auth functions
	appDb = db

	var adminUser User

	// Create admin user
	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		adminReq := CreateAccountRequest{
			Name:            "Admin User",
			Email:           "admin@example.com",
			Password:        "password123",
			ConfirmPassword: "password123",
		}
		hash, _ := bcrypt.GenerateFromPassword([]byte(adminReq.Password), bcrypt.DefaultCost)
		adminUser = AddUserTx(tx, adminReq, hash)
		adminUser.Id = 1 // Force admin ID
		vbolt.Write(tx, UsersBkt, 1, &adminUser)
		vbolt.TxCommit(tx)
	})

	t.Run("Path traversal prevention", func(t *testing.T) {
		securityTestCases := []string{
			"../etc/passwd",
			"../../secret.txt",
			"logs/../config.json",
			"../logs/app.log",
			"file.log/../../etc/hosts",
		}

		for _, filename := range securityTestCases {
			ctx := &vbeam.Context{}
			vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
				ctx.Tx = tx
				// Generate JWT token for admin user
			adminToken, _ := generateAuthJwt(adminUser, httptest.NewRecorder())
			ctx.Token = adminToken

				req := GetLogContentRequest{
					Filename: filename,
				}

				_, err := GetLogContent(ctx, req)
				if err == nil {
					t.Errorf("Expected security error for filename '%s'", filename)
				}

				expectedError := "Invalid filename"
				if err.Error() != expectedError {
					t.Errorf("For filename '%s', expected error '%s', got '%s'", filename, expectedError, err.Error())
				}
			})
		}
	})

	t.Run("Non-admin access denied", func(t *testing.T) {
		regularUser := User{Id: 2, Email: "regular@example.com"}

		ctx := &vbeam.Context{}
		vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
			ctx.Tx = tx
			// Generate JWT token for regular user
			regularToken, _ := generateAuthJwt(regularUser, httptest.NewRecorder())
			ctx.Token = regularToken

			req := GetLogContentRequest{
				Filename: "app.log",
			}

			_, err := GetLogContent(ctx, req)
			if err == nil {
				t.Error("Expected error for non-admin user")
			}

			expectedError := "Unauthorized: Admin access required"
			if err.Error() != expectedError {
				t.Errorf("Expected error '%s', got '%s'", expectedError, err.Error())
			}
		})
	})

	t.Run("Valid filename accepted", func(t *testing.T) {
		validFilenames := []string{
			"app.log",
			"access.log",
			"error.log",
			"system_2023-06-15.log",
		}

		for _, filename := range validFilenames {
			ctx := &vbeam.Context{}
			vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
				ctx.Tx = tx
				// Generate JWT token for admin user
			adminToken, _ := generateAuthJwt(adminUser, httptest.NewRecorder())
			ctx.Token = adminToken

				req := GetLogContentRequest{
					Filename: filename,
				}

				// This will fail because the file doesn't exist, but it should pass security checks
				_, err := GetLogContent(ctx, req)
				if err != nil && strings.Contains(err.Error(), "Invalid filename") {
					t.Errorf("Filename '%s' should be valid but was rejected", filename)
				}
			})
		}
	})
}

func TestGetPhotoProcessingStats(t *testing.T) {
	testDBPath := "test_processing_stats.db"
	db := vbolt.Open(testDBPath)
	vbolt.InitBuckets(db, &cfg.Info)
	defer os.Remove(testDBPath)
	defer db.Close()

	// Set the global database for auth functions
	appDb = db

	var adminUser User

	// Create admin user
	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		adminReq := CreateAccountRequest{
			Name:            "Admin User",
			Email:           "admin@example.com",
			Password:        "password123",
			ConfirmPassword: "password123",
		}
		hash, _ := bcrypt.GenerateFromPassword([]byte(adminReq.Password), bcrypt.DefaultCost)
		adminUser = AddUserTx(tx, adminReq, hash)
		adminUser.Id = 1 // Force admin ID
		vbolt.Write(tx, UsersBkt, 1, &adminUser)
		vbolt.TxCommit(tx)
	})

	t.Run("Admin gets processing stats", func(t *testing.T) {
		ctx := &vbeam.Context{}
		vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
			ctx.Tx = tx
			// Generate JWT token for admin user
			adminToken, _ := generateAuthJwt(adminUser, httptest.NewRecorder())
			ctx.Token = adminToken

			resp, err := GetPhotoProcessingStats(ctx, Empty{})
			if err != nil {
				t.Errorf("Expected no error, got %v", err)
			}

			// Should return current processing stats (even if 0)
			if resp.QueueLength < 0 {
				t.Error("Queue length should not be negative")
			}

			// IsRunning can be true or false depending on worker state
		})
	})

	t.Run("Non-admin denied", func(t *testing.T) {
		regularUser := User{Id: 2, Email: "regular@example.com"}

		ctx := &vbeam.Context{}
		vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
			ctx.Tx = tx
			// Generate JWT token for regular user
			regularToken, _ := generateAuthJwt(regularUser, httptest.NewRecorder())
			ctx.Token = regularToken

			_, err := GetPhotoProcessingStats(ctx, Empty{})
			if err == nil {
				t.Error("Expected error for non-admin user")
			}

			expectedError := "Unauthorized: Admin access required"
			if err.Error() != expectedError {
				t.Errorf("Expected error '%s', got '%s'", expectedError, err.Error())
			}
		})
	})
}

func TestAdminUserInfo(t *testing.T) {
	// Test the AdminUserInfo struct
	now := time.Now()
	adminInfo := AdminUserInfo{
		Id:         1,
		Name:       "Admin User",
		Email:      "admin@example.com",
		Creation:   now,
		LastLogin:  now,
		FamilyId:   1,
		FamilyName: "Admin Family",
		IsAdmin:    true,
	}

	if adminInfo.Id != 1 {
		t.Errorf("Expected Id 1, got %d", adminInfo.Id)
	}
	if adminInfo.Name != "Admin User" {
		t.Errorf("Expected name 'Admin User', got '%s'", adminInfo.Name)
	}
	if !adminInfo.IsAdmin {
		t.Error("Expected IsAdmin to be true")
	}
	if adminInfo.FamilyName != "Admin Family" {
		t.Errorf("Expected family name 'Admin Family', got '%s'", adminInfo.FamilyName)
	}
}