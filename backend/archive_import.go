package backend

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"go.hasen.dev/vbeam"
	"go.hasen.dev/vbolt"
)

// archiveFormatVersion is written into every export. Files from before it
// existed carry none and are read as version 1.
const archiveFormatVersion = 1

var errUnreadableArchive = errors.New("That file is not a valid Family Record export. Check that you picked the right file.")

// familyArchive is one archive in the current shape, whichever file it came
// from. media holds the files a zip carries; a Records file has none.
type familyArchive struct {
	ImportDataStructure
	media map[string]*zip.File
}

// archiveLedger maps the archive's ids to the target family's. Each step of
// the pipeline fills its own map and reads the ones before it.
type archiveLedger struct {
	people     map[int]int
	tags       map[int]int
	tagByName  map[string]int
	moments    map[int]int
	photos     map[int]int
	interviews map[int]int
	activities map[int]int
	seasons    map[int]int
	entries    map[int]int
	events     map[int]int
}

func newArchiveLedger() *archiveLedger {
	return &archiveLedger{
		people: map[int]int{}, tags: map[int]int{}, tagByName: map[string]int{},
		moments: map[int]int{}, photos: map[int]int{}, interviews: map[int]int{},
		activities: map[int]int{}, seasons: map[int]int{}, entries: map[int]int{}, events: map[int]int{},
	}
}

// archiveImported is the work left for after the import commits.
type archiveImported struct {
	milestoneIds []int
	photoJobs    []PhotoProcessingJob
}

func (done archiveImported) queue() {
	for _, id := range done.milestoneIds {
		QueueMilestoneEmbedding(id)
	}
	for _, job := range done.photoJobs {
		if err := QueuePhotoProcessing(job); err != nil {
			LogWarn(LogCategoryPhoto, "Imported photo left for stuck-photo recovery", map[string]interface{}{
				"photoId": job.ImageId,
				"error":   err.Error(),
			})
		}
	}
}

func readArchiveRecords(data []byte) (archive familyArchive, err error) {
	var header struct {
		FormatVersion int `json:"formatVersion"`
	}
	if json.Unmarshal(data, &header) != nil {
		return archive, errUnreadableArchive
	}
	switch {
	case header.FormatVersion <= 1:
		archive.ImportDataStructure, err = readArchiveV1(data)
	default:
		return archive, errors.New("That file was exported by a newer version of Family Record, so it can't be imported here yet.")
	}
	if err != nil {
		return archive, err
	}
	return archive, validateImportData(archive.ImportDataStructure)
}

// readArchiveV1 reads version 1, whose people and measurements keep the
// capitalised Id, FamilyId and PersonId keys.
func readArchiveV1(data []byte) (records ImportDataStructure, err error) {
	if json.Unmarshal(data, &records) != nil {
		return records, errUnreadableArchive
	}
	records.FormatVersion = archiveFormatVersion
	return records, nil
}

func readArchiveZip(zr *zip.Reader) (archive familyArchive, err error) {
	media := make(map[string]*zip.File, len(zr.File))
	var records *zip.File
	for _, zf := range zr.File {
		if zf.Name == "data.json" {
			records = zf
		} else {
			media[zf.Name] = zf
		}
	}
	if records == nil {
		return archive, errors.New("That zip has no data.json, so it isn't a Family Record export.")
	}
	rc, err := records.Open()
	if err != nil {
		return archive, errUnreadableArchive
	}
	data, err := io.ReadAll(rc)
	rc.Close()
	if err != nil {
		return archive, errUnreadableArchive
	}
	if archive, err = readArchiveRecords(data); err != nil {
		return archive, err
	}
	archive.media = media
	return archive, nil
}

