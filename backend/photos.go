package backend

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"family/cfg"
	"fmt"
	"image"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"

	"github.com/rwcarlsen/goexif/exif"
	"go.hasen.dev/vbeam"
	"go.hasen.dev/vbolt"
	"go.hasen.dev/vpack"
)

func RegisterPhotoMethods(app *vbeam.Application) {
	app.HandleFunc("/api/upload-photo", AuthMiddleware(uploadPhotoHandler))
	app.HandleFunc("/api/photo/", AuthMiddleware(servePhotoHandler))
	vbeam.RegisterProc(app, GetPhoto)
	vbeam.RegisterProc(app, UpdatePhoto)
	vbeam.RegisterProc(app, DeletePhoto)
	vbeam.RegisterProc(app, GetPhotoStatus)
	vbeam.RegisterProc(app, ListFamilyPhotos)
	vbeam.RegisterProc(app, AddPeopleToPhoto)
	vbeam.RegisterProc(app, RemovePersonFromPhotoProc)
	vbeam.RegisterProc(app, UpdatePhotoTags)
}

type AddPhotoRequest struct {
	PersonIds   []int  `json:"personIds"`
	Title       string `json:"title"`
	Description string `json:"description"`
	InputType   string `json:"inputType"`
	PhotoDate   string `json:"photoDate,omitempty"`
	AgeYears    *int   `json:"ageYears,omitempty"`
	AgeMonths   *int   `json:"ageMonths,omitempty"`
}

type AddPhotoResponse struct {
	Image Image `json:"image"`
}

type GetPhotoRequest struct {
	Id int `json:"id"`
}

type GetPhotoResponse struct {
	Image  Image    `json:"image"`
	People []Person `json:"people"`
}

