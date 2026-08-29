package backend

import (
	"errors"
	"family/cfg"
	"fmt"
	"strings"
	"time"

	"go.hasen.dev/vbeam"
	"go.hasen.dev/vbolt"
	"go.hasen.dev/vpack"
)

func RegisterMilestoneMethods(app *vbeam.Application) {
	vbeam.RegisterProc(app, AddMilestone)
	vbeam.RegisterProc(app, GetPersonMilestones)
	vbeam.RegisterProc(app, GetMilestone)
	vbeam.RegisterProc(app, UpdateMilestone)
	vbeam.RegisterProc(app, DeleteMilestone)
	vbeam.RegisterProc(app, SearchMilestones)
	vbeam.RegisterProc(app, UpdateMilestoneTags)
}

type AddMilestoneRequest struct {
	PersonId      int     `json:"personId"`
	Description   string  `json:"description"`
	Category      string  `json:"category"`
	InputType     string  `json:"inputType"`
	MilestoneDate *string `json:"milestoneDate,omitempty"`
	AgeYears      *int    `json:"ageYears,omitempty"`
	AgeMonths     *int    `json:"ageMonths,omitempty"`
	PhotoIds      []int   `json:"photoIds,omitempty"`
}

type AddMilestoneResponse struct {
	Milestone Milestone `json:"milestone"`
}

type GetPersonMilestonesRequest struct {
	PersonId int `json:"personId"`
}

type GetPersonMilestonesResponse struct {
	Milestones []Milestone `json:"milestones"`
}

type UpdateMilestoneRequest struct {
	Id            int     `json:"id"`
	Description   string  `json:"description"`
	Category      string  `json:"category"`
	InputType     string  `json:"inputType"`
	MilestoneDate *string `json:"milestoneDate,omitempty"`
	AgeYears      *int    `json:"ageYears,omitempty"`
	AgeMonths     *int    `json:"ageMonths,omitempty"`
	PhotoIds      []int   `json:"photoIds,omitempty"`
}

type UpdateMilestoneResponse struct {
	Milestone Milestone `json:"milestone"`
}

type DeleteMilestoneRequest struct {
	Id int `json:"id"`
}

type DeleteMilestoneResponse struct {
	Success bool `json:"success"`
}

type GetMilestoneRequest struct {
	Id int `json:"id"`
}

type GetMilestoneResponse struct {
	Milestone Milestone `json:"milestone"`
}

type SearchMilestonesRequest struct {
	Query string `json:"query"`
	Limit *int   `json:"limit,omitempty"`
}

type SearchMilestonesResponse struct {
	Milestones []Milestone `json:"milestones"`
	Query      string      `json:"query"`
}

type Milestone struct {
	Id            int       `json:"id"`
	PersonId      int       `json:"personId"`
	FamilyId      int       `json:"familyId"`
	Description   string    `json:"description"`
	Category      string    `json:"category"`
	MilestoneDate time.Time `json:"milestoneDate"`
	CreatedAt     time.Time `json:"createdAt"`
	PhotoIds      []int     `json:"photoIds,omitempty"`
	TagIds        []int     `json:"tagIds,omitempty"`
}

type MilestonePhoto struct {
	Id          int       `json:"id"`
	MilestoneId int       `json:"milestoneId"`
	PhotoId     int       `json:"photoId"`
	FamilyId    int       `json:"familyId"`
	CreatedAt   time.Time `json:"createdAt"`
}

type MilestoneTag struct {
	Id          int
	MilestoneId int
	TagId       int
	FamilyId    int
	CreatedAt   time.Time
}

func PackMilestoneTag(self *MilestoneTag, buf *vpack.Buffer) {
	vpack.Version(1, buf)
	vpack.Int(&self.Id, buf)
	vpack.Int(&self.MilestoneId, buf)
	vpack.Int(&self.TagId, buf)
	vpack.Int(&self.FamilyId, buf)
	vpack.Time(&self.CreatedAt, buf)
}

func PackMilestone(self *Milestone, buf *vpack.Buffer) {
	vpack.Version(1, buf)
	vpack.Int(&self.Id, buf)
	vpack.Int(&self.PersonId, buf)
	vpack.Int(&self.FamilyId, buf)
	vpack.String(&self.Description, buf)
	vpack.String(&self.Category, buf)
	vpack.Time(&self.MilestoneDate, buf)
	vpack.Time(&self.CreatedAt, buf)
}

