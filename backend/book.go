package backend

import (
	"errors"
	"family/cfg"
	"sort"
	"strings"
	"time"

	"go.hasen.dev/vbeam"
	"go.hasen.dev/vbolt"
	"go.hasen.dev/vpack"
)

func RegisterBookMethods(app *vbeam.Application) {
	vbeam.RegisterProc(app, ListBooks)
	vbeam.RegisterProc(app, GetBookSources)
	vbeam.RegisterProc(app, GetBook)
	vbeam.RegisterProc(app, CreateBook)
	vbeam.RegisterProc(app, UpdateBook)
	vbeam.RegisterProc(app, DeleteBook)
}

type BookItemKind int

const (
	BookItemMilestone BookItemKind = iota
	BookItemPhoto
)

const BookPresetFirstYear = "first-year"

const (
	maxBookItems     = 2000
	maxBookTitle     = 120
	maxBookWriting   = 6000
	maxBookSignature = 120
	maxBookCaption   = 300
)

var ErrBookNotFound = errors.New("Book not found or not in your family")
var ErrBookChanged = errors.New("Someone else saved this book after you opened it. Reload to see their changes.")

// BookItem points at a source record. PhotoId pairs a milestone with one of
// its attached photos; Caption overrides a photo's caption in this book only.
type BookItem struct {
	Kind     BookItemKind `json:"kind"`
	SourceId int          `json:"sourceId"`
	PhotoId  int          `json:"photoId"`
	Caption  string       `json:"caption"`
	Pinned   bool         `json:"pinned"`
}

func PackBookItem(self *BookItem, buf *vpack.Buffer) {
	vpack.Version(1, buf)
	vpack.IntEnum(&self.Kind, buf)
	vpack.Int(&self.SourceId, buf)
	vpack.Int(&self.PhotoId, buf)
	vpack.String(&self.Caption, buf)
	vpack.Bool(&self.Pinned, buf)
}

// Book covers [StartDate, EndDate); records dated on EndDate itself form the
// closing chapter.
type Book struct {
	Id           int        `json:"id"`
	FamilyId     int        `json:"familyId"`
	PersonId     int        `json:"personId"`
	Preset       string     `json:"preset"`
	Title        string     `json:"title"`
	StartDate    time.Time  `json:"startDate"`
	EndDate      time.Time  `json:"endDate"`
	CoverPhotoId int        `json:"coverPhotoId"`
	Density      string     `json:"density"`
	Introduction string     `json:"introduction"`
	Letter       string     `json:"letter"`
	Signature    string     `json:"signature"`
	ShowGrowth   bool       `json:"showGrowth"`
	Items        []BookItem `json:"items"`
	Excluded     []BookItem `json:"excluded"`
	Revision     int        `json:"revision"`
	CreatedBy    int        `json:"createdBy"`
	CreatedAt    time.Time  `json:"createdAt"`
	UpdatedAt    time.Time  `json:"updatedAt"`
}

func PackBook(self *Book, buf *vpack.Buffer) {
	vpack.Version(1, buf)
	vpack.Int(&self.Id, buf)
	vpack.Int(&self.FamilyId, buf)
	vpack.Int(&self.PersonId, buf)
	vpack.String(&self.Preset, buf)
	vpack.String(&self.Title, buf)
	vpack.Time(&self.StartDate, buf)
	vpack.Time(&self.EndDate, buf)
	vpack.Int(&self.CoverPhotoId, buf)
	vpack.String(&self.Density, buf)
	vpack.String(&self.Introduction, buf)
	vpack.String(&self.Letter, buf)
	vpack.String(&self.Signature, buf)
	vpack.Bool(&self.ShowGrowth, buf)
	vpack.Slice(&self.Items, PackBookItem, buf)
	vpack.Slice(&self.Excluded, PackBookItem, buf)
	vpack.Int(&self.Revision, buf)
	vpack.Int(&self.CreatedBy, buf)
	vpack.Time(&self.CreatedAt, buf)
	vpack.Time(&self.UpdatedAt, buf)
}

var BookBkt = vbolt.Bucket(&cfg.Info, "books", vpack.FInt, PackBook)
var BookByFamilyIndex = vbolt.Index(&cfg.Info, "books_by_family", vpack.FInt, vpack.FInt)
var BookByPersonIndex = vbolt.Index(&cfg.Info, "books_by_person", vpack.FInt, vpack.FInt)

