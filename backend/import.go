package backend

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"go.hasen.dev/vbeam"
	"go.hasen.dev/vbolt"
)

func RegisterImportMethods(app *vbeam.Application) {
	vbeam.RegisterProc(app, ImportData)
	RegisterArchiveImportMethods(app)
}

type ImportDataStructure struct {
	FormatVersion int `json:"formatVersion,omitempty"`

	People     []ImportPerson    `json:"people"`
	Relations  []ExportRelation  `json:"relations,omitempty"`
	Heights    []ImportHeight    `json:"heights"`
	Weights    []ImportWeight    `json:"weights"`
	Milestones []ExportMilestone `json:"milestones"`
	Tags       []ExportTag       `json:"tags"`
	Photos     []ExportPhoto     `json:"photos,omitempty"`
	Videos     []ExportVideo     `json:"videos,omitempty"`
	Activities []ExportActivity  `json:"activities,omitempty"`
	Books      []ExportBook      `json:"books,omitempty"`

	InterviewQuestions []ExportInterviewQuestion `json:"interviewQuestions,omitempty"`
	Interviews         []ExportInterview         `json:"interviews,omitempty"`
	InterviewCadence   *InterviewCadence         `json:"interviewCadence,omitempty"`

	ExportDate      time.Time `json:"export_date"`
	TotalHeights    int       `json:"total_heights"`
	TotalWeights    int       `json:"total_weights"`
	TotalPeople     int       `json:"total_people"`
	TotalMilestones int       `json:"total_milestones"`
}

type ImportDataRequest struct {
	JsonData         string `json:"jsonData"`
	FilterFamilyIds  []int  `json:"filterFamilyIds,omitempty"`
	FilterPersonIds  []int  `json:"filterPersonIds,omitempty"`
	PreviewOnly      bool   `json:"previewOnly,omitempty"`
	ImportMilestones bool   `json:"importMilestones,omitempty"`
	ImportActivities bool   `json:"importActivities,omitempty"`
	DryRun           bool   `json:"dryRun,omitempty"`
	FamilyId         int    `json:"familyId,omitempty"`
}

type ImportDataResponse struct {
	ImportedPeople       int                  `json:"importedPeople"`
	MergedPeople         int                  `json:"mergedPeople"`
	SkippedPeople        int                  `json:"skippedPeople"`
	ImportedMeasurements int                  `json:"importedMeasurements"`
	SkippedMeasurements  int                  `json:"skippedMeasurements"`
	ImportedRelations    int                  `json:"importedRelations"`
	SkippedRelations     int                  `json:"skippedRelations"`
	ImportedMilestones   int                  `json:"importedMilestones"`
	SkippedMilestones    int                  `json:"skippedMilestones"`
	ImportedTags         int                  `json:"importedTags"`
	SkippedTags          int                  `json:"skippedTags"`
	ImportedPhotos       int                  `json:"importedPhotos"`
	SkippedPhotos        int                  `json:"skippedPhotos"`
	SkippedVideos        int                  `json:"skippedVideos"`
	ImportedActivities   ActivityImportCounts `json:"importedActivities"`
	ImportedBooks        int                  `json:"importedBooks"`
	SkippedBooks         int                  `json:"skippedBooks"`
	ImportedInterviews   int                  `json:"importedInterviews"`
	SkippedInterviews    int                  `json:"skippedInterviews"`
	Errors               []string             `json:"errors,omitempty"`
	Warnings             []string             `json:"warnings,omitempty"`
	PersonIdMapping      map[int]int          `json:"personIdMapping,omitempty"`
	AvailableFamilyIds   []int                `json:"availableFamilyIds,omitempty"`
	AvailablePeople      []ImportPerson       `json:"availablePeople,omitempty"`
	MatchedPeople        []PersonMatch        `json:"matchedPeople,omitempty"`
}

func validateImportData(data ImportDataStructure) error {
	if len(data.People) == 0 {
		return errors.New("No people found in import data")
	}

	for i, person := range data.People {
		if err := validateImportPerson(person, i); err != nil {
			return err
		}
	}

	personIds := make(map[int]bool)
	for _, person := range data.People {
		personIds[person.Id] = true
	}

	for i, height := range data.Heights {
		if err := validateImportHeight(height, i, personIds); err != nil {
			return err
		}
	}

	for i, weight := range data.Weights {
		if err := validateImportWeight(weight, i, personIds); err != nil {
			return err
		}
	}

	for i, milestone := range data.Milestones {
		if err := validateImportMilestone(milestone, i, personIds); err != nil {
			return err
		}
	}

	for i, relation := range data.Relations {
		if err := validateImportRelation(relation, i, personIds); err != nil {
			return err
		}
	}

	return nil
}

