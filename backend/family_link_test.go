// Tests for Stage 5 of the multi-family plan: families relate to each other.
// The three things that must hold are that a linked family sees the content the
// link names and nothing else, that the relationship is one-directional, and
// that it never chains — A sharing with B and B sharing with C must not give C
// anything of A's.
package backend

import (
	"family/cfg"
	"os"
	"testing"
	"time"

	"go.hasen.dev/vbeam"
	"go.hasen.dev/vbolt"
	"golang.org/x/crypto/bcrypt"
)

// familyLinkFixture is three households.
//
//	A (parents)      homes alice and bob, and shares alice with B
//	B (grandparents) homes robin, and shares robin with C
//	C (friends)      homes nobody of its own
//
// The chain A -> B -> C is deliberate: C holds a full link from B and has one
// of B's people on its roster, which is exactly the shape that would leak
// alice if access were transitive.
type familyLinkFixture struct {
	db    *vbolt.DB
	userA User
	userB User
	userC User
	famA  int
	famB  int
	famC  int

	alice Person // homed in A, shared into B
	bob   Person // homed in A, shared with nobody
	robin Person // homed in B, shared into C

	aliceMilestone Milestone
	aliceGrowth    GrowthData
	alicePhoto     Image // tagged with alice
	untaggedPhoto  Image // in A, nobody in it
	tagA           Tag

	linkAB FamilyLink
	linkBC FamilyLink
}

func setupFamilyLinkFixture(t *testing.T) (familyLinkFixture, func()) {
	t.Helper()

	testDBPath := "test_family_link.db"
	db := vbolt.Open(testDBPath)
	vbolt.InitBuckets(db, &cfg.Info)
	cleanup := func() {
		db.Close()
		os.Remove(testDBPath)
	}

	fx := familyLinkFixture{db: db}

	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		hash, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)

		fx.userA = AddUserTx(tx, CreateAccountRequest{Name: "Parent", Email: "a@example.com"}, hash)
		fx.userB = AddUserTx(tx, CreateAccountRequest{Name: "Grandparent", Email: "b@example.com"}, hash)
		fx.userC = AddUserTx(tx, CreateAccountRequest{Name: "Friend", Email: "c@example.com"}, hash)
		fx.famA = fx.userA.FamilyId
		fx.famB = fx.userB.FamilyId
		fx.famC = fx.userC.FamilyId

		var err error
		fx.alice, err = AddPersonTx(tx, AddPersonRequest{
			Name: "Alice", PersonType: int(Child), Gender: 1, Birthdate: "2018-04-01",
		}, fx.famA)
		if err != nil {
			t.Fatalf("AddPersonTx alice: %v", err)
		}
		fx.bob, err = AddPersonTx(tx, AddPersonRequest{
			Name: "Bob", PersonType: int(Child), Gender: 0, Birthdate: "2021-09-12",
		}, fx.famA)
		if err != nil {
			t.Fatalf("AddPersonTx bob: %v", err)
		}
		fx.robin, err = AddPersonTx(tx, AddPersonRequest{
			Name: "Robin", PersonType: int(Child), Gender: 0, Birthdate: "2015-01-20",
		}, fx.famB)
		if err != nil {
			t.Fatalf("AddPersonTx robin: %v", err)
		}

		fx.aliceMilestone, err = AddMilestoneTx(tx, AddMilestoneRequest{
			PersonId:    fx.alice.Id,
			Description: "First day of school",
			Category:    "development",
			InputType:   "today",
		}, fx.famA)
		if err != nil {
			t.Fatalf("AddMilestoneTx: %v", err)
		}

		fx.aliceGrowth, err = AddGrowthDataTx(tx, AddGrowthDataRequest{
			PersonId:        fx.alice.Id,
			MeasurementType: "height",
			Value:           110,
			Unit:            "cm",
			InputType:       "today",
		}, fx.famA)
		if err != nil {
			t.Fatalf("AddGrowthDataTx: %v", err)
		}

		fx.alicePhoto = writeTestImage(tx, fx.famA, fx.userA.Id, "alice.jpg")
		tagPersonInPhoto(tx, fx.alicePhoto.Id, fx.alice.Id, fx.famA)
		fx.untaggedPhoto = writeTestImage(tx, fx.famA, fx.userA.Id, "landscape.jpg")

		fx.tagA = Tag{
			Id: vbolt.NextIntId(tx, TagBkt), FamilyId: fx.famA,
			Name: "School", Color: "#4A90D9", CreatedAt: time.Now(),
		}
		vbolt.Write(tx, TagBkt, fx.tagA.Id, &fx.tagA)
		vbolt.SetTargetSingleTerm(tx, TagByFamilyIndex, fx.tagA.Id, fx.famA)

		// A shares its people, milestones and photos with B — but not its
		// measurements, which is the default.
		fx.linkAB = createFamilyLinkTx(tx, fx.famA, fx.famB, "grandparents", AccessView, DefaultLinkScopes().ToMask())
		fx.linkAB.Status = LinkAccepted
		writeFamilyLinkTx(tx, fx.linkAB)
		EnsurePersonFamilyTx(tx, fx.alice.Id, fx.famB, Child)

		// B shares everything it has with C, and puts robin on C's roster. This
		// is the second hop that must not reach back to A's people.
		everything := LinkScopes{People: true, Milestones: true, Photos: true, Growth: true}
		fx.linkBC = createFamilyLinkTx(tx, fx.famB, fx.famC, "friends", AccessView, everything.ToMask())
		fx.linkBC.Status = LinkAccepted
		writeFamilyLinkTx(tx, fx.linkBC)
		EnsurePersonFamilyTx(tx, fx.robin.Id, fx.famC, Child)

		vbolt.TxCommit(tx)
	})

	return fx, cleanup
}

