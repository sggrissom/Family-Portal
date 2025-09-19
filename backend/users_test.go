package backend

import (
	"family/cfg"
	"os"
	"testing"

	"go.hasen.dev/vbolt"
	"golang.org/x/crypto/bcrypt"
)

func TestUserCreation(t *testing.T) {
	testDBPath := "test_users.db"
	db := vbolt.Open(testDBPath)
	vbolt.InitBuckets(db, &cfg.Info)
	defer os.Remove(testDBPath)
	defer db.Close()

	// Valid user creation requests
	validReqs := []CreateAccountRequest{
		{
			Name:            "John Doe",
			Email:           "john@example.com",
			Password:        "password123",
			ConfirmPassword: "password123",
		},
		{
			Name:            "Jane Smith",
			Email:           "jane@example.com",
			Password:        "mypassword",
			ConfirmPassword: "mypassword",
		},
		{
			Name:            "Bob Wilson",
			Email:           "bob@example.com",
			Password:        "securepass123",
			ConfirmPassword: "securepass123",
		},
	}

	// Invalid user creation requests
	invalidReqs := []CreateAccountRequest{
		{
			Name:            "John Doe",
			Email:           "john@example.com", // Duplicate email
			Password:        "password123",
			ConfirmPassword: "password123",
		},
		{
			Name:            "Short Pass",
			Email:           "short@example.com",
			Password:        "short", // Too short
			ConfirmPassword: "short",
		},
		{
			Name:            "Mismatch",
			Email:           "mismatch@example.com",
			Password:        "password123",
			ConfirmPassword: "different123", // Passwords don't match
		},
		{
			Name:            "",
			Email:           "noname@example.com",
			Password:        "password123",
			ConfirmPassword: "password123",
		},
		{
			Name:            "No Email",
			Email:           "",
			Password:        "password123",
			ConfirmPassword: "password123",
		},
	}

	// Test valid user creation
	var createdUsers []int
	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		for _, req := range validReqs {
			// Hash password
			hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
			if err != nil {
				t.Fatalf("Failed to hash password: %v", err)
			}

			user := AddUserTx(tx, req, hash)
			createdUsers = append(createdUsers, user.Id)

			// Verify user was created correctly
			if user.Name != req.Name {
				t.Errorf("Expected name %s, got %s", req.Name, user.Name)
			}
			if user.Email != req.Email {
				t.Errorf("Expected email %s, got %s", req.Email, user.Email)
			}
			if user.FamilyId == 0 {
				t.Error("User should have been assigned to a family")
			}
		}
		vbolt.TxCommit(tx)
	})

	// Test invalid user creation scenarios
	for i, req := range invalidReqs {
		if i == 0 {
			// First invalid request is duplicate email - test at database level
			vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
				existingUserId := GetUserId(tx, req.Email)
				if existingUserId == 0 {
					t.Error("Duplicate email test failed - email should already exist")
				}
				// Don't commit this transaction
			})
		} else {
			// Other invalid requests should fail validation
			err := validateCreateAccountRequest(req)
			if err == nil {
				t.Errorf("Expected validation to fail for request: %+v", req)
			}
		}
	}

	// Verify users exist in database and can be retrieved
	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		for i, userId := range createdUsers {
			user := GetUser(tx, userId)
			if user.Id == 0 {
				t.Errorf("User %d not found in database", userId)
			}
			if user.Email != validReqs[i].Email {
				t.Errorf("Expected email %s, got %s", validReqs[i].Email, user.Email)
			}

			// Test email lookup
			retrievedUserId := GetUserId(tx, user.Email)
			if retrievedUserId != userId {
				t.Errorf("Email lookup failed: expected %d, got %d", userId, retrievedUserId)
			}

			// Test password hash retrieval
			hash := GetPassHash(tx, userId)
			if len(hash) == 0 {
				t.Error("Password hash not stored")
			}

			// Verify password can be validated
			err := bcrypt.CompareHashAndPassword(hash, []byte(validReqs[i].Password))
			if err != nil {
				t.Errorf("Password validation failed: %v", err)
			}
		}
	})
}

func TestFamilyManagement(t *testing.T) {
	testDBPath := "test_families.db"
	db := vbolt.Open(testDBPath)
	vbolt.InitBuckets(db, &cfg.Info)
	defer os.Remove(testDBPath)
	defer db.Close()

	var familyInviteCode string
	var firstUser, secondUser User

	// Test: First user creates a new family
	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		req := CreateAccountRequest{
			Name:            "Family Creator",
			Email:           "creator@example.com",
			Password:        "password123",
			ConfirmPassword: "password123",
		}

		hash, _ := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		firstUser = AddUserTx(tx, req, hash)

		// Verify family was created
		if firstUser.FamilyId == 0 {
			t.Error("First user should have been assigned to a new family")
		}

		family := GetFamily(tx, firstUser.FamilyId)
		if family.Id == 0 {
			t.Error("Family should have been created")
		}
		if family.CreatedBy != firstUser.Id {
			t.Errorf("Expected family creator %d, got %d", firstUser.Id, family.CreatedBy)
		}
		if family.InviteCode == "" {
			t.Error("Family should have an invite code")
		}

		familyInviteCode = family.InviteCode
		vbolt.TxCommit(tx)
	})

	// Test: Second user joins existing family using invite code
	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		req := CreateAccountRequest{
			Name:            "Family Member",
			Email:           "member@example.com",
			Password:        "password123",
			ConfirmPassword: "password123",
			FamilyCode:      familyInviteCode,
		}

		hash, _ := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		secondUser = AddUserTx(tx, req, hash)

		// Verify user joined the existing family
		if secondUser.FamilyId != firstUser.FamilyId {
			t.Errorf("Second user should join first user's family. Expected %d, got %d",
				firstUser.FamilyId, secondUser.FamilyId)
		}
		vbolt.TxCommit(tx)
	})

	// Test: User with invalid invite code creates new family
	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		req := CreateAccountRequest{
			Name:            "Invalid Code User",
			Email:           "invalid@example.com",
			Password:        "password123",
			ConfirmPassword: "password123",
			FamilyCode:      "invalidcode",
		}

		hash, _ := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		thirdUser := AddUserTx(tx, req, hash)

		// Should create a new family since invite code is invalid
		if thirdUser.FamilyId == firstUser.FamilyId {
			t.Error("User with invalid invite code should create new family")
		}
		if thirdUser.FamilyId == 0 {
			t.Error("User should have been assigned to a family")
		}
		vbolt.TxCommit(tx)
	})

	// Test invite code lookup
	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		family := GetFamilyByInviteCode(tx, familyInviteCode)
		if family.Id != firstUser.FamilyId {
			t.Errorf("Invite code lookup failed: expected family %d, got %d",
				firstUser.FamilyId, family.Id)
		}

		// Test invalid invite code
		invalidFamily := GetFamilyByInviteCode(tx, "nonexistent")
		if invalidFamily.Id != 0 {
			t.Error("Invalid invite code should return empty family")
		}
	})
}

