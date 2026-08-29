package backend

import (
	"errors"
	"strings"
	"testing"
	"time"

	"go.hasen.dev/vbeam"
	"go.hasen.dev/vbolt"
	"golang.org/x/crypto/bcrypt"
)

type sentReset struct {
	User  User
	Token string
}

func captureResetEmails(t *testing.T, sendErr error) *[]sentReset {
	t.Helper()

	var sent []sentReset
	original := passwordResetSender
	passwordResetSender = func(user User, token string) error {
		sent = append(sent, sentReset{User: user, Token: token})
		return sendErr
	}
	t.Cleanup(func() { passwordResetSender = original })
	return &sent
}

func createResetTestUser(t *testing.T, app *vbeam.Application, email, password string) User {
	t.Helper()

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	var user User
	vbolt.WithWriteTx(app.DB, func(tx *vbolt.Tx) {
		user = AddUserTx(tx, CreateAccountRequest{
			Name:  "Reset Tester",
			Email: email,
		}, hash)
		vbolt.TxCommit(tx)
	})
	return user
}

func TestRequestPasswordReset(t *testing.T) {
	t.Run("issues a token and emails the account holder", func(t *testing.T) {
		app, cleanup := setupTestApp(t)
		defer cleanup()
		sent := captureResetEmails(t, nil)

		user := createResetTestUser(t, app, "reset@example.com", "originalpw")

		var resp RequestPasswordResetResponse
		var err error
		vbolt.WithWriteTx(app.DB, func(tx *vbolt.Tx) {
			resp, err = RequestPasswordReset(&vbeam.Context{Tx: tx}, RequestPasswordResetRequest{
				Email: "reset@example.com",
			})
		})

		if err != nil {
			t.Fatalf("RequestPasswordReset() error = %v", err)
		}
		if !resp.Success {
			t.Fatalf("RequestPasswordReset() success = false, error = %q", resp.Error)
		}
		if len(*sent) != 1 {
			t.Fatalf("sent %d emails, want 1", len(*sent))
		}
		if (*sent)[0].User.Id != user.Id {
			t.Errorf("emailed user %d, want %d", (*sent)[0].User.Id, user.Id)
		}

		token := (*sent)[0].Token
		vbolt.WithReadTx(app.DB, func(tx *vbolt.Tx) {
			stored, found := getPasswordResetTokenTx(tx, token)
			if !found {
				t.Fatal("token was not persisted")
			}
			if stored.TokenHash == token {
				t.Error("raw token was stored; expected only its hash")
			}
			if stored.UserId != user.Id {
				t.Errorf("token user = %d, want %d", stored.UserId, user.Id)
			}
		})
	})

	t.Run("reports success for an unknown address without sending mail", func(t *testing.T) {
		app, cleanup := setupTestApp(t)
		defer cleanup()
		sent := captureResetEmails(t, nil)

		var resp RequestPasswordResetResponse
		vbolt.WithWriteTx(app.DB, func(tx *vbolt.Tx) {
			resp, _ = RequestPasswordReset(&vbeam.Context{Tx: tx}, RequestPasswordResetRequest{
				Email: "nobody@example.com",
			})
		})

		if !resp.Success {
			t.Error("unknown address must not be distinguishable from a known one")
		}
		if len(*sent) != 0 {
			t.Errorf("sent %d emails for an unknown address, want 0", len(*sent))
		}
	})

	t.Run("rejects an empty address", func(t *testing.T) {
		app, cleanup := setupTestApp(t)
		defer cleanup()
		captureResetEmails(t, nil)

		var resp RequestPasswordResetResponse
		vbolt.WithWriteTx(app.DB, func(tx *vbolt.Tx) {
			resp, _ = RequestPasswordReset(&vbeam.Context{Tx: tx}, RequestPasswordResetRequest{Email: ""})
		})

		if resp.Success {
			t.Error("empty address was accepted")
		}
	})

	t.Run("throttles repeat requests for one account", func(t *testing.T) {
		app, cleanup := setupTestApp(t)
		defer cleanup()
		sent := captureResetEmails(t, nil)

		createResetTestUser(t, app, "throttle@example.com", "originalpw")

		for i := 0; i < 3; i++ {
			vbolt.WithWriteTx(app.DB, func(tx *vbolt.Tx) {
				RequestPasswordReset(&vbeam.Context{Tx: tx}, RequestPasswordResetRequest{
					Email: "throttle@example.com",
				})
			})
		}

		if len(*sent) != 1 {
			t.Errorf("sent %d emails in quick succession, want 1", len(*sent))
		}
	})

	t.Run("a new request invalidates the previous link", func(t *testing.T) {
		app, cleanup := setupTestApp(t)
		defer cleanup()
		sent := captureResetEmails(t, nil)

		user := createResetTestUser(t, app, "rotate@example.com", "originalpw")

		var first, second string
		vbolt.WithWriteTx(app.DB, func(tx *vbolt.Tx) {
			token, err := createPasswordResetTokenTx(tx, user.Id, time.Now())
			if err != nil {
				t.Fatalf("createPasswordResetTokenTx() error = %v", err)
			}
			first = token
			vbolt.TxCommit(tx)
		})
		vbolt.WithWriteTx(app.DB, func(tx *vbolt.Tx) {
			token, err := createPasswordResetTokenTx(tx, user.Id, time.Now())
			if err != nil {
				t.Fatalf("createPasswordResetTokenTx() error = %v", err)
			}
			second = token
			vbolt.TxCommit(tx)
		})
		_ = sent

		vbolt.WithReadTx(app.DB, func(tx *vbolt.Tx) {
			if _, valid := validatePasswordResetTokenTx(tx, first, time.Now()); valid {
				t.Error("superseded token is still valid")
			}
			if _, valid := validatePasswordResetTokenTx(tx, second, time.Now()); !valid {
				t.Error("newest token should be valid")
			}
		})
	})

	t.Run("still reports success when delivery fails", func(t *testing.T) {
		app, cleanup := setupTestApp(t)
		defer cleanup()
		captureResetEmails(t, errors.New("smtp unavailable"))

		createResetTestUser(t, app, "smtpdown@example.com", "originalpw")

		var resp RequestPasswordResetResponse
		vbolt.WithWriteTx(app.DB, func(tx *vbolt.Tx) {
			resp, _ = RequestPasswordReset(&vbeam.Context{Tx: tx}, RequestPasswordResetRequest{
				Email: "smtpdown@example.com",
			})
		})

		if !resp.Success {
			t.Error("a delivery failure must not reveal that the address exists")
		}
	})
}

