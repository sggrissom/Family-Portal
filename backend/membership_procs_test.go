package backend

import (
	"family/cfg"
	"testing"

	"go.hasen.dev/vbeam"
	"go.hasen.dev/vbolt"
	"golang.org/x/crypto/bcrypt"
)

type membershipFixture struct {
	db *vbolt.DB

	owner    User
	member   User
	outsider User
	familyId int
}

func setupMembershipProcFixture(t *testing.T) membershipFixture {
	t.Helper()

	db := vbolt.Open(t.TempDir() + "/membership_procs.db")
	vbolt.InitBuckets(db, &cfg.Info)
	t.Cleanup(func() { _ = db.Close() })
	appDb = db
	jwtKey = []byte("membership-test-secret-key-at-least-32")

	fx := membershipFixture{db: db}
	hash, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.MinCost)

	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		fx.owner = AddUserTx(tx, CreateAccountRequest{Name: "Owner", Email: "owner@example.com"}, hash)
		fx.familyId = fx.owner.FamilyId

		family := GetFamily(tx, fx.familyId)
		fx.member = AddUserTx(tx, CreateAccountRequest{
			Name: "Member", Email: "member@example.com", FamilyCode: family.InviteCode,
		}, hash)
		fx.outsider = AddUserTx(tx, CreateAccountRequest{Name: "Outsider", Email: "outsider@example.com"}, hash)
		vbolt.TxCommit(tx)
	})
	return fx
}

func (fx membershipFixture) as(t *testing.T, user User, fn func(ctx *vbeam.Context)) {
	t.Helper()

	token, err := generateJwtTokenString(user)
	if err != nil {
		t.Fatalf("generateJwtTokenString() error = %v", err)
	}
	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		fn(&vbeam.Context{Tx: tx, Token: token})
	})
}

func (fx membershipFixture) isMember(t *testing.T, userId int, familyId int) bool {
	t.Helper()

	var found bool
	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		_, found = FindMembership(tx, userId, familyId)
	})
	return found
}

func TestListFamilyMembersNamesTheOwnerAndTheCaller(t *testing.T) {
	fx := setupMembershipProcFixture(t)

	var resp ListFamilyMembersResponse
	var err error
	fx.as(t, fx.member, func(ctx *vbeam.Context) {
		resp, err = ListFamilyMembers(ctx, ListFamilyMembersRequest{FamilyId: fx.familyId})
	})
	if err != nil {
		t.Fatalf("ListFamilyMembers() error = %v", err)
	}
	if len(resp.Members) != 2 {
		t.Fatalf("listed %d members, want 2", len(resp.Members))
	}
	if resp.Members[0].UserId != fx.owner.Id || !resp.Members[0].IsOwner {
		t.Errorf("first member = %+v, want the owner", resp.Members[0])
	}
	if resp.Members[1].UserId != fx.member.Id || !resp.Members[1].IsSelf {
		t.Errorf("second member = %+v, want the caller", resp.Members[1])
	}
	if resp.CallerIsOwner {
		t.Error("callerIsOwner = true for a non-owner")
	}
}

func TestListFamilyMembersRefusesAnOutsider(t *testing.T) {
	fx := setupMembershipProcFixture(t)

	var err error
	fx.as(t, fx.outsider, func(ctx *vbeam.Context) {
		_, err = ListFamilyMembers(ctx, ListFamilyMembersRequest{FamilyId: fx.familyId})
	})
	if err == nil {
		t.Fatal("an outsider listed another family's members")
	}
}

func TestLeaveFamilyDropsMembershipAndLeavesContentBehind(t *testing.T) {
	fx := setupMembershipProcFixture(t)

	var person Person
	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		var err error
		person, err = AddPersonTx(tx, AddPersonRequest{
			Name: "Kid", PersonType: 1, Gender: 0, Birthdate: "2020-06-15",
		}, fx.familyId)
		if err != nil {
			t.Fatalf("AddPersonTx() error = %v", err)
		}
		vbolt.TxCommit(tx)
	})

	var resp LeaveFamilyResponse
	fx.as(t, fx.member, func(ctx *vbeam.Context) {
		resp, _ = LeaveFamily(ctx, FamilyIdRequest{FamilyId: fx.familyId})
	})
	if !resp.Success {
		t.Fatalf("LeaveFamily() success = false, error = %q", resp.Error)
	}

	if fx.isMember(t, fx.member.Id, fx.familyId) {
		t.Error("membership survived leaving")
	}

	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		if GetPersonById(tx, person.Id).Id == 0 {
			t.Error("family content was deleted when a member left")
		}
		updated := GetUser(tx, fx.member.Id)
		if CanAccessFamily(tx, updated, fx.familyId, AccessView) {
			t.Error("departed member can still read the family")
		}
		if updated.FamilyId == 0 || updated.FamilyId == fx.familyId {
			t.Errorf("primary family = %d, want a new one", updated.FamilyId)
		}
		if _, member := FindMembership(tx, updated.Id, updated.FamilyId); !member {
			t.Error("replacement family has no membership row")
		}
	})
}

