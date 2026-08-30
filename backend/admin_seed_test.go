package backend

import (
	"errors"
	"net/http/httptest"
	"testing"

	"go.hasen.dev/vbeam"
	"go.hasen.dev/vbolt"
	"golang.org/x/crypto/bcrypt"
)

const testSeedPassword = "review-account-pw"

func createSeedData(t *testing.T, db *vbolt.DB, token string, req CreateSeedDataRequest) (CreateSeedDataResponse, error) {
	t.Helper()
	var resp CreateSeedDataResponse
	var err error
	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		resp, err = CreateSeedData(&vbeam.Context{Tx: tx, Token: token}, req)
	})
	return resp, err
}

func removeSeedData(t *testing.T, db *vbolt.DB, token string, req RemoveSeedDataRequest) (RemoveSeedDataResponse, error) {
	t.Helper()
	var resp RemoveSeedDataResponse
	var err error
	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		resp, err = RemoveSeedData(&vbeam.Context{Tx: tx, Token: token}, req)
	})
	return resp, err
}

func userIdFor(t *testing.T, db *vbolt.DB, email string) (id int) {
	t.Helper()
	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		id = GetUserId(tx, email)
	})
	return
}

func TestCreateSeedDataWritesAccountsAndAReceipt(t *testing.T) {
	db := logTestDB(t, "test_admin_seed_create.db")
	token := adminContext(t, db)

	resp, err := createSeedData(t, db, token, CreateSeedDataRequest{
		Password:    testSeedPassword,
		EmailDomain: "review.test",
	})
	if err != nil {
		t.Fatalf("CreateSeedData() error = %v", err)
	}
	if len(resp.Accounts) != len(seedLocalParts) {
		t.Errorf("Accounts = %d, want %d", len(resp.Accounts), len(seedLocalParts))
	}
	if resp.Run.Id == 0 || resp.Run.Domain != "review.test" {
		t.Errorf("Run = %+v, want a recorded run on review.test", resp.Run)
	}
	if resp.People == 0 || resp.Measurements == 0 {
		t.Errorf("People = %d, Measurements = %d, want a populated dataset", resp.People, resp.Measurements)
	}

	if userIdFor(t, db, "admin@example.com") != AdminUserId {
		t.Error("Seeding moved the existing administrator's login")
	}

	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		dadId := GetUserId(tx, "dad@review.test")
		if dadId == 0 {
			t.Fatal("dad@review.test was not created")
		}
		if err := bcrypt.CompareHashAndPassword(GetPassHash(tx, dadId), []byte(testSeedPassword)); err != nil {
			t.Errorf("Seeded account does not accept the supplied password: %v", err)
		}
		if runs := ListSeedRunsTx(tx); len(runs) != 1 || runs[0].Id != resp.Run.Id {
			t.Errorf("ListSeedRunsTx() = %+v, want the one run just created", runs)
		}
	})

	for _, account := range resp.Accounts {
		if account.Access == "admin (site admin, user 1)" {
			t.Errorf("%s is labelled site admin, but user 1 already existed", account.Email)
		}
	}
}

func TestCreateSeedDataRefusesADomainAlreadyInUse(t *testing.T) {
	db := logTestDB(t, "test_admin_seed_conflict.db")
	token := adminContext(t, db)

	if _, err := createSeedData(t, db, token, CreateSeedDataRequest{Password: testSeedPassword, EmailDomain: "review.test"}); err != nil {
		t.Fatalf("first CreateSeedData() error = %v", err)
	}
	firstDadId := userIdFor(t, db, "dad@review.test")

	_, err := createSeedData(t, db, token, CreateSeedDataRequest{Password: testSeedPassword, EmailDomain: "review.test"})
	if !errors.Is(err, ErrSeedEmailsExist) {
		t.Fatalf("second CreateSeedData() error = %v, want ErrSeedEmailsExist", err)
	}

	if got := userIdFor(t, db, "dad@review.test"); got != firstDadId {
		t.Errorf("dad@review.test now resolves to user %d, was %d — the refused run took the login", got, firstDadId)
	}
	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		if runs := ListSeedRunsTx(tx); len(runs) != 1 {
			t.Errorf("ListSeedRunsTx() = %d runs, want 1 — the refused run wrote a receipt", len(runs))
		}
	})

	if _, err := createSeedData(t, db, token, CreateSeedDataRequest{Password: testSeedPassword, EmailDomain: "second.test"}); err != nil {
		t.Errorf("CreateSeedData() on a free domain error = %v", err)
	}
}

func TestCreateSeedDataGuards(t *testing.T) {
	db := logTestDB(t, "test_admin_seed_guards.db")
	token := adminContext(t, db)

	t.Run("non-admin is refused", func(t *testing.T) {
		regular, _ := generateAuthJwt(User{Id: 2, Email: "regular@example.com"}, httptest.NewRecorder())
		if _, err := createSeedData(t, db, regular, CreateSeedDataRequest{Password: testSeedPassword}); err != ErrAdminRequired {
			t.Errorf("Expected ErrAdminRequired, got %v", err)
		}
	})

	t.Run("a short password is refused", func(t *testing.T) {
		if _, err := createSeedData(t, db, token, CreateSeedDataRequest{Password: "short"}); err != ErrSeedPasswordRequired {
			t.Errorf("Expected ErrSeedPasswordRequired, got %v", err)
		}
	})

	t.Run("a nonsense domain is refused", func(t *testing.T) {
		if _, err := createSeedData(t, db, token, CreateSeedDataRequest{Password: testSeedPassword, EmailDomain: "not a domain"}); err != ErrSeedDomainInvalid {
			t.Errorf("Expected ErrSeedDomainInvalid, got %v", err)
		}
	})

	t.Run("nothing was written", func(t *testing.T) {
		vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
			if runs := ListSeedRunsTx(tx); len(runs) != 0 {
				t.Errorf("ListSeedRunsTx() = %d runs, want 0", len(runs))
			}
		})
	})
}