func TestValidatePasswordResetToken(t *testing.T) {
	app, cleanup := setupTestApp(t)
	defer cleanup()

	user := createResetTestUser(t, app, "validate@example.com", "originalpw")

	var token string
	vbolt.WithWriteTx(app.DB, func(tx *vbolt.Tx) {
		var err error
		token, err = createPasswordResetTokenTx(tx, user.Id, time.Now())
		if err != nil {
			t.Fatalf("createPasswordResetTokenTx() error = %v", err)
		}
		vbolt.TxCommit(tx)
	})

	tests := []struct {
		name  string
		token string
		want  bool
	}{
		{"issued token", token, true},
		{"unknown token", "deadbeef", false},
		{"empty token", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var resp ValidatePasswordResetTokenResponse
			vbolt.WithReadTx(app.DB, func(tx *vbolt.Tx) {
				resp, _ = ValidatePasswordResetToken(&vbeam.Context{Tx: tx}, ValidatePasswordResetTokenRequest{
					Token: tt.token,
				})
			})
			if resp.Valid != tt.want {
				t.Errorf("valid = %v, want %v", resp.Valid, tt.want)
			}
		})
	}
}

func TestValidatePasswordResetTokenExpiry(t *testing.T) {
	app, cleanup := setupTestApp(t)
	defer cleanup()

	user := createResetTestUser(t, app, "expiry@example.com", "originalpw")

	issued := time.Now().Add(-2 * passwordResetTokenLifetime)
	var token string
	vbolt.WithWriteTx(app.DB, func(tx *vbolt.Tx) {
		var err error
		token, err = createPasswordResetTokenTx(tx, user.Id, issued)
		if err != nil {
			t.Fatalf("createPasswordResetTokenTx() error = %v", err)
		}
		vbolt.TxCommit(tx)
	})

	vbolt.WithReadTx(app.DB, func(tx *vbolt.Tx) {
		if _, valid := validatePasswordResetTokenTx(tx, token, time.Now()); valid {
			t.Error("expired token was accepted")
		}
		if _, valid := validatePasswordResetTokenTx(tx, token, issued.Add(time.Minute)); !valid {
			t.Error("token should have been valid shortly after it was issued")
		}
	})
}

