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
}

// Import data types matching the JSON structure
type ImportPerson struct {
	Id       int       `json:"Id"`
	FamilyId int       `json:"FamilyId"`
	Type     int       `json:"Type"`
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
	Heights         []ImportHeight    `json:"heights"`
	Weights         []ImportWeight    `json:"weights"`
	Milestones      []ExportMilestone `json:"milestones"`
	ExportDate      time.Time         `json:"export_date"`
	TotalHeights    int               `json:"total_heights"`
	TotalWeights    int               `json:"total_weights"`
	TotalPeople     int               `json:"total_people"`
	TotalMilestones int               `json:"total_milestones"`
}

// Request/Response types
type ImportDataRequest struct {
	JsonData         string `json:"jsonData"`
	FilterFamilyIds  []int  `json:"filterFamilyIds,omitempty"`  // Only import people from these family IDs
	FilterPersonIds  []int  `json:"filterPersonIds,omitempty"`  // Only import these specific person IDs
	PreviewOnly      bool   `json:"previewOnly,omitempty"`      // If true, just return available data without importing
	MergeStrategy    string `json:"mergeStrategy,omitempty"`    // "create_all", "merge_people", or "skip_duplicates"
	ImportMilestones bool   `json:"importMilestones,omitempty"` // Whether to import milestones
	DryRun           bool   `json:"dryRun,omitempty"`           // Preview changes without committing
}

type ImportDataResponse struct {
	ImportedPeople       int            `json:"importedPeople"`
	MergedPeople         int            `json:"mergedPeople"`
	SkippedPeople        int            `json:"skippedPeople"`
	ImportedMeasurements int            `json:"importedMeasurements"`
	SkippedMeasurements  int            `json:"skippedMeasurements"`
	ImportedMilestones   int            `json:"importedMilestones"`
	SkippedMilestones    int            `json:"skippedMilestones"`
	Errors               []string       `json:"errors,omitempty"`
	Warnings             []string       `json:"warnings,omitempty"`
	PersonIdMapping      map[int]int    `json:"personIdMapping,omitempty"`
	AvailableFamilyIds   []int          `json:"availableFamilyIds,omitempty"`
	AvailablePeople      []ImportPerson `json:"availablePeople,omitempty"`
	MatchedPeople        []PersonMatch  `json:"matchedPeople,omitempty"`
}

