package backend

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"family/cfg"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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
	if mode != "data_only" && mode != "with_photos" {
		RespondValidationError(w, r, "Choose either a data-only export or one with photos.", mode)
		return
	}

	var requestedFamilyId int
	if familyIdStr := r.URL.Query().Get("familyId"); familyIdStr != "" {
		parsed, convErr := strconv.Atoi(familyIdStr)
		if convErr != nil {
			RespondValidationError(w, r, "That family could not be identified.", convErr.Error())
			return
		}
		requestedFamilyId = parsed
	}

	var exportData ExportDataStructure
	var buildErr error
	vbolt.WithReadTx(appDb, func(tx *vbolt.Tx) {
		familyId, resolveErr := ResolveActingFamily(tx, user, requestedFamilyId, AccessView)
		if resolveErr != nil {
			buildErr = resolveErr
			return
		}
		exportData, buildErr = buildExportData(tx, familyId)
		if buildErr != nil {
			return
		}
		if mode == "with_photos" {
			photos := buildPhotoExportMetadata(tx, familyId)
			exportData.Photos = photos
			exportData.TotalPhotos = len(photos)
		}
	})
	if buildErr != nil {
		if errors.Is(buildErr, ErrFamilyAccessDenied) || errors.Is(buildErr, ErrNoFamily) {
			RespondForbiddenError(w, r, "You do not have access to that family's records.")
			return
		}
		RespondUnexpectedError(w, r, buildErr)
		return
	}

	jsonBytes, err := json.MarshalIndent(exportData, "", "  ")
	if err != nil {
		RespondUnexpectedError(w, r, err)
		return
	}

	filename := fmt.Sprintf("family-export-%s.zip", time.Now().Format("2006-01-02"))
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	w.Header().Set("Content-Type", "application/zip")

	if mode == "data_only" {
		var buf bytes.Buffer
		zw := zip.NewWriter(&buf)
		f, err := zw.Create("data.json")
		if err != nil {
			RespondUnexpectedError(w, r, err)
			return
		}
		f.Write(jsonBytes)
		zw.Close()
		w.Write(buf.Bytes())
		return
	}

	zw := zip.NewWriter(w)
	defer zw.Close()

	dataEntry, err := zw.Create("data.json")
	if err != nil {
		log.Printf("[EXPORT] Failed to create data.json ZIP entry: %v", err)
		return
	}
	if _, err := dataEntry.Write(jsonBytes); err != nil {
		log.Printf("[EXPORT] Failed to write data.json: %v", err)
		return
	}

	for _, ep := range exportData.Photos {
		diskPath := filepath.Join(cfg.StaticDir, ep.ZipPath)
		f, err := os.Open(diskPath)
		if err != nil {
			log.Printf("[EXPORT] Skipping photo %d (%s): %v", ep.Id, diskPath, err)
			continue
		}
		entry, err := zw.Create(ep.ZipPath)
		if err != nil {
			log.Printf("[EXPORT] Failed to create ZIP entry for photo %d: %v", ep.Id, err)
			f.Close()
			continue
		}
		if _, err := io.Copy(entry, f); err != nil {
			log.Printf("[EXPORT] Failed to write photo %d to ZIP: %v", ep.Id, err)
			f.Close()
			return
		}
		f.Close()
	}
}

func buildPhotoExportMetadata(tx *vbolt.Tx, familyId int) []ExportPhoto {
	images := GetFamilyImages(tx, familyId)
	result := make([]ExportPhoto, 0, len(images))
	for _, img := range images {
		if img.Status != 0 {
			continue
		}
		baseName := strings.TrimSuffix(filepath.Base(img.FilePath), filepath.Ext(img.FilePath))
		ext := filepath.Ext(img.FilePath)
		zipPath := fmt.Sprintf("photos/%s_original%s", baseName, ext)

		photoPersons := GetPhotoPersonsByPhoto(tx, img.Id)
		personIds := make([]int, 0, len(photoPersons))
		for _, pp := range photoPersons {
			personIds = append(personIds, pp.PersonId)
		}

		result = append(result, ExportPhoto{
			Id:          img.Id,
			Title:       img.Title,
			Description: img.Description,
			PhotoDate:   img.PhotoDate,
			ZipPath:     zipPath,
			PersonIds:   personIds,
			TagIds:      GetPhotoTagIds(tx, img.Id),
		})
	}
	return result
}

type ExportTag struct {
	Id    int    `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color"`
}

// ExportRelation is one stored relationship edge. Kind is spelled out rather
// than numbered so the file does not depend on the enum's ordering. For a
// "parent" edge From is the parent of To; "sibling" and "partner" are
// symmetric and stored once, in the direction they were entered.
type ExportRelation struct {
	Id       int    `json:"id"`
	FromId   int    `json:"fromId"`
	ToId     int    `json:"toId"`
	Kind     string `json:"kind"`
	FromName string `json:"fromName,omitempty"`
	ToName   string `json:"toName,omitempty"`
}

