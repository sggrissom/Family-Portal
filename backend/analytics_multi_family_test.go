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
		fx.admin.Id = 1
		vbolt.Write(tx, UsersBkt, 1, &fx.admin)

		fx.secondary = AddUserTx(tx, CreateAccountRequest{
			Name: "Secondary", Email: "secondary@example.com",
		}, hash)
		fx.famQuiet = fx.secondary.FamilyId

		grand := createFamilyTx(tx, "Grandparents", fx.secondary.Id)
		fx.famGrand = grand.Id
		EnsureMembershipTx(tx, fx.secondary.Id, fx.famGrand, AccessAdmin)

		var err error
		fx.sharedKid, err = AddPersonTx(tx, AddPersonRequest{
			Name: "Shared Kid", Gender: 0, Birthdate: "2020-06-15",
		}, fx.famParents)
		if err != nil {
			t.Fatalf("AddPersonTx sharedKid: %v", err)
		}
		fx.homeKid, err = AddPersonTx(tx, AddPersonRequest{
			Name: "Home Kid", Gender: 1, Birthdate: "2018-01-20",
		}, fx.famParents)
		if err != nil {
			t.Fatalf("AddPersonTx homeKid: %v", err)
		}
		fx.quietKid, err = AddPersonTx(tx, AddPersonRequest{
			Name: "Quiet Kid", Gender: 0, Birthdate: "2021-09-09",
		}, fx.famQuiet)
		if err != nil {
			t.Fatalf("AddPersonTx quietKid: %v", err)
		}

		if _, err = AddPersonTx(tx, AddPersonRequest{
			Name: "A Parent", Gender: 1, Birthdate: "1988-04-04",
		}, fx.famParents); err != nil {
			t.Fatalf("AddPersonTx parent: %v", err)
		}

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
		EnsurePersonFamilyTx(tx, fx.sharedKid.Id, fx.famGrand)

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

		if sizes["1 member"] != 3 {
			t.Errorf("expected 3 one-member families, got %d (%v)", sizes["1 member"], sizes)
		}
		if _, dropped := sizes["0 members"]; dropped {
			t.Error("a family measured as having zero members")
		}
	})
}

func TestFamilySizeCountsAUserInEveryFamilyTheyJoin(t *testing.T) {
	fx, cleanup := setupAnalyticsFixture(t)
	defer cleanup()

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
		if sizes["1 member"] != 2 {
			t.Errorf("expected the other two families to keep 1 member each, got %v", sizes)
		}
	})
}

func TestPersonCountUsesHomeFamilyNotSharedRoster(t *testing.T) {
	fx, cleanup := setupAnalyticsFixture(t)
	defer cleanup()

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

		owning := statsForFamily(t, resp, "Admin's Family")
		if owning.People != 3 {
			t.Errorf("expected the owning family to count 3 people, got %d", owning.People)
		}

		grand := statsForFamily(t, resp, "Grandparents")
		if grand.People != 0 {
			t.Errorf("expected the hosting family to count 0 of its own people, got %d", grand.People)
		}
		if grand.Photos != 1 {
			t.Errorf("expected the hosting family's own photo to be counted, got %d", grand.Photos)
		}
		if grand.PhotosPerPerson != 0 {
			t.Errorf("expected no per-person ratio for a family with no people, got %f", grand.PhotosPerPerson)
		}
	})
}

func TestPerPersonAveragesCountEveryPersonExactlyOnce(t *testing.T) {
	fx, cleanup := setupAnalyticsFixture(t)
	defer cleanup()

	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		resp, err := GetContentAnalytics(fx.adminCtx(t, tx), Empty{})
		if err != nil {
			t.Fatalf("GetContentAnalytics: %v", err)
		}

		for _, stats := range resp.ContentPerFamily {
			if stats.FamilyName == "Secondary's Family" {
				t.Fatal("a family with no content should not appear in ContentPerFamily")
			}
		}

		const totalPeople = 4
		wantPhotos := 4.0 / totalPeople
		wantMilestones := 2.0 / totalPeople

		if !nearlyEqual(resp.AveragePhotosPerPerson, wantPhotos) {
			t.Errorf("AveragePhotosPerPerson = %f, want %f (4 photos / %d people)",
				resp.AveragePhotosPerPerson, wantPhotos, totalPeople)
		}
		if !nearlyEqual(resp.AverageMilestonesPerPerson, wantMilestones) {
			t.Errorf("AverageMilestonesPerPerson = %f, want %f (2 milestones / %d people)",
				resp.AverageMilestonesPerPerson, wantMilestones, totalPeople)
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
