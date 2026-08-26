package backend

import (
	"errors"
	"family/cfg"
	"os"
	"testing"
	"time"

	"go.hasen.dev/vbolt"
	"golang.org/x/crypto/bcrypt"
)

func setupRefreshTokenTestDB(t *testing.T) *vbolt.DB {
	testDBPath := "test_refresh_tokens.db"
	db := vbolt.Open(testDBPath)
	vbolt.InitBuckets(db, &cfg.Info)
	return db
}

func cleanupRefreshTokenTestDB(db *vbolt.DB) {
	path := db.Path()
	db.Close()
	os.Remove(path)
}

func createRefreshTokenTestUser(tx *vbolt.Tx, email string, name string) User {
	user := User{
		Id:        vbolt.NextIntId(tx, UsersBkt),
		Name:      name,
		Email:     email,
		Creation:  time.Now(),
		LastLogin: time.Now(),
	}

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
		user := createRefreshTokenTestUser(tx, "test@example.com", "Test User")

		token, tokenString, err := CreateRefreshToken(tx, user.Id, 30*24*time.Hour)
		if err != nil {
			t.Fatalf("Failed to create refresh token: %v", err)
		}

		if token.Id == 0 {
			t.Error("Token ID should not be 0")
		}
		if token.UserId != user.Id {
			t.Errorf("Token UserId = %d, want %d", token.UserId, user.Id)
		}
		if len(tokenString) != 64 {
			t.Errorf("Token length = %d, want 64 hex characters", len(tokenString))
		}
		if token.SessionId != token.Id {
			t.Errorf("SessionId = %d, want the token's own id %d", token.SessionId, token.Id)
		}
		if !token.RotatedAt.IsZero() {
			t.Error("A new token should not be marked as rotated")
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

func TestRefreshTokensAreStoredHashed(t *testing.T) {
	db := setupRefreshTokenTestDB(t)
	defer cleanupRefreshTokenTestDB(db)

	var tokenString string
	var stored RefreshToken

	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		user := createRefreshTokenTestUser(tx, "test@example.com", "Test User")
		stored, tokenString, _ = CreateRefreshToken(tx, user.Id, time.Hour)
		vbolt.TxCommit(tx)
	})

	if stored.TokenHash == tokenString {
		t.Fatal("stored value equals the token handed to the client")
	}
	if stored.TokenHash != hashRefreshToken(tokenString) {
		t.Error("stored value is not the hash of the token")
	}

	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		var readBack RefreshToken
		vbolt.Read(tx, RefreshTokenBkt, stored.Id, &readBack)
		if readBack.TokenHash != stored.TokenHash {
			t.Errorf("read back hash = %q, want %q", readBack.TokenHash, stored.TokenHash)
		}

		var idByToken int
		vbolt.Read(tx, RefreshTokenByTokenBkt, tokenString, &idByToken)
		if idByToken != 0 {
			t.Error("lookup bucket contains an entry keyed by the raw token")
		}
	})
}

func TestGetRefreshTokenByToken(t *testing.T) {
	db := setupRefreshTokenTestDB(t)
	defer cleanupRefreshTokenTestDB(db)

	var tokenString string
	var created RefreshToken

	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		user := createRefreshTokenTestUser(tx, "test@example.com", "Test User")
		created, tokenString, _ = CreateRefreshToken(tx, user.Id, 30*24*time.Hour)
		vbolt.TxCommit(tx)
	})

	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		token, found := GetRefreshTokenByToken(tx, tokenString)
		if !found {
			t.Error("Token should be found")
		}
		if token.Id != created.Id {
			t.Errorf("Token id = %d, want %d", token.Id, created.Id)
		}

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
	var validId int

	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		user := createRefreshTokenTestUser(tx, "test@example.com", "Test User")

		validToken, tokenString, _ := CreateRefreshToken(tx, user.Id, 30*24*time.Hour)
		validTokenString = tokenString
		validId = validToken.Id

		_, expiredString, _ := CreateRefreshToken(tx, user.Id, -1*time.Hour)
		expiredTokenString = expiredString

		vbolt.TxCommit(tx)
	})

	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		token, valid := ValidateRefreshToken(tx, validTokenString)
		if !valid {
			t.Error("Valid token should be validated successfully")
		}
		if token.Id != validId {
			t.Errorf("Token id = %d, want %d", token.Id, validId)
		}

		_, valid = ValidateRefreshToken(tx, expiredTokenString)
		if valid {
			t.Error("Expired token should not be valid")
		}

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
		token, _, _ := CreateRefreshToken(tx, user.Id, 30*24*time.Hour)
		tokenId = token.Id
		originalLastUsed = token.LastUsedAt
		vbolt.TxCommit(tx)
	})

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

