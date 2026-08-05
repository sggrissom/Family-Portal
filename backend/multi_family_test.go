// Tests for Stage 3 of the multi-family plan: a user belongs to more than one
// family. The three things that must hold are that a two-family user reaches
// both families, that a one-family user sees exactly what they saw before, and
// that a non-member is still refused everywhere.
package backend

import (
	"family/cfg"
	"os"
	"testing"
	"time"

	"go.hasen.dev/vbolt"
	"golang.org/x/crypto/bcrypt"
)

// multiFamilyFixture is three families. The `both` user is a member of A and B
// with A as their primary; `soloA` belongs only to A; `outsider` belongs only
// to C and must never reach anything in A or B.
type multiFamilyFixture struct {
	db       *vbolt.DB
	both     User
	soloA    User
	outsider User
	famA     int
	famB     int
	famC     int
	personA  Person
	personB  Person
	tagB     Tag
	imageB   Image
}

func setupMultiFamilyFixture(t *testing.T) (multiFamilyFixture, func()) {
	t.Helper()

	testDBPath := "test_multi_family.db"
	db := vbolt.Open(testDBPath)
	vbolt.InitBuckets(db, &cfg.Info)
	cleanup := func() {
		db.Close()
		os.Remove(testDBPath)
	}

	fx := multiFamilyFixture{db: db}

	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		hash, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)

		fx.both = AddUserTx(tx, CreateAccountRequest{Name: "Both", Email: "both@example.com"}, hash)
		fx.soloA = AddUserTx(tx, CreateAccountRequest{Name: "Solo", Email: "solo@example.com"}, hash)
		fx.outsider = AddUserTx(tx, CreateAccountRequest{Name: "Out", Email: "out@example.com"}, hash)

		fx.famA = fx.both.FamilyId
		fx.famC = fx.outsider.FamilyId

		// soloA joined A through an invite code, so A has two members.
		soloOwnFamily := fx.soloA.FamilyId
		fx.soloA.FamilyId = fx.famA
		vbolt.Write(tx, UsersBkt, fx.soloA.Id, &fx.soloA)
		vbolt.SetTargetSingleTerm(tx, UsersByFamilyIndex, fx.soloA.Id, fx.famA)
		for _, membership := range GetUserMemberships(tx, fx.soloA.Id) {
			if membership.FamilyId == soloOwnFamily {
				deleteMembershipTx(tx, membership)
			}
		}
		EnsureMembershipTx(tx, fx.soloA.Id, fx.famA, AccessAdmin)

		// Family B exists on its own, and `both` joins it as a second family —
		// the additive JoinFamily path, which leaves the primary alone.
		famB := createFamilyTx(tx, "Grandparents", fx.both.Id)
		fx.famB = famB.Id
		EnsureMembershipTx(tx, fx.both.Id, fx.famB, AccessAdmin)

		var err error
		fx.personA, err = AddPersonTx(tx, AddPersonRequest{
			Name: "Kid A", PersonType: 1, Gender: 0, Birthdate: "2020-06-15",
		}, fx.famA)
		if err != nil {
			t.Fatalf("AddPersonTx famA: %v", err)
		}
		fx.personB, err = AddPersonTx(tx, AddPersonRequest{
			Name: "Kid B", PersonType: 1, Gender: 1, Birthdate: "2019-03-02",
		}, fx.famB)
		if err != nil {
			t.Fatalf("AddPersonTx famB: %v", err)
		}

		fx.tagB = Tag{
			Id: vbolt.NextIntId(tx, TagBkt), FamilyId: fx.famB,
			Name: "Visits", Color: "#4A90D9", CreatedAt: time.Now(),
		}
		vbolt.Write(tx, TagBkt, fx.tagB.Id, &fx.tagB)
		vbolt.SetTargetSingleTerm(tx, TagByFamilyIndex, fx.tagB.Id, fx.famB)

		fx.imageB = Image{
			Id: vbolt.NextIntId(tx, ImagesBkt), FamilyId: fx.famB,
			OwnerUserId: fx.both.Id, OriginalFilename: "b.jpg",
			MimeType: "image/jpeg", CreatedAt: time.Now(),
		}
		vbolt.Write(tx, ImagesBkt, fx.imageB.Id, &fx.imageB)
		vbolt.SetTargetSingleTerm(tx, ImageByFamilyIndex, fx.imageB.Id, fx.famB)

		vbolt.TxCommit(tx)
	})

	return fx, cleanup
}

