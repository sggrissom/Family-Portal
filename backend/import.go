package backend

import (
	"encoding/json"
	"errors"
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
	People       []ImportPerson `json:"people"`
	Heights      []ImportHeight `json:"heights"`
	Weights      []ImportWeight `json:"weights"`
	ExportDate   time.Time      `json:"export_date"`
	TotalHeights int            `json:"total_heights"`
	TotalWeights int            `json:"total_weights"`
	TotalPeople  int            `json:"total_people"`
}

// Request/Response types
type ImportDataRequest struct {
	JsonData        string `json:"jsonData"`
	FilterFamilyIds []int  `json:"filterFamilyIds,omitempty"` // Only import people from these family IDs
	FilterPersonIds []int  `json:"filterPersonIds,omitempty"` // Only import these specific person IDs
	PreviewOnly     bool   `json:"previewOnly,omitempty"`     // If true, just return available data without importing
}

type ImportDataResponse struct {
	ImportedPeople       int            `json:"importedPeople"`
	ImportedMeasurements int            `json:"importedMeasurements"`
	Errors               []string       `json:"errors,omitempty"`
	PersonIdMapping      map[int]int    `json:"personIdMapping,omitempty"`
	AvailableFamilyIds   []int          `json:"availableFamilyIds,omitempty"`
	AvailablePeople      []ImportPerson `json:"availablePeople,omitempty"`
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

	// Get available families and people for preview
	resp.AvailableFamilyIds = getUniqueFamilyIds(importData.People)
	resp.AvailablePeople = importData.People

	// If preview only, return the available data without importing
	if req.PreviewOnly {
		return
	}

	// Filter data based on request
	filteredPeople := filterPeople(importData.People, req.FilterFamilyIds, req.FilterPersonIds)
	if len(filteredPeople) == 0 {
		err = errors.New("No people match the specified filters")
		return
	}

	// Start write transaction
	vbeam.UseWriteTx(ctx)

	// Import people first to establish ID mappings
	personIdMapping, peopleErrors := importPeople(ctx.Tx, filteredPeople, user.FamilyId)
	resp.ImportedPeople = len(personIdMapping)
	resp.PersonIdMapping = personIdMapping
	resp.Errors = append(resp.Errors, peopleErrors...)

	// Import measurements using the person ID mappings (filter by imported people)
	filteredHeights, filteredWeights := filterMeasurements(importData.Heights, importData.Weights, personIdMapping)
	measurementCount, measurementErrors := importMeasurements(ctx.Tx, filteredHeights, filteredWeights, personIdMapping, user.FamilyId)
	resp.ImportedMeasurements = measurementCount
	resp.Errors = append(resp.Errors, measurementErrors...)

	// Commit transaction
	vbolt.TxCommit(ctx.Tx)

	return
}

func validateImportData(data ImportDataStructure) error {
	if len(data.People) == 0 {
		return errors.New("No people found in import data")
	}

	// Basic validation - ensure people have required fields
	for i, person := range data.People {
		if person.Name == "" {
			return errors.New("Person at index " + string(rune(i)) + " has no name")
		}
		if person.Birthday.IsZero() {
			return errors.New("Person '" + person.Name + "' has invalid birthday")
		}
	}

	return nil
}

func importPeople(tx *vbolt.Tx, importPeople []ImportPerson, familyId int) (map[int]int, []string) {
	personIdMapping := make(map[int]int)
	var errors []string

	for _, importPerson := range importPeople {
		// Create new person with new ID
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
	}

	return personIdMapping, errors
}

func importMeasurements(tx *vbolt.Tx, importHeights []ImportHeight, importWeights []ImportWeight, personIdMapping map[int]int, familyId int) (int, []string) {
	var errors []string
	measurementCount := 0

	// Import heights
	for _, height := range importHeights {
		newPersonId, exists := personIdMapping[height.PersonId]
		if !exists {
			errors = append(errors, "Height measurement for unknown person ID: "+string(rune(height.PersonId)))
			continue
		}

		// Skip measurements with invalid dates (year 0001)
		if height.Date.Year() == 1 {
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
		measurementCount++
	}

	// Import weights
	for _, weight := range importWeights {
		newPersonId, exists := personIdMapping[weight.PersonId]
		if !exists {
			errors = append(errors, "Weight measurement for unknown person ID: "+string(rune(weight.PersonId)))
			continue
		}

		// Skip measurements with invalid dates (year 0001)
		if weight.Date.Year() == 1 {
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
		measurementCount++
	}

	return measurementCount, errors
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