func TestLeaveFamilyKeepsAnotherFamilyAsPrimary(t *testing.T) {
	fx := setupMembershipProcFixture(t)

	var joined JoinFamilyResponse
	fx.as(t, fx.outsider, func(ctx *vbeam.Context) {
		var inviteCode string
		inviteCode = GetFamily(ctx.Tx, fx.familyId).InviteCode
		joined, _ = JoinFamily(ctx, JoinFamilyRequest{InviteCode: inviteCode})
	})
	if !joined.Success {
		t.Fatalf("JoinFamily() success = false, error = %q", joined.Error)
	}

	own := fx.outsider.FamilyId
	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		EnsureMembershipTx(tx, fx.owner.Id, own, AccessAdmin)
		vbolt.TxCommit(tx)
	})

	var resp LeaveFamilyResponse
	fx.as(t, fx.outsider, func(ctx *vbeam.Context) {
		resp, _ = LeaveFamily(ctx, FamilyIdRequest{FamilyId: own})
	})
	if !resp.Success {
		t.Fatalf("LeaveFamily() success = false, error = %q", resp.Error)
	}

	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		if got := GetUser(tx, fx.outsider.Id).FamilyId; got != fx.familyId {
			t.Errorf("primary family = %d, want the remaining family %d", got, fx.familyId)
		}
	})
}

func TestLeaveFamilyRefusesTheLastMember(t *testing.T) {
	fx := setupMembershipProcFixture(t)

	var resp LeaveFamilyResponse
	fx.as(t, fx.outsider, func(ctx *vbeam.Context) {
		resp, _ = LeaveFamily(ctx, FamilyIdRequest{FamilyId: fx.outsider.FamilyId})
	})

	if resp.Success {
		t.Fatal("leaving stranded a family's content with no members")
	}
	if resp.Error != ErrLastMemberLeave.Error() {
		t.Errorf("error = %q, want %q", resp.Error, ErrLastMemberLeave.Error())
	}
	if !fx.isMember(t, fx.outsider.Id, fx.outsider.FamilyId) {
		t.Error("membership was dropped by a refused leave")
	}
}

func TestLeaveFamilyRefusesANonMember(t *testing.T) {
	fx := setupMembershipProcFixture(t)

	var resp LeaveFamilyResponse
	fx.as(t, fx.outsider, func(ctx *vbeam.Context) {
		resp, _ = LeaveFamily(ctx, FamilyIdRequest{FamilyId: fx.familyId})
	})
	if resp.Success || resp.Error != ErrNotAMember.Error() {
		t.Errorf("response = %+v, want %q", resp, ErrNotAMember.Error())
	}
}

func TestLeaveFamilyHandsOwnershipToARemainingMember(t *testing.T) {
	fx := setupMembershipProcFixture(t)

	var resp LeaveFamilyResponse
	fx.as(t, fx.owner, func(ctx *vbeam.Context) {
		resp, _ = LeaveFamily(ctx, FamilyIdRequest{FamilyId: fx.familyId})
	})
	if !resp.Success {
		t.Fatalf("LeaveFamily() success = false, error = %q", resp.Error)
	}

	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		if got := familyOwnerId(tx, fx.familyId); got != fx.member.Id {
			t.Errorf("owner = %d, want the remaining member %d", got, fx.member.Id)
		}
	})
}