// A user in two families holds access in both, at every level of the ladder.
func TestMemberOfTwoFamiliesReachesBoth(t *testing.T) {
	fx, cleanup := setupMultiFamilyFixture(t)
	defer cleanup()

	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		for _, need := range []AccessLevel{AccessView, AccessContribute, AccessAdmin} {
			if !CanAccessFamily(tx, fx.both, fx.famA, need) {
				t.Errorf("primary family denied at level %d", need)
			}
			if !CanAccessFamily(tx, fx.both, fx.famB, need) {
				t.Errorf("secondary family denied at level %d", need)
			}
			if CanAccessFamily(tx, fx.both, fx.famC, need) {
				t.Errorf("unrelated family allowed at level %d", need)
			}
		}
	})
}

// The resolver must return both families, primary first, and callers built on
// it must merge the two rosters rather than showing only the primary.
func TestVisibleFamiliesAndRostersSpanMemberships(t *testing.T) {
	fx, cleanup := setupMultiFamilyFixture(t)
	defer cleanup()

	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		families := familiesVisibleTo(tx, fx.both)
		if len(families) != 2 {
			t.Fatalf("expected 2 visible families, got %v", families)
		}
		if families[0] != fx.famA {
			t.Errorf("primary family should come first, got %v", families)
		}
		if families[1] != fx.famB {
			t.Errorf("secondary family missing from %v", families)
		}

		people := GetVisiblePeople(tx, fx.both)
		if len(people) != 2 {
			t.Fatalf("expected people from both families, got %d", len(people))
		}
		seen := map[int]bool{}
		for _, person := range people {
			seen[person.FamilyId] = true
		}
		if !seen[fx.famA] || !seen[fx.famB] {
			t.Errorf("roster did not span both families: %v", seen)
		}

		if tags := getVisibleTags(tx, fx.both); len(tags) != 1 || tags[0].Id != fx.tagB.Id {
			t.Errorf("expected the secondary family's tag to be visible, got %v", tags)
		}
		if images := GetVisibleImages(tx, fx.both); len(images) != 1 || images[0].Id != fx.imageB.Id {
			t.Errorf("expected the secondary family's image to be visible, got %v", images)
		}
	})
}

// A single-family user must see exactly what they saw before Stage 3.
func TestSingleFamilyUserIsUnchanged(t *testing.T) {
	fx, cleanup := setupMultiFamilyFixture(t)
	defer cleanup()

	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		families := familiesVisibleTo(tx, fx.soloA)
		if len(families) != 1 || families[0] != fx.famA {
			t.Fatalf("expected only family A, got %v", families)
		}

		people := GetVisiblePeople(tx, fx.soloA)
		if len(people) != 1 || people[0].Id != fx.personA.Id {
			t.Errorf("single-family roster changed: %v", people)
		}

		if CanAccessFamily(tx, fx.soloA, fx.famB, AccessView) {
			t.Error("a single-family user reached family B")
		}
		if getVisibleTags(tx, fx.soloA) == nil {
			t.Error("getVisibleTags should return an empty slice, not nil")
		}
		if len(getVisibleTags(tx, fx.soloA)) != 0 {
			t.Error("family B's tag leaked to a single-family user")
		}
	})
}

