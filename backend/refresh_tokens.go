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

const refreshTokenReuseGrace = time.Minute

var (
	ErrRefreshTokenInvalid = errors.New("refresh token is invalid")
	ErrRefreshTokenReused  = errors.New("refresh token was reused")
)

type RefreshToken struct {
	Id         int       `json:"id"`
	UserId     int       `json:"userId"`
	TokenHash  string    `json:"-"`
	SessionId  int       `json:"sessionId"`
	ExpiresAt  time.Time `json:"expiresAt"`
	CreatedAt  time.Time `json:"createdAt"`
	LastUsedAt time.Time `json:"lastUsedAt"`
	RotatedAt  time.Time `json:"rotatedAt"`
}

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

var RefreshTokenBkt = vbolt.Bucket(&cfg.Info, "refresh_tokens", vpack.FInt, PackRefreshToken)

var RefreshTokenByTokenBkt = vbolt.Bucket(&cfg.Info, "refresh_tokens_by_token", vpack.StringZ, vpack.Int)

var RefreshTokenByUserIndex = vbolt.Index(&cfg.Info, "refresh_tokens_by_user", vpack.FInt, vpack.FInt)

var RefreshTokenBySessionIndex = vbolt.Index(&cfg.Info, "refresh_tokens_by_session", vpack.FInt, vpack.FInt)

func generateRefreshToken() (string, error) {
	b := make([]byte, 32)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func hashRefreshToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

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
	token.SessionId = token.Id

	writeRefreshToken(tx, token)
	return token, tokenString, nil
}

func writeRefreshToken(tx *vbolt.Tx, token RefreshToken) {
	vbolt.Write(tx, RefreshTokenBkt, token.Id, &token)
	vbolt.Write(tx, RefreshTokenByTokenBkt, token.TokenHash, &token.Id)
	vbolt.SetTargetSingleTerm(tx, RefreshTokenByUserIndex, token.Id, token.UserId)
	vbolt.SetTargetSingleTerm(tx, RefreshTokenBySessionIndex, token.Id, token.SessionId)
}

func deleteRefreshTokenRecord(tx *vbolt.Tx, token RefreshToken) {
	vbolt.Delete(tx, RefreshTokenBkt, token.Id)
	vbolt.Delete(tx, RefreshTokenByTokenBkt, token.TokenHash)
	vbolt.SetTargetSingleTerm(tx, RefreshTokenByUserIndex, token.Id, -1)
	vbolt.SetTargetSingleTerm(tx, RefreshTokenBySessionIndex, token.Id, -1)
}

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

func UpdateRefreshTokenLastUsed(tx *vbolt.Tx, tokenId int) {
	var token RefreshToken
	vbolt.Read(tx, RefreshTokenBkt, tokenId, &token)
	if token.Id == 0 {
		return
	}

	token.LastUsedAt = time.Now()
	vbolt.Write(tx, RefreshTokenBkt, token.Id, &token)
}

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

	if now.Sub(token.RotatedAt) <= refreshTokenReuseGrace {
		if head, ok := activeSessionToken(tx, token.SessionId, now); ok {
			return rotateFrom(tx, head, now)
		}
	}

	DeleteRefreshTokenSession(tx, token.SessionId)
	return RefreshToken{}, "", ErrRefreshTokenReused
}

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

func activeSessionToken(tx *vbolt.Tx, sessionId int, now time.Time) (RefreshToken, bool) {
	for _, token := range sessionTokens(tx, sessionId) {
		if token.RotatedAt.IsZero() && token.ExpiresAt.After(now) {
			return token, true
		}
	}
	return RefreshToken{}, false
}

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

func DeleteRefreshTokenSession(tx *vbolt.Tx, sessionId int) {
	for _, token := range sessionTokens(tx, sessionId) {
		deleteRefreshTokenRecord(tx, token)
	}
}

func DeleteRefreshToken(tx *vbolt.Tx, tokenString string) {
	token, found := GetRefreshTokenByToken(tx, tokenString)
	if !found {
		return
	}
	DeleteRefreshTokenSession(tx, token.SessionId)
}

func DeleteUserRefreshTokens(tx *vbolt.Tx, userId int) {
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

func RunTokenCleanup(ctx context.Context, db *vbolt.DB) {
	cleanup := func() {
		vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
			now := time.Now()
			CleanupExpiredRefreshTokens(tx, now)
			CleanupExpiredPasswordResetTokens(tx, now)
			CleanupExpiredVerificationTokens(tx, now)
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

func HashStoredRefreshTokens(tx *vbolt.Tx) int {
	var legacy []RefreshToken
	vbolt.IterateAll(tx, RefreshTokenBkt, func(_ int, token RefreshToken) bool {
		if token.SessionId == 0 {
			legacy = append(legacy, token)
		}
		return true
	})

	for _, token := range legacy {
		raw := token.TokenHash
		vbolt.Delete(tx, RefreshTokenByTokenBkt, raw)

		token.TokenHash = hashRefreshToken(raw)
		token.SessionId = token.Id
		writeRefreshToken(tx, token)
	}
	return len(legacy)
}
