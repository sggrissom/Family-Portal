package backend

import (
	"errors"
	"time"

	"go.hasen.dev/vbeam"
	"go.hasen.dev/vbolt"
)

// Membership self-service: seeing who is in a family, leaving one, removing
// somebody from one, and replacing a leaked invite code.
//
// What happens to shared content when somebody goes
// ---------------------------------------------------
// Content stays with the family. People, growth measurements, milestones,
// photos, tags and chat belong to the household, not to whichever member
// happened to type them in, and a family that lost half its records because one
// parent left would be a worse outcome than one that keeps a departed member's
// entries. Leaving therefore removes exactly one thing: the membership row that
// grants access. The departing user immediately stops being able to read or
// write anything in that family, and nothing they contributed is touched.
//
// Deleting the account is a stronger request and is handled in
// account_deletion.go: family records still stay, but the account's own data
// goes, including its chat messages — and a family the deletion empties is
// destroyed outright, because nobody could ever reach it again.
//
// A user who leaves their last family is given a fresh empty one, the same way
// signup gives a new account a household to put people in. Being family-less is
// not a state the rest of the application handles, and stranding somebody there
// would break their account rather than free it.
func RegisterMembershipMethods(app *vbeam.Application) {
	vbeam.RegisterProc(app, ListFamilyMembers)
	vbeam.RegisterProc(app, LeaveFamily)
	vbeam.RegisterProc(app, RemoveFamilyMember)
	vbeam.RegisterProc(app, RotateInviteCode)
}

var (
	ErrNotAMember       = errors.New("You are not a member of this family")
	ErrLastMemberLeave  = errors.New("You are the only member of this family. Delete your account to remove it, or invite someone else first.")
	ErrOwnerOnlyRemoval = errors.New("Only the family owner can remove members")
	ErrCannotRemoveSelf = errors.New("Use Leave Family to remove yourself")
	ErrMemberNotFound   = errors.New("That person is not a member of this family")
)

// FamilyMemberView is one member as the rest of the family sees them. Email is
// included because members of a household already know each other's addresses
// and the owner needs it to tell two people with the same name apart.
type FamilyMemberView struct {
	UserId   int         `json:"userId"`
	Name     string      `json:"name"`
	Email    string      `json:"email"`
	Role     AccessLevel `json:"role"`
	JoinedAt time.Time   `json:"joinedAt"`
	// IsOwner marks the member who may remove others.
	IsOwner bool `json:"isOwner"`
	// IsSelf marks the caller, so the UI can offer Leave instead of Remove.
	IsSelf bool `json:"isSelf"`
}

type ListFamilyMembersRequest struct {
	// FamilyId names which family to list; zero means the caller's primary one.
	FamilyId int `json:"familyId,omitempty"`
}

type ListFamilyMembersResponse struct {
	FamilyId int                `json:"familyId"`
	Members  []FamilyMemberView `json:"members"`
	// CallerIsOwner saves the UI from re-deriving who may remove whom.
	CallerIsOwner bool `json:"callerIsOwner"`
}

type FamilyIdRequest struct {
	FamilyId int `json:"familyId,omitempty"`
}

type LeaveFamilyResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
	// Auth is the caller's refreshed context: leaving changes which families
	// they belong to, and may have moved their primary family.
	Auth AuthResponse `json:"auth,omitempty"`
}

type RemoveFamilyMemberRequest struct {
	FamilyId int `json:"familyId,omitempty"`
	UserId   int `json:"userId"`
}

type RemoveFamilyMemberResponse struct {
	Success bool               `json:"success"`
	Error   string             `json:"error,omitempty"`
	Members []FamilyMemberView `json:"members"`
}

type RotateInviteCodeResponse struct {
	Success    bool   `json:"success"`
	Error      string `json:"error,omitempty"`
	FamilyId   int    `json:"familyId,omitempty"`
	InviteCode string `json:"inviteCode,omitempty"`
}

// familyOwnerId is the user who may remove members. Families have one role
// today, so the creator is the only distinction available — and the creator is
// who owns the household in every case the application can produce.
func familyOwnerId(tx *vbolt.Tx, familyId int) int {
	return GetFamily(tx, familyId).CreatedBy
}

