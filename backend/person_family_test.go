package backend

import (
	"family/cfg"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"go.hasen.dev/vbeam"
	"go.hasen.dev/vbolt"
	"golang.org/x/crypto/bcrypt"
)

type personFamilyFixture struct {
	db     *vbolt.DB
	both   User
	soloB  User
	famA   int
	famB   int
	shared Person
}

func setupPersonFamilyFixture(t *testing.T) (personFamilyFixture, func()) {
	t.Helper()

	testDBPath := "test_person_family.db"
	db := vbolt.Open(testDBPath)
	vbolt.InitBuckets(db, &cfg.Info)
	previousDb := appDb
	appDb = db
	cleanup := func() {
		appDb = previousDb
		db.Close()
		os.Remove(testDBPath)
	}

	fx := personFamilyFixture{db: db}

	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		hash, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)

		fx.both = AddUserTx(tx, CreateAccountRequest{Name: "Both", Email: "pf-both@example.com"}, hash)
		fx.soloB = AddUserTx(tx, CreateAccountRequest{Name: "Grandma", Email: "pf-solo@example.com"}, hash)

		fx.famA = fx.both.FamilyId
		fx.famB = fx.soloB.FamilyId
		EnsureMembershipTx(tx, fx.both.Id, fx.famB, AccessAdmin)

		var err error
		fx.shared, err = AddPersonTx(tx, AddPersonRequest{
			Name: "Dana", Gender: 1, Birthdate: "1990-04-20",
		}, fx.famA)
		if err != nil {
			t.Fatalf("AddPersonTx: %v", err)
		}
		EnsurePersonFamilyTx(tx, fx.shared.Id, fx.famB)

		vbolt.TxCommit(tx)
	})

	return fx, cleanup
}

func countPeople(t *testing.T, db *vbolt.DB) (count int) {
	t.Helper()
	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		vbolt.IterateAll(tx, PeopleBkt, func(key int, person Person) bool {
			count++
			return true
		})
	})
	return
}

func TestPersonAppearsOnTwoRostersWithoutDuplicating(t *testing.T) {
	fx, cleanup := setupPersonFamilyFixture(t)
	defer cleanup()

	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		rosterA := GetFamilyPeople(tx, fx.famA)
		if len(rosterA) != 1 {
			t.Fatalf("expected 1 person on family A's roster, got %d", len(rosterA))
		}
		rosterB := GetFamilyPeople(tx, fx.famB)
		if len(rosterB) != 1 {
			t.Fatalf("expected 1 person on family B's roster, got %d", len(rosterB))
		}

		if rosterA[0].Id != fx.shared.Id || rosterB[0].Id != fx.shared.Id {
			t.Fatalf("rosters hold different people: A=%d B=%d want %d",
				rosterA[0].Id, rosterB[0].Id, fx.shared.Id)
		}
		if rosterB[0].FamilyId != fx.famA {
			t.Errorf("extended roster changed the home family to %d, want %d",
				rosterB[0].FamilyId, fx.famA)
		}
	})
}

func TestSharingCreatesNoDuplicatePerson(t *testing.T) {
	fx, cleanup := setupPersonFamilyFixture(t)
	defer cleanup()

	if got := countPeople(t, fx.db); got != 1 {
		t.Fatalf("expected 1 person record after sharing, got %d", got)
	}

	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		EnsurePersonFamilyTx(tx, fx.shared.Id, fx.famB)
		vbolt.TxCommit(tx)
	})

	if got := countPeople(t, fx.db); got != 1 {
		t.Fatalf("re-sharing created a person record, count = %d", got)
	}
	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		if rows := GetPersonFamilies(tx, fx.shared.Id); len(rows) != 2 {
			t.Errorf("expected 2 roster rows, got %d", len(rows))
		}
	})
}

func TestVisiblePeopleDeduplicatesSharedPerson(t *testing.T) {
	fx, cleanup := setupPersonFamilyFixture(t)
	defer cleanup()

	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		people := GetVisiblePeople(tx, fx.both)
		if len(people) != 1 {
			t.Fatalf("shared person listed %d times, want 1", len(people))
		}

		soloPeople := GetVisiblePeople(tx, fx.soloB)
		if len(soloPeople) != 1 {
			t.Fatalf("family B member saw %d people, want 1", len(soloPeople))
		}
		if soloPeople[0].Id != fx.shared.Id {
			t.Errorf("family B saw person %d, want %d", soloPeople[0].Id, fx.shared.Id)
		}
	})
}

func TestRosterDoesNotLeakToUnrelatedFamily(t *testing.T) {
	fx, cleanup := setupPersonFamilyFixture(t)
	defer cleanup()

	var outsider User
	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		hash, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)
		outsider = AddUserTx(tx, CreateAccountRequest{Name: "Out", Email: "pf-out@example.com"}, hash)
		vbolt.TxCommit(tx)
	})

	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		if people := GetVisiblePeople(tx, outsider); len(people) != 0 {
			t.Errorf("unrelated family saw %d people, want 0", len(people))
		}
	})
}

