package backend

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"family/cfg"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"go.hasen.dev/vbeam"
	"go.hasen.dev/vbolt"
)

func RegisterExportMethods(app *vbeam.Application) {
	vbeam.RegisterProc(app, ExportData)
	vbeam.RegisterProc(app, ListVideoOriginals)
	vbeam.RegisterProc(app, GetVideoOriginalLink)
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
		LogErrorWithRequest(r, LogCategoryAPI, "Export could not write data.json", map[string]interface{}{"error": err.Error()})
		return
	}
	if _, err := dataEntry.Write(jsonBytes); err != nil {
		LogErrorWithRequest(r, LogCategoryAPI, "Export could not write data.json", map[string]interface{}{"error": err.Error()})
		return
	}

	for _, ep := range exportData.Photos {
		diskPath := filepath.Join(cfg.StaticDir, ep.ZipPath)
		f, err := os.Open(diskPath)
		if err != nil {
			LogWarnWithRequest(r, LogCategoryPhoto, "Export skipped a photo whose original is unreadable", map[string]interface{}{"photoId": ep.Id, "error": err.Error()})
			continue
		}
		entry, err := zw.Create(ep.ZipPath)
		if err != nil {
			LogWarnWithRequest(r, LogCategoryPhoto, "Export could not add a photo", map[string]interface{}{"photoId": ep.Id, "error": err.Error()})
			f.Close()
			continue
		}
		if _, err := io.Copy(entry, f); err != nil {
			LogErrorWithRequest(r, LogCategoryPhoto, "Export stopped writing a photo", map[string]interface{}{"photoId": ep.Id, "error": err.Error()})
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
		if img.Status != PhotoReady || img.IsVideo() {
			continue
		}
		zipPath := "photos/" + originalFileName(img.FilePath)

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
			RecordTime:  &img.RecordTime,
			DateLabel:   img.RecordTime.Label(),
			ZipPath:     zipPath,
			PersonIds:   personIds,
			TagIds:      GetPhotoTagIds(tx, img.Id),
		})
	}
	return result
}

type ExportPhoto struct {
	Id          int         `json:"id"`
	Title       string      `json:"title"`
	Description string      `json:"description"`
	PhotoDate   time.Time   `json:"photo_date"`
	RecordTime  *RecordTime `json:"record_time,omitempty"`
	DateLabel   string      `json:"date_label,omitempty"`
	ZipPath     string      `json:"zip_path"`
	PersonIds   []int       `json:"person_ids"`
	TagIds      []int       `json:"tag_ids"`
}

type ExportDataStructure struct {
	FormatVersion int `json:"formatVersion"`

	People     []ImportPerson    `json:"people"`
	Relations  []ExportRelation  `json:"relations"`
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
	InterviewDismissed []string                  `json:"interviewDismissedKeys,omitempty"`

	ExportDate      time.Time `json:"export_date"`
	TotalHeights    int       `json:"total_heights"`
	TotalWeights    int       `json:"total_weights"`
	TotalPeople     int       `json:"total_people"`
	TotalRelations  int       `json:"total_relations"`
	TotalMilestones int       `json:"total_milestones"`
	TotalTags       int       `json:"total_tags"`
	TotalPhotos     int       `json:"total_photos,omitempty"`
	TotalVideos     int       `json:"total_videos,omitempty"`

	TotalActivities  int `json:"total_activities,omitempty"`
	TotalSeasons     int `json:"total_seasons,omitempty"`
	TotalEvents      int `json:"total_events,omitempty"`
	TotalEntries     int `json:"total_entries,omitempty"`
	TotalAppearances int `json:"total_appearances,omitempty"`
	TotalResults     int `json:"total_results,omitempty"`
	TotalBooks       int `json:"total_books,omitempty"`
	TotalInterviews  int `json:"total_interviews,omitempty"`
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
	exportData := ExportDataStructure{FormatVersion: archiveFormatVersion}

	tags := getTagsByFamily(tx, familyId)
	tagIdToName := make(map[int]string, len(tags))
	for _, tag := range tags {
		tagIdToName[tag.Id] = tag.Name
	}
	exportTags := exportTagRecords(tags)

	people := GetFamilyOwnPeople(tx, familyId)
	recorderOf := exportRecorders(tx)
	exportData.People = exportPeople(people)
	heights, weights := exportMeasurements(tx, familyId, people, recorderOf)

	exportMilestones := buildMilestoneExport(tx, familyId, people, tagIdToName, recorderOf)

	personNames := make(map[int]string, len(people))
	for _, person := range people {
		personNames[person.Id] = person.Name
	}

	exportData.Relations = buildRelationExport(tx, people, personNames)

	exportData.Videos = buildVideoExport(tx, familyId)
	exportData.TotalVideos = len(exportData.Videos)

	exportData.Activities = buildActivityExport(tx, familyId, personNames)
	seasonCount, eventCount, entryCount, appearanceCount, resultCount :=
		countExportedActivities(exportData.Activities)

	exportData.Books = buildBookExport(tx, familyId)
	exportData.TotalBooks = len(exportData.Books)

	exportData.InterviewQuestions, exportData.Interviews = buildInterviewExport(tx, familyId)
	exportData.TotalInterviews = len(exportData.Interviews)
	exportData.InterviewCadence = exportInterviewCadence(tx, familyId)
	exportData.InterviewDismissed = familyQuestionList(tx, familyId).DismissedKeys

	exportData.Heights = heights
	exportData.Weights = weights
	exportData.Milestones = exportMilestones
	exportData.Tags = exportTags
	exportData.ExportDate = time.Now()
	exportData.TotalHeights = len(heights)
	exportData.TotalWeights = len(weights)
	exportData.TotalPeople = len(people)
	exportData.TotalRelations = len(exportData.Relations)
	exportData.TotalMilestones = len(exportMilestones)
	exportData.TotalTags = len(tags)
	exportData.TotalActivities = len(exportData.Activities)
	exportData.TotalSeasons = seasonCount
	exportData.TotalEvents = eventCount
	exportData.TotalEntries = entryCount
	exportData.TotalAppearances = appearanceCount
	exportData.TotalResults = resultCount

	return exportData, nil
}
