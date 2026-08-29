package backend

import (
	"family/cfg"
	"os"
	"testing"

	"go.hasen.dev/vbolt"
	"golang.org/x/crypto/bcrypt"
)

func setupMembershipDB(t *testing.T) (*vbolt.DB, func()) {
	t.Helper()

	testDBPath := "test_membership.db"
	db := vbolt.Open(testDBPath)
	vbolt.InitBuckets(db, &cfg.Info)
	return db, func() {
		db.Close()
		os.Remove(testDBPath)
	}
}

func addTestUser(t *testing.T, tx *vbolt.Tx, name, email, familyCode string) User {
	t.Helper()
	hash, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)
	return AddUserTx(tx, CreateAccountRequest{
		Name:       name,
		Email:      email,
		FamilyCode: familyCode,
	}, hash)
}

func assertSingleMembership(t *testing.T, tx *vbolt.Tx, user User) FamilyMembership {
	t.Helper()
	memberships := GetUserMemberships(tx, user.Id)
	if len(memberships) != 1 {
		t.Fatalf("user %d: expected exactly 1 membership, got %d", user.Id, len(memberships))
	}
	if memberships[0].FamilyId != user.FamilyId {
		t.Fatalf("user %d: membership family %d does not match primary family %d",
			user.Id, memberships[0].FamilyId, user.FamilyId)
	}
	return memberships[0]
}

func TestAddUserRecordsMembership(t *testing.T) {
	db, cleanup := setupMembershipDB(t)
	defer cleanup()

	var userA, userB User
	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		userA = addTestUser(t, tx, "User A", "a@example.com", "")
		userB = addTestUser(t, tx, "User B", "b@example.com", "")
		vbolt.TxCommit(tx)
	})

	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		membershipA := assertSingleMembership(t, tx, userA)
		if membershipA.Role != AccessAdmin {
			t.Errorf("expected AccessAdmin on own family, got %d", membershipA.Role)
		}
		if membershipA.JoinedAt.IsZero() {
			t.Error("JoinedAt should be set")
		}
		assertSingleMembership(t, tx, userB)

		byFamily := GetFamilyMemberships(tx, userA.FamilyId)
		if len(byFamily) != 1 || byFamily[0].UserId != userA.Id {
			t.Errorf("MembershipByFamilyIndex: expected only user %d, got %v", userA.Id, byFamily)
		}

		if _, found := FindMembership(tx, userA.Id, userB.FamilyId); found {
			t.Error("userA must not hold a membership in userB's family")
		}
	})
}

func TestAddUserWithInviteCodeRecordsMembership(t *testing.T) {
	db, cleanup := setupMembershipDB(t)
	defer cleanup()

	var owner, joiner User
	var inviteCode string
	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		owner = addTestUser(t, tx, "Owner", "owner@example.com", "")
		inviteCode = GetFamily(tx, owner.FamilyId).InviteCode
		vbolt.TxCommit(tx)
	})
	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		joiner = addTestUser(t, tx, "Joiner", "joiner@example.com", inviteCode)
		vbolt.TxCommit(tx)
	})

	if joiner.FamilyId != owner.FamilyId {
		t.Fatalf("invite code should place joiner in family %d, got %d", owner.FamilyId, joiner.FamilyId)
	}

	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		assertSingleMembership(t, tx, joiner)
		if got := len(GetFamilyMemberships(tx, owner.FamilyId)); got != 2 {
			t.Errorf("expected 2 memberships in the shared family, got %d", got)
		}
	})
}

func TestEnsureMembershipIsIdempotent(t *testing.T) {
	db, cleanup := setupMembershipDB(t)
	defer cleanup()

	var user User
	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		user = addTestUser(t, tx, "User", "user@example.com", "")
		vbolt.TxCommit(tx)
	})

	var first FamilyMembership
	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		first = assertSingleMembership(t, tx, user)
	})

	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		again := EnsureMembershipTx(tx, user.Id, user.FamilyId, AccessView)
		if again.Id != first.Id {
			t.Errorf("expected the existing membership %d, got %d", first.Id, again.Id)
		}
		vbolt.TxCommit(tx)
	})

	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		membership := assertSingleMembership(t, tx, user)
		if membership.Role != AccessAdmin {
			t.Errorf("role was downgraded to %d", membership.Role)
		}
	})
}

