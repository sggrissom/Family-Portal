// Tests for the Stage 6 analytics item of the multi-family plan: the
// aggregation loops used to assume families are disjoint sets of users and
// people. Three things must hold now that they are not:
//
//   - family size counts membership, so a household nobody has as their primary
//     family is still measured;
//   - a family's child count is its *home* roster, so a shared person is counted
//     once in the family that owns them and not again in the family hosting them;
//   - the system-wide per-child averages divide by every child, not only the
//     children in families that happen to have content.
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

// analyticsFixture is three households arranged to make each of the three
// failures above visible:
//
//	famParents  - admin's primary. Owns two children and all the milestones.
//	             Has content, so it lands in ContentPerFamily.
//	famGrand    - nobody's primary; `secondary` belongs to it by membership
//	             only. Hosts one of famParents' children on its roster through
//	             an accepted link, and owns one photo of its own so that it too
//	             lands in ContentPerFamily with a checkable child count.
//	famQuiet    - `secondary`'s primary. Owns one child and no content at all,
//	             so it is dropped from ContentPerFamily and only shows up in the
//	             averages' denominator.
type analyticsFixture struct {
	db         *vbolt.DB
	admin      User
	secondary  User
	famParents int
	famGrand   int
	famQuiet   int
	sharedKid  Person
	homeKid    Person
	quietKid   Person
}

func setupAnalyticsFixture(t *testing.T) (analyticsFixture, func()) {
	t.Helper()

	testDBPath := "test_analytics_multi_family.db"
	db := vbolt.Open(testDBPath)
	vbolt.InitBuckets(db, &cfg.Info)
	cleanup := func() {
		db.Close()
		os.Remove(testDBPath)
	}
	appDb = db

	fx := analyticsFixture{db: db}

	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		hash, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)

		fx.admin = AddUserTx(tx, CreateAccountRequest{
			Name: "Admin", Email: "admin@example.com",
		}, hash)
		fx.famParents = fx.admin.FamilyId
		// requireAdminAccess is still user.Id == 1 (its own Stage 6 item).
		fx.admin.Id = 1
		vbolt.Write(tx, UsersBkt, 1, &fx.admin)

		fx.secondary = AddUserTx(tx, CreateAccountRequest{
			Name: "Secondary", Email: "secondary@example.com",
		}, hash)
		fx.famQuiet = fx.secondary.FamilyId

		// famGrand is nobody's primary family: `secondary` joins it additively,
		// which writes a FamilyMembership and leaves User.FamilyId alone. Before
		// the fix this family measured as having zero members.
		grand := createFamilyTx(tx, "Grandparents", fx.secondary.Id)
		fx.famGrand = grand.Id
		EnsureMembershipTx(tx, fx.secondary.Id, fx.famGrand, AccessAdmin)

		var err error
		fx.sharedKid, err = AddPersonTx(tx, AddPersonRequest{
			Name: "Shared Kid", PersonType: int(Child), Gender: 0, Birthdate: "2020-06-15",
		}, fx.famParents)
		if err != nil {
			t.Fatalf("AddPersonTx sharedKid: %v", err)
		}
		fx.homeKid, err = AddPersonTx(tx, AddPersonRequest{
			Name: "Home Kid", PersonType: int(Child), Gender: 1, Birthdate: "2018-01-20",
		}, fx.famParents)
		if err != nil {
			t.Fatalf("AddPersonTx homeKid: %v", err)
		}
		fx.quietKid, err = AddPersonTx(tx, AddPersonRequest{
			Name: "Quiet Kid", PersonType: int(Child), Gender: 0, Birthdate: "2021-09-09",
		}, fx.famQuiet)
		if err != nil {
			t.Fatalf("AddPersonTx quietKid: %v", err)
		}

		// A parent in famParents, to prove the child count filters by role and
		// does not just count roster size.
		if _, err = AddPersonTx(tx, AddPersonRequest{
			Name: "A Parent", PersonType: int(Parent), Gender: 1, Birthdate: "1988-04-04",
		}, fx.famParents); err != nil {
			t.Fatalf("AddPersonTx parent: %v", err)
		}

		// The grandparents host sharedKid: an accepted link plus a roster row,
		// which is the only way a person reaches a second roster.
		link := FamilyLink{
			Id:           vbolt.NextIntId(tx, FamilyLinkBkt),
			FromFamilyId: fx.famParents,
			ToFamilyId:   fx.famGrand,
			Kind:         "grandparents",
			Access:       AccessView,
			Scopes:       normalizeLinkScopes(DefaultLinkScopes()).ToMask(),
			Status:       LinkAccepted,
			CreatedAt:    time.Now(),
		}
		vbolt.Write(tx, FamilyLinkBkt, link.Id, &link)
		vbolt.SetTargetSingleTerm(tx, FamilyLinkByFromIndex, link.Id, link.FromFamilyId)
		vbolt.SetTargetSingleTerm(tx, FamilyLinkByToIndex, link.Id, link.ToFamilyId)
		// Child at home, and a grandchild on the grandparents' roster — the same
		// Person record with a different role on each.
		EnsurePersonFamilyTx(tx, fx.sharedKid.Id, fx.famGrand, Child)

		// Content. famParents owns three photos and two milestones; famGrand owns
		// one photo of its own; famQuiet owns nothing.
		photos := []Image{
			{FamilyId: fx.famParents, MimeType: "image/jpeg"},
			{FamilyId: fx.famParents, MimeType: "image/jpeg"},
			{FamilyId: fx.famParents, MimeType: "image/png"},
			{FamilyId: fx.famGrand, MimeType: "image/jpeg"},
		}
		for _, photo := range photos {
			photo.Id = vbolt.NextIntId(tx, ImagesBkt)
			photo.CreatedAt = time.Now().AddDate(0, 0, -1)
			vbolt.Write(tx, ImagesBkt, photo.Id, &photo)
			vbolt.SetTargetSingleTerm(tx, ImageByFamilyIndex, photo.Id, photo.FamilyId)
		}

		milestones := []Milestone{
			{PersonId: fx.sharedKid.Id, FamilyId: fx.famParents, Category: "development"},
			{PersonId: fx.homeKid.Id, FamilyId: fx.famParents, Category: "achievement"},
		}
		for _, milestone := range milestones {
			milestone.Id = vbolt.NextIntId(tx, MilestoneBkt)
			milestone.CreatedAt = time.Now().AddDate(0, 0, -1)
			vbolt.Write(tx, MilestoneBkt, milestone.Id, &milestone)
		}

		vbolt.TxCommit(tx)
	})

	return fx, cleanup
}