// vbeam procedure
func ImportData(ctx *vbeam.Context, req ImportDataRequest) (resp ImportDataResponse, err error) {
	// Get authenticated user
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}

	// Parse JSON data
	var importData ImportDataStructure
	if err = json.Unmarshal([]byte(req.JsonData), &importData); err != nil {
		err = errors.New("Invalid JSON data: " + err.Error())
		return
	}

	// Validate import data
	if err = validateImportData(importData); err != nil {
		return
	}

	// Set default merge strategy if not provided
	mergeStrategy := req.MergeStrategy
	if mergeStrategy == "" {
		mergeStrategy = "create_all"
	}

	// Get available families and people for preview
	resp.AvailableFamilyIds = getUniqueFamilyIds(importData.People)
	resp.AvailablePeople = importData.People

	// For preview mode, show potential matches
	if req.PreviewOnly {
		filteredPeople := filterPeople(importData.People, req.FilterFamilyIds, req.FilterPersonIds)
		for _, importPerson := range filteredPeople {
			matches := findPotentialMatches(ctx.Tx, importPerson, user.FamilyId)
			if len(matches) > 0 {
				resp.MatchedPeople = append(resp.MatchedPeople, matches...)
			} else {
				// No matches found
				resp.MatchedPeople = append(resp.MatchedPeople, PersonMatch{
					ImportPerson: importPerson,
					MatchType:    "none",
					Confidence:   0.0,
				})
			}
		}
		return
	}

	// Filter data based on request
	filteredPeople := filterPeople(importData.People, req.FilterFamilyIds, req.FilterPersonIds)
	if len(filteredPeople) == 0 {
		err = errors.New("No people match the specified filters")
		return
	}

	// Start write transaction (or dry run)
	if !req.DryRun {
		vbeam.UseWriteTx(ctx)
	}

	// Import people first to establish ID mappings
	personIdMapping, importedPeople, mergedPeople, peopleErrors, peopleWarnings := importPeople(ctx.Tx, filteredPeople, user.FamilyId, mergeStrategy)
	resp.ImportedPeople = importedPeople
	resp.MergedPeople = mergedPeople
	resp.PersonIdMapping = personIdMapping
	resp.Errors = append(resp.Errors, peopleErrors...)
	resp.Warnings = append(resp.Warnings, peopleWarnings...)

	// Only proceed with data import if we have people to import to
	if len(personIdMapping) > 0 {
		// Import measurements using the person ID mappings (filter by imported people)
		filteredHeights, filteredWeights := filterMeasurements(importData.Heights, importData.Weights, personIdMapping)
		importedMeasurements, skippedMeasurements, measurementErrors := importMeasurements(ctx.Tx, filteredHeights, filteredWeights, personIdMapping, user.FamilyId)
		resp.ImportedMeasurements = importedMeasurements
		resp.SkippedMeasurements = skippedMeasurements
		resp.Errors = append(resp.Errors, measurementErrors...)

		// Import milestones if requested
		if req.ImportMilestones && len(importData.Milestones) > 0 {
			filteredMilestones := filterMilestones(importData.Milestones, personIdMapping)
			importedMilestones, skippedMilestones, milestoneErrors := importMilestones(ctx.Tx, filteredMilestones, personIdMapping, user.FamilyId)
			resp.ImportedMilestones = importedMilestones
			resp.SkippedMilestones = skippedMilestones
			resp.Errors = append(resp.Errors, milestoneErrors...)
		}
	}

	// Calculate skipped people
	resp.SkippedPeople = len(filteredPeople) - resp.ImportedPeople - resp.MergedPeople

	// Commit transaction (unless dry run)
	if !req.DryRun {
		vbolt.TxCommit(ctx.Tx)
	}

	return
}