func PackMilestonePhoto(self *MilestonePhoto, buf *vpack.Buffer) {
	vpack.Version(1, buf)
	vpack.Int(&self.Id, buf)
	vpack.Int(&self.MilestoneId, buf)
	vpack.Int(&self.PhotoId, buf)
	vpack.Int(&self.FamilyId, buf)
	vpack.Time(&self.CreatedAt, buf)
}

var MilestoneBkt = vbolt.Bucket(&cfg.Info, "milestones", vpack.FInt, PackMilestone)
var MilestonePhotoBkt = vbolt.Bucket(&cfg.Info, "milestone_photos", vpack.FInt, PackMilestonePhoto)
var MilestoneTagBkt = vbolt.Bucket(&cfg.Info, "milestone_tags", vpack.FInt, PackMilestoneTag)

var MilestoneByPersonIndex = vbolt.Index(&cfg.Info, "milestones_by_person", vpack.FInt, vpack.FInt)

var MilestoneByFamilyIndex = vbolt.Index(&cfg.Info, "milestones_by_family", vpack.FInt, vpack.FInt)

var MilestoneSearchIndex = vbolt.IndexExt(&cfg.Info, "milestones_search", vpack.StringZ, vpack.UnixTimeKey, vpack.FInt)

var MilestonePhotoByMilestoneIndex = vbolt.Index(&cfg.Info, "milestone_photo_by_milestone", vpack.FInt, vpack.FInt)

var MilestonePhotoByPhotoIndex = vbolt.Index(&cfg.Info, "milestone_photo_by_photo", vpack.FInt, vpack.FInt)

var MilestonePhotoByFamilyIndex = vbolt.Index(&cfg.Info, "milestone_photo_by_family", vpack.FInt, vpack.FInt)

var MilestoneTagByMilestoneIndex = vbolt.Index(&cfg.Info, "milestone_tag_by_milestone", vpack.FInt, vpack.FInt)

var MilestoneTagByTagIndex = vbolt.Index(&cfg.Info, "milestone_tag_by_tag", vpack.FInt, vpack.FInt)

var MilestoneTagByFamilyIndex = vbolt.Index(&cfg.Info, "milestone_tag_by_family", vpack.FInt, vpack.FInt)

func GetMilestoneById(tx *vbolt.Tx, milestoneId int) (milestone Milestone) {
	vbolt.Read(tx, MilestoneBkt, milestoneId, &milestone)
	return
}

func GetPersonMilestonesTx(tx *vbolt.Tx, personId int) []Milestone {
	milestones := []Milestone{}
	var milestoneIds []int
	vbolt.ReadTermTargets(tx, MilestoneByPersonIndex, personId, &milestoneIds, vbolt.Window{})
	if len(milestoneIds) > 0 {
		vbolt.ReadSlice(tx, MilestoneBkt, milestoneIds, &milestones)
	}
	return milestones
}

func GetMilestonePhotoIds(tx *vbolt.Tx, milestoneId int) []int {
	var milestonePhotoIds []int
	vbolt.ReadTermTargets(tx, MilestonePhotoByMilestoneIndex, milestoneId, &milestonePhotoIds, vbolt.Window{})
	if len(milestonePhotoIds) == 0 {
		return []int{}
	}

	var milestonePhotos []MilestonePhoto
	vbolt.ReadSlice(tx, MilestonePhotoBkt, milestonePhotoIds, &milestonePhotos)

	photoIds := make([]int, 0, len(milestonePhotos))
	for _, milestonePhoto := range milestonePhotos {
		photoIds = append(photoIds, milestonePhoto.PhotoId)
	}

	return photoIds
}

func GetMilestoneTagIds(tx *vbolt.Tx, milestoneId int) []int {
	var mtIds []int
	vbolt.ReadTermTargets(tx, MilestoneTagByMilestoneIndex, milestoneId, &mtIds, vbolt.Window{})
	if len(mtIds) == 0 {
		return []int{}
	}
	var mts []MilestoneTag
	vbolt.ReadSlice(tx, MilestoneTagBkt, mtIds, &mts)
	tagIds := make([]int, 0, len(mts))
	for _, mt := range mts {
		tagIds = append(tagIds, mt.TagId)
	}
	return tagIds
}

func GetMilestoneByIdAndFamily(tx *vbolt.Tx, milestoneId int, familyId int) (Milestone, error) {
	milestone := GetMilestoneById(tx, milestoneId)
	if milestone.Id == 0 {
		return milestone, errors.New("Milestone not found")
	}
	if !CanFamilyAccess(tx, familyId, milestone.FamilyId, AccessView) {
		return milestone, errors.New("Access denied: milestone belongs to another family")
	}
	return milestone, nil
}