func GetBookById(tx *vbolt.Tx, id int) (book Book) {
	vbolt.Read(tx, BookBkt, id, &book)
	return
}

func getBooksBy(tx *vbolt.Tx, index *vbolt.IndexInfo[int, int, uint16], term int) []Book {
	var ids []int
	vbolt.ReadTermTargets(tx, index, term, &ids, vbolt.Window{})
	books := []Book{}
	if len(ids) > 0 {
		vbolt.ReadSlice(tx, BookBkt, ids, &books)
	}
	return books
}

func GetFamilyBooks(tx *vbolt.Tx, familyId int) []Book {
	return getBooksBy(tx, BookByFamilyIndex, familyId)
}

func GetPersonBooks(tx *vbolt.Tx, personId int) []Book {
	return getBooksBy(tx, BookByPersonIndex, personId)
}

func writeBookTx(tx *vbolt.Tx, book Book) {
	vbolt.Write(tx, BookBkt, book.Id, &book)
	vbolt.SetTargetSingleTerm(tx, BookByFamilyIndex, book.Id, book.FamilyId)
	vbolt.SetTargetSingleTerm(tx, BookByPersonIndex, book.Id, book.PersonId)
}

func deleteBookTx(tx *vbolt.Tx, bookId int) {
	vbolt.Delete(tx, BookBkt, bookId)
	vbolt.SetTargetSingleTerm(tx, BookByFamilyIndex, bookId, -1)
	vbolt.SetTargetSingleTerm(tx, BookByPersonIndex, bookId, -1)
}

func deletePersonBooksTx(tx *vbolt.Tx, personId int) {
	for _, book := range GetPersonBooks(tx, personId) {
		deleteBookTx(tx, book.Id)
	}
}

func movePersonBooksTx(tx *vbolt.Tx, fromPersonId, toPersonId int) {
	for _, book := range GetPersonBooks(tx, fromPersonId) {
		book.PersonId = toPersonId
		writeBookTx(tx, book)
	}
}

func firstYearRange(birthday time.Time) (start, end time.Time) {
	start = time.Date(birthday.Year(), birthday.Month(), birthday.Day(), 0, 0, 0, 0, time.UTC)
	end = time.Date(birthday.Year()+1, birthday.Month(), birthday.Day(), 0, 0, 0, 0, time.UTC)
	return
}

// BookSources are the records a book may draw on, already filtered to what the
// viewer is allowed to see.
type BookSources struct {
	Person     Person       `json:"person"`
	Milestones []Milestone  `json:"milestones"`
	Photos     []Image      `json:"photos"`
	GrowthData []GrowthData `json:"growthData"`
}

func inBookRange(t time.Time, start, end time.Time) bool {
	return !t.Before(start) && t.Before(end.AddDate(0, 0, 1))
}

func bookSourcesTx(tx *vbolt.Tx, user User, person Person, start, end time.Time, keepPhotoIds map[int]bool) BookSources {
	sources := BookSources{
		Person:     person,
		Milestones: []Milestone{},
		Photos:     []Image{},
		GrowthData: []GrowthData{},
	}
	sources.Person.Age = calculatePersonAge(person.Birthday, person.IsPregnancy)

	photoIds := map[int]bool{}
	addPhoto := func(image Image) {
		if image.Id == 0 || photoIds[image.Id] || !CanAccessPhoto(tx, user, image, AccessView) {
			return
		}
		photoIds[image.Id] = true
		image.TagIds = []int{}
		sources.Photos = append(sources.Photos, image)
	}

	for _, milestone := range GetPersonMilestonesTx(tx, person.Id) {
		if milestone.FamilyId != person.FamilyId || !inBookRange(milestone.MilestoneDate, start, end) {
			continue
		}
		milestone.PhotoIds = GetMilestonePhotoIds(tx, milestone.Id)
		milestone.TagIds = []int{}
		sources.Milestones = append(sources.Milestones, milestone)
		for _, photoId := range milestone.PhotoIds {
			addPhoto(GetImageById(tx, photoId))
		}
	}
	for _, image := range GetPersonImages(tx, person.Id) {
		if inBookRange(image.PhotoDate, start, end) || keepPhotoIds[image.Id] {
			addPhoto(image)
		}
	}
	for _, growth := range GetPersonGrowthDataTx(tx, person.Id) {
		if inBookRange(growth.MeasurementDate, start, end) {
			sources.GrowthData = append(sources.GrowthData, growth)
		}
	}

	sort.Slice(sources.Milestones, func(i, j int) bool {
		return sources.Milestones[i].MilestoneDate.Before(sources.Milestones[j].MilestoneDate)
	})
	sort.Slice(sources.Photos, func(i, j int) bool {
		a, b := sources.Photos[i], sources.Photos[j]
		if a.PhotoDate.Equal(b.PhotoDate) {
			return a.Id < b.Id
		}
		return a.PhotoDate.Before(b.PhotoDate)
	})
	return sources
}

