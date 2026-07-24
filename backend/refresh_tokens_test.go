package backend

import (
	"family/cfg"
	"os"
	"testing"
	"time"

	"go.hasen.dev/vbolt"
	"golang.org/x/crypto/bcrypt"
)

// setupRefreshTokenTestDB creates a test database
func setupRefreshTokenTestDB(t *testing.T) *vbolt.DB {
	testDBPath := "test_refresh_tokens.db"
	db := vbolt.Open(testDBPath)
	vbolt.InitBuckets(db, &cfg.Info)
	return db
}

// cleanupRefreshTokenTestDB removes the test database
func cleanupRefreshTokenTestDB(db *vbolt.DB) {
	path := db.Path()
	db.Close()
	os.Remove(path)
}

// createRefreshTokenTestUser creates a test user
func createRefreshTokenTestUser(tx *vbolt.Tx, email string, name string) User {
	user := User{
		Id:        vbolt.NextIntId(tx, UsersBkt),
		Name:      name,
		Email:     email,
		Creation:  time.Now(),
		LastLogin: time.Now(),
	}

	// Create a family for the user
	family := Family{
		Id:         vbolt.NextIntId(tx, FamiliesBkt),
		Name:       name + "'s Family",
		InviteCode: "testcode",
		Creation:   time.Now(),
		CreatedBy:  user.Id,
	}

	vbolt.Write(tx, FamiliesBkt, family.Id, &family)
	user.FamilyId = family.Id

	hash, _ := bcrypt.GenerateFromPassword([]byte("password"), bcrypt.DefaultCost)
	vbolt.Write(tx, UsersBkt, user.Id, &user)
	vbolt.Write(tx, PasswdBkt, user.Id, &hash)
	vbolt.Write(tx, EmailBkt, user.Email, &user.Id)

	return user
}

func TestCreateRefreshToken(t *testing.T) {
	db := setupRefreshTokenTestDB(t)
	defer cleanupRefreshTokenTestDB(db)

	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		// Create a test user
		user := createRefreshTokenTestUser(tx, "test@example.com", "Test User")

		// Create refresh token
		token, err := CreateRefreshToken(tx, user.Id, 30*24*time.Hour)
		if err != nil {
			t.Fatalf("Failed to create refresh token: %v", err)
		}

		// Verify token properties
		if token.Id == 0 {
			t.Error("Token ID should not be 0")
		}
		if token.UserId != user.Id {
			t.Errorf("Token UserId = %d, want %d", token.UserId, user.Id)
		}
		if token.Token == "" {
			t.Error("Token string should not be empty")
		}
		if len(token.Token) != 64 {
			t.Errorf("Token length = %d, want 64 hex characters", len(token.Token))
		}
		if token.ExpiresAt.Before(time.Now()) {
			t.Error("Token should not be expired")
		}
		if token.CreatedAt.IsZero() {
			t.Error("CreatedAt should be set")
		}
		if token.LastUsedAt.IsZero() {
			t.Error("LastUsedAt should be set")
		}

		vbolt.TxCommit(tx)
	})
}

func TestGetRefreshTokenByToken(t *testing.T) {
	db := setupRefreshTokenTestDB(t)
	defer cleanupRefreshTokenTestDB(db)

	var tokenString string

	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		user := createRefreshTokenTestUser(tx, "test@example.com", "Test User")
		token, _ := CreateRefreshToken(tx, user.Id, 30*24*time.Hour)
		tokenString = token.Token
		vbolt.TxCommit(tx)
	})

	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		// Test retrieving existing token
		token, found := GetRefreshTokenByToken(tx, tokenString)
		if !found {
			t.Error("Token should be found")
		}
		if token.Token != tokenString {
			t.Errorf("Token string = %s, want %s", token.Token, tokenString)
		}

		// Test retrieving non-existent token
		_, found = GetRefreshTokenByToken(tx, "nonexistent")
		if found {
			t.Error("Non-existent token should not be found")
		}
	})
}

func TestValidateRefreshToken(t *testing.T) {
	db := setupRefreshTokenTestDB(t)
	defer cleanupRefreshTokenTestDB(db)

	var validTokenString string
	var expiredTokenString string

	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		user := createRefreshTokenTestUser(tx, "test@example.com", "Test User")

		// Create valid token
		validToken, _ := CreateRefreshToken(tx, user.Id, 30*24*time.Hour)
		validTokenString = validToken.Token

		// Create expired token
		expiredToken, _ := CreateRefreshToken(tx, user.Id, -1*time.Hour) // Already expired
		expiredTokenString = expiredToken.Token

		vbolt.TxCommit(tx)
	})

	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		// Test valid token
		token, valid := ValidateRefreshToken(tx, validTokenString)
		if !valid {
			t.Error("Valid token should be validated successfully")
		}
		if token.Token != validTokenString {
			t.Errorf("Token string = %s, want %s", token.Token, validTokenString)
		}

		// Test expired token
		_, valid = ValidateRefreshToken(tx, expiredTokenString)
		if valid {
			t.Error("Expired token should not be valid")
		}

		// Test non-existent token
		_, valid = ValidateRefreshToken(tx, "nonexistent")
		if valid {
			t.Error("Non-existent token should not be valid")
		}
	})
}