// importFamilyArchive is the one import pipeline. It merges the archive into
// familyId inside tx and leaves committing, or rolling back a dry run, to the
// caller.
func importFamilyArchive(tx *vbolt.Tx, user User, familyId int, archive familyArchive, req ImportDataRequest) (resp ImportDataResponse, done archiveImported, err error) {
	resp.AvailableFamilyIds = getUniqueFamilyIds(archive.People)
	resp.AvailablePeople = archive.People
	people := filterPeople(archive.People, req.FilterFamilyIds, req.FilterPersonIds)

	if req.PreviewOnly {
		for _, importPerson := range people {
			matches := findPotentialMatches(tx, importPerson, familyId)
			if len(matches) == 0 {
				matches = []PersonMatch{{ImportPerson: importPerson, MatchType: "none"}}
			}
			resp.MatchedPeople = append(resp.MatchedPeople, matches...)
		}
		return
	}
	if len(people) == 0 {
		err = errors.New("No people match the specified filters")
		return
	}

	l := newArchiveLedger()
	resp.ImportedPeople, resp.MergedPeople, resp.Warnings = importPeople(tx, people, familyId, l)
	resp.SkippedPeople = len(people) - resp.ImportedPeople - resp.MergedPeople
	resp.PersonIdMapping = l.people
	resp.ImportedTags, resp.SkippedTags = importTags(tx, archive.Tags, familyId, l)

	heights, weights := filterMeasurements(archive.Heights, archive.Weights, l.people)
	var errs []string
	resp.ImportedMeasurements, resp.SkippedMeasurements, errs = importMeasurements(tx, heights, weights, l.people, familyId)
	resp.Errors = append(resp.Errors, errs...)
	resp.ImportedRelations, resp.SkippedRelations, errs = importRelations(tx, archive.Relations, l.people)
	resp.Errors = append(resp.Errors, errs...)

	if req.ImportMilestones {
		resp.ImportedMilestones, resp.SkippedMilestones, errs, done.milestoneIds =
			importMilestones(tx, filterMilestones(archive.Milestones, l.people), familyId, l)
		resp.Errors = append(resp.Errors, errs...)
	} else {
		mapExistingMoments(tx, archive.Milestones, l)
	}

	if archive.media != nil {
		var warnings []string
		resp.ImportedPhotos, resp.SkippedPhotos, done.photoJobs, warnings =
			importPhotos(tx, familyId, user.Id, archive.Photos, l, archive.media, req.DryRun)
		resp.Warnings = append(resp.Warnings, warnings...)
		setImportedProfilePhotos(tx, archive.People, l)
	} else {
		resp.SkippedPhotos = missingPhotoFiles(archive, l)
	}
	resp.SkippedVideos = videosInScope(archive.Videos, l)
	if photos := missingPhotoFiles(archive, l); photos+resp.SkippedVideos > 0 {
		resp.Warnings = append(resp.Warnings, fmt.Sprintf(
			"%d photo(s) and %d video(s) were not imported because their files aren't in this file.",
			photos, resp.SkippedVideos))
	}

	if req.ImportMilestones {
		interviews := interviewImport{familyId: familyId, ledger: l}.run(tx, archive.InterviewQuestions, archive.Interviews, archive.Milestones)
		resp.ImportedInterviews = interviews.imported
		resp.SkippedInterviews = interviews.skipped
		resp.Warnings = append(resp.Warnings, interviews.warnings...)
		done.milestoneIds = append(done.milestoneIds, interviews.answerIds...)
	}
	importInterviewCadence(tx, familyId, archive.InterviewCadence)

	if req.ImportActivities && len(archive.Activities) > 0 {
		var warnings []string
		resp.ImportedActivities, warnings = importActivities(tx, archive.Activities, familyId, l)
		resp.Warnings = append(resp.Warnings, warnings...)
	}

	var warnings []string
	resp.ImportedBooks, resp.SkippedBooks, warnings = importBooks(tx, archive.Books, bookImport{
		familyId: familyId, userId: user.Id, ledger: l,
	})
	resp.Warnings = append(resp.Warnings, warnings...)
	return
}

// inScope leaves out media of only people the import filtered out.
func inScope(personIds []int, l *archiveLedger) bool {
	for _, id := range personIds {
		if _, ok := l.people[id]; ok {
			return true
		}
	}
	return len(personIds) == 0
}

func videosInScope(videos []ExportVideo, l *archiveLedger) (n int) {
	for _, video := range videos {
		if inScope(video.PersonIds, l) {
			n++
		}
	}
	return
}

