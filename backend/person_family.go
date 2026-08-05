package backend

import (
	"errors"
	"family/cfg"
	"time"

	"go.hasen.dev/vbolt"
	"go.hasen.dev/vpack"
)

// PersonFamily places a person on a family's roster.
//
// A person has exactly one home family — Person.FamilyId — which owns them and
// everything hanging off them, and one roster row per family they appear on,
// the home family included. Whether a roster row is immediate or extended is
// derived, not stored: it is immediate when FamilyId == Person.FamilyId. Storing
// it would create a second source of truth that can disagree with the home
// family.
//
// Role is this person's role in *this* family, which is why it lives here rather
// than on Person: the same person is a Parent in their own household and a Child
// in their parents' household.
type PersonFamily struct {
	Id           int        `json:"id"`
	PersonId     int        `json:"personId"`
	FamilyId     int        `json:"familyId"`
	Role         PersonType `json:"role"`
	Relationship string     `json:"relationship,omitempty"` // optional: "daughter", "grandchild"
	AddedAt      time.Time  `json:"addedAt"`
}

func PackPersonFamily(self *PersonFamily, buf *vpack.Buffer) {
	vpack.Version(1, buf)
	vpack.Int(&self.Id, buf)
	vpack.Int(&self.PersonId, buf)
	vpack.Int(&self.FamilyId, buf)
	vpack.IntEnum(&self.Role, buf)
	vpack.String(&self.Relationship, buf)
	vpack.Time(&self.AddedAt, buf)
}

var PersonFamilyBkt = vbolt.Bucket(&cfg.Info, "person_family", vpack.FInt, PackPersonFamily)

// PersonFamilyByPersonIndex: term = person_id, target = person_family_id
var PersonFamilyByPersonIndex = vbolt.Index(&cfg.Info, "person_family_by_person", vpack.FInt, vpack.FInt)

// PersonFamilyByFamilyIndex: term = family_id, target = person_family_id
var PersonFamilyByFamilyIndex = vbolt.Index(&cfg.Info, "person_family_by_family", vpack.FInt, vpack.FInt)

var ErrCannotRemoveHomeRoster = errors.New("Cannot remove a person from their home family")

// GetPersonFamilies returns every roster the person appears on.
func GetPersonFamilies(tx *vbolt.Tx, personId int) (rows []PersonFamily) {
	var ids []int
	vbolt.ReadTermTargets(tx, PersonFamilyByPersonIndex, personId, &ids, vbolt.Window{})
	vbolt.ReadSlice(tx, PersonFamilyBkt, ids, &rows)
	return
}

// GetFamilyRoster returns every roster row for the family.
func GetFamilyRoster(tx *vbolt.Tx, familyId int) (rows []PersonFamily) {
	var ids []int
	vbolt.ReadTermTargets(tx, PersonFamilyByFamilyIndex, familyId, &ids, vbolt.Window{})
	vbolt.ReadSlice(tx, PersonFamilyBkt, ids, &rows)
	return
}

// FindPersonFamily looks up one person's row on one specific roster.
func FindPersonFamily(tx *vbolt.Tx, personId int, familyId int) (PersonFamily, bool) {
	for _, row := range GetPersonFamilies(tx, personId) {
		if row.FamilyId == familyId {
			return row, true
		}
	}
	return PersonFamily{}, false
}

func addPersonFamilyTx(tx *vbolt.Tx, personId int, familyId int, role PersonType, relationship string, addedAt time.Time) PersonFamily {
	row := PersonFamily{
		Id:           vbolt.NextIntId(tx, PersonFamilyBkt),
		PersonId:     personId,
		FamilyId:     familyId,
		Role:         role,
		Relationship: relationship,
		AddedAt:      addedAt,
	}
	vbolt.Write(tx, PersonFamilyBkt, row.Id, &row)
	vbolt.SetTargetSingleTerm(tx, PersonFamilyByPersonIndex, row.Id, personId)
	vbolt.SetTargetSingleTerm(tx, PersonFamilyByFamilyIndex, row.Id, familyId)
	return row
}

func deletePersonFamilyTx(tx *vbolt.Tx, row PersonFamily) {
	vbolt.Delete(tx, PersonFamilyBkt, row.Id)
	vbolt.SetTargetSingleTerm(tx, PersonFamilyByPersonIndex, row.Id, -1)
	vbolt.SetTargetSingleTerm(tx, PersonFamilyByFamilyIndex, row.Id, -1)
}

// EnsurePersonFamilyTx places the person on the family's roster, doing nothing
// if they are already on it. An existing row's Role is left alone — changing a
// role is a separate operation, not a side effect of re-recording the roster.
func EnsurePersonFamilyTx(tx *vbolt.Tx, personId int, familyId int, role PersonType) PersonFamily {
	return ensurePersonFamilyTx(tx, personId, familyId, role, time.Now())
}

func ensurePersonFamilyTx(tx *vbolt.Tx, personId int, familyId int, role PersonType, addedAt time.Time) PersonFamily {
	if personId == 0 || familyId == 0 {
		return PersonFamily{}
	}
	if existing, found := FindPersonFamily(tx, personId, familyId); found {
		return existing
	}
	return addPersonFamilyTx(tx, personId, familyId, role, "", addedAt)
}

// SetPersonFamilyRoleTx changes the person's role on one roster, creating the
// row if it is missing.
func SetPersonFamilyRoleTx(tx *vbolt.Tx, personId int, familyId int, role PersonType) PersonFamily {
	if personId == 0 || familyId == 0 {
		return PersonFamily{}
	}
	row, found := FindPersonFamily(tx, personId, familyId)
	if !found {
		return addPersonFamilyTx(tx, personId, familyId, role, "", time.Now())
	}
	if row.Role != role {
		row.Role = role
		vbolt.Write(tx, PersonFamilyBkt, row.Id, &row)
	}
	return row
}

// RemovePersonFromFamilyTx takes the person off an extended roster. The home
// family's row is not removable: it is the ownership chain that every leaf
// record's authorization resolves through.
func RemovePersonFromFamilyTx(tx *vbolt.Tx, personId int, familyId int) error {
	person := GetPersonById(tx, personId)
	if person.Id == 0 {
		return errors.New("Person not found")
	}
	if person.FamilyId == familyId {
		return ErrCannotRemoveHomeRoster
	}
	row, found := FindPersonFamily(tx, personId, familyId)
	if !found {
		return nil
	}
	deletePersonFamilyTx(tx, row)
	return nil
}

// deletePersonRostersTx removes every roster row for a person, for use when the
// person record itself goes away.
func deletePersonRostersTx(tx *vbolt.Tx, personId int) {
	for _, row := range GetPersonFamilies(tx, personId) {
		deletePersonFamilyTx(tx, row)
	}
}

// BackfillPersonFamilies writes one roster row per existing person, mirroring
// Person.FamilyId and Person.Type. Safe to re-run: people that already have the
// row are skipped, so re-running creates nothing and changes nothing.
func BackfillPersonFamilies(tx *vbolt.Tx) (created int) {
	vbolt.IterateAll(tx, PeopleBkt, func(personId int, person Person) bool {
		if person.FamilyId == 0 {
			return true
		}
		if _, found := FindPersonFamily(tx, person.Id, person.FamilyId); found {
			return true
		}
		// AddedAt is unknowable for existing rows. The person's birthday is not
		// a join time, and there is no created-at on Person, so the zero time is
		// used to mark "predates the roster table" rather than inventing one.
		addPersonFamilyTx(tx, person.Id, person.FamilyId, person.Type, "", time.Time{})
		created++
		return true
	})
	return
}
