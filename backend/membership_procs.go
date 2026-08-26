package backend

import (
	"errors"
	"time"

	"go.hasen.dev/vbeam"
	"go.hasen.dev/vbolt"
)

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

type FamilyMemberView struct {
	UserId   int         `json:"userId"`
	Name     string      `json:"name"`
	Email    string      `json:"email"`
	Role     AccessLevel `json:"role"`
	JoinedAt time.Time   `json:"joinedAt"`
	IsOwner  bool        `json:"isOwner"`
	IsSelf   bool        `json:"isSelf"`
}

type ListFamilyMembersRequest struct {
	FamilyId int `json:"familyId,omitempty"`
}

type ListFamilyMembersResponse struct {
	FamilyId      int                `json:"familyId"`
	Members       []FamilyMemberView `json:"members"`
	CallerIsOwner bool               `json:"callerIsOwner"`
}

type FamilyIdRequest struct {
	FamilyId int `json:"familyId,omitempty"`
}

type LeaveFamilyResponse struct {
	Success bool         `json:"success"`
	Error   string       `json:"error,omitempty"`
	Auth    AuthResponse `json:"auth,omitempty"`
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

func familyOwnerId(tx *vbolt.Tx, familyId int) int {
	return GetFamily(tx, familyId).CreatedBy
}

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

	if len(familyMemberViews(ctx.Tx, familyId, user.Id)) <= 1 {
		resp.Error = ErrLastMemberLeave.Error()
		return
	}

	vbeam.UseWriteTx(ctx)
	detachUserFromFamilyTx(ctx.Tx, user, familyId)
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