func writeTestImage(tx *vbolt.Tx, familyId int, ownerId int, filename string) Image {
	image := Image{
		Id: vbolt.NextIntId(tx, ImagesBkt), FamilyId: familyId,
		OwnerUserId: ownerId, OriginalFilename: filename,
		MimeType: "image/jpeg", CreatedAt: time.Now(),
	}
	vbolt.Write(tx, ImagesBkt, image.Id, &image)
	vbolt.SetTargetSingleTerm(tx, ImageByFamilyIndex, image.Id, familyId)
	return image
}

func tagPersonInPhoto(tx *vbolt.Tx, photoId int, personId int, familyId int) {
	link := PhotoPerson{
		Id: vbolt.NextIntId(tx, PhotoPersonBkt), PhotoId: photoId,
		PersonId: personId, FamilyId: familyId, CreatedAt: time.Now(),
	}
	vbolt.Write(tx, PhotoPersonBkt, link.Id, &link)
	vbolt.SetTargetSingleTerm(tx, PhotoPersonByPhotoIndex, link.Id, photoId)
	vbolt.SetTargetSingleTerm(tx, PhotoPersonByPersonIndex, link.Id, personId)
}

func setLinkScopes(t *testing.T, fx familyLinkFixture, link FamilyLink, scopes LinkScopes) {
	t.Helper()
	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		link.Scopes = normalizeLinkScopes(scopes).ToMask()
		writeFamilyLinkTx(tx, link)
		vbolt.TxCommit(tx)
	})
}

func setLinkStatus(t *testing.T, fx familyLinkFixture, link FamilyLink, status LinkStatus) {
	t.Helper()
	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		link.Status = status
		writeFamilyLinkTx(tx, link)
		vbolt.TxCommit(tx)
	})
}