func TestHomeRosterCannotBeRemoved(t *testing.T) {
	fx, cleanup := setupPersonFamilyFixture(t)
	defer cleanup()

	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		if err := RemovePersonFromFamilyTx(tx, fx.shared.Id, fx.famA); err != ErrCannotRemoveHomeRoster {
			t.Errorf("removing home roster returned %v, want ErrCannotRemoveHomeRoster", err)
		}
		if err := RemovePersonFromFamilyTx(tx, fx.shared.Id, fx.famB); err != nil {
			t.Errorf("removing extended roster: %v", err)
		}
		vbolt.TxCommit(tx)
	})

	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		if people := GetFamilyPeople(tx, fx.famB); len(people) != 0 {
			t.Errorf("family B roster still holds %d people", len(people))
		}
		if people := GetFamilyPeople(tx, fx.famA); len(people) != 1 {
			t.Errorf("family A roster holds %d people, want 1", len(people))
		}
		if countPeople(t, fx.db) != 1 {
			t.Error("unsharing deleted the person record")
		}
	})
}

func TestSetRelationshipIsScopedToOneRoster(t *testing.T) {
	fx, cleanup := setupPersonFamilyFixture(t)
	defer cleanup()

	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		SetPersonFamilyRelationshipTx(tx, fx.shared.Id, fx.famB, "Daughter")
		vbolt.TxCommit(tx)
	})

	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		rowA, _ := FindPersonFamily(tx, fx.shared.Id, fx.famA)
		if rowA.Relationship != "" {
			t.Errorf("home relationship changed to %q", rowA.Relationship)
		}
		rowB, _ := FindPersonFamily(tx, fx.shared.Id, fx.famB)
		if rowB.Relationship != "Daughter" {
			t.Errorf("extended relationship = %q, want %q", rowB.Relationship, "Daughter")
		}
	})

	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		EnsurePersonFamilyTx(tx, fx.shared.Id, fx.famB)
		vbolt.TxCommit(tx)
	})
	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		row, _ := FindPersonFamily(tx, fx.shared.Id, fx.famB)
		if row.Relationship != "Daughter" {
			t.Errorf("EnsurePersonFamilyTx overwrote an existing relationship with %q", row.Relationship)
		}
	})
}

func TestUpdatePersonNameTracksHomeRoster(t *testing.T) {
	fx, cleanup := setupPersonFamilyFixture(t)
	defer cleanup()

	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		person := GetPersonById(tx, fx.shared.Id)
		person.Name = "Dana Renamed"
		vbolt.Write(tx, PeopleBkt, person.Id, &person)
		vbolt.TxCommit(tx)
	})

	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		rosterA := GetFamilyPeople(tx, fx.famA)
		if len(rosterA) != 1 || rosterA[0].Name != "Dana Renamed" {
			t.Errorf("home roster did not follow the edit: %+v", rosterA)
		}
		rosterB := GetFamilyPeople(tx, fx.famB)
		if len(rosterB) != 1 || rosterB[0].Name != "Dana Renamed" {
			t.Errorf("extended roster did not follow the edit: %+v", rosterB)
		}
	})
}

func TestBackfillPersonFamiliesIsIdempotent(t *testing.T) {
	testDBPath := "test_person_family_backfill.db"
	db := vbolt.Open(testDBPath)
	vbolt.InitBuckets(db, &cfg.Info)
	defer func() {
		db.Close()
		os.Remove(testDBPath)
	}()

	var legacyIds []int
	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		for i, spec := range []struct {
			name     string
			familyId int
		}{
			{"Legacy One", 7},
			{"Legacy Two", 7},
			{"Other Family", 9},
		} {
			person := Person{
				Id:       vbolt.NextIntId(tx, PeopleBkt),
				FamilyId: spec.familyId,
				Name:     spec.name,
				Birthday: time.Date(2015+i, 1, 1, 0, 0, 0, 0, time.UTC),
			}
			vbolt.Write(tx, PeopleBkt, person.Id, &person)
			vbolt.SetTargetSingleTerm(tx, PersonIndex, person.Id, person.FamilyId)
			legacyIds = append(legacyIds, person.Id)
		}

		orphan := Person{Id: vbolt.NextIntId(tx, PeopleBkt), Name: "No Family"}
		vbolt.Write(tx, PeopleBkt, orphan.Id, &orphan)

		vbolt.TxCommit(tx)
	})

	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		if people := GetFamilyPeople(tx, 7); len(people) != 0 {
			t.Fatalf("roster was non-empty before backfill: %d", len(people))
		}
	})

	var first, second int
	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		first = BackfillPersonFamilies(tx)
		vbolt.TxCommit(tx)
	})
	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		second = BackfillPersonFamilies(tx)
		vbolt.TxCommit(tx)
	})

	if first != 3 {
		t.Errorf("first backfill created %d rows, want 3", first)
	}
	if second != 0 {
		t.Errorf("re-running the backfill created %d rows, want 0", second)
	}

	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		roster := GetFamilyPeople(tx, 7)
		if len(roster) != 2 {
			t.Fatalf("family 7 roster = %d people, want 2", len(roster))
		}
		names := map[string]bool{}
		for _, person := range roster {
			names[person.Name] = true
		}
		if !names["Legacy One"] || !names["Legacy Two"] {
			t.Errorf("backfill did not place both people on the roster: %v", names)
		}
		if len(GetFamilyPeople(tx, 9)) != 1 {
			t.Error("family 9 roster is wrong")
		}
		if len(GetFamilyRoster(tx, 0)) != 0 {
			t.Error("backfill placed a person on the family-0 sentinel roster")
		}

		for _, personId := range legacyIds {
			person := GetPersonById(tx, personId)
			rows := GetPersonFamilies(tx, personId)
			if len(rows) != 1 {
				t.Errorf("person %d has %d roster rows, want 1", personId, len(rows))
				continue
			}
			if rows[0].FamilyId != person.FamilyId {
				t.Errorf("person %d roster family = %d, want home family %d",
					personId, rows[0].FamilyId, person.FamilyId)
			}
		}
	})
}