func TestRotateRefreshTokenIssuesASuccessor(t *testing.T) {
	db := setupRefreshTokenTestDB(t)
	defer cleanupRefreshTokenTestDB(db)

	now := time.Now()
	var original RefreshToken
	var originalString string

	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		user := createRefreshTokenTestUser(tx, "test@example.com", "Test User")
		original, originalString, _ = CreateRefreshToken(tx, user.Id, 30*24*time.Hour)
		vbolt.TxCommit(tx)
	})

	var successor RefreshToken
	var successorString string
	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		var err error
		successor, successorString, err = RotateRefreshToken(tx, originalString, now)
		if err != nil {
			t.Fatalf("RotateRefreshToken() error = %v", err)
		}
		vbolt.TxCommit(tx)
	})

	if successorString == originalString {
		t.Error("rotation returned the same token string")
	}
	if successor.SessionId != original.SessionId {
		t.Errorf("successor session = %d, want %d", successor.SessionId, original.SessionId)
	}
	if !successor.ExpiresAt.Equal(original.ExpiresAt) {
		t.Error("rotation extended the session's expiry")
	}

	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		if _, valid := ValidateRefreshToken(tx, originalString); valid {
			t.Error("the rotated token is still accepted")
		}
		if _, valid := ValidateRefreshToken(tx, successorString); !valid {
			t.Error("the successor token was not accepted")
		}
	})
}

func TestRotateRefreshTokenRejectsUnknownAndExpiredTokens(t *testing.T) {
	db := setupRefreshTokenTestDB(t)
	defer cleanupRefreshTokenTestDB(db)

	now := time.Now()
	var expiredString string
	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		user := createRefreshTokenTestUser(tx, "test@example.com", "Test User")
		_, expiredString, _ = CreateRefreshToken(tx, user.Id, -time.Hour)
		vbolt.TxCommit(tx)
	})

	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		if _, _, err := RotateRefreshToken(tx, "nonexistent", now); !errors.Is(err, ErrRefreshTokenInvalid) {
			t.Errorf("RotateRefreshToken(unknown) error = %v, want ErrRefreshTokenInvalid", err)
		}
		if _, _, err := RotateRefreshToken(tx, expiredString, now); !errors.Is(err, ErrRefreshTokenInvalid) {
			t.Errorf("RotateRefreshToken(expired) error = %v, want ErrRefreshTokenInvalid", err)
		}
	})
}

func TestRotateRefreshTokenRevokesSessionOnReuse(t *testing.T) {
	db := setupRefreshTokenTestDB(t)
	defer cleanupRefreshTokenTestDB(db)

	start := time.Now()
	var originalString string
	var sessionId int

	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		user := createRefreshTokenTestUser(tx, "test@example.com", "Test User")
		original, tokenString, _ := CreateRefreshToken(tx, user.Id, 30*24*time.Hour)
		originalString = tokenString
		sessionId = original.SessionId
		vbolt.TxCommit(tx)
	})

	var successorString string
	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		_, successorString, _ = RotateRefreshToken(tx, originalString, start)
		vbolt.TxCommit(tx)
	})

	replayAt := start.Add(refreshTokenReuseGrace + time.Second)
	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		if _, _, err := RotateRefreshToken(tx, originalString, replayAt); !errors.Is(err, ErrRefreshTokenReused) {
			t.Fatalf("RotateRefreshToken(replay) error = %v, want ErrRefreshTokenReused", err)
		}
		vbolt.TxCommit(tx)
	})

	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		if _, valid := ValidateRefreshToken(tx, successorString); valid {
			t.Error("the successor survived a detected replay")
		}
		if tokens := sessionTokens(tx, sessionId); len(tokens) != 0 {
			t.Errorf("session still holds %d tokens after revocation", len(tokens))
		}
	})
}

