package backend

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"go.hasen.dev/vbeam"
	"go.hasen.dev/vbolt"
)

func RegisterImportMethods(app *vbeam.Application) {
	vbeam.RegisterProc(app, ImportData)
	app.HandleFunc("POST /api/import-bundle", AuthMiddleware(importBundleHandler))
}

type ImportPerson struct {
	Id       int       `json:"Id"`
	FamilyId int       `json:"FamilyId"`
	Gender   int       `json:"Gender"`
	Name     string    `json:"Name"`
	Birthday time.Time `json:"Birthday"`
	Age      string    `json:"Age"`
	ImageId  int       `json:"ImageId"`
}

type ImportHeight struct {
	Id         int       `json:"Id"`
	PersonId   int       `json:"PersonId"`
	Inches     float64   `json:"Inches"`
	Date       time.Time `json:"Date"`
	DateString string    `json:"DateString"`
	Age        float64   `json:"Age"`
	PersonName string    `json:"PersonName"`
}

type ImportWeight struct {
	Id         int       `json:"Id"`
	PersonId   int       `json:"PersonId"`
	Pounds     float64   `json:"Pounds"`
	Date       time.Time `json:"Date"`
	DateString string    `json:"DateString"`
	Age        float64   `json:"Age"`
	PersonName string    `json:"PersonName"`
}

type ImportDataStructure struct {
	People          []ImportPerson    `json:"people"`
	Relations       []ExportRelation  `json:"relations,omitempty"`
	Heights         []ImportHeight    `json:"heights"`
	Weights         []ImportWeight    `json:"weights"`
	Milestones      []ExportMilestone `json:"milestones"`
	Tags            []ExportTag       `json:"tags"`
	Photos          []ExportPhoto     `json:"photos,omitempty"`
	Activities      []ExportActivity  `json:"activities,omitempty"`
	ExportDate      time.Time         `json:"export_date"`
	TotalHeights    int               `json:"total_heights"`
	TotalWeights    int               `json:"total_weights"`
	TotalPeople     int               `json:"total_people"`
	TotalMilestones int               `json:"total_milestones"`
}

type ImportDataRequest struct {
	JsonData         string `json:"jsonData"`
	FilterFamilyIds  []int  `json:"filterFamilyIds,omitempty"`
	FilterPersonIds  []int  `json:"filterPersonIds,omitempty"`
	PreviewOnly      bool   `json:"previewOnly,omitempty"`
	MergeStrategy    string `json:"mergeStrategy,omitempty"`
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
	ImportedActivities   ActivityImportCounts `json:"importedActivities"`
	Errors               []string             `json:"errors,omitempty"`
	Warnings             []string             `json:"warnings,omitempty"`
	PersonIdMapping      map[int]int          `json:"personIdMapping,omitempty"`
	AvailableFamilyIds   []int                `json:"availableFamilyIds,omitempty"`
	AvailablePeople      []ImportPerson       `json:"availablePeople,omitempty"`
	MatchedPeople        []PersonMatch        `json:"matchedPeople,omitempty"`
}