func TestUpdateRefreshTokenLastUsed(t *testing.T) {
	db := setupRefreshTokenTestDB(t)
	defer cleanupRefreshTokenTestDB(db)

	var tokenId int
	var originalLastUsed time.Time

	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		user := createRefreshTokenTestUser(tx, "test@example.com", "Test User")
		token, _ := CreateRefreshToken(tx, user.Id, 30*24*time.Hour)
		tokenId = token.Id
		originalLastUsed = token.LastUsedAt
		vbolt.TxCommit(tx)
	})

	// Wait a bit to ensure timestamp difference
	time.Sleep(10 * time.Millisecond)

	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		UpdateRefreshTokenLastUsed(tx, tokenId)
		vbolt.TxCommit(tx)
	})

	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		var token RefreshToken
		vbolt.Read(tx, RefreshTokenBkt, tokenId, &token)

		if !token.LastUsedAt.After(originalLastUsed) {
			t.Error("LastUsedAt should be updated to a later time")
		}
	})
}

func TestDeleteRefreshToken(t *testing.T) {
	db := setupRefreshTokenTestDB(t)
	defer cleanupRefreshTokenTestDB(db)

	var tokenString string

	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		user := createRefreshTokenTestUser(tx, "test@example.com", "Test User")
		token, _ := CreateRefreshToken(tx, user.Id, 30*24*time.Hour)
		tokenString = token.Token
		vbolt.TxCommit(tx)
	})

	// Verify token exists
	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		_, found := GetRefreshTokenByToken(tx, tokenString)
		if !found {
			t.Error("Token should exist before deletion")
		}
	})

	// Delete token
	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		DeleteRefreshToken(tx, tokenString)
		vbolt.TxCommit(tx)
	})

	// Verify token is deleted
	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		_, found := GetRefreshTokenByToken(tx, tokenString)
		if found {
			t.Error("Token should not exist after deletion")
		}
	})
}

func TestDeleteUserRefreshTokens(t *testing.T) {
	db := setupRefreshTokenTestDB(t)
	defer cleanupRefreshTokenTestDB(db)

	var userId int
	var token1String, token2String, token3String string

	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		user := createRefreshTokenTestUser(tx, "test@example.com", "Test User")
		userId = user.Id

		// Create multiple tokens for the user
		token1, _ := CreateRefreshToken(tx, user.Id, 30*24*time.Hour)
		token1String = token1.Token

		token2, _ := CreateRefreshToken(tx, user.Id, 30*24*time.Hour)
		token2String = token2.Token

		token3, _ := CreateRefreshToken(tx, user.Id, 30*24*time.Hour)
		token3String = token3.Token

		vbolt.TxCommit(tx)
	})

	// Verify all tokens exist
	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		if _, found := GetRefreshTokenByToken(tx, token1String); !found {
			t.Error("Token 1 should exist")
		}
		if _, found := GetRefreshTokenByToken(tx, token2String); !found {
			t.Error("Token 2 should exist")
		}
		if _, found := GetRefreshTokenByToken(tx, token3String); !found {
			t.Error("Token 3 should exist")
		}
	})

	// Delete all user tokens
	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		DeleteUserRefreshTokens(tx, userId)
		vbolt.TxCommit(tx)
	})

	// Verify all tokens are deleted
	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		if _, found := GetRefreshTokenByToken(tx, token1String); found {
			t.Error("Token 1 should be deleted")
		}
		if _, found := GetRefreshTokenByToken(tx, token2String); found {
			t.Error("Token 2 should be deleted")
		}
		if _, found := GetRefreshTokenByToken(tx, token3String); found {
			t.Error("Token 3 should be deleted")
		}
	})
}

func TestCleanupExpiredRefreshTokens(t *testing.T) {
	db := setupRefreshTokenTestDB(t)
	defer cleanupRefreshTokenTestDB(db)

	var validToken, expiredToken RefreshToken
	var userId int
	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		user := createRefreshTokenTestUser(tx, "test@example.com", "Test User")
		userId = user.Id

		var err error
		validToken, err = CreateRefreshToken(tx, user.Id, time.Hour)
		if err != nil {
			t.Fatalf("CreateRefreshToken(valid) error = %v", err)
		}
		expiredToken, err = CreateRefreshToken(tx, user.Id, -time.Hour)
		if err != nil {
			t.Fatalf("CreateRefreshToken(expired) error = %v", err)
		}

		if removed := CleanupExpiredRefreshTokens(tx, time.Now()); removed != 1 {
			t.Fatalf("CleanupExpiredRefreshTokens() = %d, want 1", removed)
		}
		vbolt.TxCommit(tx)
	})

	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		if _, found := GetRefreshTokenByToken(tx, expiredToken.Token); found {
			t.Error("expired token lookup still exists after cleanup")
		}
		if _, found := GetRefreshTokenByToken(tx, validToken.Token); !found {
			t.Error("valid token was removed by cleanup")
		}

		var tokenIds []int
		vbolt.ReadTermTargets(tx, RefreshTokenByUserIndex, userId, &tokenIds, vbolt.Window{})
		if len(tokenIds) != 1 || tokenIds[0] != validToken.Id {
			t.Errorf("user token index = %v, want [%d]", tokenIds, validToken.Id)
		}
	})
}

func TestGenerateRefreshToken(t *testing.T) {
	// Generate multiple tokens
	token1, err := generateRefreshToken()
	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
	}

	token2, err := generateRefreshToken()
	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
	}

	// Verify token format
	if len(token1) != 64 {
		t.Errorf("Token length = %d, want 64 hex characters", len(token1))
	}

	// Verify tokens are unique
	if token1 == token2 {
		t.Error("Generated tokens should be unique")
	}
}