// The grandparents see the shared child and the kinds of record their link
// names, and nothing else in the family that shared her.
func TestLinkGrantsOnlyWhatItCarries(t *testing.T) {
	fx, cleanup := setupFamilyLinkFixture(t)
	defer cleanup()

	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		if !CanAccessPerson(tx, fx.userB, fx.alice, ScopePeople, AccessView) {
			t.Error("the shared child is not reachable by the family she was shared with")
		}
		if !CanAccessPerson(tx, fx.userB, fx.alice, ScopeMilestones, AccessView) {
			t.Error("milestones are in the link's scopes but were denied")
		}
		if !CanAccessPerson(tx, fx.userB, fx.alice, ScopePhotos, AccessView) {
			t.Error("photos are in the link's scopes but were denied")
		}
		if CanAccessPerson(tx, fx.userB, fx.alice, ScopeGrowth, AccessView) {
			t.Error("measurements are not in the link's scopes but were allowed")
		}

		if _, err := GetMilestoneForUser(tx, fx.aliceMilestone.Id, fx.userB, AccessView); err != nil {
			t.Errorf("shared milestone denied: %v", err)
		}
		if _, err := GetGrowthDataForUser(tx, fx.aliceGrowth.Id, fx.userB, AccessView); err == nil {
			t.Error("a measurement was readable through a link that does not share measurements")
		}
		if !CanAccessPhoto(tx, fx.userB, fx.alicePhoto, AccessView) {
			t.Error("a photo of the shared child was denied")
		}
		if CanAccessPhoto(tx, fx.userB, fx.untaggedPhoto, AccessView) {
			t.Error("a photo with nobody shared in it leaked through the link")
		}

		// The unshared sibling and the family itself stay out of reach.
		if CanAccessPerson(tx, fx.userB, fx.bob, ScopePeople, AccessView) {
			t.Error("a person who was never shared is reachable")
		}
		if CanAccessFamily(tx, fx.userB, fx.famA, AccessView) {
			t.Error("a link conferred whole-family access")
		}
		if families := familiesVisibleTo(tx, fx.userB); len(families) != 1 || families[0] != fx.famB {
			t.Errorf("a link should not appear as membership, got %v", families)
		}
	})
}

// Whatever a link opens up, it opens up for reading only.
func TestLinkNeverGrantsWrites(t *testing.T) {
	fx, cleanup := setupFamilyLinkFixture(t)
	defer cleanup()

	// Even a link that asks for more than view gets clamped on the way in.
	greedy := fx.linkAB
	greedy.Access = AccessAdmin
	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		greedy.Access = clampLinkAccess(greedy.Access)
		writeFamilyLinkTx(tx, greedy)
		vbolt.TxCommit(tx)
	})

	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		for _, need := range []AccessLevel{AccessContribute, AccessAdmin} {
			if CanAccessPerson(tx, fx.userB, fx.alice, ScopePeople, need) {
				t.Errorf("a link granted level %d on a shared person", need)
			}
			if CanAccessRecordOfPerson(tx, fx.userB, fx.famA, fx.alice.Id, ScopeMilestones, need) {
				t.Errorf("a link granted level %d on a shared person's records", need)
			}
			if CanAccessPhoto(tx, fx.userB, fx.alicePhoto, need) {
				t.Errorf("a link granted level %d on a shared person's photo", need)
			}
		}
		if _, err := GetMilestoneForUser(tx, fx.aliceMilestone.Id, fx.userB, AccessContribute); err == nil {
			t.Error("a milestone was writable through a read-only link")
		}
	})
}

// The relationship runs one way. B receiving A's child does not let A see B's.
func TestLinkAccessIsAsymmetric(t *testing.T) {
	fx, cleanup := setupFamilyLinkFixture(t)
	defer cleanup()

	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		for _, scope := range []LinkScope{ScopePeople, ScopeMilestones, ScopePhotos, ScopeGrowth} {
			if CanAccessPerson(tx, fx.userA, fx.robin, scope, AccessView) {
				t.Errorf("A reached B's child at scope %d with only an A->B link", scope)
			}
		}
		if CanAccessFamily(tx, fx.userA, fx.famB, AccessView) {
			t.Error("A reached family B")
		}
		if people := GetVisiblePeople(tx, fx.userA); len(people) != 2 {
			t.Errorf("A's roster should still be its own two people, got %d", len(people))
		}
	})
}