func GetMilestoneForUser(tx *vbolt.Tx, milestoneId int, user User, need AccessLevel) (Milestone, error) {
	milestone := GetMilestoneById(tx, milestoneId)
	if milestone.Id == 0 {
		return milestone, errors.New("Milestone not found")
	}
	if !CanAccessRecordOfPerson(tx, user, milestone.FamilyId, milestone.PersonId, ScopeMilestones, need) {
		return milestone, errors.New("Access denied: milestone belongs to another family")
	}
	return milestone, nil
}

func SearchMilestonesTx(tx *vbolt.Tx, query string, familyId int, limit int) []Milestone {
	return searchMilestonesTx(tx, query, limit, func(milestone Milestone) bool {
		return CanFamilyAccess(tx, familyId, milestone.FamilyId, AccessView)
	})
}

func SearchVisibleMilestones(tx *vbolt.Tx, query string, user User, limit int) []Milestone {
	return searchMilestonesTx(tx, query, limit, func(milestone Milestone) bool {
		return CanAccessRecordOfPerson(tx, user, milestone.FamilyId, milestone.PersonId, ScopeMilestones, AccessView)
	})
}

func searchMilestonesTx(tx *vbolt.Tx, query string, limit int, canSee func(Milestone) bool) (milestones []Milestone) {
	words := strings.Fields(strings.ToLower(query))
	terms := make([]string, 0, len(words))

	for _, word := range words {
		word = strings.Trim(word, ".,!?;:()[]{}\"'")
		if len(word) >= 3 {
			terms = append(terms, word)
		}
	}

	if len(terms) == 0 {
		return []Milestone{}
	}

	milestoneIdMap := make(map[int]bool)

	for _, term := range terms {
		var ids []int
		vbolt.ReadTermTargets(tx, MilestoneSearchIndex, term, &ids, vbolt.Window{Limit: limit * 2})
		for _, id := range ids {
			milestoneIdMap[id] = true
		}
	}

	var milestoneIds []int
	for id := range milestoneIdMap {
		milestoneIds = append(milestoneIds, id)
	}

	var allMilestones []Milestone
	if len(milestoneIds) > 0 {
		vbolt.ReadSlice(tx, MilestoneBkt, milestoneIds, &allMilestones)
	}

	milestones = make([]Milestone, 0, limit)
	for _, milestone := range allMilestones {
		if canSee(milestone) {
			milestones = append(milestones, milestone)
			if len(milestones) >= limit {
				break
			}
		}
	}

	return
}

func UpdateMilestoneSearchIndex(tx *vbolt.Tx, milestone Milestone) {
	terms := make([]string, 0, 10)

	words := strings.Fields(strings.ToLower(milestone.Description))
	for _, word := range words {
		word = strings.Trim(word, ".,!?;:()[]{}\"'")
		if len(word) >= 3 {
			terms = append(terms, word)
		}
	}

	terms = append(terms, fmt.Sprintf("cat:%s", milestone.Category))

	terms = append(terms, fmt.Sprintf("y:%d", milestone.MilestoneDate.Year()))
	terms = append(terms, fmt.Sprintf("m:%s", milestone.MilestoneDate.Format("2006.01")))

	terms = append(terms, fmt.Sprintf("p:%d", milestone.PersonId))

	vbolt.SetTargetTermsUniform(tx, MilestoneSearchIndex, milestone.Id, terms, milestone.MilestoneDate)
}

func normalizePhotoIds(photoIds []int) []int {
	unique := make(map[int]struct{})
	normalized := make([]int, 0, len(photoIds))

	for _, photoId := range photoIds {
		if photoId <= 0 {
			continue
		}
		if _, exists := unique[photoId]; exists {
			continue
		}
		unique[photoId] = struct{}{}
		normalized = append(normalized, photoId)
	}

	return normalized
}

func validatePhotoAccess(tx *vbolt.Tx, photoId int, familyId int) error {
	photo := GetImageById(tx, photoId)
	if photo.Id == 0 || !CanFamilyAccess(tx, familyId, photo.FamilyId, AccessContribute) {
		return errors.New("Photo not found or access denied")
	}
	return nil
}