func ImportData(ctx *vbeam.Context, req ImportDataRequest) (resp ImportDataResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}

	var importData ImportDataStructure
	if err = json.Unmarshal([]byte(req.JsonData), &importData); err != nil {
		LogWarn(LogCategoryAPI, "Import rejected unreadable JSON", map[string]interface{}{
			"userId": user.Id,
			"error":  err.Error(),
		})
		err = errors.New("That file is not a valid Family Record export. Check that you picked the right file.")
		return
	}

	if err = validateImportData(importData); err != nil {
		return
	}

	mergeStrategy := req.MergeStrategy
	if mergeStrategy == "" {
		mergeStrategy = "create_all"
	}

	familyId, err := ResolveActingFamily(ctx.Tx, user, req.FamilyId, AccessContribute)
	if err != nil {
		return
	}

	resp.AvailableFamilyIds = getUniqueFamilyIds(importData.People)
	resp.AvailablePeople = importData.People

	if req.PreviewOnly {
		filteredPeople := filterPeople(importData.People, req.FilterFamilyIds, req.FilterPersonIds)
		for _, importPerson := range filteredPeople {
			matches := findPotentialMatches(ctx.Tx, importPerson, familyId)
			if len(matches) > 0 {
				resp.MatchedPeople = append(resp.MatchedPeople, matches...)
			} else {
				resp.MatchedPeople = append(resp.MatchedPeople, PersonMatch{
					ImportPerson: importPerson,
					MatchType:    "none",
					Confidence:   0.0,
				})
			}
		}
		return
	}

	filteredPeople := filterPeople(importData.People, req.FilterFamilyIds, req.FilterPersonIds)
	if len(filteredPeople) == 0 {
		err = errors.New("No people match the specified filters")
		return
	}

	if !req.DryRun {
		vbeam.UseWriteTx(ctx)
	}

	personIdMapping, importedPeople, mergedPeople, peopleErrors, peopleWarnings := importPeople(ctx.Tx, filteredPeople, familyId, mergeStrategy)
	resp.ImportedPeople = importedPeople
	resp.MergedPeople = mergedPeople
	resp.PersonIdMapping = personIdMapping
	resp.Errors = append(resp.Errors, peopleErrors...)
	resp.Warnings = append(resp.Warnings, peopleWarnings...)

	tagNameToId, importedTags, skippedTags := importTags(ctx.Tx, importData.Tags, familyId)
	resp.ImportedTags = importedTags
	resp.SkippedTags = skippedTags

	if len(personIdMapping) > 0 {
		filteredHeights, filteredWeights := filterMeasurements(importData.Heights, importData.Weights, personIdMapping)
		importedMeasurements, skippedMeasurements, measurementErrors := importMeasurements(ctx.Tx, filteredHeights, filteredWeights, personIdMapping, familyId)
		resp.ImportedMeasurements = importedMeasurements
		resp.SkippedMeasurements = skippedMeasurements
		resp.Errors = append(resp.Errors, measurementErrors...)

		importedRelations, skippedRelations, relationErrors := importRelations(ctx.Tx, importData.Relations, personIdMapping)
		resp.ImportedRelations = importedRelations
		resp.SkippedRelations = skippedRelations
		resp.Errors = append(resp.Errors, relationErrors...)

		if req.ImportMilestones && len(importData.Milestones) > 0 {
			filteredMilestones := filterMilestones(importData.Milestones, personIdMapping)
			importedMilestones, skippedMilestones, milestoneErrors := importMilestones(ctx.Tx, filteredMilestones, personIdMapping, familyId, tagNameToId)
			resp.ImportedMilestones = importedMilestones
			resp.SkippedMilestones = skippedMilestones
			resp.Errors = append(resp.Errors, milestoneErrors...)
		}

		if req.ImportActivities && len(importData.Activities) > 0 {
			counts, activityWarnings := importActivities(ctx.Tx, importData.Activities, familyId, personIdMapping, nil)
			resp.ImportedActivities = counts
			resp.Warnings = append(resp.Warnings, activityWarnings...)
		}
	}

	resp.SkippedPeople = len(filteredPeople) - resp.ImportedPeople - resp.MergedPeople

	if !req.DryRun {
		vbolt.TxCommit(ctx.Tx)
	}

	return
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

	if milestone.MilestoneDate.Year() < 1900 || milestone.MilestoneDate.After(time.Now()) {
		return errors.New("Milestone at index " + formatIndex(index) + " has invalid date")
	}

	return nil
}

func formatIndex(index int) string {
	return fmt.Sprintf("%d", index)
}

func importPeople(tx *vbolt.Tx, importPeople []ImportPerson, familyId int, mergeStrategy string) (map[int]int, int, int, []string, []string) {
	personIdMapping := make(map[int]int)
	var errors []string
	var warnings []string
	importedCount := 0
	mergedCount := 0

	for _, importPerson := range importPeople {
		var existingPerson *Person
		if mergeStrategy == "merge_people" || mergeStrategy == "skip_duplicates" {
			existingPerson = findExistingPerson(tx, importPerson, familyId)
		}

		if existingPerson != nil {
			if mergeStrategy == "skip_duplicates" {
				warnings = append(warnings, "Skipped duplicate person: "+importPerson.Name)
				continue
			} else if mergeStrategy == "merge_people" {
				personIdMapping[importPerson.Id] = existingPerson.Id
				mergedCount++
				warnings = append(warnings, "Merged with existing person: "+importPerson.Name)
				continue
			}
		}

		var person Person
		person.Id = vbolt.NextIntId(tx, PeopleBkt)
		person.FamilyId = familyId
		person.Name = importPerson.Name
		person.Gender = GenderType(importPerson.Gender)
		person.Birthday = importPerson.Birthday
		person.Age = calculateAge(importPerson.Birthday)

		vbolt.Write(tx, PeopleBkt, person.Id, &person)
		updatePersonIndex(tx, person)

		personIdMapping[importPerson.Id] = person.Id
		importedCount++
	}

	return personIdMapping, importedCount, mergedCount, errors, warnings
}

