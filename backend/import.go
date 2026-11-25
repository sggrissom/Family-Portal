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

type ImportPerson struct {
	Id       int       `json:"Id"`
	FamilyId int       `json:"FamilyId"`
	Gender   int       `json:"Gender"`
	Name     string    `json:"Name"`
	Birthday time.Time `json:"Birthday"`
	Age      string    `json:"Age"`
	ImageId  int       `json:"ImageId"`

	BirthTime  string `json:"BirthTime,omitempty"`
	BirthPlace string `json:"BirthPlace,omitempty"`
}

type ImportHeight struct {
	Id         int       `json:"Id"`
	PersonId   int       `json:"PersonId"`
	Inches     float64   `json:"Inches"`
	Date       time.Time `json:"Date"`
	DateString string    `json:"DateString"`
	Age        float64   `json:"Age"`
	PersonName string    `json:"PersonName"`
	RecordedBy string    `json:"RecordedBy,omitempty"`
}

type ImportWeight struct {
	Id         int       `json:"Id"`
	PersonId   int       `json:"PersonId"`
	Pounds     float64   `json:"Pounds"`
	Date       time.Time `json:"Date"`
	DateString string    `json:"DateString"`
	Age        float64   `json:"Age"`
	PersonName string    `json:"PersonName"`
	RecordedBy string    `json:"RecordedBy,omitempty"`
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

func validateImportRelation(relation ExportRelation, index int, validPersonIds map[int]bool) error {
	if !validPersonIds[relation.FromId] || !validPersonIds[relation.ToId] {
		return errors.New("Relationship at index " + formatIndex(index) + " references an unknown person")
	}

	if relation.FromId == relation.ToId {
		return errors.New("Relationship at index " + formatIndex(index) + " relates a person to themselves")
	}

	if _, ok := parseRelationKind(relation.Kind); !ok {
		return errors.New("Relationship at index " + formatIndex(index) + " has unknown kind '" + relation.Kind + "'")
	}

	return nil
}

func validateImportPerson(person ImportPerson, index int) error {
	if person.Name == "" {
		return errors.New("Person at index " + formatIndex(index) + " has no name")
	}

	if len(strings.TrimSpace(person.Name)) == 0 {
		return errors.New("Person at index " + formatIndex(index) + " has empty name")
	}

	if person.Birthday.IsZero() {
		return errors.New("Person '" + person.Name + "' has invalid birthday")
	}

	now := time.Now()
	if person.Birthday.After(now) {
		return errors.New("Person '" + person.Name + "' has birthday in the future")
	}

	maxAge := now.AddDate(-150, 0, 0)
	if person.Birthday.Before(maxAge) {
		return errors.New("Person '" + person.Name + "' has birthday more than 150 years ago")
	}

	if person.Gender < 0 || person.Gender > 2 {
		return errors.New("Person '" + person.Name + "' has invalid gender value")
	}

	return nil
}

func validateImportHeight(height ImportHeight, index int, validPersonIds map[int]bool) error {
	if !validPersonIds[height.PersonId] {
		return errors.New("Height measurement at index " + formatIndex(index) + " references unknown person ID " + formatIndex(height.PersonId))
	}

	if height.Inches <= 0 || height.Inches > 120 {
		return errors.New("Height measurement at index " + formatIndex(index) + " has invalid height value")
	}

	if height.Date.Year() < 1900 || height.Date.After(time.Now()) {
		return errors.New("Height measurement at index " + formatIndex(index) + " has invalid date")
	}

	return nil
}

func validateImportWeight(weight ImportWeight, index int, validPersonIds map[int]bool) error {
	if !validPersonIds[weight.PersonId] {
		return errors.New("Weight measurement at index " + formatIndex(index) + " references unknown person ID " + formatIndex(weight.PersonId))
	}

	if weight.Pounds <= 0 || weight.Pounds > 2000 {
		return errors.New("Weight measurement at index " + formatIndex(index) + " has invalid weight value")
	}

	if weight.Date.Year() < 1900 || weight.Date.After(time.Now()) {
		return errors.New("Weight measurement at index " + formatIndex(index) + " has invalid date")
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

// importPeople links each person to a matching one already in the family,
// filling in missing birth details, and creates the rest.
func importPeople(tx *vbolt.Tx, importPeople []ImportPerson, familyId int, l *archiveLedger) (imported, merged int, warnings []string) {
	for _, importPerson := range importPeople {
		if existing := findExistingPerson(tx, importPerson, familyId); existing != nil {
			fillBirthDetails(tx, *existing, importPerson)
			l.people[importPerson.Id] = existing.Id
			merged++
			warnings = append(warnings, "Merged with existing person: "+importPerson.Name)
			continue
		}

		var person Person
		person.Id = vbolt.NextIntId(tx, PeopleBkt)
		person.FamilyId = familyId
		person.Name = importPerson.Name
		person.Gender = GenderType(importPerson.Gender)
		person.Birthday = dayStart(importPerson.Birthday)
		person.Age = calculateAge(person.Birthday)
		person.BirthTime, person.BirthPlace = importedBirthDetails(importPerson)

		vbolt.Write(tx, PeopleBkt, person.Id, &person)
		updatePersonIndex(tx, person)

		l.people[importPerson.Id] = person.Id
		imported++
	}
	return
}

func importMeasurements(tx *vbolt.Tx, importHeights []ImportHeight, importWeights []ImportWeight, personIdMapping map[int]int, familyId int) (int, int, []string) {
	type measurement struct {
		label    string
		personId int
		kind     MeasurementType
		value    float64
		unit     string
		date     time.Time
		recorder string
	}
	var measurements []measurement
	for _, height := range importHeights {
		measurements = append(measurements, measurement{"Height", height.PersonId, Height, height.Inches, "in", dayStart(height.Date), height.RecordedBy})
	}
	for _, weight := range importWeights {
		measurements = append(measurements, measurement{"Weight", weight.PersonId, Weight, weight.Pounds, "lbs", dayStart(weight.Date), weight.RecordedBy})
	}
	recorderOf := importRecorders(tx, familyId)

	var errors []string
	importedCount := 0
	skippedCount := 0
	for _, m := range measurements {
		newPersonId, exists := personIdMapping[m.personId]
		if !exists {
			errors = append(errors, fmt.Sprintf("%s measurement for unknown person ID: %d", m.label, m.personId))
			continue
		}
		if m.date.Year() == 1 || isDuplicateMeasurement(tx, newPersonId, m.date, m.kind, m.value) {
			skippedCount++
			continue
		}
		writeGrowthData(tx, GrowthData{
			Id:              vbolt.NextIntId(tx, GrowthDataBkt),
			PersonId:        newPersonId,
			FamilyId:        familyId,
			MeasurementType: m.kind,
			Value:           m.value,
			Unit:            m.unit,
			MeasurementDate: m.date,
			CreatedAt:       time.Now(),
			CreatedBy:       recorderOf(m.recorder),
		})
		importedCount++
	}

	return importedCount, skippedCount, errors
}

func getUniqueFamilyIds(people []ImportPerson) []int {
	familyIdMap := make(map[int]bool)
	for _, person := range people {
		familyIdMap[person.FamilyId] = true
	}

	var familyIds []int
	for id := range familyIdMap {
		familyIds = append(familyIds, id)
	}
	return familyIds
}

func filterPeople(people []ImportPerson, filterFamilyIds []int, filterPersonIds []int) []ImportPerson {
	if len(filterFamilyIds) == 0 && len(filterPersonIds) == 0 {
		return people
	}

	var filtered []ImportPerson

	familyIdMap := make(map[int]bool)
	for _, id := range filterFamilyIds {
		familyIdMap[id] = true
	}

	personIdMap := make(map[int]bool)
	for _, id := range filterPersonIds {
		personIdMap[id] = true
	}

	for _, person := range people {
		matchesFamilyFilter := len(filterFamilyIds) == 0 || familyIdMap[person.FamilyId]

		matchesPersonFilter := len(filterPersonIds) == 0 || personIdMap[person.Id]

		if matchesFamilyFilter && (len(filterPersonIds) == 0 || matchesPersonFilter) {
			filtered = append(filtered, person)
		}
	}

	return filtered
}

func filterMeasurements(heights []ImportHeight, weights []ImportWeight, personIdMapping map[int]int) ([]ImportHeight, []ImportWeight) {
	var filteredHeights []ImportHeight
	var filteredWeights []ImportWeight

	for _, height := range heights {
		if _, exists := personIdMapping[height.PersonId]; exists {
			filteredHeights = append(filteredHeights, height)
		}
	}

	for _, weight := range weights {
		if _, exists := personIdMapping[weight.PersonId]; exists {
			filteredWeights = append(filteredWeights, weight)
		}
	}

	return filteredHeights, filteredWeights
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

type PersonMatch struct {
	ImportPerson   ImportPerson `json:"importPerson"`
	ExistingPerson *Person      `json:"existingPerson,omitempty"`
	MatchType      string       `json:"matchType"`
	Confidence     float64      `json:"confidence"`
}

func findExistingPerson(tx *vbolt.Tx, importPerson ImportPerson, familyId int) *Person {
	familyPeople := GetFamilyOwnPeople(tx, familyId)

	for _, existing := range familyPeople {
		if existing.Name == importPerson.Name &&
			existing.Birthday.Equal(dayStart(importPerson.Birthday)) &&
			existing.Gender == GenderType(importPerson.Gender) {
			return &existing
		}
	}

	return nil
}

func findPotentialMatches(tx *vbolt.Tx, importPerson ImportPerson, familyId int) []PersonMatch {
	var matches []PersonMatch
	familyPeople := GetFamilyOwnPeople(tx, familyId)

	for _, existing := range familyPeople {
		confidence := calculateMatchConfidence(importPerson, existing)
		if confidence > 0.3 {
			matchType := "potential"
			if confidence >= 0.95 {
				matchType = "exact"
			}

			matches = append(matches, PersonMatch{
				ImportPerson:   importPerson,
				ExistingPerson: &existing,
				MatchType:      matchType,
				Confidence:     confidence,
			})
		}
	}

	return matches
}

func calculateMatchConfidence(importPerson ImportPerson, existing Person) float64 {
	score := 0.0
	totalFactors := 0.0

	if importPerson.Name == existing.Name {
		score += 0.4
	} else if strings.EqualFold(importPerson.Name, existing.Name) {
		score += 0.35
	} else {
		name1 := strings.ToLower(importPerson.Name)
		name2 := strings.ToLower(existing.Name)
		if strings.Contains(name1, name2) || strings.Contains(name2, name1) {
			score += 0.2
		}
	}
	totalFactors += 0.4

	if importPerson.Birthday.Equal(existing.Birthday) {
		score += 0.4
	} else {
		daysDiff := importPerson.Birthday.Sub(existing.Birthday).Hours() / 24
		if daysDiff < 0 {
			daysDiff = -daysDiff
		}
		if daysDiff <= 3 {
			score += 0.3
		} else if daysDiff <= 7 {
			score += 0.1
		}
	}
	totalFactors += 0.4

	if GenderType(importPerson.Gender) == existing.Gender {
		score += 0.2
	}
	totalFactors += 0.2

	return score / totalFactors
}

func isDuplicateMeasurement(tx *vbolt.Tx, personId int, date time.Time, measurementType MeasurementType, value float64) bool {
	growthData := GetPersonGrowthDataTx(tx, personId)

	for _, measurement := range growthData {
		if measurement.MeasurementType == measurementType &&
			measurement.MeasurementDate.Equal(date) &&
			abs(measurement.Value-value) < 0.01 {
			return true
		}
	}

	return false
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
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

// importRelations rewrites each edge onto the people this import actually
// created or merged onto. Skipped are edges reaching a person who was filtered
// out, edges already stated in either direction, and edges whose ends collapse
// onto one person because two import rows merged onto the same existing one.
func importRelations(tx *vbolt.Tx, relations []ExportRelation, personIdMapping map[int]int) (int, int, []string) {
	var errors []string
	importedCount := 0
	skippedCount := 0

	for _, relation := range relations {
		fromId, fromExists := personIdMapping[relation.FromId]
		toId, toExists := personIdMapping[relation.ToId]
		if !fromExists || !toExists || fromId == toId {
			skippedCount++
			continue
		}

		kind, ok := parseRelationKind(relation.Kind)
		if !ok {
			errors = append(errors, fmt.Sprintf("Relationship with unknown kind: %s", relation.Kind))
			continue
		}

		edge := Relation{FromId: fromId, ToId: toId, Kind: kind}
		if _, found := findRelationTx(tx, edge); found {
			skippedCount++
			continue
		}

		if _, err := AddRelationTx(tx, edge); err != nil {
			errors = append(errors, fmt.Sprintf("Failed to import relationship: %s", err.Error()))
			continue
		}
		importedCount++
	}

	return importedCount, skippedCount, errors
}

func importTags(tx *vbolt.Tx, exportedTags []ExportTag, familyId int, l *archiveLedger) (imported, skipped int) {
	for _, exportTag := range exportedTags {
		lowerName := strings.ToLower(exportTag.Name)
		tagId := l.tagByName[lowerName]
		for _, t := range getTagsByFamily(tx, familyId) {
			if tagId == 0 && strings.ToLower(t.Name) == lowerName {
				tagId = t.Id
			}
		}

		if tagId != 0 {
			skipped++
		} else {
			tag := Tag{
				Id:        vbolt.NextIntId(tx, TagBkt),
				FamilyId:  familyId,
				Name:      exportTag.Name,
				Color:     exportTag.Color,
				CreatedAt: time.Now(),
			}
			vbolt.Write(tx, TagBkt, tag.Id, &tag)
			vbolt.SetTargetSingleTerm(tx, TagByFamilyIndex, tag.Id, tag.FamilyId)
			tagId = tag.Id
			imported++
		}
		l.tagByName[lowerName] = tagId
		l.tags[exportTag.Id] = tagId
	}
	return
}
