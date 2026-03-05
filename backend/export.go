package backend

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"go.hasen.dev/vbeam"
	"go.hasen.dev/vbolt"
)

func RegisterExportMethods(app *vbeam.Application) {
	vbeam.RegisterProc(app, ExportData)
	app.HandleFunc("GET /api/export-bundle", AuthMiddleware(exportBundleHandler))
}

func exportBundleHandler(w http.ResponseWriter, r *http.Request) {
	user, ok := GetUserFromContext(r)
	if !ok {
		RespondAuthError(w, r, "Authentication required")
		return
	}

	mode := r.URL.Query().Get("mode")
	if mode == "" {
		mode = "data_only"
	}
	if mode != "data_only" {
		RespondValidationError(w, r, "Export mode not yet supported", mode)
		return
	}

	var exportData ExportDataStructure
	var buildErr error
	vbolt.WithReadTx(appDb, func(tx *vbolt.Tx) {
		exportData, buildErr = buildExportData(tx, user.FamilyId)
	})
	if buildErr != nil {
		RespondInternalError(w, r, "Failed to build export data", buildErr.Error())
		return
	}

	jsonBytes, err := json.MarshalIndent(exportData, "", "  ")
	if err != nil {
		RespondInternalError(w, r, "Failed to marshal export data", err.Error())
		return
	}

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	f, err := zw.Create("data.json")
	if err != nil {
		RespondInternalError(w, r, "Failed to create ZIP entry", err.Error())
		return
	}
	f.Write(jsonBytes)
	zw.Close()

	filename := fmt.Sprintf("family-export-%s.zip", time.Now().Format("2006-01-02"))
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	w.Header().Set("Content-Type", "application/zip")
	w.Write(buf.Bytes())
}

// Export tag structure
type ExportTag struct {
	Id    int    `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color"`
}

// Export data types matching the import structure
type ExportDataStructure struct {
	People          []ImportPerson    `json:"people"`
	Heights         []ImportHeight    `json:"heights"`
	Weights         []ImportWeight    `json:"weights"`
	Milestones      []ExportMilestone `json:"milestones"`
	Tags            []ExportTag       `json:"tags"`
	ExportDate      time.Time         `json:"export_date"`
	TotalHeights    int               `json:"total_heights"`
	TotalWeights    int               `json:"total_weights"`
	TotalPeople     int               `json:"total_people"`
	TotalMilestones int               `json:"total_milestones"`
	TotalTags       int               `json:"total_tags"`
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
	TagNames      []string  `json:"tagNames,omitempty"`
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

	// Get all family tags and build id->name map
	tags := getTagsByFamily(tx, familyId)
	tagIdToName := make(map[int]string, len(tags))
	exportTags := make([]ExportTag, len(tags))
	for i, tag := range tags {
		tagIdToName[tag.Id] = tag.Name
		exportTags[i] = ExportTag{Id: tag.Id, Name: tag.Name, Color: tag.Color}
	}

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

		tagIds := GetMilestoneTagIds(tx, milestone.Id)
		var tagNames []string
		for _, tagId := range tagIds {
			if name, ok := tagIdToName[tagId]; ok {
				tagNames = append(tagNames, name)
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
			TagNames:      tagNames,
		}
	}

	// Set export data
	exportData.Heights = heights
	exportData.Weights = weights
	exportData.Milestones = exportMilestones
	exportData.Tags = exportTags
	exportData.ExportDate = time.Now()
	exportData.TotalHeights = len(heights)
	exportData.TotalWeights = len(weights)
	exportData.TotalPeople = len(people)
	exportData.TotalMilestones = len(milestones)
	exportData.TotalTags = len(tags)

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
