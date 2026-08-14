package backend

import (
	"encoding/json"
	"family/cfg"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"go.hasen.dev/vbolt"
	"golang.org/x/crypto/bcrypt"
)

// accountTestUser sets up the globals the plain HTTP handlers read and returns a
// user with a known password plus a signed auth token for them.
func accountTestUser(t *testing.T, email, password string) (*vbolt.DB, User, string) {
	t.Helper()

	db := vbolt.Open(t.TempDir() + "/account.db")
	vbolt.InitBuckets(db, &cfg.Info)
	t.Cleanup(func() { _ = db.Close() })
	appDb = db
	jwtKey = []byte("account-test-secret-key-at-least-32-chars")

	var hash []byte
	if password != "" {
		var err error
		hash, err = bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
		if err != nil {
			t.Fatalf("hash password: %v", err)
		}
	}

	var user User
	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		user = AddUserTx(tx, CreateAccountRequest{Name: "Account Tester", Email: email}, hash)
		vbolt.TxCommit(tx)
	})

	token, err := generateJwtTokenString(user)
	if err != nil {
		t.Fatalf("generate auth token: %v", err)
	}
	return db, user, token
}

// silencePasswordChangedEmail keeps the notice off the real mail queue and
// reports how many times it was asked for.
func silencePasswordChangedEmail(t *testing.T) *int {
	t.Helper()

	sent := 0
	original := passwordChangedSender
	passwordChangedSender = func(User) error {
		sent++
		return nil
	}
	t.Cleanup(func() { passwordChangedSender = original })
	return &sent
}

func changePasswordRequest(t *testing.T, authToken string, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/api/change-password", strings.NewReader(body))
	if authToken != "" {
		req.AddCookie(&http.Cookie{Name: "authToken", Value: authToken})
	}
	recorder := httptest.NewRecorder()
	changePasswordHandler(recorder, req)
	return recorder
}

func decodeChangePassword(t *testing.T, recorder *httptest.ResponseRecorder) ChangePasswordResponse {
	t.Helper()

	var resp ChangePasswordResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response %q: %v", recorder.Body.String(), err)
	}
	return resp
}

func TestChangePasswordReplacesTheStoredPassword(t *testing.T) {
	db, user, authToken := accountTestUser(t, "change@example.com", "originalpw")
	sent := silencePasswordChangedEmail(t)

	recorder := changePasswordRequest(t, authToken,
		`{"currentPassword":"originalpw","newPassword":"replacementpw","confirmPassword":"replacementpw"}`)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	resp := decodeChangePassword(t, recorder)
	if !resp.Success {
		t.Fatalf("success = false, error = %q", resp.Error)
	}
	if resp.Token == "" {
		t.Error("no replacement auth token issued")
	}

	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		hash := GetPassHash(tx, user.Id)
		if err := bcrypt.CompareHashAndPassword(hash, []byte("replacementpw")); err != nil {
			t.Errorf("new password does not match stored hash: %v", err)
		}
		if err := bcrypt.CompareHashAndPassword(hash, []byte("originalpw")); err == nil {
			t.Error("old password still matches stored hash")
		}
	})

	if *sent != 1 {
		t.Errorf("queued %d password-changed notices, want 1", *sent)
	}
}

func TestChangePasswordRevokesOtherSessions(t *testing.T) {
	db, user, authToken := accountTestUser(t, "sessions@example.com", "originalpw")
	silencePasswordChangedEmail(t)

	// Two sessions the user is not making this request from.
	var otherTokens []string
	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		for range 2 {
			_, tokenString, err := CreateRefreshToken(tx, user.Id, refreshTokenLifetime)
			if err != nil {
				t.Fatalf("create refresh token: %v", err)
			}
			otherTokens = append(otherTokens, tokenString)
		}
		vbolt.TxCommit(tx)
	})

	recorder := changePasswordRequest(t, authToken,
		`{"currentPassword":"originalpw","newPassword":"replacementpw","confirmPassword":"replacementpw"}`)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}

	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		for i, tokenString := range otherTokens {
			if _, valid := ValidateRefreshToken(tx, tokenString); valid {
				t.Errorf("session %d survived the password change", i)
			}
		}
	})

	// The browser that made the change keeps working: it is handed a refresh
	// cookie naming a live token.
	var refreshCookie *http.Cookie
	for _, cookie := range recorder.Result().Cookies() {
		if cookie.Name == "refreshToken" && cookie.Value != "" {
			refreshCookie = cookie
		}
	}
	if refreshCookie == nil {
		t.Fatal("no replacement refresh cookie was set")
	}
	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		if _, valid := ValidateRefreshToken(tx, refreshCookie.Value); !valid {
			t.Error("replacement refresh token is not usable")
		}
	})
}