func TestRemoveFamilyMember(t *testing.T) {
	t.Run("the owner may remove another member", func(t *testing.T) {
		fx := setupMembershipProcFixture(t)

		var resp RemoveFamilyMemberResponse
		fx.as(t, fx.owner, func(ctx *vbeam.Context) {
			resp, _ = RemoveFamilyMember(ctx, RemoveFamilyMemberRequest{
				FamilyId: fx.familyId, UserId: fx.member.Id,
			})
		})
		if !resp.Success {
			t.Fatalf("RemoveFamilyMember() success = false, error = %q", resp.Error)
		}
		if len(resp.Members) != 1 || resp.Members[0].UserId != fx.owner.Id {
			t.Errorf("remaining members = %+v, want only the owner", resp.Members)
		}
		if fx.isMember(t, fx.member.Id, fx.familyId) {
			t.Error("membership survived removal")
		}
		vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
			removed := GetUser(tx, fx.member.Id)
			if CanAccessFamily(tx, removed, fx.familyId, AccessView) {
				t.Error("removed member can still read the family")
			}
			if removed.FamilyId == 0 || removed.FamilyId == fx.familyId {
				t.Errorf("removed member primary family = %d, want a new one", removed.FamilyId)
			}
		})
	})

	t.Run("a non-owner member may not remove anyone", func(t *testing.T) {
		fx := setupMembershipProcFixture(t)

		var resp RemoveFamilyMemberResponse
		fx.as(t, fx.member, func(ctx *vbeam.Context) {
			resp, _ = RemoveFamilyMember(ctx, RemoveFamilyMemberRequest{
				FamilyId: fx.familyId, UserId: fx.owner.Id,
			})
		})
		if resp.Success || resp.Error != ErrOwnerOnlyRemoval.Error() {
			t.Errorf("response = %+v, want %q", resp, ErrOwnerOnlyRemoval.Error())
		}
		if !fx.isMember(t, fx.owner.Id, fx.familyId) {
			t.Error("the owner was removed by a non-owner")
		}
	})

	t.Run("an outsider may not remove anyone", func(t *testing.T) {
		fx := setupMembershipProcFixture(t)

		var err error
		fx.as(t, fx.outsider, func(ctx *vbeam.Context) {
			_, err = RemoveFamilyMember(ctx, RemoveFamilyMemberRequest{
				FamilyId: fx.familyId, UserId: fx.member.Id,
			})
		})
		if err == nil {
			t.Fatal("an outsider removed a member of another family")
		}
		if !fx.isMember(t, fx.member.Id, fx.familyId) {
			t.Error("membership was dropped by an outsider's request")
		}
	})

	t.Run("the owner may not remove themselves", func(t *testing.T) {
		fx := setupMembershipProcFixture(t)

		var resp RemoveFamilyMemberResponse
		fx.as(t, fx.owner, func(ctx *vbeam.Context) {
			resp, _ = RemoveFamilyMember(ctx, RemoveFamilyMemberRequest{
				FamilyId: fx.familyId, UserId: fx.owner.Id,
			})
		})
		if resp.Success || resp.Error != ErrCannotRemoveSelf.Error() {
			t.Errorf("response = %+v, want %q", resp, ErrCannotRemoveSelf.Error())
		}
	})

	t.Run("a non-member target is refused", func(t *testing.T) {
		fx := setupMembershipProcFixture(t)

		var resp RemoveFamilyMemberResponse
		fx.as(t, fx.owner, func(ctx *vbeam.Context) {
			resp, _ = RemoveFamilyMember(ctx, RemoveFamilyMemberRequest{
				FamilyId: fx.familyId, UserId: fx.outsider.Id,
			})
		})
		if resp.Success || resp.Error != ErrMemberNotFound.Error() {
			t.Errorf("response = %+v, want %q", resp, ErrMemberNotFound.Error())
		}
	})
}

func TestRotateInviteCodeRetiresTheOldCode(t *testing.T) {
	fx := setupMembershipProcFixture(t)

	var oldCode string
	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		oldCode = GetFamily(tx, fx.familyId).InviteCode
	})

	var resp RotateInviteCodeResponse
	fx.as(t, fx.owner, func(ctx *vbeam.Context) {
		resp, _ = RotateInviteCode(ctx, FamilyIdRequest{FamilyId: fx.familyId})
	})
	if !resp.Success {
		t.Fatalf("RotateInviteCode() success = false, error = %q", resp.Error)
	}
	if resp.InviteCode == "" || resp.InviteCode == oldCode {
		t.Fatalf("invite code = %q, want a new one (was %q)", resp.InviteCode, oldCode)
	}

	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		if GetFamilyByInviteCode(tx, oldCode).Id != 0 {
			t.Error("the old invite code still resolves to the family")
		}
		if GetFamilyByInviteCode(tx, resp.InviteCode).Id != fx.familyId {
			t.Error("the new invite code does not resolve to the family")
		}
		if got := GetFamily(tx, fx.familyId).InviteCode; got != resp.InviteCode {
			t.Errorf("stored code = %q, want %q", got, resp.InviteCode)
		}
	})
}

func TestRotateInviteCodeRefusesAnOutsider(t *testing.T) {
	fx := setupMembershipProcFixture(t)

	var before string
	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		before = GetFamily(tx, fx.familyId).InviteCode
	})

	var err error
	fx.as(t, fx.outsider, func(ctx *vbeam.Context) {
		_, err = RotateInviteCode(ctx, FamilyIdRequest{FamilyId: fx.familyId})
	})
	if err == nil {
		t.Fatal("an outsider rotated another family's invite code")
	}

	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		if got := GetFamily(tx, fx.familyId).InviteCode; got != before {
			t.Errorf("invite code changed to %q despite a refused request", got)
		}
	})
}

func TestGenerateUniqueInviteCodeAvoidsAnExistingCode(t *testing.T) {
	fx := setupMembershipProcFixture(t)

	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		code := generateUniqueInviteCodeTx(tx)
		var existing int
		vbolt.Read(tx, InviteCodeBkt, code, &existing)
		if existing != 0 {
			t.Errorf("generated code %q is already in use by family %d", code, existing)
		}
	})
}