func addPhotoToMilestone(tx *vbolt.Tx, milestoneId int, photoId int, familyId int) error {
	if err := validatePhotoAccess(tx, photoId, familyId); err != nil {
		return err
	}

	existingPhotoIds := GetMilestonePhotoIds(tx, milestoneId)
	for _, existingId := range existingPhotoIds {
		if existingId == photoId {
			return nil
		}
	}

	milestonePhoto := MilestonePhoto{
		Id:          vbolt.NextIntId(tx, MilestonePhotoBkt),
		MilestoneId: milestoneId,
		PhotoId:     photoId,
		FamilyId:    familyId,
		CreatedAt:   time.Now(),
	}

	vbolt.Write(tx, MilestonePhotoBkt, milestonePhoto.Id, &milestonePhoto)
	vbolt.SetTargetSingleTerm(tx, MilestonePhotoByMilestoneIndex, milestonePhoto.Id, milestoneId)
	vbolt.SetTargetSingleTerm(tx, MilestonePhotoByPhotoIndex, milestonePhoto.Id, photoId)
	vbolt.SetTargetSingleTerm(tx, MilestonePhotoByFamilyIndex, milestonePhoto.Id, familyId)
	return nil
}

func removePhotoFromMilestone(tx *vbolt.Tx, milestoneId int, photoId int) {
	var milestonePhotoIds []int
	vbolt.ReadTermTargets(tx, MilestonePhotoByMilestoneIndex, milestoneId, &milestonePhotoIds, vbolt.Window{})
	if len(milestonePhotoIds) == 0 {
		return
	}

	var milestonePhotos []MilestonePhoto
	vbolt.ReadSlice(tx, MilestonePhotoBkt, milestonePhotoIds, &milestonePhotos)
	for _, milestonePhoto := range milestonePhotos {
		if milestonePhoto.PhotoId != photoId {
			continue
		}
		vbolt.Delete(tx, MilestonePhotoBkt, milestonePhoto.Id)
		vbolt.SetTargetSingleTerm(tx, MilestonePhotoByMilestoneIndex, milestonePhoto.Id, -1)
		vbolt.SetTargetSingleTerm(tx, MilestonePhotoByPhotoIndex, milestonePhoto.Id, -1)
		vbolt.SetTargetSingleTerm(tx, MilestonePhotoByFamilyIndex, milestonePhoto.Id, -1)
	}
}

func removeAllMilestonePhotos(tx *vbolt.Tx, milestoneId int) {
	var milestonePhotoIds []int
	vbolt.ReadTermTargets(tx, MilestonePhotoByMilestoneIndex, milestoneId, &milestonePhotoIds, vbolt.Window{})
	if len(milestonePhotoIds) == 0 {
		return
	}

	var milestonePhotos []MilestonePhoto
	vbolt.ReadSlice(tx, MilestonePhotoBkt, milestonePhotoIds, &milestonePhotos)
	for _, milestonePhoto := range milestonePhotos {
		vbolt.Delete(tx, MilestonePhotoBkt, milestonePhoto.Id)
		vbolt.SetTargetSingleTerm(tx, MilestonePhotoByMilestoneIndex, milestonePhoto.Id, -1)
		vbolt.SetTargetSingleTerm(tx, MilestonePhotoByPhotoIndex, milestonePhoto.Id, -1)
		vbolt.SetTargetSingleTerm(tx, MilestonePhotoByFamilyIndex, milestonePhoto.Id, -1)
	}
}

func removePhotoFromMilestones(tx *vbolt.Tx, photoId int) {
	var milestonePhotoIds []int
	vbolt.ReadTermTargets(tx, MilestonePhotoByPhotoIndex, photoId, &milestonePhotoIds, vbolt.Window{})
	if len(milestonePhotoIds) == 0 {
		return
	}

	var milestonePhotos []MilestonePhoto
	vbolt.ReadSlice(tx, MilestonePhotoBkt, milestonePhotoIds, &milestonePhotos)
	for _, milestonePhoto := range milestonePhotos {
		vbolt.Delete(tx, MilestonePhotoBkt, milestonePhoto.Id)
		vbolt.SetTargetSingleTerm(tx, MilestonePhotoByMilestoneIndex, milestonePhoto.Id, -1)
		vbolt.SetTargetSingleTerm(tx, MilestonePhotoByPhotoIndex, milestonePhoto.Id, -1)
		vbolt.SetTargetSingleTerm(tx, MilestonePhotoByFamilyIndex, milestonePhoto.Id, -1)
	}
}