func TestJoiningAddsMembershipWithoutDroppingPrior(t *testing.T) {
	db, cleanup := setupMembershipDB(t)
	defer cleanup()

	var user User
	var otherFamilyId int
	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		user = addTestUser(t, tx, "Joiner", "joiner@example.com", "")
		other := addTestUser(t, tx, "Other", "other@example.com", "")
		otherFamilyId = other.FamilyId
		vbolt.TxCommit(tx)
	})

	originalFamilyId := user.FamilyId

	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		EnsureMembershipTx(tx, user.Id, otherFamilyId, AccessAdmin)
		vbolt.TxCommit(tx)
	})

	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		if got := len(GetUserMemberships(tx, user.Id)); got != 2 {
			t.Fatalf("expected 2 memberships after joining, got %d", got)
		}
		if _, found := FindMembership(tx, user.Id, originalFamilyId); !found {
			t.Error("joining another family must not drop the original membership")
		}
		if _, found := FindMembership(tx, user.Id, otherFamilyId); !found {
			t.Error("the joined family's membership was not recorded")
		}
		if user.FamilyId != originalFamilyId {
			t.Errorf("primary family changed to %d, expected %d", user.FamilyId, originalFamilyId)
		}

		for _, familyId := range []int{originalFamilyId, otherFamilyId} {
			found := false
			for _, membership := range GetFamilyMemberships(tx, familyId) {
				if membership.UserId == user.Id {
					found = true
				}
			}
			if !found {
				t.Errorf("family %d cannot reach user %d via MembershipByFamilyIndex", familyId, user.Id)
			}
		}
	})
}

func TestBackfillFamilyMembershipsIsIdempotent(t *testing.T) {
	db, cleanup := setupMembershipDB(t)
	defer cleanup()

	var users []User
	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		users = append(users,
			addTestUser(t, tx, "User A", "a@example.com", ""),
			addTestUser(t, tx, "User B", "b@example.com", ""),
		)
		for _, membership := range append(
			GetUserMemberships(tx, users[0].Id),
			GetUserMemberships(tx, users[1].Id)...,
		) {
			deleteMembershipTx(tx, membership)
		}
		vbolt.TxCommit(tx)
	})

	familyless := User{Id: 9001, Name: "Orphan", Email: "orphan@example.com"}
	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		vbolt.Write(tx, UsersBkt, familyless.Id, &familyless)
		vbolt.TxCommit(tx)
	})

	var firstRun, secondRun int
	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		firstRun = BackfillFamilyMemberships(tx)
		vbolt.TxCommit(tx)
	})
	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		secondRun = BackfillFamilyMemberships(tx)
		vbolt.TxCommit(tx)
	})

	if firstRun != len(users) {
		t.Errorf("first run should have created %d memberships, created %d", len(users), firstRun)
	}
	if secondRun != 0 {
		t.Errorf("re-running the backfill created %d memberships, expected 0", secondRun)
	}

	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		for _, user := range users {
			membership := assertSingleMembership(t, tx, user)
			if membership.Role != AccessAdmin {
				t.Errorf("user %d: expected AccessAdmin, got %d", user.Id, membership.Role)
			}
			if !membership.JoinedAt.Equal(user.Creation) {
				t.Errorf("user %d: JoinedAt %v should fall back to account creation %v",
					user.Id, membership.JoinedAt, user.Creation)
			}
		}
		if got := len(GetUserMemberships(tx, familyless.Id)); got != 0 {
			t.Errorf("a user with no family should get no membership, got %d", got)
		}
	})
}

func TestEveryUserHasExactlyOneMatchingMembership(t *testing.T) {
	db, cleanup := setupMembershipDB(t)
	defer cleanup()

	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		owner := addTestUser(t, tx, "Owner", "owner@example.com", "")
		code := GetFamily(tx, owner.FamilyId).InviteCode
		addTestUser(t, tx, "Joiner", "joiner@example.com", code)
		addTestUser(t, tx, "Solo", "solo@example.com", "")
		addTestUser(t, tx, "Bad Code", "bad@example.com", "nosuchcode")
		vbolt.TxCommit(tx)
	})

	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		count := 0
		vbolt.IterateAll(tx, UsersBkt, func(userId int, user User) bool {
			count++
			assertSingleMembership(t, tx, user)
			return true
		})
		if count != 4 {
			t.Errorf("expected 4 users, iterated %d", count)
		}
	})
}