func TestResetPassword(t *testing.T) {
	const oldPassword = "originalpw"
	const newPassword = "brand-new-password"

	issueToken := func(t *testing.T, app *vbeam.Application, userId int) string {
		t.Helper()
		var token string
		vbolt.WithWriteTx(app.DB, func(tx *vbolt.Tx) {
			var err error
			token, err = createPasswordResetTokenTx(tx, userId, time.Now())
			if err != nil {
				t.Fatalf("createPasswordResetTokenTx() error = %v", err)
			}
			vbolt.TxCommit(tx)
		})
		return token
	}

	t.Run("replaces the password and consumes the token", func(t *testing.T) {
		app, cleanup := setupTestApp(t)
		defer cleanup()

		user := createResetTestUser(t, app, "change@example.com", oldPassword)
		token := issueToken(t, app, user.Id)

		var resp ResetPasswordResponse
		var err error
		vbolt.WithWriteTx(app.DB, func(tx *vbolt.Tx) {
			resp, err = ResetPassword(&vbeam.Context{Tx: tx}, ResetPasswordRequest{
				Token:           token,
				Password:        newPassword,
				ConfirmPassword: newPassword,
			})
		})

		if err != nil {
			t.Fatalf("ResetPassword() error = %v", err)
		}
		if !resp.Success {
			t.Fatalf("ResetPassword() success = false, error = %q", resp.Error)
		}

		vbolt.WithReadTx(app.DB, func(tx *vbolt.Tx) {
			hash := GetPassHash(tx, user.Id)
			if bcrypt.CompareHashAndPassword(hash, []byte(newPassword)) != nil {
				t.Error("new password does not verify against the stored hash")
			}
			if bcrypt.CompareHashAndPassword(hash, []byte(oldPassword)) == nil {
				t.Error("old password still verifies after a reset")
			}
			if _, valid := validatePasswordResetTokenTx(tx, token, time.Now()); valid {
				t.Error("token is still usable after a successful reset")
			}
		})
	})

	t.Run("revokes every refresh token for the account", func(t *testing.T) {
		app, cleanup := setupTestApp(t)
		defer cleanup()

		user := createResetTestUser(t, app, "sessions@example.com", oldPassword)

		var refreshToken string
		vbolt.WithWriteTx(app.DB, func(tx *vbolt.Tx) {
			_, created, err := CreateRefreshToken(tx, user.Id, time.Hour)
			if err != nil {
				t.Fatalf("CreateRefreshToken() error = %v", err)
			}
			refreshToken = created
			vbolt.TxCommit(tx)
		})

		token := issueToken(t, app, user.Id)
		vbolt.WithWriteTx(app.DB, func(tx *vbolt.Tx) {
			ResetPassword(&vbeam.Context{Tx: tx}, ResetPasswordRequest{
				Token:           token,
				Password:        newPassword,
				ConfirmPassword: newPassword,
			})
		})

		vbolt.WithReadTx(app.DB, func(tx *vbolt.Tx) {
			if _, valid := ValidateRefreshToken(tx, refreshToken); valid {
				t.Error("refresh token survived a password reset")
			}
		})
	})

	t.Run("rejects a token that has already been used", func(t *testing.T) {
		app, cleanup := setupTestApp(t)
		defer cleanup()

		user := createResetTestUser(t, app, "replay@example.com", oldPassword)
		token := issueToken(t, app, user.Id)

		vbolt.WithWriteTx(app.DB, func(tx *vbolt.Tx) {
			ResetPassword(&vbeam.Context{Tx: tx}, ResetPasswordRequest{
				Token:           token,
				Password:        newPassword,
				ConfirmPassword: newPassword,
			})
		})

		var resp ResetPasswordResponse
		vbolt.WithWriteTx(app.DB, func(tx *vbolt.Tx) {
			resp, _ = ResetPassword(&vbeam.Context{Tx: tx}, ResetPasswordRequest{
				Token:           token,
				Password:        "second-attempt-password",
				ConfirmPassword: "second-attempt-password",
			})
		})

		if resp.Success {
			t.Error("a reset token was accepted twice")
		}

		vbolt.WithReadTx(app.DB, func(tx *vbolt.Tx) {
			hash := GetPassHash(tx, user.Id)
			if bcrypt.CompareHashAndPassword(hash, []byte(newPassword)) != nil {
				t.Error("replayed reset overwrote the password set by the first use")
			}
		})
	})

	t.Run("rejects an expired token", func(t *testing.T) {
		app, cleanup := setupTestApp(t)
		defer cleanup()

		user := createResetTestUser(t, app, "stale@example.com", oldPassword)

		var token string
		vbolt.WithWriteTx(app.DB, func(tx *vbolt.Tx) {
			var err error
			token, err = createPasswordResetTokenTx(tx, user.Id, time.Now().Add(-2*passwordResetTokenLifetime))
			if err != nil {
				t.Fatalf("createPasswordResetTokenTx() error = %v", err)
			}
			vbolt.TxCommit(tx)
		})

		var resp ResetPasswordResponse
		vbolt.WithWriteTx(app.DB, func(tx *vbolt.Tx) {
			resp, _ = ResetPassword(&vbeam.Context{Tx: tx}, ResetPasswordRequest{
				Token:           token,
				Password:        newPassword,
				ConfirmPassword: newPassword,
			})
		})

		if resp.Success {
			t.Fatal("expired token was accepted")
		}

		vbolt.WithReadTx(app.DB, func(tx *vbolt.Tx) {
			if bcrypt.CompareHashAndPassword(GetPassHash(tx, user.Id), []byte(oldPassword)) != nil {
				t.Error("password changed despite the expired token")
			}
		})
	})

	t.Run("rejects an unknown token", func(t *testing.T) {
		app, cleanup := setupTestApp(t)
		defer cleanup()

		var resp ResetPasswordResponse
		vbolt.WithWriteTx(app.DB, func(tx *vbolt.Tx) {
			resp, _ = ResetPassword(&vbeam.Context{Tx: tx}, ResetPasswordRequest{
				Token:           "not-a-real-token",
				Password:        newPassword,
				ConfirmPassword: newPassword,
			})
		})

		if resp.Success {
			t.Error("unknown token was accepted")
		}
	})

	t.Run("enforces password rules", func(t *testing.T) {
		app, cleanup := setupTestApp(t)
		defer cleanup()

		user := createResetTestUser(t, app, "rules@example.com", oldPassword)

		tests := []struct {
			name     string
			password string
			confirm  string
		}{
			{"too short", "short", "short"},
			{"mismatched", "longenoughpassword", "different-password"},
			{"empty", "", ""},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				token := issueToken(t, app, user.Id)

				var resp ResetPasswordResponse
				vbolt.WithWriteTx(app.DB, func(tx *vbolt.Tx) {
					resp, _ = ResetPassword(&vbeam.Context{Tx: tx}, ResetPasswordRequest{
						Token:           token,
						Password:        tt.password,
						ConfirmPassword: tt.confirm,
					})
				})

				if resp.Success {
					t.Fatalf("accepted invalid password %q", tt.password)
				}
				if resp.Error == "" {
					t.Error("no error message for a rejected password")
				}

				vbolt.WithReadTx(app.DB, func(tx *vbolt.Tx) {
					if _, valid := validatePasswordResetTokenTx(tx, token, time.Now()); !valid {
						t.Error("token was consumed by a failed attempt")
					}
				})
			})
		}
	})
}