func TestRemoveSeedDataTouchesOnlyItsOwnRun(t *testing.T) {
	db := logTestDB(t, "test_admin_seed_remove.db")
	token := adminContext(t, db)

	var bystanderId int
	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		hash, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)
		bystander := AddUserTx(tx, CreateAccountRequest{Name: "Real User", Email: "real@example.com"}, hash)
		bystanderId = bystander.Id
		vbolt.TxCommit(tx)
	})

	kept, err := createSeedData(t, db, token, CreateSeedDataRequest{Password: testSeedPassword, EmailDomain: "keep.test"})
	if err != nil {
		t.Fatalf("CreateSeedData(keep.test) error = %v", err)
	}
	doomed, err := createSeedData(t, db, token, CreateSeedDataRequest{Password: testSeedPassword, EmailDomain: "gone.test"})
	if err != nil {
		t.Fatalf("CreateSeedData(gone.test) error = %v", err)
	}

	t.Run("the domain has to be retyped", func(t *testing.T) {
		if _, err := removeSeedData(t, db, token, RemoveSeedDataRequest{RunId: doomed.Run.Id, ConfirmValue: "keep.test"}); err != ErrSeedConfirmationMismatch {
			t.Errorf("Expected ErrSeedConfirmationMismatch, got %v", err)
		}
		if userIdFor(t, db, "dad@gone.test") == 0 {
			t.Error("The refused removal deleted accounts anyway")
		}
	})

	t.Run("an unknown run is refused", func(t *testing.T) {
		if _, err := removeSeedData(t, db, token, RemoveSeedDataRequest{RunId: 9999, ConfirmValue: "gone.test"}); err != ErrSeedRunNotFound {
			t.Errorf("Expected ErrSeedRunNotFound, got %v", err)
		}
	})

	resp, err := removeSeedData(t, db, token, RemoveSeedDataRequest{RunId: doomed.Run.Id, ConfirmValue: "gone.test"})
	if err != nil {
		t.Fatalf("RemoveSeedData() error = %v", err)
	}
	if len(resp.RemovedEmails) != len(seedLocalParts) {
		t.Errorf("RemovedEmails = %d, want %d", len(resp.RemovedEmails), len(seedLocalParts))
	}
	if len(resp.SkippedEmails) != 0 {
		t.Errorf("SkippedEmails = %v, want none", resp.SkippedEmails)
	}

	for _, email := range doomed.Run.Emails {
		if userIdFor(t, db, email) != 0 {
			t.Errorf("%s survived the removal", email)
		}
	}
	for _, email := range kept.Run.Emails {
		if userIdFor(t, db, email) == 0 {
			t.Errorf("%s was removed with the other run", email)
		}
	}
	if userIdFor(t, db, "real@example.com") != bystanderId {
		t.Error("The removal took the unrelated account with it")
	}
	if userIdFor(t, db, "admin@example.com") != AdminUserId {
		t.Error("The removal took the administrator with it")
	}

	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		if runs := ListSeedRunsTx(tx); len(runs) != 1 || runs[0].Id != kept.Run.Id {
			t.Errorf("ListSeedRunsTx() = %+v, want only the kept run", runs)
		}
	})
}

func TestRemoveSeedRunSkipsRecordsThatNoLongerMatch(t *testing.T) {
	db := logTestDB(t, "test_admin_seed_skip.db")
	token := adminContext(t, db)

	created, err := createSeedData(t, db, token, CreateSeedDataRequest{Password: testSeedPassword, EmailDomain: "drifted.test"})
	if err != nil {
		t.Fatalf("CreateSeedData() error = %v", err)
	}

	// A reviewer changes their address after the run, so the receipt no longer
	// describes the account and it is not ours to delete.
	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		userId := GetUserId(tx, "mom@drifted.test")
		user := GetUser(tx, userId)
		vbolt.Delete(tx, EmailBkt, user.Email)
		user.Email = "someone@real.test"
		vbolt.Write(tx, UsersBkt, user.Id, &user)
		vbolt.Write(tx, EmailBkt, user.Email, &user.Id)
		vbolt.TxCommit(tx)
	})

	resp, err := removeSeedData(t, db, token, RemoveSeedDataRequest{RunId: created.Run.Id, ConfirmValue: "drifted.test"})
	if err != nil {
		t.Fatalf("RemoveSeedData() error = %v", err)
	}
	if len(resp.SkippedEmails) != 1 || resp.SkippedEmails[0] != "someone@real.test" {
		t.Errorf("SkippedEmails = %v, want [someone@real.test]", resp.SkippedEmails)
	}
	if userIdFor(t, db, "someone@real.test") == 0 {
		t.Error("The renamed account was deleted despite not matching the receipt")
	}
	if resp.SurvivingFamilies == 0 {
		t.Error("SurvivingFamilies = 0, want the family the renamed account still belongs to")
	}
}
