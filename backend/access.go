package backend

import (
	"errors"

	"go.hasen.dev/vbolt"
)

type AccessLevel int

const (
	AccessNone AccessLevel = iota
	AccessView
	AccessContribute
	AccessAdmin
)

var ErrFamilyAccessDenied = errors.New("Access denied: record belongs to another family")

func familyGrants(tx *vbolt.Tx, actingFamilyId int, familyId int) AccessLevel {
	if actingFamilyId == 0 || familyId == 0 {
		return AccessNone
	}
	if actingFamilyId == familyId {
		return AccessAdmin
	}
	return AccessNone
}

func CanFamilyAccess(tx *vbolt.Tx, actingFamilyId int, familyId int, need AccessLevel) bool {
	return familyGrants(tx, actingFamilyId, familyId) >= need
}

func CanAccessFamily(tx *vbolt.Tx, user User, familyId int, need AccessLevel) bool {
	return CanFamilyAccess(tx, user.FamilyId, familyId, need)
}

func RequireFamilyAccess(tx *vbolt.Tx, user User, familyId int, need AccessLevel) error {
	if !CanAccessFamily(tx, user, familyId, need) {
		return ErrFamilyAccessDenied
	}
	return nil
}

func RequireFamilyAccessFrom(tx *vbolt.Tx, actingFamilyId int, familyId int, need AccessLevel) error {
	if !CanFamilyAccess(tx, actingFamilyId, familyId, need) {
		return ErrFamilyAccessDenied
	}
	return nil
}

func familiesVisibleTo(tx *vbolt.Tx, user User) []int {
	if user.FamilyId == 0 {
		return nil
	}
	return []int{user.FamilyId}
}