// familyMemberViews lists a family's members, owner first and the rest in join
// order so the list does not reshuffle between calls.
func familyMemberViews(tx *vbolt.Tx, familyId int, callerId int) []FamilyMemberView {
	ownerId := familyOwnerId(tx, familyId)

	members := make([]FamilyMemberView, 0)
	for _, membership := range GetFamilyMemberships(tx, familyId) {
		user := GetUser(tx, membership.UserId)
		if user.Id == 0 {
			continue
		}
		members = append(members, FamilyMemberView{
			UserId:   user.Id,
			Name:     user.Name,
			Email:    user.Email,
			Role:     membership.Role,
			JoinedAt: membership.JoinedAt,
			IsOwner:  user.Id == ownerId,
			IsSelf:   user.Id == callerId,
		})
	}

	sortFamilyMembers(members)
	return members
}

func sortFamilyMembers(members []FamilyMemberView) {
	// Insertion sort: a family is a handful of people, and this keeps the
	// ordering rule readable next to the thing it orders.
	for i := 1; i < len(members); i++ {
		for j := i; j > 0 && memberBefore(members[j], members[j-1]); j-- {
			members[j], members[j-1] = members[j-1], members[j]
		}
	}
}

func memberBefore(a, b FamilyMemberView) bool {
	if a.IsOwner != b.IsOwner {
		return a.IsOwner
	}
	if !a.JoinedAt.Equal(b.JoinedAt) {
		return a.JoinedAt.Before(b.JoinedAt)
	}
	return a.UserId < b.UserId
}

// detachUserFromFamilyTx removes one membership and repairs whatever pointed at
// it. Shared by leaving and being removed, because the two differ only in who
// asked: the effect on the departing user is identical.
//
// The user's primary family moves to another family they belong to, or to a new
// empty household if they belong to none. See the package comment above for why
// family-less is not an outcome this returns.
func detachUserFromFamilyTx(tx *vbolt.Tx, user User, familyId int) {
	membership, found := FindMembership(tx, user.Id, familyId)
	if found {
		deleteMembershipTx(tx, membership)
	}

	if user.FamilyId != familyId {
		return
	}

	replacement := 0
	for _, remaining := range GetUserMemberships(tx, user.Id) {
		if remaining.FamilyId != familyId && remaining.FamilyId != 0 {
			replacement = remaining.FamilyId
			break
		}
	}
	if replacement == 0 {
		fresh := createFamilyTx(tx, user.Name+"'s Family", user.Id)
		EnsureMembershipTx(tx, user.Id, fresh.Id, AccessAdmin)
		replacement = fresh.Id
	}

	user.FamilyId = replacement
	vbolt.Write(tx, UsersBkt, user.Id, &user)
	vbolt.SetTargetSingleTerm(tx, UsersByFamilyIndex, user.Id, replacement)
}

func ListFamilyMembers(ctx *vbeam.Context, req ListFamilyMembersRequest) (resp ListFamilyMembersResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil || user.Id == 0 {
		err = ErrAuthFailure
		return
	}

	familyId, err := ResolveActingFamily(ctx.Tx, user, req.FamilyId, AccessView)
	if err != nil {
		return
	}

	resp.FamilyId = familyId
	resp.Members = familyMemberViews(ctx.Tx, familyId, user.Id)
	resp.CallerIsOwner = familyOwnerId(ctx.Tx, familyId) == user.Id
	return
}

// LeaveFamily gives up the caller's membership in one family. Content they
// added stays behind; see the package comment.
func LeaveFamily(ctx *vbeam.Context, req FamilyIdRequest) (resp LeaveFamilyResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil || user.Id == 0 {
		err = ErrAuthFailure
		return
	}

	familyId := req.FamilyId
	if familyId == 0 {
		familyId = user.FamilyId
	}
	if _, member := FindMembership(ctx.Tx, user.Id, familyId); !member {
		resp.Error = ErrNotAMember.Error()
		return
	}

	// Refuse to leave a family nobody else is in. The membership row is the only
	// route to the household's people, photos and history, so dropping the last
	// one would strand all of it: unreachable, undeletable, still on disk.
	// Deleting the account is the operation that means "and take the data".
	if len(familyMemberViews(ctx.Tx, familyId, user.Id)) <= 1 {
		resp.Error = ErrLastMemberLeave.Error()
		return
	}

	vbeam.UseWriteTx(ctx)
	detachUserFromFamilyTx(ctx.Tx, user, familyId)
	// Ownership follows the household, not the person: leaving it pointing at a
	// departed member would leave the family with nobody able to remove anyone.
	reassignFamilyOwnerTx(ctx.Tx, familyId, user.Id)

	updated := GetUser(ctx.Tx, user.Id)
	auth := GetAuthResponseFromUser(ctx.Tx, updated)
	vbolt.TxCommit(ctx.Tx)

	LogInfo(LogCategoryAuth, "User left family", map[string]interface{}{
		"userId":   user.Id,
		"familyId": familyId,
	})

	resp.Success = true
	resp.Auth = auth
	return
}

