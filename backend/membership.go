package backend

import (
	"family/cfg"
	"time"

	"go.hasen.dev/vbolt"
	"go.hasen.dev/vpack"
)

// FamilyMembership records that a user belongs to a family.
type FamilyMembership struct {
	Id       int         `json:"id"`
	UserId   int         `json:"userId"`
	FamilyId int         `json:"familyId"`
	Role     AccessLevel `json:"role"`
	JoinedAt time.Time   `json:"joinedAt"`
}

func PackFamilyMembership(self *FamilyMembership, buf *vpack.Buffer) {
	vpack.Version(1, buf)
	vpack.Int(&self.Id, buf)
	vpack.Int(&self.UserId, buf)
	vpack.Int(&self.FamilyId, buf)
	vpack.IntEnum(&self.Role, buf)
	vpack.Time(&self.JoinedAt, buf)
}

var FamilyMembershipBkt = vbolt.Bucket(&cfg.Info, "family_membership", vpack.FInt, PackFamilyMembership)

// MembershipByUserIndex: term = user_id, target = membership_id
var MembershipByUserIndex = vbolt.Index(&cfg.Info, "membership_by_user", vpack.FInt, vpack.FInt)

// MembershipByFamilyIndex: term = family_id, target = membership_id
var MembershipByFamilyIndex = vbolt.Index(&cfg.Info, "membership_by_family", vpack.FInt, vpack.FInt)

// GetUserMemberships returns every family the user is a member of.
func GetUserMemberships(tx *vbolt.Tx, userId int) (memberships []FamilyMembership) {
	var ids []int
	vbolt.ReadTermTargets(tx, MembershipByUserIndex, userId, &ids, vbolt.Window{})
	vbolt.ReadSlice(tx, FamilyMembershipBkt, ids, &memberships)
	return
}

// GetFamilyMemberships returns every user who is a member of the family.
func GetFamilyMemberships(tx *vbolt.Tx, familyId int) (memberships []FamilyMembership) {
	var ids []int
	vbolt.ReadTermTargets(tx, MembershipByFamilyIndex, familyId, &ids, vbolt.Window{})
	vbolt.ReadSlice(tx, FamilyMembershipBkt, ids, &memberships)
	return
}

// FindMembership looks up the user's membership in one specific family.
func FindMembership(tx *vbolt.Tx, userId int, familyId int) (FamilyMembership, bool) {
	for _, membership := range GetUserMemberships(tx, userId) {
		if membership.FamilyId == familyId {
			return membership, true
		}
	}
	return FamilyMembership{}, false
}

// addMembershipTx writes a new membership row. Callers are responsible for
// checking that one does not already exist.
func addMembershipTx(tx *vbolt.Tx, userId int, familyId int, role AccessLevel, joinedAt time.Time) FamilyMembership {
	membership := FamilyMembership{
		Id:       vbolt.NextIntId(tx, FamilyMembershipBkt),
		UserId:   userId,
		FamilyId: familyId,
		Role:     role,
		JoinedAt: joinedAt,
	}
	vbolt.Write(tx, FamilyMembershipBkt, membership.Id, &membership)
	vbolt.SetTargetSingleTerm(tx, MembershipByUserIndex, membership.Id, userId)
	vbolt.SetTargetSingleTerm(tx, MembershipByFamilyIndex, membership.Id, familyId)
	return membership
}

func deleteMembershipTx(tx *vbolt.Tx, membership FamilyMembership) {
	vbolt.Delete(tx, FamilyMembershipBkt, membership.Id)
	vbolt.SetTargetSingleTerm(tx, MembershipByUserIndex, membership.Id, -1)
	vbolt.SetTargetSingleTerm(tx, MembershipByFamilyIndex, membership.Id, -1)
}

// EnsureMembershipTx records that userId belongs to familyId, doing nothing if
// the row already exists. An existing row's role is left alone — role changes
// are a separate operation, not a side effect of re-recording membership.
func EnsureMembershipTx(tx *vbolt.Tx, userId int, familyId int, role AccessLevel) FamilyMembership {
	return ensureMembershipTx(tx, userId, familyId, role, time.Now())
}

func ensureMembershipTx(tx *vbolt.Tx, userId int, familyId int, role AccessLevel, joinedAt time.Time) FamilyMembership {
	if userId == 0 || familyId == 0 {
		return FamilyMembership{}
	}
	if existing, found := FindMembership(tx, userId, familyId); found {
		return existing
	}
	return addMembershipTx(tx, userId, familyId, role, joinedAt)
}

// BackfillFamilyMemberships writes one membership row per user, mirroring the
// user's primary family. Safe to re-run: users that already have the row are
// skipped, so re-running creates nothing and changes nothing.
func BackfillFamilyMemberships(tx *vbolt.Tx) (created int) {
	vbolt.IterateAll(tx, UsersBkt, func(userId int, user User) bool {
		if user.FamilyId == 0 {
			return true
		}
		if _, found := FindMembership(tx, user.Id, user.FamilyId); found {
			return true
		}
		// JoinedAt is unknowable for existing rows; account creation is the
		// closest true lower bound.
		addMembershipTx(tx, user.Id, user.FamilyId, AccessAdmin, user.Creation)
		created++
		return true
	})
	return
}