// A -> B and B -> C must not add up to A -> C, in either direction.
func TestNoTransitiveAccessLeak(t *testing.T) {
	fx, cleanup := setupFamilyLinkFixture(t)
	defer cleanup()

	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		// C holds a link carrying everything B has, and one of B's people is on
		// C's roster — but alice is A's, merely visiting B's roster.
		for _, scope := range []LinkScope{ScopePeople, ScopeMilestones, ScopePhotos, ScopeGrowth} {
			if CanAccessPerson(tx, fx.userC, fx.alice, scope, AccessView) {
				t.Errorf("C reached A's child through B at scope %d", scope)
			}
		}
		if _, err := GetMilestoneForUser(tx, fx.aliceMilestone.Id, fx.userC, AccessView); err == nil {
			t.Error("C read A's milestone through B")
		}
		if CanAccessPhoto(tx, fx.userC, fx.alicePhoto, AccessView) {
			t.Error("C saw A's photo through B")
		}

		// The one hop C does have still works, so the test is not passing
		// because nothing works.
		if !CanAccessPerson(tx, fx.userC, fx.robin, ScopeGrowth, AccessView) {
			t.Error("C's own link to B stopped working")
		}

		// Neither can A's people be pushed onward: sharing is authorized by the
		// home family's own links, and A has none to C.
		if CanShareIntoFamily(tx, fx.famA, fx.famC) {
			t.Error("A can share into C without a link to it")
		}

		for _, familyId := range sharedInFamilies(tx, fx.userC, ScopeMilestones) {
			if familyId == fx.famA {
				t.Error("A appears among the families C can read")
			}
		}
	})
}

// A link that has not been accepted, or that has been revoked, grants nothing.
func TestPendingAndRevokedLinksGrantNothing(t *testing.T) {
	fx, cleanup := setupFamilyLinkFixture(t)
	defer cleanup()

	for _, status := range []LinkStatus{LinkPending, LinkRevoked} {
		setLinkStatus(t, fx, fx.linkAB, status)
		vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
			if CanAccessPerson(tx, fx.userB, fx.alice, ScopePeople, AccessView) {
				t.Errorf("status %d still granted access to the shared person", status)
			}
			if _, err := GetMilestoneForUser(tx, fx.aliceMilestone.Id, fx.userB, AccessView); err == nil {
				t.Errorf("status %d still granted access to a milestone", status)
			}
		})
	}
}

// Turning a scope on is what makes it readable, so the scope list is doing the
// work rather than the access level.
func TestScopesAreIndependentlyGranted(t *testing.T) {
	fx, cleanup := setupFamilyLinkFixture(t)
	defer cleanup()

	setLinkScopes(t, fx, fx.linkAB, LinkScopes{People: true, Growth: true})
	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		if _, err := GetGrowthDataForUser(tx, fx.aliceGrowth.Id, fx.userB, AccessView); err != nil {
			t.Errorf("measurements denied after the scope was granted: %v", err)
		}
		if _, err := GetMilestoneForUser(tx, fx.aliceMilestone.Id, fx.userB, AccessView); err == nil {
			t.Error("milestones readable after the scope was taken away")
		}
		if CanAccessPhoto(tx, fx.userB, fx.alicePhoto, AccessView) {
			t.Error("photos readable after the scope was taken away")
		}
	})
}

// The list paths a page actually calls have to agree with the point checks.
func TestSharedPersonShowsUpInListPaths(t *testing.T) {
	fx, cleanup := setupFamilyLinkFixture(t)
	defer cleanup()

	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		people := GetVisiblePeople(tx, fx.userB)
		var sawAlice, sawBob bool
		for _, person := range people {
			sawAlice = sawAlice || person.Id == fx.alice.Id
			sawBob = sawBob || person.Id == fx.bob.Id
		}
		if !sawAlice {
			t.Error("the shared child is missing from the roster")
		}
		if sawBob {
			t.Error("an unshared sibling appeared on the roster")
		}

		images := GetVisibleImages(tx, fx.userB)
		var sawAlicePhoto, sawUntagged bool
		for _, image := range images {
			sawAlicePhoto = sawAlicePhoto || image.Id == fx.alicePhoto.Id
			sawUntagged = sawUntagged || image.Id == fx.untaggedPhoto.Id
		}
		if !sawAlicePhoto {
			t.Error("a photo of the shared child is missing from the photo list")
		}
		if sawUntagged {
			t.Error("a photo of nobody shared leaked into the photo list")
		}

		// The sharing family's tags come along, or every shared milestone would
		// render its labels as blanks.
		var sawTag bool
		for _, tag := range getVisibleTags(tx, fx.userB) {
			sawTag = sawTag || tag.Id == fx.tagA.Id
		}
		if !sawTag {
			t.Error("the sharing family's tags did not resolve")
		}

		if found := SearchVisibleMilestones(tx, "school", fx.userB, 10); len(found) != 1 {
			t.Errorf("expected the shared milestone in search results, got %d", len(found))
		}
		if found := SearchVisibleMilestones(tx, "school", fx.userC, 10); len(found) != 0 {
			t.Errorf("a milestone two hops away turned up in search, got %d", len(found))
		}
	})
}

