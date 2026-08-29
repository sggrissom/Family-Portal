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

func userRoleIn(tx *vbolt.Tx, user User, familyId int) AccessLevel {
	if membership, found := FindMembership(tx, user.Id, familyId); found {
		return membership.Role
	}
	if user.FamilyId == familyId {
		return AccessAdmin
	}
	return AccessNone
}

func canAccessPersonViaLink(tx *vbolt.Tx, user User, person Person, scope LinkScope, need AccessLevel) bool {
	if person.Id == 0 || person.FamilyId == 0 || scope == ScopeFamily {
		return false
	}
	for _, familyId := range familiesVisibleTo(tx, user) {
		if familyId == person.FamilyId {
			continue
		}
		if _, onRoster := FindPersonFamily(tx, person.Id, familyId); !onRoster {
			continue
		}
		granted := min(userRoleIn(tx, user, familyId), linkGrants(tx, familyId, person.FamilyId, scope))
		if granted >= need {
			return true
		}
	}
	return false
}

func CanAccessPerson(tx *vbolt.Tx, user User, person Person, scope LinkScope, need AccessLevel) bool {
	if person.Id == 0 {
		return false
	}
	if CanAccessFamily(tx, user, person.FamilyId, need) {
		return true
	}
	return canAccessPersonViaLink(tx, user, person, scope, need)
}

func CanAccessRecordOfPerson(tx *vbolt.Tx, user User, recordFamilyId int, personId int, scope LinkScope, need AccessLevel) bool {
	if CanAccessFamily(tx, user, recordFamilyId, need) {
		return true
	}
	return canAccessPersonViaLink(tx, user, GetPersonById(tx, personId), scope, need)
}

func CanAccessPhoto(tx *vbolt.Tx, user User, image Image, need AccessLevel) bool {
	if image.Id == 0 {
		return false
	}
	if CanAccessFamily(tx, user, image.FamilyId, need) {
		return true
	}
	for _, person := range GetPhotoPeople(tx, image.Id) {
		if canAccessPersonViaLink(tx, user, person, ScopePhotos, need) {
			return true
		}
	}
	return false
}

func sharedInFamilies(tx *vbolt.Tx, user User, scope LinkScope) []int {
	if scope == ScopeFamily {
		return nil
	}
	member := make(map[int]bool)
	own := familiesVisibleTo(tx, user)
	for _, familyId := range own {
		member[familyId] = true
	}

	seen := make(map[int]bool)
	var shared []int
	for _, familyId := range own {
		for _, row := range GetFamilyRoster(tx, familyId) {
			person := GetPersonById(tx, row.PersonId)
			if person.Id == 0 || member[person.FamilyId] || seen[person.FamilyId] {
				continue
			}
			if !canAccessPersonViaLink(tx, user, person, scope, AccessView) {
				continue
			}
			seen[person.FamilyId] = true
			shared = append(shared, person.FamilyId)
		}
	}
	sort.Ints(shared)
	return shared
}

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

func ActingFamilyFor(tx *vbolt.Tx, user User, recordFamilyId int, need AccessLevel) (int, error) {
	if recordFamilyId == 0 {
		return 0, ErrFamilyAccessDenied
	}
	if err := RequireFamilyAccess(tx, user, recordFamilyId, need); err != nil {
		return 0, err
	}
	return recordFamilyId, nil
}

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
