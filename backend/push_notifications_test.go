package backend

import (
	"family/cfg"
	"path/filepath"
	"testing"

	"go.hasen.dev/vbolt"
)

func openPushNotificationTestDB(t *testing.T) *vbolt.DB {
	t.Helper()

	db := vbolt.Open(filepath.Join(t.TempDir(), "push-notifications.db"))
	vbolt.InitBuckets(db, &cfg.Info)
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("close test database: %v", err)
		}
	})
	return db
}

func TestUpsertPushDeviceTokenReassignsUserIndex(t *testing.T) {
	db := openPushNotificationTestDB(t)
	request := RegisterPushDeviceRequest{
		Token:       "device-token",
		Platform:    "ios",
		Environment: "sandbox",
		BundleId:    "dev.family.portal",
	}

	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		created, err := upsertPushDeviceToken(tx, 10, request)
		if err != nil {
			t.Fatalf("upsertPushDeviceToken() create error = %v", err)
		}
		reassigned, err := upsertPushDeviceToken(tx, 20, request)
		if err != nil {
			t.Fatalf("upsertPushDeviceToken() reassign error = %v", err)
		}
		if reassigned.Id != created.Id {
			t.Fatalf("reassigned token ID = %d, want %d", reassigned.Id, created.Id)
		}
		vbolt.TxCommit(tx)
	})

	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		oldUserTokens := GetActiveDeviceTokensForUser(tx, 10)
		if len(oldUserTokens) != 0 {
			t.Fatalf("old user active tokens = %v, want none", oldUserTokens)
		}

		newUserTokens := GetActiveDeviceTokensForUser(tx, 20)
		if len(newUserTokens) != 1 || newUserTokens[0].Token != request.Token {
			t.Fatalf("new user active tokens = %v, want reassigned token", newUserTokens)
		}
	})
}

func TestDeactivatePushDeviceTokenEnforcesOwnership(t *testing.T) {
	db := openPushNotificationTestDB(t)
	request := RegisterPushDeviceRequest{
		Token:       "private-device-token",
		Platform:    "ios",
		Environment: "production",
		BundleId:    "dev.family.portal",
	}

	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		if _, err := upsertPushDeviceToken(tx, 10, request); err != nil {
			t.Fatalf("upsertPushDeviceToken() error = %v", err)
		}
		vbolt.TxCommit(tx)
	})

	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		if err := deactivatePushDeviceToken(tx, 20, request.Token); err == nil {
			t.Fatal("deactivatePushDeviceToken() by non-owner succeeded, want error")
		}
	})

	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		token := GetPushDeviceTokenByToken(tx, request.Token)
		if !token.IsActive {
			t.Fatal("cross-user unregister deactivated token")
		}
	})

	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		if err := deactivatePushDeviceToken(tx, 10, request.Token); err != nil {
			t.Fatalf("deactivatePushDeviceToken() by owner error = %v", err)
		}
		vbolt.TxCommit(tx)
	})

	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		token := GetPushDeviceTokenByToken(tx, request.Token)
		if token.IsActive {
			t.Fatal("owner unregister left token active")
		}
	})
}
