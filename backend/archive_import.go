package backend

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"go.hasen.dev/vbeam"
	"go.hasen.dev/vbolt"
)

// archiveFormatVersion is written into every export. Files from before it
// existed carry none and are read as version 1.
const archiveFormatVersion = 1

var errUnreadableArchive = errors.New("That file is not a valid Family Record export. Check that you picked the right file.")

// familyArchive is one archive in the current shape, whichever file it came
// from. media holds the files a zip carries; a Records file has none. staged
// holds the photos unpacked for this import, by their id in the archive.
// warnings are what reading it changed, for the import to report.
type familyArchive struct {
	ImportDataStructure
	media    map[string]*zip.File
	staged   map[int]stagedMedia
	warnings []string
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
	photoJobs    []importedPhoto
	videos       []importedVideo
}

// importedPhoto and importedVideo are committed media whose staged files
// still have to be put in place before they are processed.
type importedPhoto struct {
	PhotoProcessingJob
	file stagedMove
}

type importedVideo struct {
	id   int
	file stagedUpload
}

func (done archiveImported) queue() {
	for _, id := range done.milestoneIds {
		QueueMilestoneEmbedding(id)
	}
	for _, job := range done.photoJobs {
		err := job.file.run()
		if err == nil {
			err = QueuePhotoProcessing(job.PhotoProcessingJob)
		}
		if err != nil {
			LogWarn(LogCategoryPhoto, "Imported photo left for stuck-photo recovery", map[string]interface{}{
				"photoId": job.ImageId,
				"error":   err.Error(),
			})
		}
	}
	for _, video := range done.videos {
		if err := video.file.run(); err != nil {
			LogWarn(LogCategoryPhoto, "Imported video left for stuck-video recovery", map[string]interface{}{
				"videoId": video.id,
				"error":   err.Error(),
			})
			continue
		}
		QueueVideoProcessing(video.id)
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
		if err == nil && header.FormatVersion == 0 {
			archive.warnings = markLegacyPregnancies(archive.People)
		}
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

// markLegacyPregnancies treats a future birthday in an unversioned file as a
// due date: those files don't carry the pregnancy flag, and only a pregnancy
// can have a birthday in the future.
func markLegacyPregnancies(people []ImportPerson) (warnings []string) {
	now := time.Now()
	for i := range people {
		if people[i].Birthday.After(now) && !people[i].IsPregnancy {
			people[i].IsPregnancy = true
			warnings = append(warnings, people[i].Name+"'s birthday is in the future, so they were imported as a pregnancy.")
		}
	}
	return
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
	importFamilySettings(tx, familyId, archive.Family)
	resp.ImportedPeople, resp.MergedPeople, resp.Warnings = importPeople(tx, people, familyId, l)
	resp.Warnings = append(archive.warnings, resp.Warnings...)
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
			importPhotos(tx, familyId, user.Id, archive.Photos, l, archive.staged)
		resp.Warnings = append(resp.Warnings, warnings...)
		setImportedProfilePhotos(tx, archive.People, l)
		resp.ImportedVideos, resp.SkippedVideos, done.videos, warnings =
			importVideos(tx, familyId, user.Id, archive.Videos, l, archive.staged, req.DryRun)
		resp.Warnings = append(resp.Warnings, warnings...)
	} else {
		resp.SkippedPhotos = missingPhotoFiles(archive, l)
		resp.SkippedVideos = missingVideoFiles(archive, l)
	}
	if photos, videos := missingPhotoFiles(archive, l), missingVideoFiles(archive, l); photos+videos > 0 {
		resp.Warnings = append(resp.Warnings, fmt.Sprintf(
			"%d photo(s) and %d video(s) were not imported because their files aren't in this file.",
			photos, videos))
	}

	importInterviewListSettings(tx, familyId, archive.InterviewCadence, archive.InterviewDismissed)
	if req.ImportMilestones {
		interviews := interviewImport{familyId: familyId, ledger: l}.run(tx, archive.InterviewQuestions, archive.Interviews, archive.Milestones)
		resp.ImportedInterviews = interviews.imported
		resp.SkippedInterviews = interviews.skipped
		resp.Warnings = append(resp.Warnings, interviews.warnings...)
		done.milestoneIds = append(done.milestoneIds, interviews.answerIds...)
		linkMomentPhotos(tx, archive.Milestones, familyId, l)
	} else {
		mapExistingInterviews(tx, archive.Interviews, l)
	}
	importPlaces(tx, archive.Places, familyId)

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

func missingVideoFiles(archive familyArchive, l *archiveLedger) (missing int) {
	for _, video := range archive.Videos {
		if inScope(video.PersonIds, l) && archive.media[video.zipPath()] == nil {
			missing++
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
		person.ProfileCropX, person.ProfileCropY, person.ProfileCropScale = 50, 50, 1.0
		if importPerson.ProfileCropScale > 0 {
			person.ProfileCropX = importPerson.ProfileCropX
			person.ProfileCropY = importPerson.ProfileCropY
			person.ProfileCropScale = importPerson.ProfileCropScale
		}
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