func importMeasurements(tx *vbolt.Tx, importHeights []ImportHeight, importWeights []ImportWeight, personIdMapping map[int]int, familyId int) (int, int, []string) {
	var errors []string
	importedCount := 0
	skippedCount := 0

	for _, height := range importHeights {
		newPersonId, exists := personIdMapping[height.PersonId]
		if !exists {
			errors = append(errors, fmt.Sprintf("Height measurement for unknown person ID: %d", height.PersonId))
			continue
		}

		if height.Date.Year() == 1 {
			skippedCount++
			continue
		}

		if isDuplicateMeasurement(tx, newPersonId, height.Date, Height, height.Inches) {
			skippedCount++
			continue
		}

		var growthData GrowthData
		growthData.Id = vbolt.NextIntId(tx, GrowthDataBkt)
		growthData.PersonId = newPersonId
		growthData.FamilyId = familyId
		growthData.MeasurementType = Height
		growthData.Value = height.Inches
		growthData.Unit = "in"
		growthData.MeasurementDate = height.Date
		growthData.CreatedAt = time.Now()

		vbolt.Write(tx, GrowthDataBkt, growthData.Id, &growthData)
		updateGrowthDataIndices(tx, growthData)
		importedCount++
	}

	for _, weight := range importWeights {
		newPersonId, exists := personIdMapping[weight.PersonId]
		if !exists {
			errors = append(errors, fmt.Sprintf("Weight measurement for unknown person ID: %d", weight.PersonId))
			continue
		}

		if weight.Date.Year() == 1 {
			skippedCount++
			continue
		}

		if isDuplicateMeasurement(tx, newPersonId, weight.Date, Weight, weight.Pounds) {
			skippedCount++
			continue
		}

		var growthData GrowthData
		growthData.Id = vbolt.NextIntId(tx, GrowthDataBkt)
		growthData.PersonId = newPersonId
		growthData.FamilyId = familyId
		growthData.MeasurementType = Weight
		growthData.Value = weight.Pounds
		growthData.Unit = "lbs"
		growthData.MeasurementDate = weight.Date
		growthData.CreatedAt = time.Now()

		vbolt.Write(tx, GrowthDataBkt, growthData.Id, &growthData)
		updateGrowthDataIndices(tx, growthData)
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
			existing.Birthday.Equal(importPerson.Birthday) &&
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

func isDuplicateMilestone(tx *vbolt.Tx, personId int, date time.Time, description string) bool {
	milestones := GetPersonMilestonesTx(tx, personId)

	for _, milestone := range milestones {
		if milestone.MilestoneDate.Equal(date) &&
			milestone.Description == description {
			return true
		}
	}

	return false
}

func importMilestones(tx *vbolt.Tx, importMilestones []ExportMilestone, personIdMapping map[int]int, familyId int, tagNameToId map[string]int) (int, int, []string) {
	var errors []string
	importedCount := 0
	skippedCount := 0

	for _, milestone := range importMilestones {
		newPersonId, exists := personIdMapping[milestone.PersonId]
		if !exists {
			errors = append(errors, "Milestone for unknown person ID: "+string(rune(milestone.PersonId)))
			continue
		}

		if milestone.MilestoneDate.Year() == 1 {
			continue
		}

		if isDuplicateMilestone(tx, newPersonId, milestone.MilestoneDate, milestone.Description) {
			skippedCount++
			continue
		}

		var newMilestone Milestone
		newMilestone.Id = vbolt.NextIntId(tx, MilestoneBkt)
		newMilestone.PersonId = newPersonId
		newMilestone.FamilyId = familyId
		newMilestone.Description = milestone.Description
		newMilestone.Category = milestone.Category
		newMilestone.MilestoneDate = milestone.MilestoneDate
		newMilestone.CreatedAt = time.Now()

		vbolt.Write(tx, MilestoneBkt, newMilestone.Id, &newMilestone)

		vbolt.SetTargetSingleTerm(tx, MilestoneByPersonIndex, newMilestone.Id, newMilestone.PersonId)
		vbolt.SetTargetSingleTerm(tx, MilestoneByFamilyIndex, newMilestone.Id, newMilestone.FamilyId)

		for _, tagName := range milestone.TagNames {
			if tagId, ok := tagNameToId[strings.ToLower(tagName)]; ok {
				addTagToMilestone(tx, newMilestone.Id, tagId, familyId)
			}
		}

		importedCount++
	}

	return importedCount, skippedCount, errors
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

func importTags(tx *vbolt.Tx, exportedTags []ExportTag, familyId int) (map[string]int, int, int) {
	tagNameToId := make(map[string]int, len(exportedTags))
	importedCount := 0
	skippedCount := 0

	for _, exportTag := range exportedTags {
		lowerName := strings.ToLower(exportTag.Name)

		existingTags := getTagsByFamily(tx, familyId)
		var existingId int
		for _, t := range existingTags {
			if strings.ToLower(t.Name) == lowerName {
				existingId = t.Id
				break
			}
		}

		if existingId != 0 {
			tagNameToId[lowerName] = existingId
			skippedCount++
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
			tagNameToId[lowerName] = tag.Id
			importedCount++
		}
	}

	return tagNameToId, importedCount, skippedCount
}