type ExportPhoto struct {
	Id          int       `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	PhotoDate   time.Time `json:"photo_date"`
	ZipPath     string    `json:"zip_path"`
	PersonIds   []int     `json:"person_ids"`
	TagIds      []int     `json:"tag_ids"`
}

type ExportDataStructure struct {
	People          []ImportPerson    `json:"people"`
	Relations       []ExportRelation  `json:"relations"`
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
	TotalRelations  int               `json:"total_relations"`
	TotalMilestones int               `json:"total_milestones"`
	TotalTags       int               `json:"total_tags"`
	TotalPhotos     int               `json:"total_photos,omitempty"`

	TotalActivities  int `json:"total_activities,omitempty"`
	TotalSeasons     int `json:"total_seasons,omitempty"`
	TotalEvents      int `json:"total_events,omitempty"`
	TotalEntries     int `json:"total_entries,omitempty"`
	TotalAppearances int `json:"total_appearances,omitempty"`
	TotalResults     int `json:"total_results,omitempty"`
}

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

type ExportDataRequest struct {
	FamilyId int `json:"familyId,omitempty"`
}

type ExportDataResponse struct {
	JsonData string `json:"jsonData"`
}

func ExportData(ctx *vbeam.Context, req ExportDataRequest) (resp ExportDataResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}

	familyId, err := ResolveActingFamily(ctx.Tx, user, req.FamilyId, AccessView)
	if err != nil {
		return
	}

	exportData, err := buildExportData(ctx.Tx, familyId)
	if err != nil {
		return
	}

	jsonBytes, err := json.MarshalIndent(exportData, "", "  ")
	if err != nil {
		return
	}

	resp.JsonData = string(jsonBytes)
	return
}

func buildExportData(tx *vbolt.Tx, familyId int) (ExportDataStructure, error) {
	var exportData ExportDataStructure

	tags := getTagsByFamily(tx, familyId)
	tagIdToName := make(map[int]string, len(tags))
	exportTags := make([]ExportTag, len(tags))
	for i, tag := range tags {
		tagIdToName[tag.Id] = tag.Name
		exportTags[i] = ExportTag{Id: tag.Id, Name: tag.Name, Color: tag.Color}
	}

	people := GetFamilyOwnPeople(tx, familyId)

	exportData.People = make([]ImportPerson, len(people))
	for i, person := range people {
		exportData.People[i] = ImportPerson{
			Id:       person.Id,
			FamilyId: person.FamilyId,
			Gender:   int(person.Gender),
			Name:     person.Name,
			Birthday: person.Birthday,
			Age:      person.Age,
			ImageId:  person.ProfilePhotoId,
		}
	}

	var growthDataIds []int
	vbolt.ReadTermTargets(tx, GrowthDataByFamilyIndex, familyId, &growthDataIds, vbolt.Window{})

	var growthData []GrowthData
	if len(growthDataIds) > 0 {
		vbolt.ReadSlice(tx, GrowthDataBkt, growthDataIds, &growthData)
	}

	var heights []ImportHeight
	var weights []ImportWeight

	for _, gd := range growthData {
		var personName string
		for _, person := range people {
			if person.Id == gd.PersonId {
				personName = person.Name
				break
			}
		}

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

	var milestoneIds []int
	vbolt.ReadTermTargets(tx, MilestoneByFamilyIndex, familyId, &milestoneIds, vbolt.Window{})

	var milestones []Milestone
	if len(milestoneIds) > 0 {
		vbolt.ReadSlice(tx, MilestoneBkt, milestoneIds, &milestones)
	}

	exportMilestones := make([]ExportMilestone, len(milestones))
	for i, milestone := range milestones {
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

	personNames := make(map[int]string, len(people))
	for _, person := range people {
		personNames[person.Id] = person.Name
	}

	exportData.Relations = buildRelationExport(tx, people, personNames)

	exportData.Activities = buildActivityExport(tx, familyId, personNames)
	seasonCount, eventCount, entryCount, appearanceCount, resultCount :=
		countExportedActivities(exportData.Activities)

	exportData.Heights = heights
	exportData.Weights = weights
	exportData.Milestones = exportMilestones
	exportData.Tags = exportTags
	exportData.ExportDate = time.Now()
	exportData.TotalHeights = len(heights)
	exportData.TotalWeights = len(weights)
	exportData.TotalPeople = len(people)
	exportData.TotalRelations = len(exportData.Relations)
	exportData.TotalMilestones = len(milestones)
	exportData.TotalTags = len(tags)
	exportData.TotalActivities = len(exportData.Activities)
	exportData.TotalSeasons = seasonCount
	exportData.TotalEvents = eventCount
	exportData.TotalEntries = entryCount
	exportData.TotalAppearances = appearanceCount
	exportData.TotalResults = resultCount

	return exportData, nil
}

// buildRelationExport lists the edges whose endpoints are both people in this
// export. An edge to someone in another family is left out, since the import
// side has no record to reattach it to.
func buildRelationExport(tx *vbolt.Tx, people []Person, personNames map[int]string) []ExportRelation {
	rows := relationsAmong(tx, people)
	result := make([]ExportRelation, 0, len(rows))
	for _, row := range rows {
		kind := row.Kind.exportName()
		if kind == "" {
			continue
		}
		result = append(result, ExportRelation{
			Id:       row.Id,
			FromId:   row.FromId,
			ToId:     row.ToId,
			Kind:     kind,
			FromName: personNames[row.FromId],
			ToName:   personNames[row.ToId],
		})
	}
	return result
}

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