func validateImportData(data ImportDataStructure) error {
	if len(data.People) == 0 {
		return errors.New("No people found in import data")
	}

	// Validate people
	for i, person := range data.People {
		if err := validateImportPerson(person, i); err != nil {
			return err
		}
	}

	// Validate measurements
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

	// Validate milestones
	for i, milestone := range data.Milestones {
		if err := validateImportMilestone(milestone, i, personIds); err != nil {
			return err
		}
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

	// Check if birthday is reasonable (not in the future, not too far in the past)
	now := time.Now()
	if person.Birthday.After(now) {
		return errors.New("Person '" + person.Name + "' has birthday in the future")
	}

	// Check if birthday is not more than 150 years ago
	maxAge := now.AddDate(-150, 0, 0)
	if person.Birthday.Before(maxAge) {
		return errors.New("Person '" + person.Name + "' has birthday more than 150 years ago")
	}

	// Validate gender
	if person.Gender < 0 || person.Gender > 2 {
		return errors.New("Person '" + person.Name + "' has invalid gender value")
	}

	// Validate person type
	if person.Type < 0 || person.Type > 1 {
		return errors.New("Person '" + person.Name + "' has invalid person type")
	}

	return nil
}

func validateImportHeight(height ImportHeight, index int, validPersonIds map[int]bool) error {
	if !validPersonIds[height.PersonId] {
		return errors.New("Height measurement at index " + formatIndex(index) + " references unknown person ID " + formatIndex(height.PersonId))
	}

	if height.Inches <= 0 || height.Inches > 120 { // 120 inches = 10 feet, reasonable max
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

	if weight.Pounds <= 0 || weight.Pounds > 2000 { // 2000 lbs is reasonable max
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
		// Check for existing person based on merge strategy
		var existingPerson *Person
		if mergeStrategy == "merge_people" || mergeStrategy == "skip_duplicates" {
			existingPerson = findExistingPerson(tx, importPerson, familyId)
		}

		if existingPerson != nil {
			// Found existing person
			if mergeStrategy == "skip_duplicates" {
				// Skip this person entirely
				warnings = append(warnings, "Skipped duplicate person: "+importPerson.Name)
				continue
			} else if mergeStrategy == "merge_people" {
				// Use existing person's ID
				personIdMapping[importPerson.Id] = existingPerson.Id
				mergedCount++
				warnings = append(warnings, "Merged with existing person: "+importPerson.Name)
				continue
			}
		}

		// Create new person (either no match found, or using create_all strategy)
		var person Person
		person.Id = vbolt.NextIntId(tx, PeopleBkt)
		person.FamilyId = familyId
		person.Name = importPerson.Name
		person.Type = PersonType(importPerson.Type)
		person.Gender = GenderType(importPerson.Gender)
		person.Birthday = importPerson.Birthday
		person.Age = calculateAge(importPerson.Birthday)

		// Store in database
		vbolt.Write(tx, PeopleBkt, person.Id, &person)
		updatePersonIndex(tx, person)

		// Map old ID to new ID
		personIdMapping[importPerson.Id] = person.Id
		importedCount++
	}

	return personIdMapping, importedCount, mergedCount, errors, warnings
}

func importMeasurements(tx *vbolt.Tx, importHeights []ImportHeight, importWeights []ImportWeight, personIdMapping map[int]int, familyId int) (int, int, []string) {
	var errors []string
	importedCount := 0
	skippedCount := 0

	// Import heights
	for _, height := range importHeights {
		newPersonId, exists := personIdMapping[height.PersonId]
		if !exists {
			errors = append(errors, fmt.Sprintf("Height measurement for unknown person ID: %d", height.PersonId))
			continue
		}

		// Skip measurements with invalid dates (year 0001)
		if height.Date.Year() == 1 {
			skippedCount++
			continue
		}

		// Check for duplicate measurement
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

	// Import weights
	for _, weight := range importWeights {
		newPersonId, exists := personIdMapping[weight.PersonId]
		if !exists {
			errors = append(errors, fmt.Sprintf("Weight measurement for unknown person ID: %d", weight.PersonId))
			continue
		}

		// Skip measurements with invalid dates (year 0001)
		if weight.Date.Year() == 1 {
			skippedCount++
			continue
		}

		// Check for duplicate measurement
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
	// If no filters specified, return all people
	if len(filterFamilyIds) == 0 && len(filterPersonIds) == 0 {
		return people
	}

	var filtered []ImportPerson

	// Create lookup maps for efficiency
	familyIdMap := make(map[int]bool)
	for _, id := range filterFamilyIds {
		familyIdMap[id] = true
	}

	personIdMap := make(map[int]bool)
	for _, id := range filterPersonIds {
		personIdMap[id] = true
	}

	for _, person := range people {
		// Include if matches family ID filter (if specified)
		matchesFamilyFilter := len(filterFamilyIds) == 0 || familyIdMap[person.FamilyId]

		// Include if matches person ID filter (if specified)
		matchesPersonFilter := len(filterPersonIds) == 0 || personIdMap[person.Id]

		// Include if matches both filters (AND logic)
		if matchesFamilyFilter && (len(filterPersonIds) == 0 || matchesPersonFilter) {
			filtered = append(filtered, person)
		}
	}

	return filtered
}

func filterMeasurements(heights []ImportHeight, weights []ImportWeight, personIdMapping map[int]int) ([]ImportHeight, []ImportWeight) {
	var filteredHeights []ImportHeight
	var filteredWeights []ImportWeight

	// Only include measurements for people that were imported
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

	// Only include milestones for people that were imported
	for _, milestone := range milestones {
		if _, exists := personIdMapping[milestone.PersonId]; exists {
			filteredMilestones = append(filteredMilestones, milestone)
		}
	}

	return filteredMilestones
}

// Person matching structure for preview
type PersonMatch struct {
	ImportPerson   ImportPerson `json:"importPerson"`
	ExistingPerson *Person      `json:"existingPerson,omitempty"`
	MatchType      string       `json:"matchType"` // "exact", "potential", "none"
	Confidence     float64      `json:"confidence"`
}

// Find existing person by matching attributes
func findExistingPerson(tx *vbolt.Tx, importPerson ImportPerson, familyId int) *Person {
	// Get all family people
	familyPeople := GetFamilyPeople(tx, familyId)

	for _, existing := range familyPeople {
		// Exact match: same name, birthday, and gender
		if existing.Name == importPerson.Name &&
			existing.Birthday.Equal(importPerson.Birthday) &&
			existing.Gender == GenderType(importPerson.Gender) {
			return &existing
		}
	}

	return nil
}

// Find potential matches with confidence scoring
func findPotentialMatches(tx *vbolt.Tx, importPerson ImportPerson, familyId int) []PersonMatch {
	var matches []PersonMatch
	familyPeople := GetFamilyPeople(tx, familyId)

	for _, existing := range familyPeople {
		confidence := calculateMatchConfidence(importPerson, existing)
		if confidence > 0.3 { // Only include potential matches above 30% confidence
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

// Calculate match confidence between import and existing person
func calculateMatchConfidence(importPerson ImportPerson, existing Person) float64 {
	score := 0.0
	totalFactors := 0.0

	// Name similarity (40% weight)
	if importPerson.Name == existing.Name {
		score += 0.4
	} else if strings.EqualFold(importPerson.Name, existing.Name) {
		score += 0.35 // Case-insensitive match
	} else {
		// Calculate fuzzy string similarity here if needed
		// For now, just check if names contain each other
		name1 := strings.ToLower(importPerson.Name)
		name2 := strings.ToLower(existing.Name)
		if strings.Contains(name1, name2) || strings.Contains(name2, name1) {
			score += 0.2
		}
	}
	totalFactors += 0.4

	// Birthday match (40% weight)
	if importPerson.Birthday.Equal(existing.Birthday) {
		score += 0.4
	} else {
		// Check if birthdays are close (within a few days)
		daysDiff := importPerson.Birthday.Sub(existing.Birthday).Hours() / 24
		if daysDiff < 0 {
			daysDiff = -daysDiff
		}
		if daysDiff <= 3 {
			score += 0.3 // Close birthday
		} else if daysDiff <= 7 {
			score += 0.1 // Nearby birthday
		}
	}
	totalFactors += 0.4

	// Gender match (20% weight)
	if GenderType(importPerson.Gender) == existing.Gender {
		score += 0.2
	}
	totalFactors += 0.2

	return score / totalFactors
}

// Check if measurement already exists
func isDuplicateMeasurement(tx *vbolt.Tx, personId int, date time.Time, measurementType MeasurementType, value float64) bool {
	growthData := GetPersonGrowthDataTx(tx, personId)

	for _, measurement := range growthData {
		if measurement.MeasurementType == measurementType &&
			measurement.MeasurementDate.Equal(date) &&
			abs(measurement.Value-value) < 0.01 { // Allow small floating point differences
			return true
		}
	}

	return false
}

// Helper function for absolute value of float64
func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

// Check if milestone already exists
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

// Import milestones with deduplication
func importMilestones(tx *vbolt.Tx, importMilestones []ExportMilestone, personIdMapping map[int]int, familyId int) (int, int, []string) {
	var errors []string
	importedCount := 0
	skippedCount := 0

	for _, milestone := range importMilestones {
		newPersonId, exists := personIdMapping[milestone.PersonId]
		if !exists {
			errors = append(errors, "Milestone for unknown person ID: "+string(rune(milestone.PersonId)))
			continue
		}

		// Skip milestones with invalid dates (year 0001)
		if milestone.MilestoneDate.Year() == 1 {
			continue
		}

		// Check for duplicate milestone
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

		// Update indices
		vbolt.SetTargetSingleTerm(tx, MilestoneByPersonIndex, newMilestone.Id, newMilestone.PersonId)
		vbolt.SetTargetSingleTerm(tx, MilestoneByFamilyIndex, newMilestone.Id, newMilestone.FamilyId)

		importedCount++
	}

	return importedCount, skippedCount, errors
}