func addTagToMilestone(tx *vbolt.Tx, milestoneId int, tagId int, familyId int) {
	mt := MilestoneTag{
		Id:          vbolt.NextIntId(tx, MilestoneTagBkt),
		MilestoneId: milestoneId,
		TagId:       tagId,
		FamilyId:    familyId,
		CreatedAt:   time.Now(),
	}
	vbolt.Write(tx, MilestoneTagBkt, mt.Id, &mt)
	vbolt.SetTargetSingleTerm(tx, MilestoneTagByMilestoneIndex, mt.Id, milestoneId)
	vbolt.SetTargetSingleTerm(tx, MilestoneTagByTagIndex, mt.Id, tagId)
	vbolt.SetTargetSingleTerm(tx, MilestoneTagByFamilyIndex, mt.Id, familyId)
}

func removeTagFromMilestone(tx *vbolt.Tx, milestoneId int, tagId int) {
	var mtIds []int
	vbolt.ReadTermTargets(tx, MilestoneTagByMilestoneIndex, milestoneId, &mtIds, vbolt.Window{})
	if len(mtIds) == 0 {
		return
	}
	var mts []MilestoneTag
	vbolt.ReadSlice(tx, MilestoneTagBkt, mtIds, &mts)
	for _, mt := range mts {
		if mt.TagId != tagId {
			continue
		}
		vbolt.Delete(tx, MilestoneTagBkt, mt.Id)
		vbolt.SetTargetSingleTerm(tx, MilestoneTagByMilestoneIndex, mt.Id, -1)
		vbolt.SetTargetSingleTerm(tx, MilestoneTagByTagIndex, mt.Id, -1)
		vbolt.SetTargetSingleTerm(tx, MilestoneTagByFamilyIndex, mt.Id, -1)
	}
}

func removeAllMilestoneTags(tx *vbolt.Tx, milestoneId int) {
	var mtIds []int
	vbolt.ReadTermTargets(tx, MilestoneTagByMilestoneIndex, milestoneId, &mtIds, vbolt.Window{})
	if len(mtIds) == 0 {
		return
	}
	var mts []MilestoneTag
	vbolt.ReadSlice(tx, MilestoneTagBkt, mtIds, &mts)
	for _, mt := range mts {
		vbolt.Delete(tx, MilestoneTagBkt, mt.Id)
		vbolt.SetTargetSingleTerm(tx, MilestoneTagByMilestoneIndex, mt.Id, -1)
		vbolt.SetTargetSingleTerm(tx, MilestoneTagByTagIndex, mt.Id, -1)
		vbolt.SetTargetSingleTerm(tx, MilestoneTagByFamilyIndex, mt.Id, -1)
	}
}

func removeMilestoneTagsByTag(tx *vbolt.Tx, tagId int) {
	var mtIds []int
	vbolt.ReadTermTargets(tx, MilestoneTagByTagIndex, tagId, &mtIds, vbolt.Window{})
	if len(mtIds) == 0 {
		return
	}
	var mts []MilestoneTag
	vbolt.ReadSlice(tx, MilestoneTagBkt, mtIds, &mts)
	for _, mt := range mts {
		vbolt.Delete(tx, MilestoneTagBkt, mt.Id)
		vbolt.SetTargetSingleTerm(tx, MilestoneTagByMilestoneIndex, mt.Id, -1)
		vbolt.SetTargetSingleTerm(tx, MilestoneTagByTagIndex, mt.Id, -1)
		vbolt.SetTargetSingleTerm(tx, MilestoneTagByFamilyIndex, mt.Id, -1)
	}
}

func AddMilestoneTx(tx *vbolt.Tx, req AddMilestoneRequest, familyId int) (Milestone, error) {
	var milestone Milestone
	var err error

	person := GetPersonById(tx, req.PersonId)
	if person.Id == 0 || !CanFamilyAccess(tx, familyId, person.FamilyId, AccessContribute) {
		return milestone, errors.New("Person not found or not in your family")
	}

	milestone.MilestoneDate, err = parseMilestoneDate(req, person.Birthday)
	if err != nil {
		return milestone, err
	}

	milestone.Id = vbolt.NextIntId(tx, MilestoneBkt)
	milestone.PersonId = req.PersonId
	milestone.FamilyId = familyId
	milestone.Description = strings.TrimSpace(req.Description)
	milestone.Category = req.Category
	milestone.CreatedAt = time.Now()

	vbolt.Write(tx, MilestoneBkt, milestone.Id, &milestone)

	updateMilestoneIndices(tx, milestone)

	if req.PhotoIds != nil {
		photoIds := normalizePhotoIds(req.PhotoIds)
		for _, photoId := range photoIds {
			if err := addPhotoToMilestone(tx, milestone.Id, photoId, familyId); err != nil {
				return milestone, err
			}
		}
	}

	return milestone, nil
}