// Ending the relationship takes the shared people with it, so no roster is left
// holding names it can no longer open.
func TestRevokingALinkUnsharesItsPeople(t *testing.T) {
	fx, cleanup := setupFamilyLinkFixture(t)
	defer cleanup()

	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		link := fx.linkAB
		link.Status = LinkRevoked
		writeFamilyLinkTx(tx, link)
		unshareAllThroughLinkTx(tx, link)
		vbolt.TxCommit(tx)
	})

	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		if _, found := FindPersonFamily(tx, fx.alice.Id, fx.famB); found {
			t.Error("the shared roster row outlived the link")
		}
		if _, found := FindPersonFamily(tx, fx.alice.Id, fx.famA); !found {
			t.Error("the home roster row was removed")
		}
		if len(GetVisiblePeople(tx, fx.userB)) != 1 {
			t.Error("the grandparents' roster did not go back to their own person")
		}
		// The person record itself survives — sharing never copied them.
		if person := GetPersonById(tx, fx.alice.Id); person.Id == 0 || person.FamilyId != fx.famA {
			t.Error("unsharing damaged the person's home family")
		}
	})
}

// Who a person may be shared into is a question about links, not about the
// acting user's own memberships.
func TestShareTargetsComeFromLinks(t *testing.T) {
	fx, cleanup := setupFamilyLinkFixture(t)
	defer cleanup()

	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		// Bob is not shared yet, so B is offered as a target for him.
		sharing := personSharing(tx, fx.userA, fx.bob)
		if !sharing.Manageable {
			t.Fatal("the owning family cannot manage its own person")
		}
		if len(sharing.CanShare) != 1 || sharing.CanShare[0].FamilyId != fx.famB {
			t.Errorf("expected exactly the linked family as a target, got %v", sharing.CanShare)
		}
		if len(sharing.SharedWith) != 0 {
			t.Errorf("bob is not shared with anyone, got %v", sharing.SharedWith)
		}

		// Alice is already on B's roster, so B is no longer offered.
		sharing = personSharing(tx, fx.userA, fx.alice)
		if len(sharing.CanShare) != 0 {
			t.Errorf("a family already holding the person was offered again: %v", sharing.CanShare)
		}
		if len(sharing.SharedWith) != 1 || sharing.SharedWith[0].FamilyId != fx.famB {
			t.Errorf("expected alice shared with B, got %v", sharing.SharedWith)
		}

		// The receiving family sees the share but cannot manage it.
		sharing = personSharing(tx, fx.userB, fx.alice)
		if sharing.Manageable {
			t.Error("the receiving family can manage a person it does not own")
		}
		if len(sharing.CanShare) != 0 {
			t.Error("the receiving family was offered onward share targets")
		}
	})
}

// A person is shared, not copied: the roster gains a row and the bucket does
// not gain a record.
func TestSharingThroughALinkCreatesNoDuplicatePerson(t *testing.T) {
	fx, cleanup := setupFamilyLinkFixture(t)
	defer cleanup()

	countPeople := func() (count int) {
		vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
			vbolt.IterateAll(tx, PeopleBkt, func(_ int, _ Person) bool {
				count++
				return true
			})
		})
		return
	}

	before := countPeople()
	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		EnsurePersonFamilyTx(tx, fx.bob.Id, fx.famB, Child)
		vbolt.TxCommit(tx)
	})
	if after := countPeople(); after != before {
		t.Errorf("sharing created person records: %d -> %d", before, after)
	}

	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		if len(GetPersonFamilies(tx, fx.bob.Id)) != 2 {
			t.Error("bob should be on his home roster and the shared one")
		}
		if person := GetPersonById(tx, fx.bob.Id); person.FamilyId != fx.famA {
			t.Error("sharing moved the person's home family")
		}
	})
}

