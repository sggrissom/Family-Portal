package backend

import (
	"context"
	"os"
	"testing"
	"time"

	"go.hasen.dev/vbeam"
	"go.hasen.dev/vbolt"
)

func setupTestApp(t *testing.T) (*vbeam.Application, func()) {
	tempFile, err := os.CreateTemp("", "test_*.db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tempPath := tempFile.Name()
	tempFile.Close()

	db := vbolt.Open(tempPath)
	app := vbeam.NewApplication("TestApp", db)

	return app, func() {
		db.Close()
		os.Remove(tempPath)
	}
}

func (c *appleKeyCache) reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.keys = nil
	c.fetchedAt = time.Time{}
	c.lastAttempt = time.Time{}
}

func waitForBacklogFeeders() {
	backlogFeeders.Wait()
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

func DefaultLinkScopes() LinkScopes {
	return LinkScopes{People: true, Milestones: true, Photos: true}
}

func (l *workerLifecycle) stopImmediately() bool {
	ctx, cancel := context.WithTimeout(context.Background(), stopWaitTimeout)
	defer cancel()
	return l.stopAndWait(ctx, false)
}