func TestCleanupExpiredPasswordResetTokens(t *testing.T) {
	app, cleanup := setupTestApp(t)
	defer cleanup()

	user := createResetTestUser(t, app, "cleanup@example.com", "originalpw")
	now := time.Now()

	var live string
	vbolt.WithWriteTx(app.DB, func(tx *vbolt.Tx) {
		if _, err := createPasswordResetTokenTx(tx, user.Id, now.Add(-2*passwordResetTokenLifetime)); err != nil {
			t.Fatalf("createPasswordResetTokenTx() error = %v", err)
		}
		vbolt.TxCommit(tx)
	})

	other := createResetTestUser(t, app, "cleanup2@example.com", "originalpw")
	vbolt.WithWriteTx(app.DB, func(tx *vbolt.Tx) {
		var err error
		live, err = createPasswordResetTokenTx(tx, other.Id, now)
		if err != nil {
			t.Fatalf("createPasswordResetTokenTx() error = %v", err)
		}
		vbolt.TxCommit(tx)
	})

	var removed int
	vbolt.WithWriteTx(app.DB, func(tx *vbolt.Tx) {
		removed = CleanupExpiredPasswordResetTokens(tx, now)
		vbolt.TxCommit(tx)
	})

	if removed != 1 {
		t.Errorf("removed %d tokens, want 1", removed)
	}

	vbolt.WithReadTx(app.DB, func(tx *vbolt.Tx) {
		if _, valid := validatePasswordResetTokenTx(tx, live, now); !valid {
			t.Error("cleanup removed a token that had not expired")
		}
	})
}