type UpdatePhotoRequest struct {
	Id          int    `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	InputType   string `json:"inputType"`
	PhotoDate   string `json:"photoDate,omitempty"`
	AgeYears    *int   `json:"ageYears,omitempty"`
	AgeMonths   *int   `json:"ageMonths,omitempty"`
}

type UpdatePhotoResponse struct {
	Image Image `json:"image"`
}

type DeletePhotoRequest struct {
	Id int `json:"id"`
}

type DeletePhotoResponse struct {
	Success bool `json:"success"`
}

type GetPhotoStatusRequest struct {
	Id int `json:"id"`
}

type GetPhotoStatusResponse struct {
	Status int `json:"status"`
}

type ListFamilyPhotosRequest struct {
	PersonId int `json:"personId,omitempty"`
}

type PhotoWithPeople struct {
	Image  Image    `json:"image"`
	People []Person `json:"people"`
}

type ListFamilyPhotosResponse struct {
	Photos []PhotoWithPeople `json:"photos"`
}

type AddPeopleToPhotoRequest struct {
	PhotoId   int   `json:"photoId"`
	PersonIds []int `json:"personIds"`
}

type AddPeopleToPhotoResponse struct {
	Success bool     `json:"success"`
	People  []Person `json:"people"`
}

type RemovePersonFromPhotoRequest struct {
	PhotoId  int `json:"photoId"`
	PersonId int `json:"personId"`
}

type RemovePersonFromPhotoResponse struct {
	Success bool `json:"success"`
}

type Image struct {
	Id               int       `json:"id"`
	FamilyId         int       `json:"familyId"`
	OwnerUserId      int       `json:"ownerUserId"`
	OriginalFilename string    `json:"originalFilename"`
	MimeType         string    `json:"mimeType"`
	FileSize         int       `json:"fileSize"`
	Width            int       `json:"width"`
	Height           int       `json:"height"`
	FilePath         string    `json:"filePath"`
	Title            string    `json:"title"`
	Description      string    `json:"description"`
	PhotoDate        time.Time `json:"photoDate"`
	CreatedAt        time.Time `json:"createdAt"`
	Status           int       `json:"status"`
	AnalysisStatus   int       `json:"analysisStatus"`
	TagIds           []int     `json:"tagIds,omitempty"`
}

type PhotoPerson struct {
	Id         int       `json:"id"`
	PhotoId    int       `json:"photoId"`
	PersonId   int       `json:"personId"`
	FamilyId   int       `json:"familyId"`
	CreatedAt  time.Time `json:"createdAt"`
	AutoTagged bool      `json:"autoTagged"`
}

func PackImage(self *Image, buf *vpack.Buffer) {
	version := vpack.Version(3, buf)
	vpack.Int(&self.Id, buf)
	vpack.Int(&self.FamilyId, buf)
	vpack.Int(&self.OwnerUserId, buf)
	vpack.String(&self.OriginalFilename, buf)
	vpack.String(&self.MimeType, buf)
	vpack.Int(&self.FileSize, buf)
	vpack.Int(&self.Width, buf)
	vpack.Int(&self.Height, buf)
	vpack.String(&self.FilePath, buf)
	vpack.String(&self.Title, buf)
	vpack.String(&self.Description, buf)
	vpack.Time(&self.PhotoDate, buf)
	vpack.Time(&self.CreatedAt, buf)
	vpack.Int(&self.Status, buf)
	if version >= 3 {
		vpack.Int(&self.AnalysisStatus, buf)
	}
}

func PackPhotoPerson(self *PhotoPerson, buf *vpack.Buffer) {
	version := vpack.Version(2, buf)
	vpack.Int(&self.Id, buf)
	vpack.Int(&self.PhotoId, buf)
	vpack.Int(&self.PersonId, buf)
	vpack.Int(&self.FamilyId, buf)
	vpack.Time(&self.CreatedAt, buf)
	if version >= 2 {
		vpack.Bool(&self.AutoTagged, buf)
	}
}

type PhotoTag struct {
	Id        int
	PhotoId   int
	TagId     int
	FamilyId  int
	CreatedAt time.Time
}

func PackPhotoTag(self *PhotoTag, buf *vpack.Buffer) {
	vpack.Version(1, buf)
	vpack.Int(&self.Id, buf)
	vpack.Int(&self.PhotoId, buf)
	vpack.Int(&self.TagId, buf)
	vpack.Int(&self.FamilyId, buf)
	vpack.Time(&self.CreatedAt, buf)
}

var ImagesBkt = vbolt.Bucket(&cfg.Info, "images", vpack.FInt, PackImage)
var PhotoPersonBkt = vbolt.Bucket(&cfg.Info, "photo_person", vpack.FInt, PackPhotoPerson)

var ImageByFamilyIndex = vbolt.Index(&cfg.Info, "image_by_family", vpack.FInt, vpack.FInt)

var PhotoPersonByPhotoIndex = vbolt.Index(&cfg.Info, "photo_person_by_photo", vpack.FInt, vpack.FInt)

var PhotoPersonByPersonIndex = vbolt.Index(&cfg.Info, "photo_person_by_person", vpack.FInt, vpack.FInt)

var PhotoPersonByFamilyIndex = vbolt.Index(&cfg.Info, "photo_person_by_family", vpack.FInt, vpack.FInt)

var PhotoTagBkt = vbolt.Bucket(&cfg.Info, "photo_tags", vpack.FInt, PackPhotoTag)
var PhotoTagByPhotoIndex = vbolt.Index(&cfg.Info, "photo_tag_by_photo", vpack.FInt, vpack.FInt)
var PhotoTagByTagIndex = vbolt.Index(&cfg.Info, "photo_tag_by_tag", vpack.FInt, vpack.FInt)
var PhotoTagByFamilyIndex = vbolt.Index(&cfg.Info, "photo_tag_by_family", vpack.FInt, vpack.FInt)

func GetImageById(tx *vbolt.Tx, imageId int) (image Image) {
	vbolt.Read(tx, ImagesBkt, imageId, &image)
	return
}

func GetPhotoPersonById(tx *vbolt.Tx, photoPersonId int) (photoPerson PhotoPerson) {
	vbolt.Read(tx, PhotoPersonBkt, photoPersonId, &photoPerson)
	return
}

func GetPhotoPersonsByPhoto(tx *vbolt.Tx, photoId int) (photoPersons []PhotoPerson) {
	var photoPersonIds []int
	vbolt.ReadTermTargets(tx, PhotoPersonByPhotoIndex, photoId, &photoPersonIds, vbolt.Window{})
	vbolt.ReadSlice(tx, PhotoPersonBkt, photoPersonIds, &photoPersons)
	return
}

func GetPhotoPersonsByPerson(tx *vbolt.Tx, personId int) (photoPersons []PhotoPerson) {
	var photoPersonIds []int
	vbolt.ReadTermTargets(tx, PhotoPersonByPersonIndex, personId, &photoPersonIds, vbolt.Window{})
	vbolt.ReadSlice(tx, PhotoPersonBkt, photoPersonIds, &photoPersons)
	return
}

func GetPhotoPeople(tx *vbolt.Tx, photoId int) (people []Person) {
	photoPersons := GetPhotoPersonsByPhoto(tx, photoId)
	people = make([]Person, 0, len(photoPersons))

	for _, photoPerson := range photoPersons {
		person := GetPersonById(tx, photoPerson.PersonId)
		if person.Id != 0 {
			people = append(people, person)
		}
	}
	return
}

func GetPersonImages(tx *vbolt.Tx, personId int) (images []Image) {
	photoPersons := GetPhotoPersonsByPerson(tx, personId)
	images = make([]Image, 0, len(photoPersons))

	for _, photoPerson := range photoPersons {
		image := GetImageById(tx, photoPerson.PhotoId)
		if image.Id != 0 {
			images = append(images, image)
		}
	}
	return
}

func GetFamilyImages(tx *vbolt.Tx, familyId int) (images []Image) {
	var imageIds []int
	vbolt.ReadTermTargets(tx, ImageByFamilyIndex, familyId, &imageIds, vbolt.Window{})
	vbolt.ReadSlice(tx, ImagesBkt, imageIds, &images)
	return
}

func GetVisibleImages(tx *vbolt.Tx, user User) (images []Image) {
	seen := make(map[int]bool)
	add := func(image Image) {
		if image.Id == 0 || seen[image.Id] {
			return
		}
		seen[image.Id] = true
		images = append(images, image)
	}

	own := familiesVisibleTo(tx, user)
	member := make(map[int]bool, len(own))
	for _, familyId := range own {
		member[familyId] = true
	}
	for _, familyId := range own {
		for _, image := range GetFamilyImages(tx, familyId) {
			add(image)
		}
	}

	for _, familyId := range own {
		for _, row := range GetFamilyRoster(tx, familyId) {
			person := GetPersonById(tx, row.PersonId)
			if person.Id == 0 || member[person.FamilyId] {
				continue
			}
			if !canAccessPersonViaLink(tx, user, person, ScopePhotos, AccessView) {
				continue
			}
			for _, image := range GetPersonImages(tx, person.Id) {
				add(image)
			}
		}
	}
	return
}

func GetPhotoTagIds(tx *vbolt.Tx, photoId int) []int {
	var ptIds []int
	vbolt.ReadTermTargets(tx, PhotoTagByPhotoIndex, photoId, &ptIds, vbolt.Window{})
	if len(ptIds) == 0 {
		return []int{}
	}
	var pts []PhotoTag
	vbolt.ReadSlice(tx, PhotoTagBkt, ptIds, &pts)
	tagIds := make([]int, 0, len(pts))
	for _, pt := range pts {
		tagIds = append(tagIds, pt.TagId)
	}
	return tagIds
}

func addTagToPhoto(tx *vbolt.Tx, photoId int, tagId int, familyId int) {
	pt := PhotoTag{
		Id:        vbolt.NextIntId(tx, PhotoTagBkt),
		PhotoId:   photoId,
		TagId:     tagId,
		FamilyId:  familyId,
		CreatedAt: time.Now(),
	}
	vbolt.Write(tx, PhotoTagBkt, pt.Id, &pt)
	vbolt.SetTargetSingleTerm(tx, PhotoTagByPhotoIndex, pt.Id, photoId)
	vbolt.SetTargetSingleTerm(tx, PhotoTagByTagIndex, pt.Id, tagId)
	vbolt.SetTargetSingleTerm(tx, PhotoTagByFamilyIndex, pt.Id, familyId)
}

func removeTagFromPhoto(tx *vbolt.Tx, photoId int, tagId int) {
	var ptIds []int
	vbolt.ReadTermTargets(tx, PhotoTagByPhotoIndex, photoId, &ptIds, vbolt.Window{})
	if len(ptIds) == 0 {
		return
	}
	var pts []PhotoTag
	vbolt.ReadSlice(tx, PhotoTagBkt, ptIds, &pts)
	for _, pt := range pts {
		if pt.TagId == tagId {
			vbolt.Delete(tx, PhotoTagBkt, pt.Id)
			vbolt.SetTargetSingleTerm(tx, PhotoTagByPhotoIndex, pt.Id, -1)
			vbolt.SetTargetSingleTerm(tx, PhotoTagByTagIndex, pt.Id, -1)
			vbolt.SetTargetSingleTerm(tx, PhotoTagByFamilyIndex, pt.Id, -1)
			break
		}
	}
}

func removeAllPhotoTags(tx *vbolt.Tx, photoId int) {
	var ptIds []int
	vbolt.ReadTermTargets(tx, PhotoTagByPhotoIndex, photoId, &ptIds, vbolt.Window{})
	if len(ptIds) == 0 {
		return
	}
	var pts []PhotoTag
	vbolt.ReadSlice(tx, PhotoTagBkt, ptIds, &pts)
	for _, pt := range pts {
		vbolt.Delete(tx, PhotoTagBkt, pt.Id)
		vbolt.SetTargetSingleTerm(tx, PhotoTagByPhotoIndex, pt.Id, -1)
		vbolt.SetTargetSingleTerm(tx, PhotoTagByTagIndex, pt.Id, -1)
		vbolt.SetTargetSingleTerm(tx, PhotoTagByFamilyIndex, pt.Id, -1)
	}
}

func removePhotoTagsByTag(tx *vbolt.Tx, tagId int) {
	var ptIds []int
	vbolt.ReadTermTargets(tx, PhotoTagByTagIndex, tagId, &ptIds, vbolt.Window{})
	if len(ptIds) == 0 {
		return
	}
	var pts []PhotoTag
	vbolt.ReadSlice(tx, PhotoTagBkt, ptIds, &pts)
	for _, pt := range pts {
		vbolt.Delete(tx, PhotoTagBkt, pt.Id)
		vbolt.SetTargetSingleTerm(tx, PhotoTagByPhotoIndex, pt.Id, -1)
		vbolt.SetTargetSingleTerm(tx, PhotoTagByTagIndex, pt.Id, -1)
		vbolt.SetTargetSingleTerm(tx, PhotoTagByFamilyIndex, pt.Id, -1)
	}
}

func AddPersonToPhoto(tx *vbolt.Tx, photoId int, personId int, familyId int) (photoPersonId int) {
	photoPerson := PhotoPerson{
		Id:        vbolt.NextIntId(tx, PhotoPersonBkt),
		PhotoId:   photoId,
		PersonId:  personId,
		FamilyId:  familyId,
		CreatedAt: time.Now(),
	}

	vbolt.Write(tx, PhotoPersonBkt, photoPerson.Id, &photoPerson)
	vbolt.SetTargetSingleTerm(tx, PhotoPersonByPhotoIndex, photoPerson.Id, photoId)
	vbolt.SetTargetSingleTerm(tx, PhotoPersonByPersonIndex, photoPerson.Id, personId)
	vbolt.SetTargetSingleTerm(tx, PhotoPersonByFamilyIndex, photoPerson.Id, familyId)

	return photoPerson.Id
}

func RemovePersonFromPhoto(tx *vbolt.Tx, photoId int, personId int) {
	photoPersons := GetPhotoPersonsByPhoto(tx, photoId)

	for _, photoPerson := range photoPersons {
		if photoPerson.PersonId == personId {
			vbolt.Delete(tx, PhotoPersonBkt, photoPerson.Id)
			vbolt.SetTargetSingleTerm(tx, PhotoPersonByPhotoIndex, photoPerson.Id, -1)
			vbolt.SetTargetSingleTerm(tx, PhotoPersonByPersonIndex, photoPerson.Id, -1)
			vbolt.SetTargetSingleTerm(tx, PhotoPersonByFamilyIndex, photoPerson.Id, -1)
			break
		}
	}
}

func generateUniqueFilename(originalFilename string) (string, error) {
	ext := filepath.Ext(originalFilename)
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	filename := hex.EncodeToString(bytes) + ext
	return filename, nil
}

func isValidImageType(mimeType string) bool {
	validTypes := []string{
		"image/jpeg",
		"image/jpg",
		"image/png",
		"image/gif",
	}
	for _, validType := range validTypes {
		if mimeType == validType {
			return true
		}
	}
	return false
}

func getImageDimensions(file multipart.File) (int, int, error) {
	file.Seek(0, 0)

	config, _, err := image.DecodeConfig(file)
	if err != nil {
		return 0, 0, err
	}

	file.Seek(0, 0)

	return config.Width, config.Height, nil
}

func extractExifDate(fileData []byte) (time.Time, error) {
	reader := bytes.NewReader(fileData)

	x, err := exif.Decode(reader)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to decode EXIF: %w", err)
	}

	tm, err := x.DateTime()
	if err != nil {
		return time.Time{}, fmt.Errorf("no DateTime found in EXIF: %w", err)
	}

	return tm, nil
}

func generateDefaultTitle(originalFilename string, photoDate time.Time) string {
	if !photoDate.IsZero() {
		return fmt.Sprintf("Photo from %s", photoDate.Format("Jan 2, 2006"))
	}
	return strings.TrimSuffix(originalFilename, filepath.Ext(originalFilename))
}

func calculatePhotoDate(inputType string, photoDate string, ageYears *int, ageMonths *int, person Person, fileData []byte) (time.Time, error) {
	switch inputType {
	case "auto":
		if exifDate, err := extractExifDate(fileData); err == nil {
			return exifDate, nil
		}
		return time.Now(), nil
	case "today":
		return time.Now(), nil
	case "date":
		if photoDate == "" {
			return time.Time{}, errors.New("photo date is required")
		}
		return time.Parse("2006-01-02", photoDate)
	case "age":
		if ageYears == nil {
			return time.Time{}, errors.New("age years is required")
		}

		months := 0
		if ageMonths != nil {
			months = *ageMonths
		}

		targetAge := time.Duration(*ageYears)*365*24*time.Hour + time.Duration(months)*30*24*time.Hour
		photoDateTime := person.Birthday.Add(targetAge)

		return photoDateTime, nil
	default:
		return time.Time{}, errors.New("invalid input type")
	}
}

func uploadPhotoHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	const maxFileSize = 50 << 20
	if r.ContentLength > maxFileSize {
		RespondFileTooLargeError(w, r, "50MB")
		return
	}

	err := r.ParseMultipartForm(32 << 20)
	if err != nil {
		RespondValidationError(w, r, "That upload could not be read. Please try again.", err.Error())
		return
	}

	user, ok := GetUserFromContext(r)
	if !ok {
		RespondAuthError(w, r, "Authentication required")
		return
	}

	LogInfoWithRequest(r, LogCategoryPhoto, "Photo upload started", map[string]interface{}{
		"userId": user.Id,
	})

	personIdsStr := r.FormValue("personIds")
	var personIds []int

	if personIdsStr != "" {
		if err := json.Unmarshal([]byte(personIdsStr), &personIds); err != nil {
			RespondValidationError(w, r, "The people tagged on this photo could not be read.", err.Error())
			return
		}
	}

	var requestedFamilyId int
	if familyIdStr := r.FormValue("familyId"); familyIdStr != "" {
		parsed, convErr := strconv.Atoi(familyIdStr)
		if convErr != nil {
			RespondValidationError(w, r, "That family could not be identified.", convErr.Error())
			return
		}
		requestedFamilyId = parsed
	}

	title := strings.TrimSpace(r.FormValue("title"))

	description := strings.TrimSpace(r.FormValue("description"))
	inputType := r.FormValue("inputType")
	photoDate := r.FormValue("photoDate")

	var ageYears, ageMonths *int
	if ageYearsStr := r.FormValue("ageYears"); ageYearsStr != "" {
		if years, err := strconv.Atoi(ageYearsStr); err == nil {
			ageYears = &years
		}
	}
	if ageMonthsStr := r.FormValue("ageMonths"); ageMonthsStr != "" {
		if months, err := strconv.Atoi(ageMonthsStr); err == nil {
			ageMonths = &months
		}
	}

	file, fileHeader, err := r.FormFile("photo")
	if err != nil {
		RespondValidationError(w, r, "Choose a photo to upload.", err.Error())
		return
	}
	defer file.Close()

	if fileHeader.Size > 32<<20 {
		RespondFileTooLargeError(w, r, "32MB")
		return
	}

	mimeType := fileHeader.Header.Get("Content-Type")
	if !isValidImageType(mimeType) {
		RespondInvalidFileTypeError(w, r, "JPEG, PNG, GIF")
		return
	}

	width, height, err := getImageDimensions(file)
	if err != nil {
		RespondValidationError(w, r, "That file could not be read as an image. Try a JPEG or PNG.", err.Error())
		return
	}

	var image Image
	var validPersons []Person
	var uploadErr *AppError

	vbolt.WithWriteTx(appDb, func(tx *vbolt.Tx) {
		familyId, err := ResolveActingFamily(tx, user, requestedFamilyId, AccessContribute)
		if err != nil {
			uploadErr = NewAppError(ErrCodeForbidden, "You cannot add photos to that family.", err.Error())
			return
		}

		if len(personIds) > 0 {
			validPersons = make([]Person, 0, len(personIds))
			for _, personId := range personIds {
				person := GetPersonById(tx, personId)
				if person.Id == 0 || !CanFamilyAccess(tx, familyId, person.FamilyId, AccessContribute) {
					uploadErr = NewAppError(ErrCodeValidation, "One of the tagged people is not in your family.")
					return
				}
				validPersons = append(validPersons, person)
			}
		}

		uniqueFilename, err := generateUniqueFilename(fileHeader.Filename)
		if err != nil {
			uploadErr = NewAppError(ErrCodeInternal, unexpectedErrorMessage, err.Error())
			return
		}

		photosDir := filepath.Join(cfg.StaticDir, "photos")
		err = os.MkdirAll(photosDir, 0755)
		if err != nil {
			uploadErr = NewAppError(ErrCodeInternal, unexpectedErrorMessage, err.Error())
			return
		}

		file.Seek(0, 0)
		fileData, err := io.ReadAll(file)
		if err != nil {
			uploadErr = NewAppError(ErrCodeInternal, unexpectedErrorMessage, err.Error())
			return
		}

		var referencePerson Person
		if len(validPersons) > 0 {
			referencePerson = validPersons[0]
		}
		calculatedPhotoDate, err := calculatePhotoDate(inputType, photoDate, ageYears, ageMonths, referencePerson, fileData)
		if err != nil {
			uploadErr = NewAppError(ErrCodeValidation, "That photo date could not be worked out. Check the date or age you entered.", err.Error())
			return
		}

		if title == "" {
			title = generateDefaultTitle(fileHeader.Filename, calculatedPhotoDate)
		}

		baseFilename := strings.TrimSuffix(uniqueFilename, filepath.Ext(uniqueFilename))
		originalPath := filepath.Join(photosDir, baseFilename+"_original"+filepath.Ext(uniqueFilename))
		if origFile, err := os.Create(originalPath); err != nil {
			uploadErr = NewAppError(ErrCodeInternal, unexpectedErrorMessage, err.Error())
			return
		} else {
			origFile.Write(fileData)
			origFile.Close()
		}

		image = Image{
			Id:               vbolt.NextIntId(tx, ImagesBkt),
			FamilyId:         familyId,
			OwnerUserId:      user.Id,
			OriginalFilename: fileHeader.Filename,
			MimeType:         mimeType,
			FileSize:         int(fileHeader.Size),
			Width:            width,
			Height:           height,
			FilePath:         fmt.Sprintf("photos/%s", uniqueFilename),
			Title:            title,
			Description:      description,
			PhotoDate:        calculatedPhotoDate,
			CreatedAt:        time.Now(),
			Status:           1,
		}

		vbolt.Write(tx, ImagesBkt, image.Id, &image)
		vbolt.SetTargetSingleTerm(tx, ImageByFamilyIndex, image.Id, familyId)

		for _, person := range validPersons {
			AddPersonToPhoto(tx, image.Id, person.Id, familyId)
		}

		vbolt.TxCommit(tx)
	})

	if uploadErr != nil {
		RespondWithError(w, r, uploadErr, statusForErrorCode(uploadErr.Code))
		return
	}
	if image.Id == 0 {
		RespondUnexpectedError(w, r, errors.New("photo upload transaction produced no image"))
		return
	}

	file.Seek(0, 0)
	fileData, err := io.ReadAll(file)
	if err != nil {
		log.Printf("Failed to read file for processing queue: %v", err)
	} else {
		job := PhotoProcessingJob{
			ImageId:        image.Id,
			FamilyId:       image.FamilyId,
			FilePath:       image.FilePath,
			FileData:       fileData,
			MimeType:       mimeType,
			OriginalWidth:  width,
			OriginalHeight: height,
		}

		if err := QueuePhotoProcessing(job); err != nil {
			log.Printf("Failed to queue photo %d for processing: %v", image.Id, err)
			markPhotoFailed(image.Id)
			RespondUnavailableError(w, r,
				"The photo could not be processed right now. Please try again in a few minutes.",
				err.Error())
			return
		}
	}

	LogInfoWithRequest(r, LogCategoryPhoto, "Photo upload completed", map[string]interface{}{
		"userId":      user.Id,
		"photoId":     image.Id,
		"personIds":   personIds,
		"peopleCount": len(personIds),
		"fileSize":    fileHeader.Size,
		"mimeType":    mimeType,
		"filename":    fileHeader.Filename,
	})

	image.TagIds = []int{}
	response := AddPhotoResponse{
		Image: image,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func markPhotoFailed(imageId int) {
	vbolt.WithWriteTx(appDb, func(tx *vbolt.Tx) {
		var image Image
		if !vbolt.Read(tx, ImagesBkt, imageId, &image) || image.Id == 0 {
			return
		}
		image.Status = 2
		vbolt.Write(tx, ImagesBkt, image.Id, &image)
		vbolt.TxCommit(tx)
	})
}

const photoCacheControl = "private, max-age=300, must-revalidate"

func servePhotoHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	pathParts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/photo/"), "/")
	if len(pathParts) == 0 || pathParts[0] == "" {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	imageId, err := strconv.Atoi(pathParts[0])
	if err != nil {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	sizeVariant := ""
	if len(pathParts) > 1 {
		sizeVariant = pathParts[1]
	}

	user, ok := GetUserFromContext(r)
	if !ok {
		RespondNotFoundError(w, r, "Not found")
		return
	}

	var image Image
	var canAccess bool
	vbolt.WithReadTx(appDb, func(tx *vbolt.Tx) {
		image = GetImageById(tx, imageId)
		canAccess = CanAccessPhoto(tx, user, image, AccessView)
	})

	if image.Id == 0 || !canAccess {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	if image.Status == 1 {
		serveProcessingPlaceholder(w, r)
		return
	}

	if image.Status == 2 {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	acceptHeader := r.Header.Get("Accept")
	optimalFormat := GetOptimalImageFormat(acceptHeader)

	validSizes := map[string]bool{
		"small": true, "thumb": true, "medium": true,
		"large": true, "xlarge": true, "xxlarge": true, "original": true,
	}

	if sizeVariant != "" && !validSizes[sizeVariant] {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	if sizeVariant == "" {
		sizeVariant = "large"
	}

	basePath := filepath.Join(cfg.StaticDir, image.FilePath)
	baseFilename := strings.TrimSuffix(basePath, filepath.Ext(basePath))

	var fullPath string
	var contentType string

	if sizeVariant == "original" {
		fullPath = baseFilename + "_original" + filepath.Ext(basePath)
		contentType = image.MimeType
	} else {
		for _, format := range []string{optimalFormat, "webp", "jpeg"} {
			var ext string
			switch format {
			case "webp":
				ext = ".webp"
				contentType = "image/webp"
			case "avif":
				ext = ".avif"
				contentType = "image/avif"
			default:
				ext = ".jpg"
				contentType = "image/jpeg"
			}

			if sizeVariant == "large" {
				fullPath = baseFilename + ext
			} else {
				fullPath = baseFilename + "_" + sizeVariant + ext
			}

			if _, err := os.Stat(fullPath); err == nil {
				break
			}
		}
	}

	cleanPath := filepath.Clean(fullPath)
	staticDir := filepath.Clean(cfg.StaticDir)
	if !strings.HasPrefix(cleanPath, staticDir) {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		if sizeVariant != "original" {
			fallbackPath := baseFilename + ".jpg"
			if _, err := os.Stat(fallbackPath); err == nil {
				fullPath = fallbackPath
				contentType = "image/jpeg"
			} else {
				originalPath := baseFilename + "_original" + filepath.Ext(basePath)
				if _, err := os.Stat(originalPath); err == nil {
					fullPath = originalPath
					contentType = image.MimeType
				} else {
					http.Error(w, "Not found", http.StatusNotFound)
					return
				}
			}
		} else {
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}
	}

	w.Header().Set("Content-Type", contentType)

	w.Header().Set("Cache-Control", photoCacheControl)
	w.Header().Set("ETag", fmt.Sprintf("\"%d-%s-%d-%d\"", image.Id, sizeVariant, image.CreatedAt.Unix(), image.Status))

	w.Header().Set("Vary", "Accept")

	http.ServeFile(w, r, fullPath)
}

type UpdatePhotoTagsRequest struct {
	PhotoId int   `json:"photoId"`
	TagIds  []int `json:"tagIds"`
}

type UpdatePhotoTagsResponse struct{}

func UpdatePhotoTags(ctx *vbeam.Context, req UpdatePhotoTagsRequest) (resp UpdatePhotoTagsResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}

	if req.PhotoId <= 0 {
		err = errors.New("Photo ID is required")
		return
	}

	vbeam.UseWriteTx(ctx)

	photo := GetImageById(ctx.Tx, req.PhotoId)
	if photo.Id == 0 || !CanAccessFamily(ctx.Tx, user, photo.FamilyId, AccessContribute) {
		err = errors.New("Photo not found or access denied")
		return
	}

	tagIds := req.TagIds
	if tagIds == nil {
		tagIds = []int{}
	}
	for _, tagId := range tagIds {
		tag := getTagById(ctx.Tx, tagId)
		if tag.Id == 0 || !CanFamilyAccess(ctx.Tx, photo.FamilyId, tag.FamilyId, AccessContribute) {
			err = errors.New("Tag not found or access denied")
			return
		}
	}

	existingTagIds := GetPhotoTagIds(ctx.Tx, req.PhotoId)
	existingSet := make(map[int]struct{}, len(existingTagIds))
	for _, tagId := range existingTagIds {
		existingSet[tagId] = struct{}{}
	}

	desiredSet := make(map[int]struct{}, len(tagIds))
	for _, tagId := range tagIds {
		desiredSet[tagId] = struct{}{}
	}

	for tagId := range existingSet {
		if _, keep := desiredSet[tagId]; !keep {
			removeTagFromPhoto(ctx.Tx, req.PhotoId, tagId)
		}
	}

	for tagId := range desiredSet {
		if _, exists := existingSet[tagId]; !exists {
			addTagToPhoto(ctx.Tx, req.PhotoId, tagId, photo.FamilyId)
		}
	}

	vbolt.TxCommit(ctx.Tx)
	return
}

func GetPhoto(ctx *vbeam.Context, req GetPhotoRequest) (resp GetPhotoResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}

	photo := GetImageById(ctx.Tx, req.Id)

	if !CanAccessPhoto(ctx.Tx, user, photo, AccessView) {
		err = errors.New("Photo not found or access denied")
		return
	}

	people := GetPhotoPeople(ctx.Tx, photo.Id)

	for i := range people {
		people[i].Age = calculateAge(people[i].Birthday)
	}

	resp.Image = photo
	resp.Image.TagIds = GetPhotoTagIds(ctx.Tx, photo.Id)
	resp.People = people
	return
}

func UpdatePhoto(ctx *vbeam.Context, req UpdatePhotoRequest) (resp UpdatePhotoResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}

	if err = validateUpdatePhotoRequest(req); err != nil {
		return
	}

	vbeam.UseWriteTx(ctx)

	photo := GetImageById(ctx.Tx, req.Id)
	if photo.Id == 0 || !CanAccessFamily(ctx.Tx, user, photo.FamilyId, AccessContribute) {
		err = errors.New("Photo not found or access denied")
		return
	}

	people := GetPhotoPeople(ctx.Tx, photo.Id)
	var referencePerson Person
	if len(people) > 0 {
		referencePerson = people[0]
	}

	calculatedPhotoDate, err := calculatePhotoDate(req.InputType, req.PhotoDate, req.AgeYears, req.AgeMonths, referencePerson, nil)
	if err != nil {
		return
	}

	photo.Title = strings.TrimSpace(req.Title)
	photo.Description = strings.TrimSpace(req.Description)
	photo.PhotoDate = calculatedPhotoDate

	if photo.Title == "" {
		photo.Title = generateDefaultTitle(photo.OriginalFilename, calculatedPhotoDate)
	}

	vbolt.Write(ctx.Tx, ImagesBkt, photo.Id, &photo)

	resp.Image = photo
	resp.Image.TagIds = GetPhotoTagIds(ctx.Tx, photo.Id)
	vbolt.TxCommit(ctx.Tx)
	return
}

func DeletePhoto(ctx *vbeam.Context, req DeletePhotoRequest) (resp DeletePhotoResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}

	vbeam.UseWriteTx(ctx)

	photo := GetImageById(ctx.Tx, req.Id)
	if photo.Id == 0 || !CanAccessFamily(ctx.Tx, user, photo.FamilyId, AccessAdmin) {
		err = errors.New("Photo not found or access denied")
		return
	}

	err = deletePhotoFiles(photo)
	if err != nil {
		fmt.Printf("Warning: Failed to delete photo files for ID %d: %v\n", photo.Id, err)
	}

	deletePhotoRecordTx(ctx.Tx, photo)

	vbolt.TxCommit(ctx.Tx)

	resp.Success = true
	return
}

func deletePhotoRecordTx(tx *vbolt.Tx, photo Image) {
	for _, photoPerson := range GetPhotoPersonsByPhoto(tx, photo.Id) {
		vbolt.Delete(tx, PhotoPersonBkt, photoPerson.Id)
		vbolt.SetTargetSingleTerm(tx, PhotoPersonByPhotoIndex, photoPerson.Id, -1)
		vbolt.SetTargetSingleTerm(tx, PhotoPersonByPersonIndex, photoPerson.Id, -1)
		vbolt.SetTargetSingleTerm(tx, PhotoPersonByFamilyIndex, photoPerson.Id, -1)
	}

	removePhotoFromMilestones(tx, photo.Id)
	removePhotoFromActivities(tx, photo.Id)
	removeAllPhotoTags(tx, photo.Id)

	vbolt.Delete(tx, ImagesBkt, photo.Id)
	vbolt.SetTargetSingleTerm(tx, ImageByFamilyIndex, photo.Id, -1)
}

func deletePhotoFiles(photo Image) error {
	basePath := filepath.Join(cfg.StaticDir, photo.FilePath)
	base := strings.TrimSuffix(basePath, filepath.Ext(basePath))

	sizes := []string{"small", "thumb", "medium", "large", "xlarge", "xxlarge"}
	formats := []string{"jpg", "webp", "avif", "png"}

	var filesToDelete []string

	for _, size := range sizes {
		for _, format := range formats {
			var fileName string
			if size == "large" {
				fileName = base + "." + format
			} else {
				fileName = base + "_" + size + "." + format
			}
			filesToDelete = append(filesToDelete, fileName)
		}
	}

	filesToDelete = append(filesToDelete, base+"_original"+filepath.Ext(basePath))

	var lastError error
	for _, filePath := range filesToDelete {
		if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
			lastError = err
		}
	}

	return lastError
}

func validateUpdatePhotoRequest(req UpdatePhotoRequest) error {
	if req.Id <= 0 {
		return errors.New("Invalid photo ID")
	}

	if req.InputType == "" {
		return errors.New("Input type is required")
	}

	validInputTypes := []string{"auto", "today", "date", "age"}
	isValid := false
	for _, validType := range validInputTypes {
		if req.InputType == validType {
			isValid = true
			break
		}
	}
	if !isValid {
		return errors.New("Invalid input type")
	}

	if req.InputType == "date" && req.PhotoDate == "" {
		return errors.New("Photo date is required when input type is 'date'")
	}

	if req.InputType == "age" && req.AgeYears == nil {
		return errors.New("Age years is required when input type is 'age'")
	}

	return nil
}

func serveProcessingPlaceholder(w http.ResponseWriter, r *http.Request) {
	svgContent := `<svg width="400" height="300" xmlns="http://www.w3.org/2000/svg">
		<rect width="100%" height="100%" fill="#f0f0f0"/>
		<circle cx="200" cy="120" r="30" fill="#d0d0d0">
			<animateTransform attributeName="transform" attributeType="XML" type="rotate"
				from="0 200 120" to="360 200 120" dur="1s" repeatCount="indefinite"/>
		</circle>
		<text x="200" y="180" font-family="Arial, sans-serif" font-size="16" text-anchor="middle" fill="#666">
			Processing image...
		</text>
	</svg>`

	w.Header().Set("Content-Type", "image/svg+xml")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")

	w.Write([]byte(svgContent))
}

func GetPhotoStatus(ctx *vbeam.Context, req GetPhotoStatusRequest) (resp GetPhotoStatusResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}

	photo := GetImageById(ctx.Tx, req.Id)

	if !CanAccessPhoto(ctx.Tx, user, photo, AccessView) {
		err = ErrAuthFailure
		return
	}

	resp.Status = photo.Status
	return
}

func ListFamilyPhotos(ctx *vbeam.Context, req ListFamilyPhotosRequest) (resp ListFamilyPhotosResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}

	images := GetVisibleImages(ctx.Tx, user)

	if req.PersonId > 0 {
		photoPersons := GetPhotoPersonsByPerson(ctx.Tx, req.PersonId)
		images = make([]Image, 0, len(photoPersons))
		seenImageIds := make(map[int]struct{}, len(photoPersons))

		for _, photoPerson := range photoPersons {
			if _, exists := seenImageIds[photoPerson.PhotoId]; exists {
				continue
			}

			image := GetImageById(ctx.Tx, photoPerson.PhotoId)
			if !CanAccessPhoto(ctx.Tx, user, image, AccessView) {
				continue
			}

			seenImageIds[photoPerson.PhotoId] = struct{}{}
			images = append(images, image)
		}
	}

	resp.Photos = make([]PhotoWithPeople, 0, len(images))

	for _, image := range images {
		if image.Status == 2 {
			continue
		}

		people := GetPhotoPeople(ctx.Tx, image.Id)

		for i := range people {
			people[i].Age = calculateAge(people[i].Birthday)
		}

		image.TagIds = GetPhotoTagIds(ctx.Tx, image.Id)

		resp.Photos = append(resp.Photos, PhotoWithPeople{
			Image:  image,
			People: people,
		})
	}

	return
}

func AddPeopleToPhoto(ctx *vbeam.Context, req AddPeopleToPhotoRequest) (resp AddPeopleToPhotoResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}

	if req.PhotoId <= 0 {
		err = errors.New("Invalid photo ID")
		return
	}

	if len(req.PersonIds) == 0 {
		err = errors.New("At least one person ID is required")
		return
	}

	vbeam.UseWriteTx(ctx)

	photo := GetImageById(ctx.Tx, req.PhotoId)
	if photo.Id == 0 || !CanAccessFamily(ctx.Tx, user, photo.FamilyId, AccessContribute) {
		err = errors.New("Photo not found or access denied")
		return
	}

	existingPeople := GetPhotoPeople(ctx.Tx, req.PhotoId)
	existingPersonIds := make(map[int]bool)
	for _, person := range existingPeople {
		existingPersonIds[person.Id] = true
	}

	var addedPeople []Person
	for _, personId := range req.PersonIds {
		if existingPersonIds[personId] {
			continue
		}

		person := GetPersonById(ctx.Tx, personId)
		if person.Id == 0 || !CanFamilyAccess(ctx.Tx, photo.FamilyId, person.FamilyId, AccessContribute) {
			continue
		}

		AddPersonToPhoto(ctx.Tx, req.PhotoId, personId, photo.FamilyId)
		person.Age = calculateAge(person.Birthday)
		addedPeople = append(addedPeople, person)
	}

	vbolt.TxCommit(ctx.Tx)

	resp.Success = true
	resp.People = addedPeople
	return
}

func RemovePersonFromPhotoProc(ctx *vbeam.Context, req RemovePersonFromPhotoRequest) (resp RemovePersonFromPhotoResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}

	if req.PhotoId <= 0 {
		err = errors.New("Invalid photo ID")
		return
	}

	if req.PersonId <= 0 {
		err = errors.New("Invalid person ID")
		return
	}

	vbeam.UseWriteTx(ctx)

	photo := GetImageById(ctx.Tx, req.PhotoId)
	if photo.Id == 0 || !CanAccessFamily(ctx.Tx, user, photo.FamilyId, AccessContribute) {
		err = errors.New("Photo not found or access denied")
		return
	}

	person := GetPersonById(ctx.Tx, req.PersonId)
	if person.Id == 0 || !CanFamilyAccess(ctx.Tx, photo.FamilyId, person.FamilyId, AccessContribute) {
		err = errors.New("Person not found or access denied")
		return
	}

	RemovePersonFromPhoto(ctx.Tx, req.PhotoId, req.PersonId)

	vbolt.TxCommit(ctx.Tx)

	resp.Success = true
	return
}