func validateImportMilestone(milestone ExportMilestone, index int, validPersonIds map[int]bool) error {
	if !validPersonIds[milestone.PersonId] {
		return errors.New("Milestone at index " + formatIndex(index) + " references unknown person ID " + formatIndex(milestone.PersonId))
	}

	if strings.TrimSpace(milestone.Description) == "" {
		return errors.New("Milestone at index " + formatIndex(index) + " has empty description")
	}

	if milestone.RecordTime != nil {
		if err := validateRecordTime(*milestone.RecordTime, true); err != nil {
			return errors.New("Milestone at index " + formatIndex(index) + " has an invalid date: " + err.Error())
		}
		if milestone.RecordTime.Basis == BasisAge && !validPersonIds[milestone.RecordTime.AnchorPersonId] {
			return errors.New("Milestone at index " + formatIndex(index) + " is dated by the age of unknown person ID " + formatIndex(milestone.RecordTime.AnchorPersonId))
		}
		return nil
	}
	if milestone.MilestoneDate.Year() < 1900 || milestone.MilestoneDate.After(time.Now()) {
		return errors.New("Milestone at index " + formatIndex(index) + " has invalid date")
	}

	return nil
}

func formatIndex(index int) string {
	return fmt.Sprintf("%d", index)
}

func filterMilestones(milestones []ExportMilestone, personIdMapping map[int]int) []ExportMilestone {
	var filteredMilestones []ExportMilestone

	for _, milestone := range milestones {
		if _, exists := personIdMapping[milestone.PersonId]; exists {
			filteredMilestones = append(filteredMilestones, milestone)
		}
	}

	return filteredMilestones
}

func existingMilestoneId(tx *vbolt.Tx, personId int, date time.Time, description string) int {
	for _, milestone := range GetPersonMilestonesTx(tx, personId) {
		if milestone.MilestoneDate.Equal(date) && milestone.Description == description {
			return milestone.Id
		}
	}
	return 0
}

func importMilestones(tx *vbolt.Tx, importMilestones []ExportMilestone, familyId int, l *archiveLedger) (int, int, []string, []int) {
	var ids []int
	var errors []string
	importedCount := 0
	skippedCount := 0
	recorderOf := importRecorders(tx, familyId)

	for _, milestone := range importMilestones {
		if milestone.InterviewId > 0 || milestone.Category == CategoryInterview {
			continue
		}
		newPersonId, exists := l.people[milestone.PersonId]
		if !exists {
			errors = append(errors, fmt.Sprintf("Moment for unknown person ID: %d", milestone.PersonId))
			continue
		}

		if milestone.RecordTime == nil {
			if milestone.MilestoneDate.Year() == 1 {
				continue
			}
			milestone.MilestoneDate = dayStart(milestone.MilestoneDate)
		}
		recordTime, date, err := importRecordTime(milestone.RecordTime, milestone.MilestoneDate, l.people, true)
		if err != nil {
			errors = append(errors, fmt.Sprintf("Moment %q: %s", milestone.Description, err.Error()))
			continue
		}
		milestone.MilestoneDate = date

		if id := existingMilestoneId(tx, newPersonId, milestone.MilestoneDate, milestone.Description); id != 0 {
			l.moments[milestone.Id] = id
			skippedCount++
			continue
		}

		newMilestone := Milestone{
			Id:            vbolt.NextIntId(tx, MilestoneBkt),
			PersonId:      newPersonId,
			FamilyId:      familyId,
			Description:   milestone.Description,
			Category:      milestone.Category,
			Context:       milestone.Context,
			MilestoneDate: milestone.MilestoneDate,
			RecordTime:    recordTime,
			CreatedAt:     time.Now(),
			CreatedBy:     recorderOf(milestone.RecordedBy),
		}
		writeMilestone(tx, &newMilestone)
		l.moments[milestone.Id] = newMilestone.Id

		for _, tagName := range milestone.TagNames {
			if tagId, ok := l.tagByName[strings.ToLower(tagName)]; ok {
				addTagToMilestone(tx, newMilestone.Id, tagId, familyId)
			}
		}

		ids = append(ids, newMilestone.Id)
		importedCount++
	}

	return importedCount, skippedCount, errors, ids
}