// Cross-family denial still holds for every entity type once memberships are
// consulted — a non-member of A and B reaches nothing in either.
func TestNonMemberIsDeniedEveryEntity(t *testing.T) {
	fx, cleanup := setupMultiFamilyFixture(t)
	defer cleanup()

	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		for _, familyId := range []int{fx.famA, fx.famB} {
			if CanAccessFamily(tx, fx.outsider, familyId, AccessView) {
				t.Errorf("outsider reached family %d", familyId)
			}
		}

		if _, err := ActingFamilyForPerson(tx, fx.outsider, fx.personA.Id, AccessView); err == nil {
			t.Error("ActingFamilyForPerson allowed a non-member")
		}
		if _, err := ActingFamilyFor(tx, fx.outsider, fx.imageB.FamilyId, AccessView); err == nil {
			t.Error("ActingFamilyFor allowed a non-member")
		}
		if _, err := ResolveActingFamily(tx, fx.outsider, fx.famB, AccessContribute); err == nil {
			t.Error("ResolveActingFamily allowed a non-member")
		}

		if people := GetVisiblePeople(tx, fx.outsider); len(people) != 0 {
			t.Errorf("outsider saw %d people", len(people))
		}
		if images := GetVisibleImages(tx, fx.outsider); len(images) != 0 {
			t.Errorf("outsider saw %d images", len(images))
		}
		if tags := getVisibleTags(tx, fx.outsider); len(tags) != 0 {
			t.Errorf("outsider saw %d tags", len(tags))
		}
	})
}

// The active family for a mutation is the requested one when named and the
// primary when not, and it is always checked against membership.
func TestResolveActingFamilyDefaultsToPrimary(t *testing.T) {
	fx, cleanup := setupMultiFamilyFixture(t)
	defer cleanup()

	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		familyId, err := ResolveActingFamily(tx, fx.both, 0, AccessContribute)
		if err != nil {
			t.Fatalf("unnamed family should fall back to primary: %v", err)
		}
		if familyId != fx.famA {
			t.Errorf("expected primary family %d, got %d", fx.famA, familyId)
		}

		familyId, err = ResolveActingFamily(tx, fx.both, fx.famB, AccessContribute)
		if err != nil {
			t.Fatalf("naming a family the user belongs to should be allowed: %v", err)
		}
		if familyId != fx.famB {
			t.Errorf("expected named family %d, got %d", fx.famB, familyId)
		}

		if _, err := ResolveActingFamily(tx, fx.both, fx.famC, AccessContribute); err == nil {
			t.Error("naming a family the user does not belong to should be refused")
		}

		familyless := User{Id: 9002, Name: "Orphan"}
		if _, err := ResolveActingFamily(tx, familyless, 0, AccessContribute); err == nil {
			t.Error("a user with no family should have no acting family")
		}
	})
}

