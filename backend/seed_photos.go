package backend

import (
	"bytes"
	"embed"
	"fmt"
	"image"
	_ "image/jpeg"
	"time"

	"go.hasen.dev/vbolt"
)

// Stock photos from Pexels, downscaled to 1600px on the long edge. Pexels
// licensing allows use without attribution.
//
//go:embed seedphotos/*.jpg
var seedPhotoFiles embed.FS

// photo writes an Image row in the processing state and tags the people in it.
// The bytes ride along in a PhotoProcessingJob, because variants cannot be
// written until the caller has committed; see ProcessSeedPhotos.
func (s *seeder) photo(familyId int, owner User, file, title, description string, on time.Time, people []Person, tags ...Tag) Image {
	data, err := seedPhotoFiles.ReadFile("seedphotos/" + file)
	if err != nil {
		s.fail(fmt.Errorf("seed photo %q: %w", file, err))
		return Image{}
	}
	config, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		s.fail(fmt.Errorf("seed photo %q: %w", file, err))
		return Image{}
	}
	filename, err := generateUniqueFilename(file)
	if err != nil {
		s.fail(err)
		return Image{}
	}

	photo := Image{
		Id:               vbolt.NextIntId(s.tx, ImagesBkt),
		FamilyId:         familyId,
		OwnerUserId:      owner.Id,
		OriginalFilename: file,
		MimeType:         "image/jpeg",
		FileSize:         len(data),
		Width:            config.Width,
		Height:           config.Height,
		FilePath:         "photos/" + filename,
		Title:            title,
		Description:      description,
		PhotoDate:        on,
		CreatedAt:        s.now,
		Status:           1,
	}
	vbolt.Write(s.tx, ImagesBkt, photo.Id, &photo)
	vbolt.SetTargetSingleTerm(s.tx, ImageByFamilyIndex, photo.Id, familyId)
	ReindexPhotoDates(s.tx, photo.Id)

	for _, person := range people {
		AddPersonToPhoto(s.tx, photo.Id, person.Id, familyId)
	}
	for _, tag := range tags {
		addTagToPhoto(s.tx, photo.Id, tag.Id, familyId)
	}

	s.sum.Photos++
	s.sum.PhotoJobs = append(s.sum.PhotoJobs, PhotoProcessingJob{
		ImageId:        photo.Id,
		FamilyId:       familyId,
		FilePath:       photo.FilePath,
		FileData:       data,
		MimeType:       photo.MimeType,
		OriginalWidth:  config.Width,
		OriginalHeight: config.Height,
	})
	return photo
}

// profile makes a photo the person's avatar. The crop is the transform origin
// in percent of the square-cropped photo, and the zoom about it.
func (s *seeder) profile(person Person, photo Image, cropX, cropY, scale float64) {
	if person.Id == 0 || photo.Id == 0 {
		return
	}
	person = GetPersonById(s.tx, person.Id)
	person.ProfilePhotoId = photo.Id
	person.ProfileCropX = cropX
	person.ProfileCropY = cropY
	person.ProfileCropScale = scale
	vbolt.Write(s.tx, PeopleBkt, person.Id, &person)
}

func (s *seeder) appearancePhotos(appearance Appearance, photos ...Image) {
	ids := make([]int, 0, len(photos))
	for _, photo := range photos {
		ids = append(ids, photo.Id)
	}
	if err := setAppearancePhotosTx(s.tx, appearance, ids); err != nil {
		s.fail(fmt.Errorf("appearance photos: %w", err))
	}
}

// ProcessSeedPhotos renders the variants for a committed seed run. A running
// server hands them to its photo worker; the CLI has none, so it does the work
// inline against its own database handle.
var processSeedPhotos = ProcessSeedPhotos

func ProcessSeedPhotos(db *vbolt.DB, jobs []PhotoProcessingJob) {
	if len(jobs) == 0 {
		return
	}
	if pw := activePhotoWorker(); pw != nil {
		queueBacklog(pw, jobs,
			"Failed to queue seeded photo for processing",
			"Seeded photos fully queued")
		return
	}
	pw := &PhotoWorker{db: db}
	for _, job := range jobs {
		pw.processPhotoJob(job)
	}
}