func TestChangePasswordRejectsWrongCurrentPassword(t *testing.T) {
	db, user, authToken := accountTestUser(t, "wrong@example.com", "originalpw")
	sent := silencePasswordChangedEmail(t)

	recorder := changePasswordRequest(t, authToken,
		`{"currentPassword":"notthepassword","newPassword":"replacementpw","confirmPassword":"replacementpw"}`)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
	if resp := decodeChangePassword(t, recorder); resp.Success || resp.Error != incorrectPasswordMessage {
		t.Errorf("response = %+v, want failure with %q", resp, incorrectPasswordMessage)
	}
	if *sent != 0 {
		t.Errorf("queued %d notices for a rejected change, want 0", *sent)
	}

	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		if err := bcrypt.CompareHashAndPassword(GetPassHash(tx, user.Id), []byte("originalpw")); err != nil {
			t.Error("password was changed despite the wrong current password")
		}
	})
}

func TestChangePasswordRejectsWeakOrMismatchedNewPassword(t *testing.T) {
	cases := map[string]string{
		"too short":    `{"currentPassword":"originalpw","newPassword":"short","confirmPassword":"short"}`,
		"mismatched":   `{"currentPassword":"originalpw","newPassword":"replacementpw","confirmPassword":"replacementpx"}`,
		"empty":        `{"currentPassword":"originalpw","newPassword":"","confirmPassword":""}`,
		"invalid body": `not json`,
	}

	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			db, user, authToken := accountTestUser(t, "weak@example.com", "originalpw")
			silencePasswordChangedEmail(t)

			recorder := changePasswordRequest(t, authToken, body)
			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusBadRequest, recorder.Body.String())
			}
			vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
				if err := bcrypt.CompareHashAndPassword(GetPassHash(tx, user.Id), []byte("originalpw")); err != nil {
					t.Error("password changed despite an invalid request")
				}
			})
		})
	}
}

func TestChangePasswordRejectsAnAccountWithNoPassword(t *testing.T) {
	_, _, authToken := accountTestUser(t, "google@example.com", "")
	silencePasswordChangedEmail(t)

	recorder := changePasswordRequest(t, authToken,
		`{"currentPassword":"anything","newPassword":"replacementpw","confirmPassword":"replacementpw"}`)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
	if resp := decodeChangePassword(t, recorder); resp.Success || resp.Error != googleOnlyAccountMessage {
		t.Errorf("response = %+v, want failure with %q", resp, googleOnlyAccountMessage)
	}
}

func TestChangePasswordRequiresAuthentication(t *testing.T) {
	accountTestUser(t, "anon@example.com", "originalpw")

	recorder := changePasswordRequest(t, "",
		`{"currentPassword":"originalpw","newPassword":"replacementpw","confirmPassword":"replacementpw"}`)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusUnauthorized, recorder.Body.String())
	}
}

func TestChangePasswordRetiresOutstandingResetLinks(t *testing.T) {
	db, user, authToken := accountTestUser(t, "reset-link@example.com", "originalpw")
	silencePasswordChangedEmail(t)

	var resetToken string
	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		var err error
		resetToken, err = createPasswordResetTokenTx(tx, user.Id, time.Now())
		if err != nil {
			t.Fatalf("create reset token: %v", err)
		}
		vbolt.TxCommit(tx)
	})

	recorder := changePasswordRequest(t, authToken,
		`{"currentPassword":"originalpw","newPassword":"replacementpw","confirmPassword":"replacementpw"}`)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}

	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		if _, valid := validatePasswordResetTokenTx(tx, resetToken, time.Now()); valid {
			t.Error("reset link issued against the old password is still redeemable")
		}
	})
}