func TestRotateRefreshTokenToleratesConcurrentTabs(t *testing.T) {
	db := setupRefreshTokenTestDB(t)
	defer cleanupRefreshTokenTestDB(db)

	start := time.Now()
	var originalString string

	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		user := createRefreshTokenTestUser(tx, "test@example.com", "Test User")
		_, originalString, _ = CreateRefreshToken(tx, user.Id, 30*24*time.Hour)
		vbolt.TxCommit(tx)
	})

	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		if _, _, err := RotateRefreshToken(tx, originalString, start); err != nil {
			t.Fatalf("first rotation error = %v", err)
		}
		vbolt.TxCommit(tx)
	})

	var raceString string
	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		var err error
		_, raceString, err = RotateRefreshToken(tx, originalString, start.Add(time.Second))
		if err != nil {
			t.Fatalf("racing rotation error = %v, want success inside the grace window", err)
		}
		vbolt.TxCommit(tx)
	})

	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		if _, valid := ValidateRefreshToken(tx, raceString); !valid {
			t.Error("the token issued to the racing tab is not usable")
		}
	})
}

func TestDeleteRefreshToken(t *testing.T) {
	db := setupRefreshTokenTestDB(t)
	defer cleanupRefreshTokenTestDB(db)

	var tokenString string

	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		user := createRefreshTokenTestUser(tx, "test@example.com", "Test User")
		_, tokenString, _ = CreateRefreshToken(tx, user.Id, 30*24*time.Hour)
		vbolt.TxCommit(tx)
	})

	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		_, found := GetRefreshTokenByToken(tx, tokenString)
		if !found {
			t.Error("Token should exist before deletion")
		}
	})

	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		DeleteRefreshToken(tx, tokenString)
		vbolt.TxCommit(tx)
	})

	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		_, found := GetRefreshTokenByToken(tx, tokenString)
		if found {
			t.Error("Token should not exist after deletion")
		}
	})
}

func TestDeleteRefreshTokenRemovesTheWholeSession(t *testing.T) {
	db := setupRefreshTokenTestDB(t)
	defer cleanupRefreshTokenTestDB(db)

	now := time.Now()
	var sessionId int
	var successorString string

	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		user := createRefreshTokenTestUser(tx, "test@example.com", "Test User")
		original, originalString, _ := CreateRefreshToken(tx, user.Id, 30*24*time.Hour)
		sessionId = original.SessionId
		_, successorString, _ = RotateRefreshToken(tx, originalString, now)
		vbolt.TxCommit(tx)
	})

	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		DeleteRefreshToken(tx, successorString)
		vbolt.TxCommit(tx)
	})

	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		if tokens := sessionTokens(tx, sessionId); len(tokens) != 0 {
			t.Errorf("session still holds %d tokens after logout", len(tokens))
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

		_, token1String, _ = CreateRefreshToken(tx, user.Id, 30*24*time.Hour)
		_, token2String, _ = CreateRefreshToken(tx, user.Id, 30*24*time.Hour)
		_, token3String, _ = CreateRefreshToken(tx, user.Id, 30*24*time.Hour)

		vbolt.TxCommit(tx)
	})

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

	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		DeleteUserRefreshTokens(tx, userId)
		vbolt.TxCommit(tx)
	})

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

	var validToken RefreshToken
	var validString, expiredString string
	var userId int
	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		user := createRefreshTokenTestUser(tx, "test@example.com", "Test User")
		userId = user.Id

		var err error
		validToken, validString, err = CreateRefreshToken(tx, user.Id, time.Hour)
		if err != nil {
			t.Fatalf("CreateRefreshToken(valid) error = %v", err)
		}
		_, expiredString, err = CreateRefreshToken(tx, user.Id, -time.Hour)
		if err != nil {
			t.Fatalf("CreateRefreshToken(expired) error = %v", err)
		}

		if removed := CleanupExpiredRefreshTokens(tx, time.Now()); removed != 1 {
			t.Fatalf("CleanupExpiredRefreshTokens() = %d, want 1", removed)
		}
		vbolt.TxCommit(tx)
	})

	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		if _, found := GetRefreshTokenByToken(tx, expiredString); found {
			t.Error("expired token lookup still exists after cleanup")
		}
		if _, found := GetRefreshTokenByToken(tx, validString); !found {
			t.Error("valid token was removed by cleanup")
		}

		var tokenIds []int
		vbolt.ReadTermTargets(tx, RefreshTokenByUserIndex, userId, &tokenIds, vbolt.Window{})
		if len(tokenIds) != 1 || tokenIds[0] != validToken.Id {
			t.Errorf("user token index = %v, want [%d]", tokenIds, validToken.Id)
		}
	})
}