func bookPhotoIds(book Book) map[int]bool {
	ids := map[int]bool{}
	if book.CoverPhotoId != 0 {
		ids[book.CoverPhotoId] = true
	}
	for _, item := range book.Items {
		if item.Kind == BookItemPhoto {
			ids[item.SourceId] = true
		}
	}
	return ids
}

func bookPerson(tx *vbolt.Tx, user User, personId int, need AccessLevel) (Person, error) {
	person := GetPersonById(tx, personId)
	if person.Id == 0 || !CanAccessFamily(tx, user, person.FamilyId, need) {
		return Person{}, ErrPersonNotFound
	}
	return person, nil
}

func bookFor(tx *vbolt.Tx, user User, bookId int, need AccessLevel) (Book, Person, error) {
	book := GetBookById(tx, bookId)
	if book.Id == 0 || !CanAccessFamily(tx, user, book.FamilyId, need) {
		return Book{}, Person{}, ErrBookNotFound
	}
	person := GetPersonById(tx, book.PersonId)
	if person.Id == 0 || person.FamilyId != book.FamilyId {
		return Book{}, Person{}, ErrBookNotFound
	}
	return book, person, nil
}

type BookSummary struct {
	Id           int       `json:"id"`
	PersonId     int       `json:"personId"`
	PersonName   string    `json:"personName"`
	Title        string    `json:"title"`
	StartDate    time.Time `json:"startDate"`
	EndDate      time.Time `json:"endDate"`
	CoverPhotoId int       `json:"coverPhotoId"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

type ListBooksRequest struct {
	FamilyId int `json:"familyId,omitempty"`
}

type ListBooksResponse struct {
	Books   []BookSummary `json:"books"`
	CanEdit bool          `json:"canEdit"`
}

func ListBooks(ctx *vbeam.Context, req ListBooksRequest) (resp ListBooksResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}
	familyId, err := ResolveActingFamily(ctx.Tx, user, req.FamilyId, AccessView)
	if err != nil {
		return
	}
	resp.CanEdit = CanAccessFamily(ctx.Tx, user, familyId, AccessContribute)
	resp.Books = []BookSummary{}
	for _, book := range GetFamilyBooks(ctx.Tx, familyId) {
		person := GetPersonById(ctx.Tx, book.PersonId)
		if person.Id == 0 {
			continue
		}
		cover := book.CoverPhotoId
		if cover != 0 && !CanAccessPhoto(ctx.Tx, user, GetImageById(ctx.Tx, cover), AccessView) {
			cover = 0
		}
		resp.Books = append(resp.Books, BookSummary{
			Id:           book.Id,
			PersonId:     book.PersonId,
			PersonName:   person.Name,
			Title:        book.Title,
			StartDate:    book.StartDate,
			EndDate:      book.EndDate,
			CoverPhotoId: cover,
			UpdatedAt:    book.UpdatedAt,
		})
	}
	sort.Slice(resp.Books, func(i, j int) bool {
		return resp.Books[i].UpdatedAt.After(resp.Books[j].UpdatedAt)
	})
	return
}

type GetBookSourcesRequest struct {
	PersonId int `json:"personId"`
}

type GetBookSourcesResponse struct {
	Sources   BookSources `json:"sources"`
	StartDate time.Time   `json:"startDate"`
	EndDate   time.Time   `json:"endDate"`
}

func GetBookSources(ctx *vbeam.Context, req GetBookSourcesRequest) (resp GetBookSourcesResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}
	person, err := bookPerson(ctx.Tx, user, req.PersonId, AccessContribute)
	if err != nil {
		return
	}
	if person.IsPregnancy || person.Birthday.Year() < 1000 {
		err = errors.New("A first-year book needs a birthday")
		return
	}
	resp.StartDate, resp.EndDate = firstYearRange(person.Birthday)
	resp.Sources = bookSourcesTx(ctx.Tx, user, person, resp.StartDate, resp.EndDate, nil)
	return
}

type GetBookRequest struct {
	Id int `json:"id"`
}

type GetBookResponse struct {
	Book    Book        `json:"book"`
	Sources BookSources `json:"sources"`
	CanEdit bool        `json:"canEdit"`
}

func GetBook(ctx *vbeam.Context, req GetBookRequest) (resp GetBookResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}
	book, person, err := bookFor(ctx.Tx, user, req.Id, AccessView)
	if err != nil {
		return
	}
	resp.Book = book
	resp.Sources = bookSourcesTx(ctx.Tx, user, person, book.StartDate, book.EndDate, bookPhotoIds(book))
	resp.CanEdit = CanAccessFamily(ctx.Tx, user, book.FamilyId, AccessContribute)
	return
}

type BookContent struct {
	Title        string     `json:"title"`
	CoverPhotoId int        `json:"coverPhotoId"`
	Density      string     `json:"density"`
	Introduction string     `json:"introduction"`
	Letter       string     `json:"letter"`
	Signature    string     `json:"signature"`
	ShowGrowth   bool       `json:"showGrowth"`
	Items        []BookItem `json:"items"`
	Excluded     []BookItem `json:"excluded"`
}

type CreateBookRequest struct {
	PersonId int         `json:"personId"`
	Content  BookContent `json:"content"`
}

type UpdateBookRequest struct {
	Id       int         `json:"id"`
	Revision int         `json:"revision"`
	Content  BookContent `json:"content"`
}

type BookResponse struct {
	Book Book `json:"book"`
}

type DeleteBookRequest struct {
	Id int `json:"id"`
}

type DeleteBookResponse struct{}

func cleanDensity(density string) string {
	switch density {
	case "brief", "balanced", "detailed":
		return density
	}
	return "balanced"
}

func limitText(value string, max int, label string) (string, error) {
	value = strings.TrimSpace(value)
	if len([]rune(value)) > max {
		return "", errors.New(label + " is too long")
	}
	return value, nil
}

// applyBookContent checks every reference against the book's person and the
// editor's access before anything is stored.
func applyBookContent(tx *vbolt.Tx, user User, book *Book, person Person, content BookContent) error {
	var err error
	if book.Title, err = limitText(content.Title, maxBookTitle, "The title"); err != nil {
		return err
	}
	if book.Title == "" {
		book.Title = person.Name + "'s first year"
	}
	if book.Introduction, err = limitText(content.Introduction, maxBookWriting, "The introduction"); err != nil {
		return err
	}
	if book.Letter, err = limitText(content.Letter, maxBookWriting, "The letter"); err != nil {
		return err
	}
	if book.Signature, err = limitText(content.Signature, maxBookSignature, "The signature"); err != nil {
		return err
	}
	if len(content.Items) > maxBookItems || len(content.Excluded) > maxBookItems {
		return errors.New("That is more items than one book can hold")
	}
	book.Density = cleanDensity(content.Density)
	book.ShowGrowth = content.ShowGrowth

	personPhotos := map[int]bool{}
	for _, row := range GetPhotoPersonsByPerson(tx, person.Id) {
		personPhotos[row.PhotoId] = true
	}
	milestonePhotos := map[int]map[int]bool{}
	checkMilestone := func(id int) bool {
		if _, ok := milestonePhotos[id]; ok {
			return true
		}
		milestone := GetMilestoneById(tx, id)
		if milestone.Id == 0 || milestone.PersonId != person.Id || milestone.FamilyId != book.FamilyId {
			return false
		}
		attached := map[int]bool{}
		for _, photoId := range GetMilestonePhotoIds(tx, id) {
			attached[photoId] = true
		}
		milestonePhotos[id] = attached
		return true
	}
	visible := func(photoId int) bool {
		return CanAccessPhoto(tx, user, GetImageById(tx, photoId), AccessView)
	}

	seen := map[BookItemKind]map[int]bool{BookItemMilestone: {}, BookItemPhoto: {}}
	book.Items = []BookItem{}
	usedPhotos := map[int]bool{}
	for _, item := range content.Items {
		if seen[item.Kind] == nil || seen[item.Kind][item.SourceId] {
			continue
		}
		caption, err := limitText(item.Caption, maxBookCaption, "A caption")
		if err != nil {
			return err
		}
		clean := BookItem{Kind: item.Kind, SourceId: item.SourceId, Caption: caption, Pinned: item.Pinned}
		switch item.Kind {
		case BookItemMilestone:
			if !checkMilestone(item.SourceId) {
				return errors.New("A milestone in this book does not belong to " + person.Name)
			}
			if item.PhotoId != 0 {
				if !milestonePhotos[item.SourceId][item.PhotoId] || !visible(item.PhotoId) {
					return errors.New("A milestone's photo is not attached to it")
				}
				clean.PhotoId = item.PhotoId
				usedPhotos[item.PhotoId] = true
			}
		case BookItemPhoto:
			if !personPhotos[item.SourceId] || !visible(item.SourceId) {
				return errors.New("A photo in this book is not tagged with " + person.Name)
			}
			usedPhotos[item.SourceId] = true
		}
		seen[item.Kind][item.SourceId] = true
		book.Items = append(book.Items, clean)
	}

	book.Excluded = []BookItem{}
	for _, item := range content.Excluded {
		if (item.Kind != BookItemMilestone && item.Kind != BookItemPhoto) || seen[item.Kind][item.SourceId] {
			continue
		}
		seen[item.Kind][item.SourceId] = true
		book.Excluded = append(book.Excluded, BookItem{Kind: item.Kind, SourceId: item.SourceId})
	}

	book.CoverPhotoId = 0
	if content.CoverPhotoId != 0 {
		if !(personPhotos[content.CoverPhotoId] || usedPhotos[content.CoverPhotoId]) || !visible(content.CoverPhotoId) {
			return errors.New("The cover photo is not tagged with " + person.Name)
		}
		book.CoverPhotoId = content.CoverPhotoId
	}
	return nil
}

func CreateBook(ctx *vbeam.Context, req CreateBookRequest) (resp BookResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}
	person, err := bookPerson(ctx.Tx, user, req.PersonId, AccessContribute)
	if err != nil {
		return
	}
	if person.IsPregnancy || person.Birthday.Year() < 1000 {
		err = errors.New("A first-year book needs a birthday")
		return
	}

	vbeam.UseWriteTx(ctx)
	now := time.Now()
	book := Book{
		FamilyId:  person.FamilyId,
		PersonId:  person.Id,
		Preset:    BookPresetFirstYear,
		CreatedBy: user.Id,
		CreatedAt: now,
		UpdatedAt: now,
		Revision:  1,
	}
	book.StartDate, book.EndDate = firstYearRange(person.Birthday)
	if err = applyBookContent(ctx.Tx, user, &book, person, req.Content); err != nil {
		return
	}
	book.Id = vbolt.NextIntId(ctx.Tx, BookBkt)
	writeBookTx(ctx.Tx, book)
	resp.Book = book
	vbolt.TxCommit(ctx.Tx)
	return
}

func UpdateBook(ctx *vbeam.Context, req UpdateBookRequest) (resp BookResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}
	vbeam.UseWriteTx(ctx)
	book, person, err := bookFor(ctx.Tx, user, req.Id, AccessContribute)
	if err != nil {
		return
	}
	if req.Revision != book.Revision {
		err = ErrBookChanged
		return
	}
	if err = applyBookContent(ctx.Tx, user, &book, person, req.Content); err != nil {
		return
	}
	book.Revision++
	book.UpdatedAt = time.Now()
	writeBookTx(ctx.Tx, book)
	resp.Book = book
	vbolt.TxCommit(ctx.Tx)
	return
}

func DeleteBook(ctx *vbeam.Context, req DeleteBookRequest) (resp DeleteBookResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}
	vbeam.UseWriteTx(ctx)
	book, _, err := bookFor(ctx.Tx, user, req.Id, AccessContribute)
	if err != nil {
		return
	}
	deleteBookTx(ctx.Tx, book.Id)
	vbolt.TxCommit(ctx.Tx)
	return
}