func (fx analyticsFixture) adminCtx(t *testing.T, tx *vbolt.Tx) *vbeam.Context {
	t.Helper()
	token, err := generateAuthJwt(fx.admin, httptest.NewRecorder())
	if err != nil {
		t.Fatalf("generateAuthJwt: %v", err)
	}
	return &vbeam.Context{Tx: tx, Token: token}
}

func statsForFamily(t *testing.T, resp ContentAnalyticsResponse, name string) FamilyContentStats {
	t.Helper()
	for _, stats := range resp.ContentPerFamily {
		if stats.FamilyName == name {
			return stats
		}
	}
	t.Fatalf("family %q missing from ContentPerFamily", name)
	return FamilyContentStats{}
}

// A family that is nobody's primary still has the members who joined it.
func TestFamilySizeCountsMembershipNotPrimary(t *testing.T) {
	fx, cleanup := setupAnalyticsFixture(t)
	defer cleanup()

	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		resp, err := GetUserAnalytics(fx.adminCtx(t, tx), Empty{})
		if err != nil {
			t.Fatalf("GetUserAnalytics: %v", err)
		}

		sizes := make(map[string]int)
		for _, point := range resp.FamilySizeDistribution {
			sizes[point.Label] = point.Value
		}

		// All three families hold exactly one member, and famGrand's is a
		// membership row rather than a User.FamilyId. Before the fix famGrand
		// measured as empty and the size > 0 filter dropped it, leaving 2.
		if sizes["1 member"] != 3 {
			t.Errorf("expected 3 one-member families, got %d (%v)", sizes["1 member"], sizes)
		}
		if _, dropped := sizes["0 members"]; dropped {
			t.Error("a family measured as having zero members")
		}
	})
}

// A user in two families counts toward the size of both.
func TestFamilySizeCountsAUserInEveryFamilyTheyJoin(t *testing.T) {
	fx, cleanup := setupAnalyticsFixture(t)
	defer cleanup()

	// The admin joins the grandparents too, making it a two-member household
	// while remaining a member of their own.
	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		EnsureMembershipTx(tx, fx.admin.Id, fx.famGrand, AccessAdmin)
		vbolt.TxCommit(tx)
	})

	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		resp, err := GetUserAnalytics(fx.adminCtx(t, tx), Empty{})
		if err != nil {
			t.Fatalf("GetUserAnalytics: %v", err)
		}

		sizes := make(map[string]int)
		for _, point := range resp.FamilySizeDistribution {
			sizes[point.Label] = point.Value
		}

		if sizes["2 members"] != 1 {
			t.Errorf("expected famGrand to have 2 members, got %v", sizes)
		}
		// famParents keeps its member: joining a second family is additive and
		// must not move anyone out of their own household.
		if sizes["1 member"] != 2 {
			t.Errorf("expected the other two families to keep 1 member each, got %v", sizes)
		}
	})
}