func updateMilestoneIndices(tx *vbolt.Tx, milestone Milestone) {
	vbolt.SetTargetSingleTerm(tx, MilestoneByPersonIndex, milestone.Id, milestone.PersonId)
	vbolt.SetTargetSingleTerm(tx, MilestoneByFamilyIndex, milestone.Id, milestone.FamilyId)
	UpdateMilestoneSearchIndex(tx, milestone)
}

func UpdateMilestoneTx(tx *vbolt.Tx, req UpdateMilestoneRequest, familyId int) (Milestone, error) {
	var err error

	milestone, err := GetMilestoneByIdAndFamily(tx, req.Id, familyId)
	if err != nil {
		return milestone, err
	}

	person := GetPersonById(tx, milestone.PersonId)
	if person.Id == 0 {
		return milestone, errors.New("Person not found")
	}

	milestone.MilestoneDate, err = parseMilestoneDate(req, person.Birthday)
	if err != nil {
		return milestone, err
	}

	milestone.Description = strings.TrimSpace(req.Description)
	milestone.Category = req.Category

	vbolt.Write(tx, MilestoneBkt, milestone.Id, &milestone)

	UpdateMilestoneSearchIndex(tx, milestone)

	if req.PhotoIds != nil {
		photoIds := normalizePhotoIds(req.PhotoIds)
		for _, photoId := range photoIds {
			if err := validatePhotoAccess(tx, photoId, familyId); err != nil {
				return milestone, err
			}
		}

		existingPhotoIds := GetMilestonePhotoIds(tx, milestone.Id)
		existingSet := make(map[int]struct{}, len(existingPhotoIds))
		for _, photoId := range existingPhotoIds {
			existingSet[photoId] = struct{}{}
		}

		desiredSet := make(map[int]struct{}, len(photoIds))
		for _, photoId := range photoIds {
			desiredSet[photoId] = struct{}{}
		}

		for photoId := range existingSet {
			if _, keep := desiredSet[photoId]; !keep {
				removePhotoFromMilestone(tx, milestone.Id, photoId)
			}
		}

		for photoId := range desiredSet {
			if _, exists := existingSet[photoId]; !exists {
				if err := addPhotoToMilestone(tx, milestone.Id, photoId, familyId); err != nil {
					return milestone, err
				}
			}
		}
	}

	return milestone, nil
}

func getFamilyMilestones(tx *vbolt.Tx, familyId int) (milestones []Milestone) {
	var milestoneIds []int
	vbolt.ReadTermTargets(tx, MilestoneByFamilyIndex, familyId, &milestoneIds, vbolt.Window{})
	if len(milestoneIds) > 0 {
		vbolt.ReadSlice(tx, MilestoneBkt, milestoneIds, &milestones)
	}
	return
}

func DeleteMilestoneTx(tx *vbolt.Tx, milestoneId int, familyId int) error {
	milestone, err := GetMilestoneByIdAndFamily(tx, milestoneId, familyId)
	if err != nil {
		return err
	}

	vbolt.SetTargetSingleTerm(tx, MilestoneByPersonIndex, milestone.Id, -1)
	vbolt.SetTargetSingleTerm(tx, MilestoneByFamilyIndex, milestone.Id, -1)
	vbolt.SetTargetTermsUniform(tx, MilestoneSearchIndex, milestone.Id, []string{}, time.Time{})

	removeAllMilestonePhotos(tx, milestone.Id)
	removeAllMilestoneTags(tx, milestone.Id)

	vbolt.Delete(tx, MilestoneBkt, milestone.Id)

	return nil
}

type MilestoneDateRequest interface {
	GetInputType() string
	GetMilestoneDate() *string
	GetAgeYears() *int
	GetAgeMonths() *int
}

func (req AddMilestoneRequest) GetInputType() string      { return req.InputType }
func (req AddMilestoneRequest) GetMilestoneDate() *string { return req.MilestoneDate }
func (req AddMilestoneRequest) GetAgeYears() *int         { return req.AgeYears }
func (req AddMilestoneRequest) GetAgeMonths() *int        { return req.AgeMonths }

