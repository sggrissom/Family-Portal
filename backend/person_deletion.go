package backend

import (
	"errors"

	"go.hasen.dev/vbeam"
	"go.hasen.dev/vbolt"
)

type PersonDeletionRequest struct {
	PersonId int `json:"personId"`
}

type PersonDeletionSummary struct {
	PersonId       int    `json:"personId"`
	Name           string `json:"name"`
	Milestones     int    `json:"milestones"`
	GrowthRecords  int    `json:"growthRecords"`
	PhotoTags      int    `json:"photoTags"`
	Faces          int    `json:"faces"`
	ActivityRoles  int    `json:"activityRoles"`
	Results        int    `json:"results"`
	Relations      int    `json:"relations"`
	SharedFamilies int    `json:"sharedFamilies"`
}

type DeletePersonResponse struct {
	Success bool                  `json:"success"`
	Deleted PersonDeletionSummary `json:"deleted"`
}

func personForDeletion(ctx *vbeam.Context, personId int) (person Person, user User, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}
	if personId <= 0 {
		err = errors.New("Invalid person ID")
		return
	}
	person = GetPersonById(ctx.Tx, personId)
	if person.Id == 0 || !CanAccessFamily(ctx.Tx, user, person.FamilyId, AccessAdmin) {
		err = errors.New("Person not found or access denied")
	}
	return
}

func personDeletionSummary(tx *vbolt.Tx, person Person) PersonDeletionSummary {
	sharedFamilies := 0
	for _, row := range GetPersonFamilies(tx, person.Id) {
		if row.FamilyId != person.FamilyId {
			sharedFamilies++
		}
	}
	return PersonDeletionSummary{
		PersonId:       person.Id,
		Name:           person.Name,
		Milestones:     len(GetPersonMilestonesTx(tx, person.Id)),
		GrowthRecords:  len(GetPersonGrowthDataTx(tx, person.Id)),
		PhotoTags:      len(GetPhotoPersonsByPerson(tx, person.Id)),
		Faces:          len(GetPersonFaces(tx, person.Id)),
		ActivityRoles:  len(GetPersonEntryMembers(tx, person.Id)),
		Results:        len(GetPersonResults(tx, person.Id)),
		Relations:      len(GetPersonRelationsTx(tx, person.Id)),
		SharedFamilies: sharedFamilies,
	}
}

func GetPersonDeletionSummary(ctx *vbeam.Context, req PersonDeletionRequest) (resp PersonDeletionSummary, err error) {
	person, _, err := personForDeletion(ctx, req.PersonId)
	if err != nil {
		return
	}
	resp = personDeletionSummary(ctx.Tx, person)
	return
}

func DeletePerson(ctx *vbeam.Context, req PersonDeletionRequest) (resp DeletePersonResponse, err error) {
	vbeam.UseWriteTx(ctx)

	person, user, err := personForDeletion(ctx, req.PersonId)
	if err != nil {
		return
	}

	resp.Deleted = personDeletionSummary(ctx.Tx, person)
	deletePersonTx(ctx.Tx, person)
	vbolt.TxCommit(ctx.Tx)
	resp.Success = true

	LogInfo("DATA", "Person deleted", map[string]any{
		"userId":   user.Id,
		"familyId": person.FamilyId,
		"personId": person.Id,
	})
	return
}

func deletePersonTx(tx *vbolt.Tx, person Person) {
	for _, milestone := range GetPersonMilestonesTx(tx, person.Id) {
		_ = DeleteMilestoneTx(tx, milestone.Id, milestone.FamilyId)
	}
	for _, growth := range GetPersonGrowthDataTx(tx, person.Id) {
		_ = DeleteGrowthDataTx(tx, growth.Id, growth.FamilyId)
	}
	repointUserPersonTx(tx, person.Id, 0)
	deletePersonRecordTx(tx, person)
}

func repointUserPersonTx(tx *vbolt.Tx, fromPersonId int, toPersonId int) {
	familyIds := []int{GetPersonById(tx, fromPersonId).FamilyId}
	for _, row := range GetPersonFamilies(tx, fromPersonId) {
		familyIds = append(familyIds, row.FamilyId)
	}
	seen := make(map[int]bool)
	for _, familyId := range familyIds {
		for _, userId := range GetFamilyUserIds(tx, familyId) {
			if seen[userId] {
				continue
			}
			seen[userId] = true
			user := GetUser(tx, userId)
			if user.Id == 0 || user.PersonId != fromPersonId {
				continue
			}
			user.PersonId = toPersonId
			vbolt.Write(tx, UsersBkt, user.Id, &user)
		}
	}
}
