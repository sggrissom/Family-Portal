package backend

import (
	"errors"
	"family/cfg"
	"fmt"
	"slices"
	"sort"
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
	Context       string  `json:"context,omitempty"`
	InputType     string  `json:"inputType"`
	MilestoneDate *string `json:"milestoneDate,omitempty"`
	AgeYears      *int    `json:"ageYears,omitempty"`
	AgeMonths     *int    `json:"ageMonths,omitempty"`
	PhotoIds      []int   `json:"photoIds,omitempty"`
	TagIds        []int   `json:"tagIds,omitempty"`
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
	Context       string  `json:"context,omitempty"`
	InputType     string  `json:"inputType"`
	MilestoneDate *string `json:"milestoneDate,omitempty"`
	AgeYears      *int    `json:"ageYears,omitempty"`
	AgeMonths     *int    `json:"ageMonths,omitempty"`
	PhotoIds      []int   `json:"photoIds,omitempty"`
	TagIds        []int   `json:"tagIds,omitempty"`
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
	Context       string    `json:"context"`
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
	version := vpack.Version(2, buf)
	vpack.Int(&self.Id, buf)
	vpack.Int(&self.PersonId, buf)
	vpack.Int(&self.FamilyId, buf)
	vpack.String(&self.Description, buf)
	vpack.String(&self.Category, buf)
	vpack.Time(&self.MilestoneDate, buf)
	vpack.Time(&self.CreatedAt, buf)
	if version >= 2 {
		vpack.String(&self.Context, buf)
	}
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
	links := milestonePhotoLinks(tx, MilestonePhotoByMilestoneIndex, milestoneId)
	photoIds := make([]int, 0, len(links))
	for _, link := range links {
		photoIds = append(photoIds, link.PhotoId)
	}
	return photoIds
}