func TestPasswordHandling(t *testing.T) {
	testDBPath := "test_passwords.db"
	db := vbolt.Open(testDBPath)
	vbolt.InitBuckets(db, &cfg.Info)
	defer os.Remove(testDBPath)
	defer db.Close()

	passwords := []string{
		"password123",
		"mySecurePassword!",
		"anotherPass456",
	}

	var userIds []int

	// Test password hashing and storage
	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		for i, password := range passwords {
			req := CreateAccountRequest{
				Name:            "User " + string(rune('A'+i)),
				Email:           "user" + string(rune('a'+i)) + "@example.com",
				Password:        password,
				ConfirmPassword: password,
			}

			hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
			if err != nil {
				t.Fatalf("Failed to hash password: %v", err)
			}

			user := AddUserTx(tx, req, hash)
			userIds = append(userIds, user.Id)

			// Verify hash is different from password
			if string(hash) == password {
				t.Error("Password should be hashed, not stored in plaintext")
			}
		}
		vbolt.TxCommit(tx)
	})

	// Test password verification
	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		for i, userId := range userIds {
			hash := GetPassHash(tx, userId)

			// Test correct password
			err := bcrypt.CompareHashAndPassword(hash, []byte(passwords[i]))
			if err != nil {
				t.Errorf("Valid password should verify: %v", err)
			}

			// Test incorrect password
			err = bcrypt.CompareHashAndPassword(hash, []byte("wrongpassword"))
			if err == nil {
				t.Error("Invalid password should not verify")
			}
		}
	})

	// Test password validation rules
	validationTests := []struct {
		password        string
		confirmPassword string
		shouldFail      bool
		description     string
	}{
		{"password123", "password123", false, "valid password"},
		{"short", "short", true, "too short"},
		{"", "", true, "empty password"},
		{"validpass", "different", true, "passwords don't match"},
		{"12345678", "12345678", false, "minimum length exactly"},
		{"1234567", "1234567", true, "below minimum length"},
	}

	for _, test := range validationTests {
		req := CreateAccountRequest{
			Name:            "Test User",
			Email:           "test@example.com",
			Password:        test.password,
			ConfirmPassword: test.confirmPassword,
		}

		err := validateCreateAccountRequest(req)
		if test.shouldFail && err == nil {
			t.Errorf("Expected validation to fail for %s", test.description)
		}
		if !test.shouldFail && err != nil {
			t.Errorf("Expected validation to pass for %s, got error: %v", test.description, err)
		}
	}
}

func TestUserRetrieval(t *testing.T) {
	testDBPath := "test_retrieval.db"
	db := vbolt.Open(testDBPath)
	vbolt.InitBuckets(db, &cfg.Info)
	defer os.Remove(testDBPath)
	defer db.Close()

	var testUser User

	// Create a test user
	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		req := CreateAccountRequest{
			Name:            "Test User",
			Email:           "test@example.com",
			Password:        "password123",
			ConfirmPassword: "password123",
		}

		hash, _ := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		testUser = AddUserTx(tx, req, hash)
		vbolt.TxCommit(tx)
	})

	// Test user retrieval by ID
	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		retrievedUser := GetUser(tx, testUser.Id)
		if retrievedUser.Id != testUser.Id {
			t.Errorf("Expected user ID %d, got %d", testUser.Id, retrievedUser.Id)
		}
		if retrievedUser.Email != testUser.Email {
			t.Errorf("Expected email %s, got %s", testUser.Email, retrievedUser.Email)
		}

		// Test nonexistent user
		nonexistentUser := GetUser(tx, 99999)
		if nonexistentUser.Id != 0 {
			t.Error("Nonexistent user should return zero value")
		}
	})

	// Test user retrieval by email
	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		userId := GetUserId(tx, testUser.Email)
		if userId != testUser.Id {
			t.Errorf("Expected user ID %d, got %d", testUser.Id, userId)
		}

		// Test nonexistent email
		nonexistentId := GetUserId(tx, "nonexistent@example.com")
		if nonexistentId != 0 {
			t.Error("Nonexistent email should return 0")
		}
	})
}