func TestHashStoredRefreshTokensUpgradesLegacyRows(t *testing.T) {
	db := setupRefreshTokenTestDB(t)
	defer cleanupRefreshTokenTestDB(db)

	rawToken := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	var legacyId int

	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		user := createRefreshTokenTestUser(tx, "test@example.com", "Test User")
		legacy := RefreshToken{
			Id:         vbolt.NextIntId(tx, RefreshTokenBkt),
			UserId:     user.Id,
			TokenHash:  rawToken,
			ExpiresAt:  time.Now().Add(time.Hour),
			CreatedAt:  time.Now(),
			LastUsedAt: time.Now(),
		}
		legacyId = legacy.Id
		vbolt.Write(tx, RefreshTokenBkt, legacy.Id, &legacy)
		vbolt.Write(tx, RefreshTokenByTokenBkt, rawToken, &legacy.Id)
		vbolt.SetTargetSingleTerm(tx, RefreshTokenByUserIndex, legacy.Id, legacy.UserId)
		vbolt.TxCommit(tx)
	})

	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		if converted := HashStoredRefreshTokens(tx); converted != 1 {
			t.Fatalf("HashStoredRefreshTokens() = %d, want 1", converted)
		}
		vbolt.TxCommit(tx)
	})

	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		token, found := GetRefreshTokenByToken(tx, rawToken)
		if !found {
			t.Fatal("the cookie a user already holds stopped working after the migration")
		}
		if token.Id != legacyId {
			t.Errorf("resolved token id = %d, want %d", token.Id, legacyId)
		}
		if token.TokenHash != hashRefreshToken(rawToken) {
			t.Error("migrated row does not hold the hash")
		}
		if token.SessionId != legacyId {
			t.Errorf("migrated session = %d, want %d", token.SessionId, legacyId)
		}

		var strayId int
		vbolt.Read(tx, RefreshTokenByTokenBkt, rawToken, &strayId)
		if strayId != 0 {
			t.Error("the lookup entry keyed by the raw token was left behind")
		}
	})

	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		if converted := HashStoredRefreshTokens(tx); converted != 0 {
			t.Errorf("second HashStoredRefreshTokens() = %d, want 0", converted)
		}
		vbolt.TxCommit(tx)
	})
}

func TestGenerateRefreshToken(t *testing.T) {
	token1, err := generateRefreshToken()
	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
	}

	token2, err := generateRefreshToken()
	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
	}

	if len(token1) != 64 {
		t.Errorf("Token length = %d, want 64 hex characters", len(token1))
	}

	if token1 == token2 {
		t.Error("Generated tokens should be unique")
	}
}
