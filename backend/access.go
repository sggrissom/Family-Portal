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

// familyGrants answers what a caller acting in actingFamilyId may do inside
// familyId on the strength of family membership alone.
//
// Link-derived access lives in canAccessPersonViaLink instead, which is reached
// only from checks that name the person and the kind of content involved.
// Converting a read site is therefore opt-in, and a site nobody converts stays
// membership-only: forgetting one narrows what a linked family sees rather than
// widening it.
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

// familiesVisibleTo returns every family the user is a *member* of, primary
// family first and the rest in ascending id order so list output is stable.
//
// This is "my families", not "families I can see into". Linked families are
// deliberately excluded: a link shares content, and treating it as membership
// here would put another household's roster, tags and invite codes into every
// list that resolves through this function. Callers that want linked content
// ask sharedInFamilies for the specific scope they mean.
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

// userRoleIn is the user's role in one of their own families, and the ceiling
// on anything that family's links can pass through to them. The primary family
// falls back to AccessAdmin for the same reason CanAccessFamily does: a missing
// membership row must not revoke a user's rights in their own household.
func userRoleIn(tx *vbolt.Tx, user User, familyId int) AccessLevel {
	if membership, found := FindMembership(tx, user.Id, familyId); found {
		return membership.Role
	}
	if user.FamilyId == familyId {
		return AccessAdmin
	}
	return AccessNone
}

// canAccessPersonViaLink is the whole of what a family link adds to a read.
//
// A link does not hand the receiving family the sharing family's whole roster.
// It authorizes *sharing*, and which people are shared is the PersonFamily
// roster: a person is reachable through a link when one of the user's own
// families has that person on its roster, and that family holds `scope` over
// the person's home family through an accepted link. Scope then says which of
// that person's records come with them — milestones and photos by default,
// growth measurements only if the home family says so.
//
// Nothing here consults the home family's own links, so access is one hop:
// sharing a person into B does not put them on the rosters of families B links
// to, and B's links grant nothing over the person's home family.
func canAccessPersonViaLink(tx *vbolt.Tx, user User, person Person, scope LinkScope, need AccessLevel) bool {
	if person.Id == 0 || person.FamilyId == 0 || scope == ScopeFamily {
		return false
	}
	for _, familyId := range familiesVisibleTo(tx, user) {
		if familyId == person.FamilyId {
			continue // membership already answered this
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

// CanAccessPerson reports whether the user may reach this person at all: they
// belong to the person's family, or the person has been shared onto a roster of
// theirs by a link carrying `scope`.
//
// The membership check runs first and is untouched by links, so a link can only
// ever widen access, never narrow it.
func CanAccessPerson(tx *vbolt.Tx, user User, person Person, scope LinkScope, need AccessLevel) bool {
	if person.Id == 0 {
		return false
	}
	if CanAccessFamily(tx, user, person.FamilyId, need) {
		return true
	}
	return canAccessPersonViaLink(tx, user, person, scope, need)
}

// CanAccessRecordOfPerson is the same question for a record hanging off a
// person — a milestone, a measurement — which the person's family owns.
func CanAccessRecordOfPerson(tx *vbolt.Tx, user User, recordFamilyId int, personId int, scope LinkScope, need AccessLevel) bool {
	if CanAccessFamily(tx, user, recordFamilyId, need) {
		return true
	}
	return canAccessPersonViaLink(tx, user, GetPersonById(tx, personId), scope, need)
}

// CanAccessPhoto reports whether the user may see this photo. A photo is owned
// by a family rather than by a person, so the link path asks whether anyone
// tagged in it is a person shared into one of the user's families by a link
// carrying photos. A photo of nobody in particular is therefore never reachable
// through a link — only its own family sees it.
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

// sharedInFamilies returns the home families of people who have been shared
// onto one of the user's rosters, where the link carries `scope`, in ascending
// id order. It is the "families I can view" list that pairs with
// familiesVisibleTo's "my families", and the two are kept apart because only
// the second is somewhere the user may write.
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
