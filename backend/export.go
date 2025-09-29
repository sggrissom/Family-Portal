package backend

import (
	"encoding/json"
	"time"

	"go.hasen.dev/vbeam"
	"go.hasen.dev/vbolt"
)

func RegisterExportMethods(app *vbeam.Application) {
	vbeam.RegisterProc(app, ExportData)
}

// Export data types matching the import structure
type ExportDataStructure struct {
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

// Export milestone structure
type ExportMilestone struct {
	Id            int       `json:"id"`
	PersonId      int       `json:"personId"`
	FamilyId      int       `json:"familyId"`
	Description   string    `json:"description"`
	Category      string    `json:"category"`
	MilestoneDate time.Time `json:"milestoneDate"`
	CreatedAt     time.Time `json:"createdAt"`
	PersonName    string    `json:"personName"`
}

// Request/Response types
type ExportDataRequest struct {
	// No parameters needed - exports all family data
}

type ExportDataResponse struct {
	JsonData string `json:"jsonData"`
}

// vbeam procedure
func ExportData(ctx *vbeam.Context, req ExportDataRequest) (resp ExportDataResponse, err error) {
	// Get authenticated user
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}

	// Get all family data
	exportData, err := buildExportData(ctx.Tx, user.FamilyId)
	if err != nil {
		return
	}

	// Marshal to JSON
	jsonBytes, err := json.MarshalIndent(exportData, "", "  ")
	if err != nil {
		return
	}

	resp.JsonData = string(jsonBytes)
	return
}

// Helper function to build export data structure
func buildExportData(tx *vbolt.Tx, familyId int) (ExportDataStructure, error) {
	var exportData ExportDataStructure

	// Get all family people
	people := GetFamilyPeople(tx, familyId)

	// Convert people to export format
	exportData.People = make([]ImportPerson, len(people))
	for i, person := range people {
		exportData.People[i] = ImportPerson{
			Id:       person.Id,
			FamilyId: person.FamilyId,
			Type:     int(person.Type),
			Gender:   int(person.Gender),
			Name:     person.Name,
			Birthday: person.Birthday,
			Age:      person.Age,
			ImageId:  person.ProfilePhotoId,
		}
	}

	// Get all family growth data
	var growthDataIds []int
	vbolt.ReadTermTargets(tx, GrowthDataByFamilyIndex, familyId, &growthDataIds, vbolt.Window{})

	var growthData []GrowthData
	if len(growthDataIds) > 0 {
		vbolt.ReadSlice(tx, GrowthDataBkt, growthDataIds, &growthData)
	}

	// Separate heights and weights, convert to export format
	var heights []ImportHeight
	var weights []ImportWeight

	for _, gd := range growthData {
		// Get person name for the measurement
		var personName string
		for _, person := range people {
			if person.Id == gd.PersonId {
				personName = person.Name
				break
			}
		}

		// Calculate age at measurement date
		var personBirthday time.Time
		for _, person := range people {
			if person.Id == gd.PersonId {
				personBirthday = person.Birthday
				break
			}
		}

		age := calculateAgeAtDate(personBirthday, gd.MeasurementDate)
		dateString := gd.MeasurementDate.Format("2006-01-02")

		if gd.MeasurementType == Height {
			// Convert to inches if needed
			inches := gd.Value
			if gd.Unit == "cm" {
				inches = gd.Value / 2.54
			}

			heights = append(heights, ImportHeight{
				Id:         gd.Id,
				PersonId:   gd.PersonId,
				Inches:     inches,
				Date:       gd.MeasurementDate,
				DateString: dateString,
				Age:        age,
				PersonName: personName,
			})
		} else if gd.MeasurementType == Weight {
			// Convert to pounds if needed
			pounds := gd.Value
			if gd.Unit == "kg" {
				pounds = gd.Value * 2.20462
			}

			weights = append(weights, ImportWeight{
				Id:         gd.Id,
				PersonId:   gd.PersonId,
				Pounds:     pounds,
				Date:       gd.MeasurementDate,
				DateString: dateString,
				Age:        age,
				PersonName: personName,
			})
		}
	}

	// Get all family milestones
	var milestoneIds []int
	vbolt.ReadTermTargets(tx, MilestoneByFamilyIndex, familyId, &milestoneIds, vbolt.Window{})

	var milestones []Milestone
	if len(milestoneIds) > 0 {
		vbolt.ReadSlice(tx, MilestoneBkt, milestoneIds, &milestones)
	}

	// Convert milestones to export format
	exportMilestones := make([]ExportMilestone, len(milestones))
	for i, milestone := range milestones {
		// Get person name for the milestone
		var personName string
		for _, person := range people {
			if person.Id == milestone.PersonId {
				personName = person.Name
				break
			}
		}

		exportMilestones[i] = ExportMilestone{
			Id:            milestone.Id,
			PersonId:      milestone.PersonId,
			FamilyId:      milestone.FamilyId,
			Description:   milestone.Description,
			Category:      milestone.Category,
			MilestoneDate: milestone.MilestoneDate,
			CreatedAt:     milestone.CreatedAt,
			PersonName:    personName,
		}
	}

	// Set export data
	exportData.Heights = heights
	exportData.Weights = weights
	exportData.Milestones = exportMilestones
	exportData.ExportDate = time.Now()
	exportData.TotalHeights = len(heights)
	exportData.TotalWeights = len(weights)
	exportData.TotalPeople = len(people)
	exportData.TotalMilestones = len(milestones)

	return exportData, nil
}

// Helper function to calculate age at a specific date
func calculateAgeAtDate(birthday, targetDate time.Time) float64 {
	years := targetDate.Year() - birthday.Year()
	months := int(targetDate.Month()) - int(birthday.Month())
	days := targetDate.Day() - birthday.Day()

	if days < 0 {
		months--
	}
	if months < 0 {
		years--
		months += 12
	}

	return float64(years) + float64(months)/12.0
}
