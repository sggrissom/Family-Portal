package backend

import (
	"time"

	"go.hasen.dev/vbeam"
	"go.hasen.dev/vbolt"
)

type GetInviteClaimsRequest struct {
	Code string `json:"code"`
}

type InviteClaim struct {
	PersonId int    `json:"personId"`
	Name     string `json:"name"`
}

type GetInviteClaimsResponse struct {
	FamilyName string        `json:"familyName"`
	Claims     []InviteClaim `json:"claims"`
}

// GetInviteClaims lists the adults an invitee might already be in the family
// as, so signing up can link to them instead of adding a duplicate.
func GetInviteClaims(ctx *vbeam.Context, req GetInviteClaimsRequest) (resp GetInviteClaimsResponse, err error) {
	resp.Claims = []InviteClaim{}
	family := GetFamilyByInviteCode(ctx.Tx, req.Code)
	if family.Id == 0 {
		return
	}
	resp.FamilyName = family.Name
	for _, person := range claimablePeople(ctx.Tx, family.Id, time.Now()) {
		resp.Claims = append(resp.Claims, InviteClaim{PersonId: person.Id, Name: person.Name})
	}
	return
}

func claimablePeople(tx *vbolt.Tx, familyId int, now time.Time) (people []Person) {
	claimed := map[int]bool{}
	for _, membership := range GetFamilyMemberships(tx, familyId) {
		if user := GetUser(tx, membership.UserId); user.PersonId != 0 {
			claimed[user.PersonId] = true
		}
	}
	for _, person := range GetFamilyOwnPeople(tx, familyId) {
		if person.IsPregnancy || claimed[person.Id] || monthsBetween(person.Birthday, now) < 18*12 {
			continue
		}
		people = append(people, person)
	}
	return
}

func claimableBy(tx *vbolt.Tx, familyId, personId int) bool {
	for _, person := range claimablePeople(tx, familyId, time.Now()) {
		if person.Id == personId {
			return true
		}
	}
	return false
}
