//go:build !release

package backend

import (
	"testing"
	"time"

	"go.hasen.dev/vbolt"
	"golang.org/x/crypto/bcrypt"
)

// The seeded dataset makes promises about who can see what — that is most of
// the point of it. These tests hold it to them, so the demo data cannot quietly
// stop demonstrating the access rules it was built to demonstrate.

func seedForTest(t *testing.T) (*vbolt.DB, SeedSummary, func()) {
	t.Helper()
	app, cleanup := setupTestApp(t)

	var summary SeedSummary
	var seedErr error
	vbolt.WithWriteTx(app.DB, func(tx *vbolt.Tx) {
		summary, seedErr = SeedDemoData(tx, SeedOptions{Now: time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC)})
		if seedErr != nil {
			return
		}
		vbolt.TxCommit(tx)
	})
	if seedErr != nil {
		cleanup()
		t.Fatalf("SeedDemoData: %v", seedErr)
	}
	return app.DB, summary, cleanup
}

func seedUser(t *testing.T, tx *vbolt.Tx, email string) User {
	t.Helper()
	userId := GetUserId(tx, email)
	if userId == 0 {
		t.Fatalf("no seeded account for %s", email)
	}
	return GetUser(tx, userId)
}

func seedPerson(t *testing.T, tx *vbolt.Tx, familyId int, name string) Person {
	t.Helper()
	for _, person := range GetFamilyOwnPeople(tx, familyId) {
		if person.Name == name {
			return person
		}
	}
	t.Fatalf("no person named %s in family %d", name, familyId)
	return Person{}
}

func TestSeedProducesData(t *testing.T) {
	_, summary, cleanup := seedForTest(t)
	defer cleanup()

	checks := []struct {
		label string
		count int
		least int
	}{
		{"accounts", len(summary.Accounts), 8},
		{"families", summary.Families, 7},
		{"people", summary.People, 12},
		{"relations", summary.Relations, 12},
		{"links", summary.Links, 5},
		{"shares", summary.Shares, 10},
		{"tags", summary.Tags, 6},
		{"milestones", summary.Milestones, 100},
		{"measurements", summary.Measurements, 300},
		{"activities", summary.Activities, 3},
		{"results", summary.Results, 10},
		{"chat messages", summary.ChatMessages, 10},
	}
	for _, check := range checks {
		if check.count < check.least {
			t.Errorf("seeded %d %s, want at least %d", check.count, check.label, check.least)
		}
	}
}

// The first account has to be user 1: backend/admin.go recognises no other
// administrator, so a reordering here would lock the admin pages away.
func TestSeedFirstAccountIsSiteAdmin(t *testing.T) {
	db, _, cleanup := seedForTest(t)
	defer cleanup()

	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		admin := seedUser(t, tx, "dad@example.test")
		if admin.Id != AdminUserId {
			t.Errorf("dad@example.test is user %d, want %d", admin.Id, AdminUserId)
		}
	})
}

func TestSeedIssuesSubAdminMemberships(t *testing.T) {
	db, _, cleanup := seedForTest(t)
	defer cleanup()

	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		dad := seedUser(t, tx, "dad@example.test")
		for _, want := range []struct {
			email string
			role  AccessLevel
		}{
			{"mom@example.test", AccessAdmin},
			{"nanny@example.test", AccessContribute},
			{"sitter@example.test", AccessView},
		} {
			user := seedUser(t, tx, want.email)
			membership, found := FindMembership(tx, user.Id, dad.FamilyId)
			if !found {
				t.Errorf("%s has no membership in the Rivera family", want.email)
				continue
			}
			if membership.Role != want.role {
				t.Errorf("%s has role %d in the Rivera family, want %d", want.email, membership.Role, want.role)
			}
		}

		// A guest's reduced role only bites because the Rivera family is not
		// their primary one; CanAccessFamily grants admin on a user's own
		// household whatever the membership row says.
		sitter := seedUser(t, tx, "sitter@example.test")
		if sitter.FamilyId == dad.FamilyId {
			t.Fatal("the read-only guest's primary family is the Rivera family, which would grant them admin")
		}
		if !CanAccessFamily(tx, sitter, dad.FamilyId, AccessView) {
			t.Error("the read-only guest cannot view the Rivera family")
		}
		if CanAccessFamily(tx, sitter, dad.FamilyId, AccessContribute) {
			t.Error("the read-only guest can contribute to the Rivera family")
		}

		nanny := seedUser(t, tx, "nanny@example.test")
		if !CanAccessFamily(tx, nanny, dad.FamilyId, AccessContribute) {
			t.Error("the contributing guest cannot contribute to the Rivera family")
		}
		if CanAccessFamily(tx, nanny, dad.FamilyId, AccessAdmin) {
			t.Error("the contributing guest has admin on the Rivera family")
		}
	})
}

