package backend

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"family/cfg"
	"time"

	"go.hasen.dev/vbolt"
	"go.hasen.dev/vpack"
)

const refreshTokenCleanupInterval = 24 * time.Hour

// refreshTokenReuseGrace is how long a just-rotated token keeps working.
//
// Rotation and multiple browser tabs disagree: two tabs can present the same
// cookie within milliseconds of each other, and without a grace period the
// second one looks exactly like a stolen token being replayed. Everyone would
// get signed out of everything the first time two tabs woke up together. Inside
// the grace window a replay is treated as that race and served from the live
// end of the session; outside it, it is treated as theft.
const refreshTokenReuseGrace = time.Minute

var (
	// ErrRefreshTokenInvalid means the presented token is unknown or expired.
	ErrRefreshTokenInvalid = errors.New("refresh token is invalid")
	// ErrRefreshTokenReused means a token that had already been rotated came
	// back after the grace window. The session it belonged to is revoked.
	ErrRefreshTokenReused = errors.New("refresh token was reused")
)

// RefreshToken is one link in a login session's chain of tokens.
//
// Only the hash of the token is stored. A refresh token is a bearer credential
// with a month-long life, so a leaked database file — a backup, a snapshot, a
// stolen disk — would otherwise hand over every live session. Hashing means the
// stored value cannot be presented to the server.
//
// SHA-256 rather than bcrypt: the input is 32 bytes from crypto/rand, not a
// human-chosen password, so there is no dictionary to run and nothing for a slow
// hash to buy. Every refresh would pay the cost instead.
type RefreshToken struct {
	Id        int    `json:"id"`
	UserId    int    `json:"userId"`
	TokenHash string `json:"-"`
	// SessionId ties every token issued by one login together, so a detected
	// replay can revoke the whole chain rather than a single link.
	SessionId  int       `json:"sessionId"`
	ExpiresAt  time.Time `json:"expiresAt"`
	CreatedAt  time.Time `json:"createdAt"`
	LastUsedAt time.Time `json:"lastUsedAt"`
	// RotatedAt is when this token was exchanged for its successor. A rotated
	// token is kept, not deleted: it is the only evidence that lets a replay be
	// recognized instead of looking like an unknown token.
	RotatedAt time.Time `json:"rotatedAt"`
}

// PackRefreshToken serializes a RefreshToken for vbolt storage.
//
// Version 2 renamed the third field from the raw token to its hash — the same
// string slot holding a different value — and appended the session chain. The
// 2026-0813-hash-refresh-tokens migration rewrites version 1 rows so existing
// logins survive the change.
func PackRefreshToken(self *RefreshToken, buf *vpack.Buffer) {
	version := vpack.Version(2, buf)
	vpack.Int(&self.Id, buf)
	vpack.Int(&self.UserId, buf)
	vpack.String(&self.TokenHash, buf)
	vpack.Time(&self.ExpiresAt, buf)
	vpack.Time(&self.CreatedAt, buf)
	vpack.Time(&self.LastUsedAt, buf)
	if version >= 2 {
		vpack.Int(&self.SessionId, buf)
		vpack.Time(&self.RotatedAt, buf)
	}
}

// Buckets for refresh token storage
var RefreshTokenBkt = vbolt.Bucket(&cfg.Info, "refresh_tokens", vpack.FInt, PackRefreshToken)

// token hash => token id
var RefreshTokenByTokenBkt = vbolt.Bucket(&cfg.Info, "refresh_tokens_by_token", vpack.StringZ, vpack.Int)

// user id => token ids (for tracking user's tokens)
var RefreshTokenByUserIndex = vbolt.Index(&cfg.Info, "refresh_tokens_by_user", vpack.FInt, vpack.FInt)

// session id => token ids (every token one login has issued)
var RefreshTokenBySessionIndex = vbolt.Index(&cfg.Info, "refresh_tokens_by_session", vpack.FInt, vpack.FInt)