// Records created in a secondary family are owned by that family, and the
// records they hang off resolve to it without the request naming it.
func TestWritesLandInTheNamedFamily(t *testing.T) {
	fx, cleanup := setupMultiFamilyFixture(t)
	defer cleanup()

	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		// A person added to the secondary family belongs to it, not to primary.
		familyId, err := ResolveActingFamily(tx, fx.both, fx.famB, AccessContribute)
		if err != nil {
			t.Fatalf("ResolveActingFamily: %v", err)
		}
		person, err := AddPersonTx(tx, AddPersonRequest{
			Name: "Kid B2", PersonType: 1, Gender: 2, Birthdate: "2022-01-10",
		}, familyId)
		if err != nil {
			t.Fatalf("AddPersonTx: %v", err)
		}
		if person.FamilyId != fx.famB {
			t.Errorf("person landed in family %d, expected %d", person.FamilyId, fx.famB)
		}

		// A measurement follows the person's family with nothing named.
		measurementFamily, err := ActingFamilyForPerson(tx, fx.both, fx.personB.Id, AccessContribute)
		if err != nil {
			t.Fatalf("ActingFamilyForPerson: %v", err)
		}
		if measurementFamily != fx.famB {
			t.Errorf("measurement context was %d, expected %d", measurementFamily, fx.famB)
		}
		growth, err := AddGrowthDataTx(tx, AddGrowthDataRequest{
			PersonId:        fx.personB.Id,
			MeasurementType: "height",
			Value:           102,
			Unit:            "cm",
			InputType:       "date",
			MeasurementDate: stringPtr("2024-02-01"),
		}, measurementFamily)
		if err != nil {
			t.Fatalf("AddGrowthDataTx: %v", err)
		}
		if growth.FamilyId != fx.famB {
			t.Errorf("measurement landed in family %d, expected %d", growth.FamilyId, fx.famB)
		}

		// And it is readable back by the member, but not by the outsider.
		if _, err := GetGrowthDataForUser(tx, growth.Id, fx.both, AccessView); err != nil {
			t.Errorf("member cannot read back their own secondary-family record: %v", err)
		}
		if _, err := GetGrowthDataForUser(tx, growth.Id, fx.outsider, AccessView); err == nil {
			t.Error("outsider read a secondary-family record")
		}
		if _, err := GetGrowthDataForUser(tx, growth.Id, fx.soloA, AccessView); err == nil {
			t.Error("a family A member read a family B record")
		}
	})
}

// Notifications and the auth payload both have to know about the second family.
func TestFamilyFanoutFollowsMembership(t *testing.T) {
	fx, cleanup := setupMultiFamilyFixture(t)
	defer cleanup()

	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		// UsersByFamilyIndex only tracks primary families, so family B knows
		// about its secondary member only through FamilyMembership.
		userIds := GetFamilyUserIds(tx, fx.famB)
		if len(userIds) != 1 || userIds[0] != fx.both.Id {
			t.Errorf("family B should notify user %d, got %v", fx.both.Id, userIds)
		}

		auth := GetAuthResponseFromUser(tx, fx.both)
		if auth.FamilyId != fx.famA {
			t.Errorf("auth primary family was %d, expected %d", auth.FamilyId, fx.famA)
		}
		if len(auth.Families) != 2 {
			t.Fatalf("auth should list both families, got %v", auth.Families)
		}
		if !auth.Families[0].IsPrimary || auth.Families[0].Id != fx.famA {
			t.Errorf("primary family should come first, got %v", auth.Families[0])
		}
		if auth.Families[1].Id != fx.famB || auth.Families[1].IsPrimary {
			t.Errorf("second entry should be the non-primary family B, got %v", auth.Families[1])
		}

		soloAuth := GetAuthResponseFromUser(tx, fx.soloA)
		if len(soloAuth.Families) != 1 || soloAuth.Families[0].Id != fx.famA {
			t.Errorf("single-family auth changed: %v", soloAuth.Families)
		}
	})
}

// A membership role caps what it grants, so an AccessView member can read but
// not write.
func TestMembershipRoleCapsAccess(t *testing.T) {
	fx, cleanup := setupMultiFamilyFixture(t)
	defer cleanup()

	var viewer User
	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		hash, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)
		viewer = AddUserTx(tx, CreateAccountRequest{Name: "Viewer", Email: "viewer@example.com"}, hash)
		EnsureMembershipTx(tx, viewer.Id, fx.famB, AccessView)
		vbolt.TxCommit(tx)
	})

	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		if !CanAccessFamily(tx, viewer, fx.famB, AccessView) {
			t.Error("an AccessView member should be able to read")
		}
		if CanAccessFamily(tx, viewer, fx.famB, AccessContribute) {
			t.Error("an AccessView member should not be able to write")
		}
		if CanAccessFamily(tx, viewer, fx.famB, AccessAdmin) {
			t.Error("an AccessView member should not hold admin")
		}
		if _, err := ResolveActingFamily(tx, viewer, fx.famB, AccessContribute); err == nil {
			t.Error("an AccessView member should not be able to name family B for a write")
		}
	})
}
