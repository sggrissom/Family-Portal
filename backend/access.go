package backend

import (
	"errors"
	"sort"

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
var ErrNoFamily = errors.New("User is not part of a family")

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
	if familyId == 0 || user.Id == 0 {
		return false
	}
	for _, membership := range GetUserMemberships(tx, user.Id) {
		granted := min(membership.Role, familyGrants(tx, membership.FamilyId, familyId))
		if granted >= need {
			return true
		}
	}
	return familyGrants(tx, user.FamilyId, familyId) >= need
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

// familiesVisibleTo returns every family the user can read, primary family
// first and the rest in ascending id order so list output is stable.
func familiesVisibleTo(tx *vbolt.Tx, user User) []int {
	seen := make(map[int]bool)
	var rest []int
	for _, membership := range GetUserMemberships(tx, user.Id) {
		if membership.FamilyId == 0 || membership.Role < AccessView || seen[membership.FamilyId] {
			continue
		}
		seen[membership.FamilyId] = true
		if membership.FamilyId != user.FamilyId {
			rest = append(rest, membership.FamilyId)
		}
	}
	sort.Ints(rest)

	var families []int
	if user.FamilyId != 0 {
		families = append(families, user.FamilyId)
	}
	return append(families, rest...)
}

// ResolveActingFamily picks the family a mutation acts in: the one the request
// names, or the user's primary family when it names none. The result is always
// validated against membership, so naming the wrong family is a rejected
// request rather than a record written into the wrong place.
func ResolveActingFamily(tx *vbolt.Tx, user User, requestedFamilyId int, need AccessLevel) (int, error) {
	familyId := requestedFamilyId
	if familyId == 0 {
		familyId = user.FamilyId
	}
	if familyId == 0 {
		return 0, ErrNoFamily
	}
	if err := RequireFamilyAccess(tx, user, familyId, need); err != nil {
		return 0, err
	}
	return familyId, nil
}

// ActingFamilyFor resolves the family context for an operation on a record that
// already has a home family: that family, once the caller is confirmed to hold
// `need` on it. Requests targeting an existing record carry no FamilyId of
// their own — the record names its family, and a second copy on the request
// could only ever disagree with it.
func ActingFamilyFor(tx *vbolt.Tx, user User, recordFamilyId int, need AccessLevel) (int, error) {
	if recordFamilyId == 0 {
		return 0, ErrFamilyAccessDenied
	}
	if err := RequireFamilyAccess(tx, user, recordFamilyId, need); err != nil {
		return 0, err
	}
	return recordFamilyId, nil
}

// ActingFamilyForPerson resolves the family context for an operation that hangs
// a record off a person, such as a measurement or a milestone. The person's
// home family owns what is attached to them, which keeps the
// record -> person -> family ownership chain intact.
func ActingFamilyForPerson(tx *vbolt.Tx, user User, personId int, need AccessLevel) (int, error) {
	person := GetPersonById(tx, personId)
	if person.Id == 0 {
		return 0, errors.New("Person not found or not in your family")
	}
	if !CanAccessFamily(tx, user, person.FamilyId, need) {
		return 0, errors.New("Person not found or not in your family")
	}
	return person.FamilyId, nil
}