func (req UpdateMilestoneRequest) GetInputType() string      { return req.InputType }
func (req UpdateMilestoneRequest) GetMilestoneDate() *string { return req.MilestoneDate }
func (req UpdateMilestoneRequest) GetAgeYears() *int         { return req.AgeYears }
func (req UpdateMilestoneRequest) GetAgeMonths() *int        { return req.AgeMonths }

func parseMilestoneDate(req MilestoneDateRequest, personBirthday time.Time) (time.Time, error) {
	if req.GetInputType() == "today" {
		return time.Now(), nil
	} else if req.GetInputType() == "date" {
		milestoneDate := req.GetMilestoneDate()
		if milestoneDate == nil || *milestoneDate == "" {
			return time.Time{}, errors.New("Milestone date is required when input type is 'date'")
		}
		return time.Parse("2006-01-02", *milestoneDate)
	} else if req.GetInputType() == "age" {
		ageYears := req.GetAgeYears()
		if ageYears == nil || *ageYears < 0 {
			return time.Time{}, errors.New("Age years must be non-negative")
		}
		ageMonths := 0
		if req.GetAgeMonths() != nil {
			if *req.GetAgeMonths() < 0 || *req.GetAgeMonths() > 11 {
				return time.Time{}, errors.New("Age months must be between 0 and 11")
			}
			ageMonths = *req.GetAgeMonths()
		}

		targetDate := personBirthday.AddDate(*ageYears, ageMonths, 0)
		return targetDate, nil
	} else {
		return time.Time{}, errors.New("Input type must be 'today', 'date' or 'age'")
	}
}

func validateAddMilestoneRequest(req AddMilestoneRequest) error {
	if req.PersonId <= 0 {
		return errors.New("Person ID is required")
	}
	if strings.TrimSpace(req.Description) == "" {
		return errors.New("Description is required")
	}
	validCategories := []string{"development", "behavior", "health", "achievement", "first", "other"}
	isValidCategory := false
	for _, category := range validCategories {
		if req.Category == category {
			isValidCategory = true
			break
		}
	}
	if !isValidCategory {
		return errors.New("Category must be one of: development, behavior, health, achievement, first, other")
	}
	if req.InputType != "today" && req.InputType != "date" && req.InputType != "age" {
		return errors.New("Input type must be 'today', 'date' or 'age'")
	}
	return nil
}

func validateUpdateMilestoneRequest(req UpdateMilestoneRequest) error {
	if req.Id <= 0 {
		return errors.New("Milestone ID is required")
	}
	if strings.TrimSpace(req.Description) == "" {
		return errors.New("Description is required")
	}
	validCategories := []string{"development", "behavior", "health", "achievement", "first", "other"}
	isValidCategory := false
	for _, category := range validCategories {
		if req.Category == category {
			isValidCategory = true
			break
		}
	}
	if !isValidCategory {
		return errors.New("Category must be one of: development, behavior, health, achievement, first, other")
	}
	if req.InputType != "today" && req.InputType != "date" && req.InputType != "age" {
		return errors.New("Input type must be 'today', 'date' or 'age'")
	}
	return nil
}

func AddMilestone(ctx *vbeam.Context, req AddMilestoneRequest) (resp AddMilestoneResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}

	if err = validateAddMilestoneRequest(req); err != nil {
		return
	}

	familyId, err := ActingFamilyForPerson(ctx.Tx, user, req.PersonId, AccessContribute)
	if err != nil {
		return
	}

	vbeam.UseWriteTx(ctx)
	milestone, err := AddMilestoneTx(ctx.Tx, req, familyId)
	if err != nil {
		return
	}

	resp.Milestone = milestone
	resp.Milestone.PhotoIds = GetMilestonePhotoIds(ctx.Tx, milestone.Id)
	resp.Milestone.TagIds = GetMilestoneTagIds(ctx.Tx, milestone.Id)

	vbolt.TxCommit(ctx.Tx)
	return
}

func GetPersonMilestones(ctx *vbeam.Context, req GetPersonMilestonesRequest) (resp GetPersonMilestonesResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}

	person := GetPersonById(ctx.Tx, req.PersonId)
	if !CanAccessPerson(ctx.Tx, user, person, ScopeMilestones, AccessView) {
		err = errors.New("Person not found or not in your family")
		return
	}

	milestones := GetPersonMilestonesTx(ctx.Tx, req.PersonId)
	for i := range milestones {
		milestones[i].PhotoIds = GetMilestonePhotoIds(ctx.Tx, milestones[i].Id)
		milestones[i].TagIds = GetMilestoneTagIds(ctx.Tx, milestones[i].Id)
	}
	resp.Milestones = milestones
	return
}

