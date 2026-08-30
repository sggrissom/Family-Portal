package backend

import (
	"errors"
	"family/cfg"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"go.hasen.dev/vbolt"
	"go.hasen.dev/vpack"
)

// App Store Review Guideline 5.1.1(v) requires an app offering Sign in with
// Apple to revoke the user's tokens when their account is deleted, which is
// what this endpoint does.
const appleRevokeURL = "https://appleid.apple.com/auth/revoke"

// appleRevokeEndpoint is a variable so tests can exercise revocation without
// reaching appleid.apple.com.
var appleRevokeEndpoint = appleRevokeURL

// AppleRefreshToken is the credential the revoke call spends. Apple only
// accepts a token from the client it was minted for, so the client ID is kept
// beside it: a user who signed in on the web holds a token for the Services ID
// and one who signed in on the phone holds a token for the bundle ID.
type AppleRefreshToken struct {
	UserId   int       `json:"userId"`
	ClientId string    `json:"clientId"`
	Token    string    `json:"token"`
	Issued   time.Time `json:"issued"`
}

func PackAppleRefreshToken(self *AppleRefreshToken, buf *vpack.Buffer) {
	vpack.Version(1, buf)
	vpack.Int(&self.UserId, buf)
	vpack.String(&self.ClientId, buf)
	vpack.String(&self.Token, buf)
	vpack.Time(&self.Issued, buf)
}

// One row per user rather than one per authorization: revoking any token in a
// grant revokes the whole grant, so the newest is the only one worth keeping.
var AppleRefreshTokenBkt = vbolt.Bucket(&cfg.Info, "apple_refresh_tokens", vpack.FInt, PackAppleRefreshToken)

func GetAppleRefreshToken(tx *vbolt.Tx, userId int) (record AppleRefreshToken) {
	vbolt.Read(tx, AppleRefreshTokenBkt, userId, &record)
	return
}

func storeAppleRefreshTokenTx(tx *vbolt.Tx, userId int, clientId, token string, now time.Time) {
	if userId == 0 || token == "" {
		return
	}
	record := AppleRefreshToken{
		UserId:   userId,
		ClientId: clientId,
		Token:    token,
		Issued:   now,
	}
	vbolt.Write(tx, AppleRefreshTokenBkt, userId, &record)
}

func deleteAppleRefreshTokenTx(tx *vbolt.Tx, userId int) {
	vbolt.Delete(tx, AppleRefreshTokenBkt, userId)
}

// captureAppleRefreshToken exchanges the authorization code the client just
// used and files the refresh token against the account. Sign-in has already
// succeeded by the time this runs, so every failure here is logged and
// swallowed: a user who cannot be given a revocable token should still be let
// in, and deletion falls back on skipping the revoke call.
func captureAppleRefreshToken(r *http.Request, userId int, clientID, code, redirectURI string) {
	if code == "" || userId == 0 {
		return
	}

	exchange, err := exchangeAppleCode(clientID, code, redirectURI)
	if err != nil {
		LogWarnWithRequest(r, LogCategoryAuth, "Apple refresh token could not be obtained", map[string]interface{}{
			"userId": userId,
			"error":  err.Error(),
		})
		return
	}
	if exchange.RefreshToken == "" {
		LogWarnWithRequest(r, LogCategoryAuth, "Apple returned no refresh token to revoke later", map[string]interface{}{
			"userId": userId,
		})
		return
	}

	now := time.Now()
	vbolt.WithWriteTx(appDb, func(tx *vbolt.Tx) {
		storeAppleRefreshTokenTx(tx, userId, clientID, exchange.RefreshToken, now)
		vbolt.TxCommit(tx)
	})
}

// revokeAppleRefreshToken asks Apple to drop the grant behind the stored token.
// Apple answers 200 with an empty body, and treats a token it has already
// revoked as success.
func revokeAppleRefreshToken(record AppleRefreshToken) error {
	if record.Token == "" {
		return errors.New("no Apple refresh token on file")
	}
	if appleWebConfig == nil {
		return errors.New("Apple Sign In not configured")
	}

	clientID := record.ClientId
	if clientID == "" {
		clientID = appleWebConfig.ClientID
	}

	secret, err := appleWebConfig.clientSecret(clientID, time.Now())
	if err != nil {
		return fmt.Errorf("failed to sign client secret: %v", err)
	}

	form := url.Values{
		"client_id":       {clientID},
		"client_secret":   {secret},
		"token":           {record.Token},
		"token_type_hint": {"refresh_token"},
	}

	resp, err := appleHTTPClient.PostForm(appleRevokeEndpoint, form)
	if err != nil {
		var urlErr *url.Error
		if errors.As(err, &urlErr) {
			err = urlErr.Err
		}
		return fmt.Errorf("revoke request failed: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<10))
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("revoke request rejected: %s", strings.TrimSpace(string(body)))
	}

	return nil
}

// revokeAppleTokensForUser is the deletion-time half of 5.1.1(v). It never
// reports failure to the caller: Apple also requires that deletion not be
// blocked, so an unreachable revoke endpoint leaves a log line and the account
// still goes.
func revokeAppleTokensForUser(r *http.Request, userId int) {
	var record AppleRefreshToken
	vbolt.WithReadTx(appDb, func(tx *vbolt.Tx) {
		record = GetAppleRefreshToken(tx, userId)
	})

	// Nothing on file for a user who never used Apple, and nothing for an Apple
	// user who last signed in before the token was captured.
	if record.Token == "" {
		return
	}

	if err := revokeAppleRefreshToken(record); err != nil {
		LogErrorWithRequest(r, LogCategoryAuth, "Apple token revocation failed during account deletion", map[string]interface{}{
			"userId": userId,
			"error":  err.Error(),
		})
		return
	}

	LogInfoWithRequest(r, LogCategoryAuth, "Apple tokens revoked for deleted account", map[string]interface{}{
		"userId": userId,
	})
}