func GetMilestoneTagIds(tx *vbolt.Tx, milestoneId int) []int {
	links := milestoneTagLinks(tx, MilestoneTagByMilestoneIndex, milestoneId)
	tagIds := make([]int, 0, len(links))
	for _, link := range links {
		tagIds = append(tagIds, link.TagId)
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

func milestoneSearchWords(text string) []string {
	words := strings.Fields(strings.ToLower(text))
	terms := make([]string, 0, len(words))
	for _, word := range words {
		word = strings.Trim(word, ".,!?;:()[]{}\"'")
		if len(word) >= 3 {
			terms = append(terms, word)
		}
	}
	return terms
}

func searchMilestonesTx(tx *vbolt.Tx, query string, limit int, canSee func(Milestone) bool) (milestones []Milestone) {
	terms := milestoneSearchWords(query)
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
	terms := milestoneSearchWords(milestone.Description + " " + milestone.Context)
	terms = append(terms,
		fmt.Sprintf("cat:%s", milestone.Category),
		fmt.Sprintf("y:%d", milestone.MilestoneDate.Year()),
		fmt.Sprintf("m:%s", milestone.MilestoneDate.Format("2006.01")),
		fmt.Sprintf("p:%d", milestone.PersonId),
	)
	vbolt.SetTargetTermsUniform(tx, MilestoneSearchIndex, milestone.Id, terms, milestone.MilestoneDate)
}

func RebuildMilestoneSearchIndex(tx *vbolt.Tx) {
	vbolt.IterateAll(tx, MilestoneBkt, func(_ int, milestone Milestone) bool {
		UpdateMilestoneSearchIndex(tx, milestone)
		return true
	})
}

func writeMilestone(tx *vbolt.Tx, milestone Milestone) {
	vbolt.Write(tx, MilestoneBkt, milestone.Id, &milestone)
	vbolt.SetTargetSingleTerm(tx, MilestoneByPersonIndex, milestone.Id, milestone.PersonId)
	vbolt.SetTargetSingleTerm(tx, MilestoneByFamilyIndex, milestone.Id, milestone.FamilyId)
	UpdateMilestoneSearchIndex(tx, milestone)
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

func milestonePhotoLinks(tx *vbolt.Tx, index *vbolt.IndexInfo[int, int, uint16], key int) (links []MilestonePhoto) {
	var ids []int
	vbolt.ReadTermTargets(tx, index, key, &ids, vbolt.Window{})
	if len(ids) > 0 {
		vbolt.ReadSlice(tx, MilestonePhotoBkt, ids, &links)
	}
	return
}

func deleteMilestonePhotoLink(tx *vbolt.Tx, link MilestonePhoto) {
	vbolt.Delete(tx, MilestonePhotoBkt, link.Id)
	vbolt.DeleteTargetTerms(tx, MilestonePhotoByMilestoneIndex, link.Id)
	vbolt.DeleteTargetTerms(tx, MilestonePhotoByPhotoIndex, link.Id)
	vbolt.DeleteTargetTerms(tx, MilestonePhotoByFamilyIndex, link.Id)
}

func addPhotoToMilestone(tx *vbolt.Tx, milestoneId int, photoId int, familyId int) error {
	if err := validatePhotoAccess(tx, photoId, familyId); err != nil {
		return err
	}
	for _, link := range milestonePhotoLinks(tx, MilestonePhotoByMilestoneIndex, milestoneId) {
		if link.PhotoId == photoId {
			return nil
		}
	}

	link := MilestonePhoto{
		Id:          vbolt.NextIntId(tx, MilestonePhotoBkt),
		MilestoneId: milestoneId,
		PhotoId:     photoId,
		FamilyId:    familyId,
		CreatedAt:   time.Now(),
	}
	vbolt.Write(tx, MilestonePhotoBkt, link.Id, &link)
	vbolt.SetTargetSingleTerm(tx, MilestonePhotoByMilestoneIndex, link.Id, milestoneId)
	vbolt.SetTargetSingleTerm(tx, MilestonePhotoByPhotoIndex, link.Id, photoId)
	vbolt.SetTargetSingleTerm(tx, MilestonePhotoByFamilyIndex, link.Id, familyId)
	return nil
}

func removePhotoFromMilestone(tx *vbolt.Tx, milestoneId int, photoId int) {
	for _, link := range milestonePhotoLinks(tx, MilestonePhotoByMilestoneIndex, milestoneId) {
		if link.PhotoId == photoId {
			deleteMilestonePhotoLink(tx, link)
		}
	}
}

func removeAllMilestonePhotos(tx *vbolt.Tx, milestoneId int) {
	for _, link := range milestonePhotoLinks(tx, MilestonePhotoByMilestoneIndex, milestoneId) {
		deleteMilestonePhotoLink(tx, link)
	}
}

func removePhotoFromMilestones(tx *vbolt.Tx, photoId int) {
	for _, link := range milestonePhotoLinks(tx, MilestonePhotoByPhotoIndex, photoId) {
		deleteMilestonePhotoLink(tx, link)
	}
}

func milestoneTagLinks(tx *vbolt.Tx, index *vbolt.IndexInfo[int, int, uint16], key int) (links []MilestoneTag) {
	var ids []int
	vbolt.ReadTermTargets(tx, index, key, &ids, vbolt.Window{})
	if len(ids) > 0 {
		vbolt.ReadSlice(tx, MilestoneTagBkt, ids, &links)
	}
	return
}

func deleteMilestoneTagLink(tx *vbolt.Tx, link MilestoneTag) {
	vbolt.Delete(tx, MilestoneTagBkt, link.Id)
	vbolt.DeleteTargetTerms(tx, MilestoneTagByMilestoneIndex, link.Id)
	vbolt.DeleteTargetTerms(tx, MilestoneTagByTagIndex, link.Id)
	vbolt.DeleteTargetTerms(tx, MilestoneTagByFamilyIndex, link.Id)
}

func addTagToMilestone(tx *vbolt.Tx, milestoneId int, tagId int, familyId int) {
	link := MilestoneTag{
		Id:          vbolt.NextIntId(tx, MilestoneTagBkt),
		MilestoneId: milestoneId,
		TagId:       tagId,
		FamilyId:    familyId,
		CreatedAt:   time.Now(),
	}
	vbolt.Write(tx, MilestoneTagBkt, link.Id, &link)
	vbolt.SetTargetSingleTerm(tx, MilestoneTagByMilestoneIndex, link.Id, milestoneId)
	vbolt.SetTargetSingleTerm(tx, MilestoneTagByTagIndex, link.Id, tagId)
	vbolt.SetTargetSingleTerm(tx, MilestoneTagByFamilyIndex, link.Id, familyId)
}

func removeTagFromMilestone(tx *vbolt.Tx, milestoneId int, tagId int) {
	for _, link := range milestoneTagLinks(tx, MilestoneTagByMilestoneIndex, milestoneId) {
		if link.TagId == tagId {
			deleteMilestoneTagLink(tx, link)
		}
	}
}

func removeAllMilestoneTags(tx *vbolt.Tx, milestoneId int) {
	for _, link := range milestoneTagLinks(tx, MilestoneTagByMilestoneIndex, milestoneId) {
		deleteMilestoneTagLink(tx, link)
	}
}

func removeMilestoneTagsByTag(tx *vbolt.Tx, tagId int) {
	for _, link := range milestoneTagLinks(tx, MilestoneTagByTagIndex, tagId) {
		deleteMilestoneTagLink(tx, link)
	}
}

func AddMilestoneTx(tx *vbolt.Tx, req AddMilestoneRequest, familyId int) (Milestone, error) {
	var milestone Milestone
	var err error

	person := GetPersonById(tx, req.PersonId)
	if person.Id == 0 || !CanFamilyAccess(tx, familyId, person.FamilyId, AccessContribute) {
		return milestone, errors.New("Person not found or not in your family")
	}

	milestone.MilestoneDate, err = resolveEntryDate("Milestone", req.InputType, req.MilestoneDate, req.AgeYears, req.AgeMonths, person.Birthday)
	if err != nil {
		return milestone, err
	}

	milestone.Id = vbolt.NextIntId(tx, MilestoneBkt)
	milestone.PersonId = req.PersonId
	milestone.FamilyId = familyId
	setMilestoneText(&milestone, req.Description, req.Category, req.Context)
	milestone.CreatedAt = time.Now()

	writeMilestone(tx, milestone)

	if err := setMilestonePhotos(tx, milestone, req.PhotoIds); err != nil {
		return milestone, err
	}
	if err := setMilestoneTags(tx, milestone, req.TagIds); err != nil {
		return milestone, err
	}
	return milestone, nil
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

	milestone.MilestoneDate, err = resolveEntryDate("Milestone", req.InputType, req.MilestoneDate, req.AgeYears, req.AgeMonths, person.Birthday)
	if err != nil {
		return milestone, err
	}

	setMilestoneText(&milestone, req.Description, req.Category, req.Context)

	writeMilestone(tx, milestone)

	if req.PhotoIds != nil {
		if err := setMilestonePhotos(tx, milestone, req.PhotoIds); err != nil {
			return milestone, err
		}
	}
	if req.TagIds != nil {
		if err := setMilestoneTags(tx, milestone, req.TagIds); err != nil {
			return milestone, err
		}
	}
	return milestone, nil
}

func setMilestoneText(milestone *Milestone, description string, category string, context string) {
	milestone.Description = strings.TrimSpace(description)
	milestone.Category = category
	milestone.Context = ""
	if category == "quote" {
		milestone.Description = unquote(milestone.Description)
		milestone.Context = strings.TrimSpace(context)
	}
}

// Quotes display in quotation marks, so typed ones around the whole quote go.
var quoteMarks = map[rune]rune{'"': '"', '“': '”', '„': '“', '«': '»', '\'': '\'', '‘': '’'}

func unquote(text string) string {
	runes := []rune(text)
	if len(runes) < 2 {
		return text
	}
	opening := runes[0]
	closing, ok := quoteMarks[opening]
	if !ok || runes[len(runes)-1] != closing {
		return text
	}
	inner := strings.TrimSpace(string(runes[1 : len(runes)-1]))
	if inner == "" || strings.ContainsRune(inner, opening) || strings.ContainsRune(inner, closing) {
		return text
	}
	return inner
}

func looksLikeQuote(text string) bool {
	return strings.ContainsRune(`"“„«`, []rune(strings.TrimSpace(text) + " ")[0])
}

func setMilestonePhotos(tx *vbolt.Tx, milestone Milestone, photoIds []int) error {
	photoIds = normalizePhotoIds(photoIds)
	for _, photoId := range photoIds {
		if err := validatePhotoAccess(tx, photoId, milestone.FamilyId); err != nil {
			return err
		}
	}
	existing := GetMilestonePhotoIds(tx, milestone.Id)
	for _, photoId := range existing {
		if !slices.Contains(photoIds, photoId) {
			removePhotoFromMilestone(tx, milestone.Id, photoId)
		}
	}
	for _, photoId := range photoIds {
		if !slices.Contains(existing, photoId) {
			if err := addPhotoToMilestone(tx, milestone.Id, photoId, milestone.FamilyId); err != nil {
				return err
			}
		}
	}
	return nil
}

func setMilestoneTags(tx *vbolt.Tx, milestone Milestone, tagIds []int) error {
	for _, tagId := range tagIds {
		tag := getTagById(tx, tagId)
		if tag.Id == 0 || !CanFamilyAccess(tx, milestone.FamilyId, tag.FamilyId, AccessContribute) {
			return errors.New("Tag not found or access denied")
		}
	}
	existing := GetMilestoneTagIds(tx, milestone.Id)
	for _, tagId := range existing {
		if !slices.Contains(tagIds, tagId) {
			removeTagFromMilestone(tx, milestone.Id, tagId)
		}
	}
	added := map[int]bool{}
	for _, tagId := range tagIds {
		if !slices.Contains(existing, tagId) && !added[tagId] {
			added[tagId] = true
			addTagToMilestone(tx, milestone.Id, tagId, milestone.FamilyId)
		}
	}
	return nil
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

	removeAllMilestonePhotos(tx, milestone.Id)
	removeAllMilestoneTags(tx, milestone.Id)
	deleteMilestoneEmbeddingTx(tx, milestone.Id)
	vbolt.DeleteTargetTerms(tx, MilestoneByPersonIndex, milestone.Id)
	vbolt.DeleteTargetTerms(tx, MilestoneByFamilyIndex, milestone.Id)
	vbolt.DeleteTargetTerms(tx, MilestoneSearchIndex, milestone.Id)
	vbolt.Delete(tx, MilestoneBkt, milestone.Id)

	return nil
}

func validateMilestoneFields(description string, category string, inputType string) error {
	if strings.TrimSpace(description) == "" {
		return errors.New("Description is required")
	}
	switch category {
	case "development", "behavior", "health", "achievement", "first", "quote", "artwork", "other":
	default:
		return errors.New("Category must be one of: development, behavior, health, achievement, first, quote, artwork, other")
	}
	return validateEntryInputType(inputType)
}

func AddMilestone(ctx *vbeam.Context, req AddMilestoneRequest) (resp AddMilestoneResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}

	if req.PersonId <= 0 {
		err = errors.New("Person ID is required")
		return
	}
	if err = validateMilestoneFields(req.Description, req.Category, req.InputType); err != nil {
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
	QueueMilestoneEmbedding(milestone.Id)
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

	if req.Id <= 0 {
		err = errors.New("Milestone ID is required")
		return
	}
	if err = validateMilestoneFields(req.Description, req.Category, req.InputType); err != nil {
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
	QueueMilestoneEmbedding(milestone.Id)
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

	if err = setMilestoneTags(ctx.Tx, milestone, req.TagIds); err != nil {
		return
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
	if candidates := visibleMilestones(ctx.Tx, user); len(candidates) > 0 {
		if scores := semanticMilestoneScores(ctx.Tx, query, candidates); scores != nil {
			milestones = blendMilestoneResults(milestones, candidates, scores, limit)
		}
	}

	resp.Milestones = milestones
	resp.Query = query
	return
}

// blendMilestoneResults puts word matches and close-in-meaning milestones in
// one list, ordered by meaning, with word matches given a head start.
func blendMilestoneResults(textHits, candidates []Milestone, scores map[int]float64, limit int) []Milestone {
	const textBonus = 0.3
	hit := map[int]bool{}
	for _, m := range textHits {
		hit[m.Id] = true
	}
	type ranked struct {
		m     Milestone
		score float64
	}
	var all []ranked
	for _, m := range textHits {
		all = append(all, ranked{m, scores[m.Id] + textBonus})
	}
	for _, m := range candidates {
		if !hit[m.Id] && scores[m.Id] >= milestoneSearchMinScore {
			all = append(all, ranked{m, scores[m.Id]})
		}
	}
	sort.SliceStable(all, func(i, j int) bool { return all[i].score > all[j].score })
	out := make([]Milestone, 0, min(limit, len(all)))
	for _, r := range all[:min(limit, len(all))] {
		out = append(out, r.m)
	}
	return out
}