func missingPhotoFiles(archive familyArchive, l *archiveLedger) (missing int) {
	for _, photo := range archive.Photos {
		if inScope(photo.PersonIds, l) && archive.media[photo.ZipPath] == nil {
			missing++
		}
	}
	return
}

// mapExistingMoments links the moments a family already has, so books can
// point at them when the import leaves moments out.
func mapExistingMoments(tx *vbolt.Tx, milestones []ExportMilestone, l *archiveLedger) {
	for _, m := range milestones {
		if personId, ok := l.people[m.PersonId]; ok {
			if id := existingMilestoneId(tx, personId, m.MilestoneDate, m.Description); id != 0 {
				l.moments[m.Id] = id
			}
		}
	}
}

func setImportedProfilePhotos(tx *vbolt.Tx, people []ImportPerson, l *archiveLedger) {
	for _, importPerson := range people {
		personId, ok := l.people[importPerson.Id]
		photoId, hasPhoto := l.photos[importPerson.ImageId]
		if !ok || !hasPhoto || importPerson.ImageId == 0 {
			continue
		}
		var person Person
		vbolt.Read(tx, PeopleBkt, personId, &person)
		if person.ProfilePhotoId != 0 {
			continue
		}
		person.ProfilePhotoId = photoId
		person.ProfileCropX = 50
		person.ProfileCropY = 50
		person.ProfileCropScale = 1.0
		vbolt.Write(tx, PeopleBkt, person.Id, &person)
	}
}

// ImportData imports a Records file.
func ImportData(ctx *vbeam.Context, req ImportDataRequest) (resp ImportDataResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}
	familyId, err := ResolveActingFamily(ctx.Tx, user, req.FamilyId, AccessContribute)
	if err != nil {
		return
	}
	archive, err := readArchiveRecords([]byte(req.JsonData))
	if err != nil {
		if err == errUnreadableArchive {
			LogWarn(LogCategoryAPI, "Import rejected unreadable JSON", map[string]interface{}{"userId": user.Id})
		}
		return
	}

	if !req.PreviewOnly {
		vbeam.UseWriteTx(ctx)
	}
	resp, done, err := importFamilyArchive(ctx.Tx, user, familyId, archive, req)
	if err != nil || req.PreviewOnly {
		return
	}
	if req.DryRun {
		vbolt.TxClose(ctx.Tx)
		return
	}
	vbolt.TxCommit(ctx.Tx)
	done.queue()
	return
}

// importBundleHandler imports a zip. Its "options" field carries the same
// choices as ImportDataRequest, without jsonData.
func importBundleHandler(w http.ResponseWriter, r *http.Request) {
	user, ok := GetUserFromContext(r)
	if !ok {
		RespondAuthError(w, r, "Authentication required")
		return
	}
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		RespondValidationError(w, r, "Failed to parse multipart form", err.Error())
		return
	}
	var req ImportDataRequest
	if err := json.Unmarshal([]byte(r.FormValue("options")), &req); err != nil {
		RespondValidationError(w, r, "The import options are missing. Reload the page and try again.", err.Error())
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		RespondValidationError(w, r, "No file provided", err.Error())
		return
	}
	defer file.Close()
	zipReader, err := zip.NewReader(file, header.Size)
	if err != nil {
		RespondValidationError(w, r, "Invalid ZIP file", err.Error())
		return
	}
	archive, err := readArchiveZip(zipReader)
	if err != nil {
		RespondValidationError(w, r, err.Error())
		return
	}

	var resp ImportDataResponse
	var done archiveImported
	withTx := vbolt.WithWriteTx
	if req.PreviewOnly {
		withTx = vbolt.WithReadTx
	}
	withTx(appDb, func(tx *vbolt.Tx) {
		var familyId int
		if familyId, err = ResolveActingFamily(tx, user, req.FamilyId, AccessContribute); err != nil {
			return
		}
		resp, done, err = importFamilyArchive(tx, user, familyId, archive, req)
		if err == nil && !req.PreviewOnly && !req.DryRun {
			vbolt.TxCommit(tx)
		}
	})
	if err != nil {
		RespondValidationError(w, r, "Failed to import bundle", err.Error())
		return
	}
	if !req.PreviewOnly && !req.DryRun {
		done.queue()
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