// A shared person is counted by the family that owns them, not by the family
// hosting them on its roster.
func TestChildCountUsesHomeRosterNotSharedRoster(t *testing.T) {
	fx, cleanup := setupAnalyticsFixture(t)
	defer cleanup()

	// The premise: sharedKid really is on both rosters.
	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		rosters := GetPersonFamilies(tx, fx.sharedKid.Id)
		if len(rosters) != 2 {
			t.Fatalf("expected sharedKid on 2 rosters, got %d", len(rosters))
		}
		if len(GetFamilyPeople(tx, fx.famGrand)) != 1 {
			t.Fatal("expected the grandparents' roster to show the shared child")
		}
	})

	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		resp, err := GetContentAnalytics(fx.adminCtx(t, tx), Empty{})
		if err != nil {
			t.Fatalf("GetContentAnalytics: %v", err)
		}

		// The owning family counts both its children and not the parent.
		parents := statsForFamily(t, resp, "Admin's Family")
		if parents.Children != 2 {
			t.Errorf("expected the owning family to count 2 children, got %d", parents.Children)
		}

		// The hosting family owns nobody, even though its roster shows a child.
		// Counting the full roster here is what would double-count her.
		grand := statsForFamily(t, resp, "Grandparents")
		if grand.Children != 0 {
			t.Errorf("expected the hosting family to count 0 children, got %d", grand.Children)
		}
		if grand.Photos != 1 {
			t.Errorf("expected the hosting family's own photo to be counted, got %d", grand.Photos)
		}
		// No children means no ratio, rather than a division by a borrowed one.
		if grand.PhotosPerChild != 0 {
			t.Errorf("expected no per-child ratio for a family with no children, got %f", grand.PhotosPerChild)
		}
	})
}

// The child count reads the roster row's role, which is what replaced the
// deprecated Person.Type.
func TestChildCountFollowsTheHomeRosterRole(t *testing.T) {
	fx, cleanup := setupAnalyticsFixture(t)
	defer cleanup()

	// Promote homeKid to Parent on their home roster only. Their role on the
	// grandparents' roster is irrelevant to the owning family's count.
	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		SetPersonFamilyRoleTx(tx, fx.homeKid.Id, fx.famParents, Parent)
		vbolt.TxCommit(tx)
	})

	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		resp, err := GetContentAnalytics(fx.adminCtx(t, tx), Empty{})
		if err != nil {
			t.Fatalf("GetContentAnalytics: %v", err)
		}
		parents := statsForFamily(t, resp, "Admin's Family")
		if parents.Children != 1 {
			t.Errorf("expected the role change to drop the count to 1, got %d", parents.Children)
		}
	})
}

// The system-wide averages divide by every child, including those in families
// with no content, and count a shared child once.
func TestPerChildAveragesUseEveryChildExactlyOnce(t *testing.T) {
	fx, cleanup := setupAnalyticsFixture(t)
	defer cleanup()

	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		resp, err := GetContentAnalytics(fx.adminCtx(t, tx), Empty{})
		if err != nil {
			t.Fatalf("GetContentAnalytics: %v", err)
		}

		// famQuiet has a child and no content, so it is absent from
		// ContentPerFamily — which is exactly why summing that slice was the
		// wrong denominator.
		for _, stats := range resp.ContentPerFamily {
			if stats.FamilyName == "Secondary's Family" {
				t.Fatal("a family with no content should not appear in ContentPerFamily")
			}
		}

		// 3 children in total: two owned by famParents, one by famQuiet. The
		// shared child is one child, not two. 4 photos and 2 milestones exist.
		const totalChildren = 3
		wantPhotos := 4.0 / totalChildren
		wantMilestones := 2.0 / totalChildren

		if !nearlyEqual(resp.AveragePhotosPerChild, wantPhotos) {
			t.Errorf("AveragePhotosPerChild = %f, want %f (4 photos / %d children)",
				resp.AveragePhotosPerChild, wantPhotos, totalChildren)
		}
		if !nearlyEqual(resp.AverageMilestonesPerChild, wantMilestones) {
			t.Errorf("AverageMilestonesPerChild = %f, want %f (2 milestones / %d children)",
				resp.AverageMilestonesPerChild, wantMilestones, totalChildren)
		}
	})
}

func nearlyEqual(got, want float64) bool {
	diff := got - want
	if diff < 0 {
		diff = -diff
	}
	return diff < 1e-9
}