func GetMilestone(ctx *vbeam.Context, req GetMilestoneRequest) (resp GetMilestoneResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}

	if req.Id <= 0 {
		err = errors.New("Milestone ID is required")
		return
	}

	milestone, err := GetMilestoneForUser(ctx.Tx, req.Id, user, AccessView)
	if err != nil {
		return
	}

	resp.Milestone = milestone
	resp.Milestone.PhotoIds = GetMilestonePhotoIds(ctx.Tx, milestone.Id)
	resp.Milestone.TagIds = GetMilestoneTagIds(ctx.Tx, milestone.Id)
	return
}

func UpdateMilestone(ctx *vbeam.Context, req UpdateMilestoneRequest) (resp UpdateMilestoneResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}

	if err = validateUpdateMilestoneRequest(req); err != nil {
		return
	}

	existing, err := GetMilestoneForUser(ctx.Tx, req.Id, user, AccessContribute)
	if err != nil {
		return
	}

	vbeam.UseWriteTx(ctx)
	milestone, err := UpdateMilestoneTx(ctx.Tx, req, existing.FamilyId)
	if err != nil {
		return
	}

	resp.Milestone = milestone
	resp.Milestone.PhotoIds = GetMilestonePhotoIds(ctx.Tx, milestone.Id)
	resp.Milestone.TagIds = GetMilestoneTagIds(ctx.Tx, milestone.Id)

	vbolt.TxCommit(ctx.Tx)
	return
}

func DeleteMilestone(ctx *vbeam.Context, req DeleteMilestoneRequest) (resp DeleteMilestoneResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}

	if req.Id <= 0 {
		err = errors.New("Milestone ID is required")
		return
	}

	existing, err := GetMilestoneForUser(ctx.Tx, req.Id, user, AccessContribute)
	if err != nil {
		return
	}

	vbeam.UseWriteTx(ctx)
	err = DeleteMilestoneTx(ctx.Tx, req.Id, existing.FamilyId)
	if err != nil {
		return
	}

	vbolt.TxCommit(ctx.Tx)

	resp.Success = true
	return
}

type UpdateMilestoneTagsRequest struct {
	MilestoneId int   `json:"milestoneId"`
	TagIds      []int `json:"tagIds"`
}

type UpdateMilestoneTagsResponse struct{}

func UpdateMilestoneTags(ctx *vbeam.Context, req UpdateMilestoneTagsRequest) (resp UpdateMilestoneTagsResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}

	if req.MilestoneId <= 0 {
		err = errors.New("Milestone ID is required")
		return
	}

	vbeam.UseWriteTx(ctx)

	milestone, err := GetMilestoneForUser(ctx.Tx, req.MilestoneId, user, AccessContribute)
	if err != nil {
		return
	}

	tagIds := req.TagIds
	if tagIds == nil {
		tagIds = []int{}
	}
	for _, tagId := range tagIds {
		tag := getTagById(ctx.Tx, tagId)
		if tag.Id == 0 || !CanFamilyAccess(ctx.Tx, milestone.FamilyId, tag.FamilyId, AccessContribute) {
			err = errors.New("Tag not found or access denied")
			return
		}
	}

	existingTagIds := GetMilestoneTagIds(ctx.Tx, req.MilestoneId)
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
			removeTagFromMilestone(ctx.Tx, req.MilestoneId, tagId)
		}
	}

	for tagId := range desiredSet {
		if _, exists := existingSet[tagId]; !exists {
			addTagToMilestone(ctx.Tx, req.MilestoneId, tagId, milestone.FamilyId)
		}
	}

	vbolt.TxCommit(ctx.Tx)
	return
}

func SearchMilestones(ctx *vbeam.Context, req SearchMilestonesRequest) (resp SearchMilestonesResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}

	query := strings.TrimSpace(req.Query)
	if query == "" {
		err = errors.New("Search query is required")
		return
	}

	limit := 50
	if req.Limit != nil && *req.Limit > 0 {
		limit = *req.Limit
		if limit > 100 {
			limit = 100
		}
	}

	milestones := SearchVisibleMilestones(ctx.Tx, query, user, limit)

	resp.Milestones = milestones
	resp.Query = query
	return
}
