package backend

import (
	"errors"
	"family/cfg"
	"time"

	"go.hasen.dev/vbolt"
	"go.hasen.dev/vpack"
)

type PersonFamily struct {
	Id           int        `json:"id"`
	PersonId     int        `json:"personId"`
	FamilyId     int        `json:"familyId"`
	Role         PersonType `json:"role"`
	Relationship string     `json:"relationship,omitempty"`
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

var PersonFamilyByPersonIndex = vbolt.Index(&cfg.Info, "person_family_by_person", vpack.FInt, vpack.FInt)

var PersonFamilyByFamilyIndex = vbolt.Index(&cfg.Info, "person_family_by_family", vpack.FInt, vpack.FInt)

var ErrCannotRemoveHomeRoster = errors.New("Cannot remove a person from their home family")

func GetPersonFamilies(tx *vbolt.Tx, personId int) (rows []PersonFamily) {
	var ids []int
	vbolt.ReadTermTargets(tx, PersonFamilyByPersonIndex, personId, &ids, vbolt.Window{})
	vbolt.ReadSlice(tx, PersonFamilyBkt, ids, &rows)
	return
}

func GetFamilyRoster(tx *vbolt.Tx, familyId int) (rows []PersonFamily) {
	var ids []int
	vbolt.ReadTermTargets(tx, PersonFamilyByFamilyIndex, familyId, &ids, vbolt.Window{})
	vbolt.ReadSlice(tx, PersonFamilyBkt, ids, &rows)
	return
}

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

func deletePersonRostersTx(tx *vbolt.Tx, personId int) {
	for _, row := range GetPersonFamilies(tx, personId) {
		deletePersonFamilyTx(tx, row)
	}
}

func BackfillPersonFamilies(tx *vbolt.Tx) (created int) {
	vbolt.IterateAll(tx, PeopleBkt, func(personId int, person Person) bool {
		if person.FamilyId == 0 {
			return true
		}
		if _, found := FindPersonFamily(tx, person.Id, person.FamilyId); found {
			return true
		}
		addPersonFamilyTx(tx, person.Id, person.FamilyId, person.Type, "", time.Time{})
		created++
		return true
	})
	return
}