// A user with no link and no membership reaches nothing, which is the
// pre-Stage-5 behavior for every family that has not linked to them.
func TestUnlinkedFamilyIsStillFullyDenied(t *testing.T) {
	fx, cleanup := setupFamilyLinkFixture(t)
	defer cleanup()

	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		for _, need := range []AccessLevel{AccessView, AccessContribute, AccessAdmin} {
			if CanAccessFamily(tx, fx.userC, fx.famA, need) {
				t.Errorf("an unlinked family was granted level %d", need)
			}
		}
		if _, err := GetGrowthDataForUser(tx, fx.aliceGrowth.Id, fx.userC, AccessView); err == nil {
			t.Error("an unlinked family read a measurement")
		}
		if _, err := GetMilestoneForUser(tx, fx.aliceMilestone.Id, fx.userC, AccessView); err == nil {
			t.Error("an unlinked family read a milestone")
		}
		if CanAccessPhoto(tx, fx.userC, fx.alicePhoto, AccessView) {
			t.Error("an unlinked family read a photo")
		}
	})
}

// The invite/accept/share flow, driven through the procs a client actually
// calls, so the authorization on each of them is exercised rather than assumed.
func TestLinkProcFlow(t *testing.T) {
	fx, cleanup := setupFamilyLinkFixture(t)
	defer cleanup()

	// Family D is a fresh household with no relationship to anyone.
	var userD User
	var famD Family
	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		hash, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)
		userD = AddUserTx(tx, CreateAccountRequest{Name: "Aunt", Email: "d@example.com"}, hash)
		famD = GetFamily(tx, userD.FamilyId)
		vbolt.TxCommit(tx)
	})

	tokenA := testAuthToken(t, fx.userA)
	tokenB := testAuthToken(t, fx.userB)
	tokenD := testAuthToken(t, userD)

	var link FamilyLinkView

	// A offers to share with D, naming D by its invite code.
	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		ctx := &vbeam.Context{Tx: tx, Token: tokenA}
		resp, procErr := CreateFamilyLink(ctx, CreateFamilyLinkRequest{
			InviteCode: famD.InviteCode,
			Kind:       "aunt",
			Scopes:     LinkScopes{Milestones: true},
		})
		if procErr != nil || !resp.Success {
			t.Fatalf("CreateFamilyLink: %v %q", procErr, resp.Error)
		}
		// People is implied by milestones, or the shared records would hang off
		// somebody the other family cannot see.
		if !resp.Link.Scopes.People {
			t.Error("people scope was not implied by milestones")
		}
		if resp.Link.Status != LinkPending {
			t.Error("a new link should start pending")
		}
		link = resp.Link
	})

	// Until D accepts, nothing is shareable into it.
	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		if CanShareIntoFamily(tx, fx.famA, famD.Id) {
			t.Error("a pending link already allowed sharing")
		}
	})

	// The offering family cannot accept on the other's behalf.
	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		ctx := &vbeam.Context{Tx: tx, Token: tokenA}
		if _, procErr := AcceptFamilyLink(ctx, FamilyLinkIdRequest{Id: link.Id}); procErr == nil {
			t.Error("the sharing family accepted its own offer")
		}
	})
	// Nor can an unrelated family.
	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		ctx := &vbeam.Context{Tx: tx, Token: tokenB}
		if _, procErr := AcceptFamilyLink(ctx, FamilyLinkIdRequest{Id: link.Id}); procErr == nil {
			t.Error("an unrelated family accepted someone else's link")
		}
	})

	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		ctx := &vbeam.Context{Tx: tx, Token: tokenD}
		resp, procErr := AcceptFamilyLink(ctx, FamilyLinkIdRequest{Id: link.Id})
		if procErr != nil || !resp.Success {
			t.Fatalf("AcceptFamilyLink: %v %q", procErr, resp.Error)
		}
	})

	// The receiving family cannot widen what it was given.
	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		ctx := &vbeam.Context{Tx: tx, Token: tokenD}
		if _, procErr := UpdateFamilyLink(ctx, UpdateFamilyLinkRequest{
			Id:     link.Id,
			Scopes: LinkScopes{People: true, Milestones: true, Photos: true, Growth: true},
		}); procErr == nil {
			t.Error("the receiving family rewrote what the link shares")
		}
	})

	// Nor can it help itself to a person it was not given.
	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		ctx := &vbeam.Context{Tx: tx, Token: tokenD}
		if _, procErr := SharePersonWithFamily(ctx, SharePersonRequest{
			PersonId: fx.alice.Id, FamilyId: famD.Id,
		}); procErr == nil {
			t.Error("the receiving family shared someone else's person into itself")
		}
	})

	// The owner shares alice, and only then does D reach her.
	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		ctx := &vbeam.Context{Tx: tx, Token: tokenA}
		resp, procErr := SharePersonWithFamily(ctx, SharePersonRequest{
			PersonId: fx.alice.Id, FamilyId: famD.Id, Role: int(Child), Kind: "niece",
		})
		if procErr != nil || !resp.Success {
			t.Fatalf("SharePersonWithFamily: %v %q", procErr, resp.Error)
		}
	})

	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		if !CanAccessPerson(tx, userD, fx.alice, ScopeMilestones, AccessView) {
			t.Error("the shared person is not reachable after sharing")
		}
		if CanAccessPerson(tx, userD, fx.alice, ScopePhotos, AccessView) {
			t.Error("photos were readable through a milestones-only link")
		}
		if CanAccessPerson(tx, userD, fx.bob, ScopePeople, AccessView) {
			t.Error("an unshared sibling came along with the shared person")
		}

		ctx := &vbeam.Context{Tx: tx, Token: tokenD}
		links, procErr := ListFamilyLinks(ctx, ListFamilyLinksRequest{})
		if procErr != nil || len(links.Links) != 1 {
			t.Fatalf("ListFamilyLinks: %v %v", procErr, links.Links)
		}
		if links.Links[0].Outgoing {
			t.Error("the receiving family sees its link as outgoing")
		}
		if links.Links[0].SharedCount != 1 {
			t.Errorf("expected one shared person, got %d", links.Links[0].SharedCount)
		}
	})

	// Revoking hands everything back.
	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		ctx := &vbeam.Context{Tx: tx, Token: tokenD}
		resp, procErr := RevokeFamilyLink(ctx, FamilyLinkIdRequest{Id: link.Id})
		if procErr != nil || !resp.Success {
			t.Fatalf("RevokeFamilyLink: %v %q", procErr, resp.Error)
		}
	})

	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		if CanAccessPerson(tx, userD, fx.alice, ScopeMilestones, AccessView) {
			t.Error("access survived revocation")
		}
		if _, found := FindPersonFamily(tx, fx.alice.Id, famD.Id); found {
			t.Error("the shared roster row survived revocation")
		}
	})
}