func TestMergeRefusesDifferentHomeFamilies(t *testing.T) {
	fx, cleanup := setupPersonFamilyFixture(t)
	defer cleanup()

	var inB Person
	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		var err error
		inB, err = AddPersonTx(tx, AddPersonRequest{
			Name: "Dana", Gender: 1, Birthdate: "1990-04-20",
		}, fx.famB)
		if err != nil {
			t.Fatalf("AddPersonTx famB: %v", err)
		}
		vbolt.TxCommit(tx)
	})

	err := callMergePeople(t, fx.db, fx.both, MergePeopleRequest{
		SourcePersonId: inB.Id, TargetPersonId: fx.shared.Id,
	})
	if err == nil {
		t.Fatal("cross-family merge was allowed")
	}
	if err.Error() != "Cannot merge people from different families" {
		t.Errorf("unexpected error: %v", err)
	}

	if got := countPeople(t, fx.db); got != 2 {
		t.Errorf("refused merge still changed the person count: %d, want 2", got)
	}
}

func TestMergeUnionsRosters(t *testing.T) {
	fx, cleanup := setupPersonFamilyFixture(t)
	defer cleanup()

	var duplicate Person
	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		if err := RemovePersonFromFamilyTx(tx, fx.shared.Id, fx.famB); err != nil {
			t.Fatalf("unshare target: %v", err)
		}
		var err error
		duplicate, err = AddPersonTx(tx, AddPersonRequest{
			Name: "Dana (dup)", Gender: 1, Birthdate: "1990-04-20",
		}, fx.famA)
		if err != nil {
			t.Fatalf("AddPersonTx duplicate: %v", err)
		}
		EnsurePersonFamilyTx(tx, duplicate.Id, fx.famB)
		vbolt.TxCommit(tx)
	})

	if err := callMergePeople(t, fx.db, fx.both, MergePeopleRequest{
		SourcePersonId: duplicate.Id, TargetPersonId: fx.shared.Id,
	}); err != nil {
		t.Fatalf("same-family merge: %v", err)
	}

	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		if rows := GetPersonFamilies(tx, duplicate.Id); len(rows) != 0 {
			t.Errorf("merged-away person kept %d roster rows", len(rows))
		}
		rosterB := GetFamilyPeople(tx, fx.famB)
		if len(rosterB) != 1 {
			t.Fatalf("family B roster = %d people, want the survivor", len(rosterB))
		}
		if rosterB[0].Id != fx.shared.Id {
			t.Errorf("family B roster holds person %d, want the survivor %d",
				rosterB[0].Id, fx.shared.Id)
		}
	})
}

func callMergePeople(t *testing.T, db *vbolt.DB, user User, req MergePeopleRequest) (err error) {
	t.Helper()
	token, tokenErr := generateAuthJwt(user, httptest.NewRecorder())
	if tokenErr != nil {
		t.Fatalf("generateAuthJwt: %v", tokenErr)
	}
	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		ctx := &vbeam.Context{Tx: tx, Token: token}
		_, err = MergePeople(ctx, req)
	})
	return
}

func TestEveryPersonHasHomeRoster(t *testing.T) {
	fx, cleanup := setupPersonFamilyFixture(t)
	defer cleanup()

	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		vbolt.IterateAll(tx, PeopleBkt, func(key int, person Person) bool {
			if person.FamilyId == 0 {
				return true
			}
			if _, found := FindPersonFamily(tx, person.Id, person.FamilyId); !found {
				t.Errorf("person %d (%s) has no home roster row", person.Id, person.Name)
			}
			return true
		})
	})
}
