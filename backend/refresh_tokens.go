package backend

import (
	"crypto/rand"
	"encoding/hex"
	"family/cfg"
	"time"

	"go.hasen.dev/vbolt"
	"go.hasen.dev/vpack"
)

// RefreshToken represents a long-lived token for persistent login
type RefreshToken struct {
	Id         int       `json:"id"`
	UserId     int       `json:"userId"`
	Token      string    `json:"token"`
	ExpiresAt  time.Time `json:"expiresAt"`
	CreatedAt  time.Time `json:"createdAt"`
	LastUsedAt time.Time `json:"lastUsedAt"`
}

// PackRefreshToken serializes a RefreshToken for vbolt storage
func PackRefreshToken(self *RefreshToken, buf *vpack.Buffer) {
	vpack.Version(1, buf)
	vpack.Int(&self.Id, buf)
	vpack.Int(&self.UserId, buf)
	vpack.String(&self.Token, buf)
	vpack.Time(&self.ExpiresAt, buf)
	vpack.Time(&self.CreatedAt, buf)
	vpack.Time(&self.LastUsedAt, buf)
}

// Buckets for refresh token storage
var RefreshTokenBkt = vbolt.Bucket(&cfg.Info, "refresh_tokens", vpack.FInt, PackRefreshToken)

// token string => token id
var RefreshTokenByTokenBkt = vbolt.Bucket(&cfg.Info, "refresh_tokens_by_token", vpack.StringZ, vpack.Int)

// user id => token ids (for tracking user's tokens)
var RefreshTokenByUserIndex = vbolt.Index(&cfg.Info, "refresh_tokens_by_user", vpack.FInt, vpack.FInt)

// generateRefreshToken creates a cryptographically secure random token string
func generateRefreshToken() (string, error) {
	b := make([]byte, 32) // 32 bytes = 64 hex characters
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// CreateRefreshToken generates and stores a new refresh token for a user
func CreateRefreshToken(tx *vbolt.Tx, userId int, expiryDuration time.Duration) (RefreshToken, error) {
	tokenString, err := generateRefreshToken()
	if err != nil {
		return RefreshToken{}, err
	}

	token := RefreshToken{
		Id:         vbolt.NextIntId(tx, RefreshTokenBkt),
		UserId:     userId,
		Token:      tokenString,
		ExpiresAt:  time.Now().Add(expiryDuration),
		CreatedAt:  time.Now(),
		LastUsedAt: time.Now(),
	}

	// Store token in database
	vbolt.Write(tx, RefreshTokenBkt, token.Id, &token)
	vbolt.Write(tx, RefreshTokenByTokenBkt, token.Token, &token.Id)
	vbolt.SetTargetSingleTerm(tx, RefreshTokenByUserIndex, token.Id, token.UserId)

	return token, nil
}

// GetRefreshTokenByToken retrieves a refresh token by its token string
func GetRefreshTokenByToken(tx *vbolt.Tx, tokenString string) (RefreshToken, bool) {
	var tokenId int
	vbolt.Read(tx, RefreshTokenByTokenBkt, tokenString, &tokenId)
	if tokenId == 0 {
		return RefreshToken{}, false
	}

	var token RefreshToken
	vbolt.Read(tx, RefreshTokenBkt, tokenId, &token)
	return token, token.Id != 0
}

// UpdateRefreshTokenLastUsed updates the LastUsedAt timestamp for a token
func UpdateRefreshTokenLastUsed(tx *vbolt.Tx, tokenId int) {
	var token RefreshToken
	vbolt.Read(tx, RefreshTokenBkt, tokenId, &token)
	if token.Id == 0 {
		return
	}

	token.LastUsedAt = time.Now()
	vbolt.Write(tx, RefreshTokenBkt, token.Id, &token)
}

// DeleteRefreshToken removes a refresh token from the database
func DeleteRefreshToken(tx *vbolt.Tx, tokenString string) {
	var tokenId int
	vbolt.Read(tx, RefreshTokenByTokenBkt, tokenString, &tokenId)
	if tokenId == 0 {
		return
	}

	var token RefreshToken
	vbolt.Read(tx, RefreshTokenBkt, tokenId, &token)
	if token.Id == 0 {
		return
	}

	// Remove from all buckets and indexes
	vbolt.Delete(tx, RefreshTokenBkt, token.Id)
	vbolt.Delete(tx, RefreshTokenByTokenBkt, token.Token)
	vbolt.SetTargetSingleTerm(tx, RefreshTokenByUserIndex, token.Id, -1)
}

// DeleteUserRefreshTokens removes all refresh tokens for a user
func DeleteUserRefreshTokens(tx *vbolt.Tx, userId int) {
	// Get all token IDs for this user
	var tokenIds []int
	vbolt.ReadTermTargets(tx, RefreshTokenByUserIndex, userId, &tokenIds, vbolt.Window{})

	for _, tokenId := range tokenIds {
		var token RefreshToken
		vbolt.Read(tx, RefreshTokenBkt, tokenId, &token)
		if token.Id == 0 {
			continue
		}

		// Remove from all buckets and indexes
		vbolt.Delete(tx, RefreshTokenBkt, token.Id)
		vbolt.Delete(tx, RefreshTokenByTokenBkt, token.Token)
		vbolt.SetTargetSingleTerm(tx, RefreshTokenByUserIndex, token.Id, -1)
	}
}

// ValidateRefreshToken checks if a token is valid (exists and not expired)
func ValidateRefreshToken(tx *vbolt.Tx, tokenString string) (RefreshToken, bool) {
	token, found := GetRefreshTokenByToken(tx, tokenString)
	if !found {
		return RefreshToken{}, false
	}

	// Check if expired
	if token.ExpiresAt.Before(time.Now()) {
		return RefreshToken{}, false
	}

	return token, true
}