// reassignFamilyOwnerTx hands ownership to the longest-standing remaining
// member when the owner is no longer one. This is not the ownership-transfer
// feature — that is deliberately after 1.0 — it is the minimum needed to keep a
// family from becoming headless the moment its creator leaves.
func reassignFamilyOwnerTx(tx *vbolt.Tx, familyId int, departingUserId int) {
	family := GetFamily(tx, familyId)
	if family.Id == 0 || family.CreatedBy != departingUserId {
		return
	}

	remaining := familyMemberViews(tx, familyId, 0)
	if len(remaining) == 0 {
		return
	}

	family.CreatedBy = remaining[0].UserId
	vbolt.Write(tx, FamiliesBkt, family.Id, &family)
}

// RemoveFamilyMember drops somebody else's membership. Only the owner may do
// this, and only to another member: removing yourself is LeaveFamily, which
// carries the last-member check this one does not need.
func RemoveFamilyMember(ctx *vbeam.Context, req RemoveFamilyMemberRequest) (resp RemoveFamilyMemberResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil || user.Id == 0 {
		err = ErrAuthFailure
		return
	}

	familyId, err := ResolveActingFamily(ctx.Tx, user, req.FamilyId, AccessAdmin)
	if err != nil {
		return
	}

	if familyOwnerId(ctx.Tx, familyId) != user.Id {
		resp.Error = ErrOwnerOnlyRemoval.Error()
		return
	}
	if req.UserId == user.Id {
		resp.Error = ErrCannotRemoveSelf.Error()
		return
	}

	target := GetUser(ctx.Tx, req.UserId)
	if target.Id == 0 {
		resp.Error = ErrMemberNotFound.Error()
		return
	}
	if _, member := FindMembership(ctx.Tx, target.Id, familyId); !member {
		resp.Error = ErrMemberNotFound.Error()
		return
	}

	vbeam.UseWriteTx(ctx)
	detachUserFromFamilyTx(ctx.Tx, target, familyId)
	members := familyMemberViews(ctx.Tx, familyId, user.Id)
	vbolt.TxCommit(ctx.Tx)

	LogInfo(LogCategoryAuth, "Family member removed", map[string]interface{}{
		"familyId":      familyId,
		"removedBy":     user.Id,
		"removedUserId": target.Id,
	})

	resp.Success = true
	resp.Members = members
	return
}

// RotateInviteCode replaces a family's invite code. The code is a bearer secret
// that gets pasted into messaging apps, so being able to retire one without
// support is the difference between a leak being a shrug and being a migration.
//
// Outstanding links that were created with the old code are unaffected: a link
// records the two families, not the code that introduced them.
func RotateInviteCode(ctx *vbeam.Context, req FamilyIdRequest) (resp RotateInviteCodeResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil || user.Id == 0 {
		err = ErrAuthFailure
		return
	}

	familyId, err := ResolveActingFamily(ctx.Tx, user, req.FamilyId, AccessAdmin)
	if err != nil {
		return
	}

	vbeam.UseWriteTx(ctx)
	family := GetFamily(ctx.Tx, familyId)
	if family.Id == 0 {
		resp.Error = "Family not found"
		return
	}

	if family.InviteCode != "" {
		vbolt.Delete(ctx.Tx, InviteCodeBkt, family.InviteCode)
	}
	family.InviteCode = generateUniqueInviteCodeTx(ctx.Tx)
	vbolt.Write(ctx.Tx, FamiliesBkt, family.Id, &family)
	vbolt.Write(ctx.Tx, InviteCodeBkt, family.InviteCode, &family.Id)
	vbolt.TxCommit(ctx.Tx)

	LogInfo(LogCategoryAuth, "Family invite code rotated", map[string]interface{}{
		"familyId":   family.Id,
		"rotatedBy":  user.Id,
		"codeLength": len(family.InviteCode),
	})

	resp.Success = true
	resp.FamilyId = family.Id
	resp.InviteCode = family.InviteCode
	return
}
