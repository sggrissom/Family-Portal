package backend

import (
	"testing"
	"time"

	"go.hasen.dev/vbeam"
	"go.hasen.dev/vbolt"
)

type sentVerification struct {
	User  User
	Token string
}

func captureVerificationEmails(t *testing.T, sendErr error) *[]sentVerification {
	t.Helper()

	var sent []sentVerification
	original := verificationSender
	verificationSender = func(user User, token string) error {
		sent = append(sent, sentVerification{User: user, Token: token})
		return sendErr
	}
	t.Cleanup(func() { verificationSender = original })
	return &sent
}

func createUnverifiedUser(t *testing.T, app *vbeam.Application, email string) User {
	t.Helper()

	var user User
	vbolt.WithWriteTx(app.DB, func(tx *vbolt.Tx) {
		user = AddUserTx(tx, CreateAccountRequest{Name: "Verify Tester", Email: email}, []byte{})
		vbolt.TxCommit(tx)
	})
	return user
}

func TestNewAccountsStartUnverified(t *testing.T) {
	app, cleanup := setupTestApp(t)
	defer cleanup()

	user := createUnverifiedUser(t, app, "new@example.com")
	if user.EmailVerified {
		t.Error("a new account should not be verified until the address is confirmed")
	}
}

func TestSendVerificationEmailStoresOnlyTheHash(t *testing.T) {
	app, cleanup := setupTestApp(t)
	defer cleanup()
	sent := captureVerificationEmails(t, nil)

	user := createUnverifiedUser(t, app, "hash@example.com")

	vbolt.WithWriteTx(app.DB, func(tx *vbolt.Tx) {
		sendVerificationEmailTx(tx, user, time.Now())
		vbolt.TxCommit(tx)
	})

	if len(*sent) != 1 {
		t.Fatalf("sent %d emails, want 1", len(*sent))
	}

	token := (*sent)[0].Token
	vbolt.WithReadTx(app.DB, func(tx *vbolt.Tx) {
		stored, found := getVerificationTokenTx(tx, token)
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
}

func TestVerifyEmailMarksTheUserAndConsumesTheToken(t *testing.T) {
	app, cleanup := setupTestApp(t)
	defer cleanup()
	sent := captureVerificationEmails(t, nil)

	user := createUnverifiedUser(t, app, "confirm@example.com")
	vbolt.WithWriteTx(app.DB, func(tx *vbolt.Tx) {
		sendVerificationEmailTx(tx, user, time.Now())
		vbolt.TxCommit(tx)
	})
	token := (*sent)[0].Token

	var resp VerifyEmailResponse
	vbolt.WithWriteTx(app.DB, func(tx *vbolt.Tx) {
		resp, _ = VerifyEmail(&vbeam.Context{Tx: tx}, VerifyEmailRequest{Token: token})
	})

	if !resp.Success {
		t.Fatalf("VerifyEmail() success = false, error = %q", resp.Error)
	}

	vbolt.WithReadTx(app.DB, func(tx *vbolt.Tx) {
		if !GetUser(tx, user.Id).EmailVerified {
			t.Error("user should be verified after confirming")
		}
		if _, found := getVerificationTokenTx(tx, token); found {
			t.Error("token should be consumed once used")
		}
	})
}

func TestVerifyEmailRejectsUnknownAndExpiredTokens(t *testing.T) {
	t.Run("unknown token", func(t *testing.T) {
		app, cleanup := setupTestApp(t)
		defer cleanup()

		var resp VerifyEmailResponse
		vbolt.WithWriteTx(app.DB, func(tx *vbolt.Tx) {
			resp, _ = VerifyEmail(&vbeam.Context{Tx: tx}, VerifyEmailRequest{Token: "not-a-real-token"})
		})

		if resp.Success {
			t.Error("an unknown token must not verify anybody")
		}
	})

	t.Run("empty token", func(t *testing.T) {
		app, cleanup := setupTestApp(t)
		defer cleanup()

		var resp VerifyEmailResponse
		vbolt.WithWriteTx(app.DB, func(tx *vbolt.Tx) {
			resp, _ = VerifyEmail(&vbeam.Context{Tx: tx}, VerifyEmailRequest{Token: ""})
		})

		if resp.Success {
			t.Error("an empty token must not verify anybody")
		}
	})

	t.Run("expired token", func(t *testing.T) {
		app, cleanup := setupTestApp(t)
		defer cleanup()
		sent := captureVerificationEmails(t, nil)

		user := createUnverifiedUser(t, app, "expired@example.com")
		stale := time.Now().Add(-emailVerificationLifetime - time.Hour)
		vbolt.WithWriteTx(app.DB, func(tx *vbolt.Tx) {
			sendVerificationEmailTx(tx, user, stale)
			vbolt.TxCommit(tx)
		})

		var resp VerifyEmailResponse
		vbolt.WithWriteTx(app.DB, func(tx *vbolt.Tx) {
			resp, _ = VerifyEmail(&vbeam.Context{Tx: tx}, VerifyEmailRequest{Token: (*sent)[0].Token})
		})

		if resp.Success {
			t.Error("an expired token must not verify anybody")
		}
		vbolt.WithReadTx(app.DB, func(tx *vbolt.Tx) {
			if GetUser(tx, user.Id).EmailVerified {
				t.Error("user must remain unverified")
			}
		})
	})
}

// The identity provider already proved the address, so no round trip is sent.
func TestGoogleVerifiedAddressSkipsConfirmation(t *testing.T) {
	app, cleanup := setupTestApp(t)
	defer cleanup()
	sent := captureVerificationEmails(t, nil)

	user := createUnverifiedUser(t, app, "google@example.com")

	var marked User
	vbolt.WithWriteTx(app.DB, func(tx *vbolt.Tx) {
		marked = markEmailVerifiedTx(tx, user.Id)
		vbolt.TxCommit(tx)
	})

	if !marked.EmailVerified {
		t.Error("markEmailVerifiedTx should verify the account")
	}
	if len(*sent) != 0 {
		t.Errorf("sent %d confirmation emails, want 0", len(*sent))
	}
}

func TestSendVerificationSkipsAlreadyVerifiedUsers(t *testing.T) {
	app, cleanup := setupTestApp(t)
	defer cleanup()
	sent := captureVerificationEmails(t, nil)

	user := createUnverifiedUser(t, app, "already@example.com")
	vbolt.WithWriteTx(app.DB, func(tx *vbolt.Tx) {
		user = markEmailVerifiedTx(tx, user.Id)
		vbolt.TxCommit(tx)
	})

	vbolt.WithWriteTx(app.DB, func(tx *vbolt.Tx) {
		sendVerificationEmailTx(tx, user, time.Now())
		vbolt.TxCommit(tx)
	})

	if len(*sent) != 0 {
		t.Errorf("sent %d emails to a verified user, want 0", len(*sent))
	}
}

func TestVerificationTokensAreReplacedNotAccumulated(t *testing.T) {
	app, cleanup := setupTestApp(t)
	defer cleanup()
	sent := captureVerificationEmails(t, nil)

	user := createUnverifiedUser(t, app, "resend@example.com")
	for i := 0; i < 3; i++ {
		vbolt.WithWriteTx(app.DB, func(tx *vbolt.Tx) {
			sendVerificationEmailTx(tx, user, time.Now())
			vbolt.TxCommit(tx)
		})
	}

	if len(*sent) != 3 {
		t.Fatalf("sent %d emails, want 3", len(*sent))
	}

	vbolt.WithReadTx(app.DB, func(tx *vbolt.Tx) {
		for _, older := range (*sent)[:2] {
			if _, found := getVerificationTokenTx(tx, older.Token); found {
				t.Error("an earlier token should be invalidated when a new one is issued")
			}
		}
		if _, found := getVerificationTokenTx(tx, (*sent)[2].Token); !found {
			t.Error("the newest token should still be valid")
		}
	})
}

func TestCleanupExpiredVerificationTokens(t *testing.T) {
	app, cleanup := setupTestApp(t)
	defer cleanup()
	sent := captureVerificationEmails(t, nil)

	fresh := createUnverifiedUser(t, app, "fresh@example.com")
	stale := createUnverifiedUser(t, app, "stale@example.com")

	vbolt.WithWriteTx(app.DB, func(tx *vbolt.Tx) {
		sendVerificationEmailTx(tx, fresh, time.Now())
		sendVerificationEmailTx(tx, stale, time.Now().Add(-emailVerificationLifetime-time.Hour))
		vbolt.TxCommit(tx)
	})

	var removed int
	vbolt.WithWriteTx(app.DB, func(tx *vbolt.Tx) {
		removed = CleanupExpiredVerificationTokens(tx, time.Now())
		vbolt.TxCommit(tx)
	})

	if removed != 1 {
		t.Errorf("removed %d tokens, want 1", removed)
	}

	vbolt.WithReadTx(app.DB, func(tx *vbolt.Tx) {
		if _, found := getVerificationTokenTx(tx, (*sent)[0].Token); !found {
			t.Error("the unexpired token should survive cleanup")
		}
		if _, found := getVerificationTokenTx(tx, (*sent)[1].Token); found {
			t.Error("the expired token should be gone")
		}
	})
}