func TestSeedLinkScopesDifferByGrandparent(t *testing.T) {
	db, _, cleanup := seedForTest(t)
	defer cleanup()

	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		dad := seedUser(t, tx, "dad@example.test")
		grandpa := seedUser(t, tx, "grandpa@example.test")
		nana := seedUser(t, tx, "nana@example.test")

		sofia := seedPerson(t, tx, dad.FamilyId, "Sofia Rivera")
		luca := seedPerson(t, tx, dad.FamilyId, "Luca Rivera")

		// The paternal link carries every scope.
		for _, scope := range []LinkScope{ScopePeople, ScopeMilestones, ScopeGrowth, ScopeActivities, ScopePhotos} {
			if !CanAccessPerson(tx, grandpa, sofia, scope, AccessView) {
				t.Errorf("grandpa cannot view Sofia through scope %d", scope)
			}
		}
		if !CanAccessPerson(tx, grandpa, luca, ScopeGrowth, AccessView) {
			t.Error("grandpa cannot view Luca's growth")
		}

		// The maternal link carries people, milestones, and photos only.
		if !CanAccessPerson(tx, nana, sofia, ScopeMilestones, AccessView) {
			t.Error("nana cannot see Sofia's milestones")
		}
		if CanAccessPerson(tx, nana, sofia, ScopeGrowth, AccessView) {
			t.Error("nana can see Sofia's growth, but the link carries no growth scope")
		}
		if CanAccessPerson(tx, nana, sofia, ScopeActivities, AccessView) {
			t.Error("nana can see Sofia's activities, but the link carries no activities scope")
		}

		// Luca was never shared onto the Chandra roster.
		if CanAccessPerson(tx, nana, luca, ScopePeople, AccessView) {
			t.Error("nana can see Luca, who was never shared with her")
		}

		// A link is view-only and never reaches the household itself.
		if CanAccessPerson(tx, grandpa, sofia, ScopeGrowth, AccessContribute) {
			t.Error("a link granted write access")
		}
		if CanAccessFamily(tx, grandpa, dad.FamilyId, AccessView) {
			t.Error("a link granted whole-family access")
		}
	})
}

func TestSeedPendingLinkAndOutsiderSeeNothing(t *testing.T) {
	db, _, cleanup := seedForTest(t)
	defer cleanup()

	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		dad := seedUser(t, tx, "dad@example.test")
		sofia := seedPerson(t, tx, dad.FamilyId, "Sofia Rivera")

		for _, email := range []string{"aunt@example.test", "outsider@example.test"} {
			user := seedUser(t, tx, email)
			if CanAccessPerson(tx, user, sofia, ScopePeople, AccessView) {
				t.Errorf("%s can see Sofia", email)
			}
			if CanAccessFamily(tx, user, dad.FamilyId, AccessView) {
				t.Errorf("%s can see the Rivera family", email)
			}
		}
	})
}

// The grandparent relation edges cross a family boundary on purpose: without
// them RelationLabel has nothing to walk and the extended family goes unnamed.
func TestSeedRelationsSpanFamilies(t *testing.T) {
	db, _, cleanup := seedForTest(t)
	defer cleanup()

	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		dad := seedUser(t, tx, "dad@example.test")
		grandpa := seedUser(t, tx, "grandpa@example.test")
		nana := seedUser(t, tx, "nana@example.test")

		sofia := seedPerson(t, tx, dad.FamilyId, "Sofia Rivera")
		mateo := seedPerson(t, tx, dad.FamilyId, "Mateo Rivera")
		eleanor := seedPerson(t, tx, grandpa.FamilyId, "Eleanor Rivera")
		vikram := seedPerson(t, tx, nana.FamilyId, "Vikram Chandra")

		for _, want := range []struct {
			subject Person
			target  Person
			label   string
		}{
			{sofia, eleanor, "grandmother"},
			{sofia, vikram, "grandfather"},
			{sofia, mateo, "brother"},
			{eleanor, sofia, "granddaughter"},
		} {
			if got := RelationLabel(tx, want.subject, want.target); got != want.label {
				t.Errorf("%s is %q to %s, want %q", want.target.Name, got, want.subject.Name, want.label)
			}
		}
	})
}

func TestSeedUsersPointAtTheirPersonRecord(t *testing.T) {
	db, _, cleanup := seedForTest(t)
	defer cleanup()

	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		for _, email := range []string{
			"dad@example.test", "mom@example.test", "teen@example.test",
			"grandpa@example.test", "grandma@example.test", "nana@example.test",
		} {
			user := seedUser(t, tx, email)
			if user.PersonId == 0 {
				t.Errorf("%s has no person record", email)
				continue
			}
			if person := GetPersonById(tx, user.PersonId); person.Name != user.Name {
				t.Errorf("%s points at person %q", email, person.Name)
			}
		}
	})
}

// GetFamilyPeople is roster-based, so shared people show up in the grandparents'
// list while their own export stays their own.
func TestSeedSharedPeopleAppearOnGrandparentRoster(t *testing.T) {
	db, _, cleanup := seedForTest(t)
	defer cleanup()

	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		grandpa := seedUser(t, tx, "grandpa@example.test")

		roster := GetFamilyPeople(tx, grandpa.FamilyId)
		if len(roster) < 8 {
			t.Errorf("grandparent roster holds %d people, want the two of them plus the Riveras", len(roster))
		}
		own := GetFamilyOwnPeople(tx, grandpa.FamilyId)
		if len(own) != 2 {
			t.Errorf("grandparents own %d people, want 2", len(own))
		}
	})
}

// The point of the seed is credentials you already know, so the password has to
// actually work.
func TestSeedPasswordSignsIn(t *testing.T) {
	db, summary, cleanup := seedForTest(t)
	defer cleanup()

	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		for _, account := range summary.Accounts {
			user := seedUser(t, tx, account.Email)
			hash := GetPassHash(tx, user.Id)
			if err := bcrypt.CompareHashAndPassword(hash, []byte(SeedPassword)); err != nil {
				t.Errorf("%s does not accept SeedPassword: %v", account.Email, err)
			}
			if !user.EmailVerified {
				t.Errorf("%s is not email-verified", account.Email)
			}
		}
	})
}