func testAuthToken(t *testing.T, user User) string {
	t.Helper()
	token, err := generateJwtTokenString(user)
	if err != nil {
		t.Fatalf("generateJwtTokenString: %v", err)
	}
	return token
}

// A shared person is on the host family's roster but is not theirs to export,
// to match on import, or to auto-tag: those all ask about ownership.
func TestOwnPeopleExcludesSharedIn(t *testing.T) {
	fx, cleanup := setupFamilyLinkFixture(t)
	defer cleanup()

	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		roster := GetFamilyPeople(tx, fx.famB)
		if len(roster) != 2 {
			t.Fatalf("expected robin and the shared alice on B's roster, got %d", len(roster))
		}

		owned := GetFamilyOwnPeople(tx, fx.famB)
		if len(owned) != 1 || owned[0].Id != fx.robin.Id {
			t.Errorf("B owns only robin, got %v", owned)
		}

		ownedA := GetFamilyOwnPeople(tx, fx.famA)
		if len(ownedA) != 2 {
			t.Errorf("A still owns both its people, got %d", len(ownedA))
		}

		// An export of B must not carry A's child out with it.
		data, err := buildExportData(tx, fx.famB)
		if err != nil {
			t.Fatalf("buildExportData: %v", err)
		}
		for _, person := range data.People {
			if person.Id == fx.alice.Id {
				t.Error("a shared person was written into the host family's export")
			}
		}

		// Import matching against B must not resolve to A's child either.
		match := findExistingPerson(tx, ImportPerson{
			Name:     fx.alice.Name,
			Birthday: fx.alice.Birthday,
			Gender:   int(fx.alice.Gender),
		}, fx.famB)
		if match != nil {
			t.Error("an import into B matched a person A owns")
		}
	})
}