// generateRefreshToken creates a cryptographically secure random token string
func generateRefreshToken() (string, error) {
	b := make([]byte, 32) // 32 bytes = 64 hex characters
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// hashRefreshToken maps a presented token to its stored form.
func hashRefreshToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// CreateRefreshToken starts a new session and returns the record along with the
// token to hand to the client. That string is the only copy — it is not stored
// and cannot be recovered afterwards.
func CreateRefreshToken(tx *vbolt.Tx, userId int, expiryDuration time.Duration) (RefreshToken, string, error) {
	tokenString, err := generateRefreshToken()
	if err != nil {
		return RefreshToken{}, "", err
	}

	now := time.Now()
	token := RefreshToken{
		Id:         vbolt.NextIntId(tx, RefreshTokenBkt),
		UserId:     userId,
		TokenHash:  hashRefreshToken(tokenString),
		ExpiresAt:  now.Add(expiryDuration),
		CreatedAt:  now,
		LastUsedAt: now,
	}
	// A new login is its own session; the first token names the chain.
	token.SessionId = token.Id

	writeRefreshToken(tx, token)
	return token, tokenString, nil
}

// writeRefreshToken stores a token and its lookup entries.
func writeRefreshToken(tx *vbolt.Tx, token RefreshToken) {
	vbolt.Write(tx, RefreshTokenBkt, token.Id, &token)
	vbolt.Write(tx, RefreshTokenByTokenBkt, token.TokenHash, &token.Id)
	vbolt.SetTargetSingleTerm(tx, RefreshTokenByUserIndex, token.Id, token.UserId)
	vbolt.SetTargetSingleTerm(tx, RefreshTokenBySessionIndex, token.Id, token.SessionId)
}

// deleteRefreshTokenRecord removes a token and every entry pointing at it.
func deleteRefreshTokenRecord(tx *vbolt.Tx, token RefreshToken) {
	vbolt.Delete(tx, RefreshTokenBkt, token.Id)
	vbolt.Delete(tx, RefreshTokenByTokenBkt, token.TokenHash)
	vbolt.SetTargetSingleTerm(tx, RefreshTokenByUserIndex, token.Id, -1)
	vbolt.SetTargetSingleTerm(tx, RefreshTokenBySessionIndex, token.Id, -1)
}

// GetRefreshTokenByToken retrieves the record for a presented token.
func GetRefreshTokenByToken(tx *vbolt.Tx, tokenString string) (RefreshToken, bool) {
	var tokenId int
	vbolt.Read(tx, RefreshTokenByTokenBkt, hashRefreshToken(tokenString), &tokenId)
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

// RotateRefreshToken exchanges a presented token for a fresh one.
//
// One-time use is what makes a stolen refresh token detectable: the thief and
// the owner cannot both keep using the session, and whichever of them presents
// the spent token second exposes the theft. On that second use the entire
// session is revoked, which signs out the thief and — deliberately — the owner
// too, since there is no way to tell which is which.
//
// The returned string is the new token to send to the client.
func RotateRefreshToken(tx *vbolt.Tx, presented string, now time.Time) (RefreshToken, string, error) {
	token, found := GetRefreshTokenByToken(tx, presented)
	if !found {
		return RefreshToken{}, "", ErrRefreshTokenInvalid
	}

	if !token.ExpiresAt.After(now) {
		return RefreshToken{}, "", ErrRefreshTokenInvalid
	}

	if token.RotatedAt.IsZero() {
		return rotateFrom(tx, token, now)
	}

	// Already rotated. Inside the grace window this is almost certainly a
	// second tab racing the first, so serve it from whatever token is now at
	// the end of the chain instead of tearing the session down.
	if now.Sub(token.RotatedAt) <= refreshTokenReuseGrace {
		if head, ok := activeSessionToken(tx, token.SessionId, now); ok {
			return rotateFrom(tx, head, now)
		}
	}

	DeleteRefreshTokenSession(tx, token.SessionId)
	return RefreshToken{}, "", ErrRefreshTokenReused
}

// rotateFrom marks token spent and issues its successor in the same session.
// The successor inherits the original expiry, so refreshing extends nothing: a
// session still ends a fixed time after the login that started it.
func rotateFrom(tx *vbolt.Tx, token RefreshToken, now time.Time) (RefreshToken, string, error) {
	tokenString, err := generateRefreshToken()
	if err != nil {
		return RefreshToken{}, "", err
	}

	token.RotatedAt = now
	token.LastUsedAt = now
	vbolt.Write(tx, RefreshTokenBkt, token.Id, &token)

	successor := RefreshToken{
		Id:         vbolt.NextIntId(tx, RefreshTokenBkt),
		UserId:     token.UserId,
		TokenHash:  hashRefreshToken(tokenString),
		SessionId:  token.SessionId,
		ExpiresAt:  token.ExpiresAt,
		CreatedAt:  now,
		LastUsedAt: now,
	}
	writeRefreshToken(tx, successor)

	return successor, tokenString, nil
}

// activeSessionToken returns the session's unrotated, unexpired token, if it
// still has one.
func activeSessionToken(tx *vbolt.Tx, sessionId int, now time.Time) (RefreshToken, bool) {
	for _, token := range sessionTokens(tx, sessionId) {
		if token.RotatedAt.IsZero() && token.ExpiresAt.After(now) {
			return token, true
		}
	}
	return RefreshToken{}, false
}

// sessionTokens reads every token belonging to a session.
func sessionTokens(tx *vbolt.Tx, sessionId int) []RefreshToken {
	var tokenIds []int
	vbolt.ReadTermTargets(tx, RefreshTokenBySessionIndex, sessionId, &tokenIds, vbolt.Window{})

	tokens := make([]RefreshToken, 0, len(tokenIds))
	for _, tokenId := range tokenIds {
		var token RefreshToken
		vbolt.Read(tx, RefreshTokenBkt, tokenId, &token)
		if token.Id != 0 {
			tokens = append(tokens, token)
		}
	}
	return tokens
}

// DeleteRefreshTokenSession removes every token a single login has issued.
func DeleteRefreshTokenSession(tx *vbolt.Tx, sessionId int) {
	for _, token := range sessionTokens(tx, sessionId) {
		deleteRefreshTokenRecord(tx, token)
	}
}

// DeleteRefreshToken ends the session the presented token belongs to. Logging
// out has to invalidate the whole chain, not just the link in the cookie, or
// the spent predecessors would stay in the database until they expired.
func DeleteRefreshToken(tx *vbolt.Tx, tokenString string) {
	token, found := GetRefreshTokenByToken(tx, tokenString)
	if !found {
		return
	}
	DeleteRefreshTokenSession(tx, token.SessionId)
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
		deleteRefreshTokenRecord(tx, token)
	}
}

// CleanupExpiredRefreshTokens removes expired token records and their lookup
// entries. The caller supplies the clock value so cleanup is deterministic in
// tests and every token in a run is evaluated against the same instant.
func CleanupExpiredRefreshTokens(tx *vbolt.Tx, now time.Time) int {
	var expired []RefreshToken
	vbolt.IterateAll(tx, RefreshTokenBkt, func(_ int, token RefreshToken) bool {
		if !token.ExpiresAt.After(now) {
			expired = append(expired, token)
		}
		return true
	})

	for _, token := range expired {
		deleteRefreshTokenRecord(tx, token)
	}
	return len(expired)
}

// RunTokenCleanup purges expired refresh and password reset tokens
// immediately and then at a daily interval until the application context is
// canceled.
func RunTokenCleanup(ctx context.Context, db *vbolt.DB) {
	cleanup := func() {
		vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
			now := time.Now()
			CleanupExpiredRefreshTokens(tx, now)
			CleanupExpiredPasswordResetTokens(tx, now)
			vbolt.TxCommit(tx)
		})
	}

	cleanup()
	ticker := time.NewTicker(refreshTokenCleanupInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			cleanup()
		}
	}
}