func TestPasswordResetLink(t *testing.T) {
	t.Setenv("SITE_ROOT", "https://example.test/")

	link := passwordResetLink("abc123")
	if link != "https://example.test/reset-password?token=abc123" {
		t.Errorf("passwordResetLink() = %q", link)
	}
}

func TestPasswordResetBodyIncludesLink(t *testing.T) {
	body := passwordResetBody("Sam", "https://example.test/reset-password?token=abc123")

	if !strings.Contains(body, "https://example.test/reset-password?token=abc123") {
		t.Error("reset email body does not contain the link")
	}
	if !strings.Contains(body, "Sam") {
		t.Error("reset email body does not greet the recipient by name")
	}
}

func capturePasswordChangedEmails(t *testing.T, sendErr error) *[]User {
	t.Helper()

	var sent []User
	original := passwordChangedSender
	passwordChangedSender = func(user User) error {
		sent = append(sent, user)
		return sendErr
	}
	t.Cleanup(func() { passwordChangedSender = original })
	return &sent
}

func TestResetPasswordNotifiesTheAccountHolder(t *testing.T) {
	const oldPassword = "originalpw"
	const newPassword = "brand-new-password"

	t.Run("confirms a completed reset", func(t *testing.T) {
		app, cleanup := setupTestApp(t)
		defer cleanup()

		notices := capturePasswordChangedEmails(t, nil)
		user := createResetTestUser(t, app, "notify@example.com", oldPassword)

		var token string
		vbolt.WithWriteTx(app.DB, func(tx *vbolt.Tx) {
			var err error
			token, err = createPasswordResetTokenTx(tx, user.Id, time.Now())
			if err != nil {
				t.Fatalf("createPasswordResetTokenTx() error = %v", err)
			}
			vbolt.TxCommit(tx)
		})

		var resp ResetPasswordResponse
		vbolt.WithWriteTx(app.DB, func(tx *vbolt.Tx) {
			var err error
			resp, err = ResetPassword(&vbeam.Context{Tx: tx}, ResetPasswordRequest{
				Token:           token,
				Password:        newPassword,
				ConfirmPassword: newPassword,
			})
			if err != nil {
				t.Fatalf("ResetPassword() error = %v", err)
			}
		})
		if !resp.Success {
			t.Fatalf("ResetPassword() success = false, error = %q", resp.Error)
		}

		if len(*notices) != 1 {
			t.Fatalf("confirmation emails = %d, want 1", len(*notices))
		}
		if (*notices)[0].Email != "notify@example.com" {
			t.Errorf("confirmation sent to %q, want notify@example.com", (*notices)[0].Email)
		}
	})

	t.Run("stays quiet when no reset happened", func(t *testing.T) {
		app, cleanup := setupTestApp(t)
		defer cleanup()

		notices := capturePasswordChangedEmails(t, nil)
		createResetTestUser(t, app, "quiet@example.com", oldPassword)

		vbolt.WithWriteTx(app.DB, func(tx *vbolt.Tx) {
			resp, err := ResetPassword(&vbeam.Context{Tx: tx}, ResetPasswordRequest{
				Token:           "not-a-real-token",
				Password:        newPassword,
				ConfirmPassword: newPassword,
			})
			if err != nil {
				t.Fatalf("ResetPassword() error = %v", err)
			}
			if resp.Success {
				t.Fatal("ResetPassword() accepted an invalid token")
			}
		})

		if len(*notices) != 0 {
			t.Errorf("confirmation emails = %d, want 0; a rejected reset must not notify", len(*notices))
		}
	})

	t.Run("still succeeds when the notice cannot be sent", func(t *testing.T) {
		app, cleanup := setupTestApp(t)
		defer cleanup()

		capturePasswordChangedEmails(t, errors.New("smtp unavailable"))
		user := createResetTestUser(t, app, "mailfail@example.com", oldPassword)

		var token string
		vbolt.WithWriteTx(app.DB, func(tx *vbolt.Tx) {
			var err error
			token, err = createPasswordResetTokenTx(tx, user.Id, time.Now())
			if err != nil {
				t.Fatalf("createPasswordResetTokenTx() error = %v", err)
			}
			vbolt.TxCommit(tx)
		})

		var resp ResetPasswordResponse
		vbolt.WithWriteTx(app.DB, func(tx *vbolt.Tx) {
			var err error
			resp, err = ResetPassword(&vbeam.Context{Tx: tx}, ResetPasswordRequest{
				Token:           token,
				Password:        newPassword,
				ConfirmPassword: newPassword,
			})
			if err != nil {
				t.Fatalf("ResetPassword() error = %v", err)
			}
		})

		if !resp.Success {
			t.Fatalf("ResetPassword() success = false, error = %q", resp.Error)
		}
		vbolt.WithReadTx(app.DB, func(tx *vbolt.Tx) {
			hash := GetPassHash(tx, user.Id)
			if bcrypt.CompareHashAndPassword(hash, []byte(newPassword)) != nil {
				t.Error("password was not changed despite a successful response")
			}
		})
	})
}

func TestPasswordChangedBodyNotifiesWithoutALink(t *testing.T) {
	body := passwordChangedBody("Sam")

	if !strings.Contains(body, "Sam") {
		t.Error("confirmation body does not greet the recipient by name")
	}
	if !strings.Contains(body, "changed") {
		t.Error("confirmation body does not say the password changed")
	}
	if strings.Contains(body, "http://") || strings.Contains(body, "https://") {
		t.Errorf("confirmation body contains a link:\n%s", body)
	}
}
