package backend

import (
	"errors"
	"net/http/httptest"
	"testing"
	"time"

	"go.hasen.dev/vbeam"
	"go.hasen.dev/vbolt"
)

func resendTestUser(t *testing.T, db *vbolt.DB, id int, email string) int {
	t.Helper()
	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		user := User{Id: id, Name: "Reset Target", Email: email, Creation: time.Now(), LastLogin: time.Now()}
		vbolt.Write(tx, UsersBkt, user.Id, &user)
		vbolt.TxCommit(tx)
	})
	return id
}

func TestResendPasswordReset(t *testing.T) {
	db := logTestDB(t, "test_resend_reset.db")
	adminToken := adminContext(t, db)
	userId := resendTestUser(t, db, 2, "target@example.com")
	sent := captureResetEmails(t, nil)

	var resp ResendPasswordResetResponse
	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		var err error
		resp, err = ResendPasswordReset(&vbeam.Context{Tx: tx, Token: adminToken}, ResendPasswordResetRequest{UserId: userId})
		if err != nil {
			t.Fatalf("ResendPasswordReset() error = %v", err)
		}
	})

	if !resp.Queued {
		t.Errorf("Queued = false, want true — detail was %q", resp.Detail)
	}
	if resp.Email != "target@example.com" {
		t.Errorf("Email = %q, want the address the mail went to", resp.Email)
	}
	if resp.InvalidatedPrevious {
		t.Error("InvalidatedPrevious = true, want false — this user had no live link")
	}
	if len(*sent) != 1 {
		t.Fatalf("sent %d reset emails, want 1", len(*sent))
	}

	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		if _, valid := validatePasswordResetTokenTx(tx, (*sent)[0].Token, time.Now()); !valid {
			t.Error("the emailed token does not validate")
		}
	})
}

func TestResendPasswordResetInvalidatesTheEarlierLink(t *testing.T) {
	db := logTestDB(t, "test_resend_reset_invalidate.db")
	adminToken := adminContext(t, db)
	userId := resendTestUser(t, db, 2, "target@example.com")
	sent := captureResetEmails(t, nil)

	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		if _, err := ResendPasswordReset(&vbeam.Context{Tx: tx, Token: adminToken}, ResendPasswordResetRequest{UserId: userId}); err != nil {
			t.Fatalf("first ResendPasswordReset() error = %v", err)
		}
	})

	var second ResendPasswordResetResponse
	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		var err error
		second, err = ResendPasswordReset(&vbeam.Context{Tx: tx, Token: adminToken}, ResendPasswordResetRequest{UserId: userId})
		if err != nil {
			t.Fatalf("second ResendPasswordReset() error = %v", err)
		}
	})

	if !second.InvalidatedPrevious {
		t.Error("InvalidatedPrevious = false, want true — the first link was still live")
	}
	if len(*sent) != 2 {
		t.Fatalf("sent %d reset emails, want 2", len(*sent))
	}

	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		if _, valid := validatePasswordResetTokenTx(tx, (*sent)[0].Token, time.Now()); valid {
			t.Error("the first token still validates; a resend must not leave two live links")
		}
		if _, valid := validatePasswordResetTokenTx(tx, (*sent)[1].Token, time.Now()); !valid {
			t.Error("the second token does not validate")
		}
	})
}

func TestResendPasswordResetReportsAQueueRefusal(t *testing.T) {
	db := logTestDB(t, "test_resend_reset_refused.db")
	adminToken := adminContext(t, db)
	userId := resendTestUser(t, db, 2, "target@example.com")
	sent := captureResetEmails(t, errors.New("mail queue is full"))

	var resp ResendPasswordResetResponse
	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		var err error
		resp, err = ResendPasswordReset(&vbeam.Context{Tx: tx, Token: adminToken}, ResendPasswordResetRequest{UserId: userId})
		if err != nil {
			t.Fatalf("ResendPasswordReset() error = %v", err)
		}
	})

	if resp.Queued {
		t.Error("Queued = true, want false — the mail worker refused the job")
	}
	if resp.Detail == "" {
		t.Error("Detail is empty; a refusal has to say why")
	}

	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		if _, valid := validatePasswordResetTokenTx(tx, (*sent)[0].Token, time.Now()); !valid {
			t.Error("the token was not created; a mail failure should not lose the link the admin can read out")
		}
	})
}

func TestResendPasswordResetRefusals(t *testing.T) {
	db := logTestDB(t, "test_resend_reset_refusals.db")
	adminToken := adminContext(t, db)
	userId := resendTestUser(t, db, 2, "target@example.com")
	captureResetEmails(t, nil)

	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		for _, id := range []int{0, -1, 9999} {
			if _, err := ResendPasswordReset(&vbeam.Context{Tx: tx, Token: adminToken}, ResendPasswordResetRequest{UserId: id}); err != ErrUserNotFound {
				t.Errorf("UserId %d: expected ErrUserNotFound, got %v", id, err)
			}
		}
	})

	regular, _ := generateAuthJwt(User{Id: 2, Email: "target@example.com"}, httptest.NewRecorder())
	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		if _, err := ResendPasswordReset(&vbeam.Context{Tx: tx, Token: regular}, ResendPasswordResetRequest{UserId: userId}); err != ErrAdminRequired {
			t.Errorf("Expected ErrAdminRequired, got %v", err)
		}
	})
}

func TestGetMailStatsRequiresAdmin(t *testing.T) {
	db := logTestDB(t, "test_mail_stats_auth.db")
	adminToken := adminContext(t, db)

	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		stats, err := GetMailStats(&vbeam.Context{Tx: tx, Token: adminToken}, GetMailStatsRequest{})
		if err != nil {
			t.Fatalf("GetMailStats() error = %v", err)
		}
		if stats.RecentAttempts == nil {
			t.Error("RecentAttempts = nil, want an empty list so the client can render it")
		}
	})

	regular, _ := generateAuthJwt(User{Id: 2, Email: "regular@example.com"}, httptest.NewRecorder())
	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		if _, err := GetMailStats(&vbeam.Context{Tx: tx, Token: regular}, GetMailStatsRequest{}); err != ErrAdminRequired {
			t.Errorf("Expected ErrAdminRequired, got %v", err)
		}
	})
}