// ValidateRefreshToken checks if a token is valid: known, unexpired, and not
// already exchanged for a successor.
func ValidateRefreshToken(tx *vbolt.Tx, tokenString string) (RefreshToken, bool) {
	token, found := GetRefreshTokenByToken(tx, tokenString)
	if !found {
		return RefreshToken{}, false
	}

	if token.ExpiresAt.Before(time.Now()) {
		return RefreshToken{}, false
	}

	if !token.RotatedAt.IsZero() {
		return RefreshToken{}, false
	}

	return token, true
}

// HashStoredRefreshTokens converts version 1 rows, which stored the token
// itself, to the hashed form. Without it every signed-in user would be signed
// out by the deploy that introduced hashing, because their cookie would no
// longer match anything in the lookup bucket.
//
// Rows are identified by having no session, which is only true of version 1
// data; the migration runner also guarantees this runs once.
func HashStoredRefreshTokens(tx *vbolt.Tx) int {
	var legacy []RefreshToken
	vbolt.IterateAll(tx, RefreshTokenBkt, func(_ int, token RefreshToken) bool {
		if token.SessionId == 0 {
			legacy = append(legacy, token)
		}
		return true
	})

	for _, token := range legacy {
		// In a version 1 row this field holds the raw token.
		raw := token.TokenHash
		vbolt.Delete(tx, RefreshTokenByTokenBkt, raw)

		token.TokenHash = hashRefreshToken(raw)
		// Each pre-existing token is the start of its own session; there is no
		// history to reconstruct.
		token.SessionId = token.Id
		writeRefreshToken(tx, token)
	}
	return len(legacy)
}
