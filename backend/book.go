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

const (
	BookPresetFirstYear  = "first-year"
	BookPresetYear       = "year"
	BookPresetFamilyYear = "family-year"
	BookPresetCustom     = "custom"
)

var bookCategories = map[string]bool{"milestones": true, "quotes": true, "artwork": true, "photos": true}

const (
	maxBookItems     = 2000
	maxBookPeople    = 12
	maxBookYears     = 30
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

// Book covers [StartDate, EndDate). A first-year book also keeps records dated
// on EndDate itself, the first birthday, for its closing chapter. PersonId is
// the first of PersonIds.
type Book struct {
	Id           int        `json:"id"`
	FamilyId     int        `json:"familyId"`
	PersonId     int        `json:"personId"`
	PersonIds    []int      `json:"personIds"`
	Preset       string     `json:"preset"`
	Title        string     `json:"title"`
	StartDate    time.Time  `json:"startDate"`
	EndDate      time.Time  `json:"endDate"`
	CoverPhotoId int        `json:"coverPhotoId"`
	Density      string     `json:"density"`
	Categories   []string   `json:"categories"`
	Match        string     `json:"match"`
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
	ReviewedAt   time.Time  `json:"reviewedAt"`
}

func PackBook(self *Book, buf *vpack.Buffer) {
	version := vpack.Version(2, buf)
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
	if version >= 2 {
		vpack.Slice(&self.PersonIds, vpack.Int, buf)
		vpack.Slice(&self.Categories, vpack.String, buf)
		vpack.String(&self.Match, buf)
		vpack.Time(&self.ReviewedAt, buf)
	} else {
		self.PersonIds = []int{self.PersonId}
		self.Categories = []string{}
		self.Match = "any"
		self.ReviewedAt = self.UpdatedAt
	}
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
	if len(book.PersonIds) > 0 {
		book.PersonId = book.PersonIds[0]
	}
	vbolt.Write(tx, BookBkt, book.Id, &book)
	vbolt.SetTargetSingleTerm(tx, BookByFamilyIndex, book.Id, book.FamilyId)
	vbolt.SetTargetTermsPlain(tx, BookByPersonIndex, book.Id, book.PersonIds)
}

func deleteBookTx(tx *vbolt.Tx, bookId int) {
	vbolt.Delete(tx, BookBkt, bookId)
	vbolt.SetTargetSingleTerm(tx, BookByFamilyIndex, bookId, -1)
	vbolt.DeleteTargetTerms(tx, BookByPersonIndex, bookId)
}

func replacePersonId(ids []int, from, to int) []int {
	out := []int{}
	seen := map[int]bool{}
	for _, id := range ids {
		if id == from {
			id = to
		}
		if id == 0 || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}

<<<<<<< HEAD
=======
// soleSubjectBooks are the books deleting this person would delete with them.
func soleSubjectBooks(tx *vbolt.Tx, personId int) []Book {
	books := []Book{}
	for _, book := range GetPersonBooks(tx, personId) {
		if len(replacePersonId(book.PersonIds, personId, 0)) == 0 {
			books = append(books, book)
		}
	}
	return books
}

>>>>>>> main
// A book about several people loses one of them; a book with nobody left goes.
func deletePersonBooksTx(tx *vbolt.Tx, personId int) {
	for _, book := range GetPersonBooks(tx, personId) {
		book.PersonIds = replacePersonId(book.PersonIds, personId, 0)
		if len(book.PersonIds) == 0 {
			deleteBookTx(tx, book.Id)
		} else {
			writeBookTx(tx, book)
		}
	}
}

func movePersonBooksTx(tx *vbolt.Tx, fromPersonId, toPersonId int) {
	for _, book := range GetPersonBooks(tx, fromPersonId) {
		book.PersonIds = replacePersonId(book.PersonIds, fromPersonId, toPersonId)
		writeBookTx(tx, book)
	}
}

func dayStart(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

func firstYearRange(birthday time.Time) (start, end time.Time) {
	start = dayStart(birthday)
	end = time.Date(birthday.Year()+1, birthday.Month(), birthday.Day(), 0, 0, 0, 0, time.UTC)
	return
}

// readLimit is the exclusive end of what a book draws on: the first birthday
// itself is included in a first-year book.
func readLimit(preset string, end time.Time) time.Time {
	if preset == BookPresetFirstYear {
		return end.AddDate(0, 0, 1)
	}
	return end
}

// BookSources are the records a book may draw on, already filtered to what the
// viewer is allowed to see. PhotoPeople lists which of the book's people each
// photo is tagged with; Untagged photos have nobody tagged at all.
type BookSources struct {
	People      []Person      `json:"people"`
	Milestones  []Milestone   `json:"milestones"`
	Photos      []Image       `json:"photos"`
	GrowthData  []GrowthData  `json:"growthData"`
	PhotoPeople map[int][]int `json:"photoPeople"`
	Untagged    []int         `json:"untagged"`
}

func bookSourcesTx(tx *vbolt.Tx, user User, familyId int, people []Person, start, limit time.Time, keepPhotoIds map[int]bool) BookSources {
	sources := BookSources{
		People:      []Person{},
		Milestones:  []Milestone{},
		Photos:      []Image{},
		GrowthData:  []GrowthData{},
		PhotoPeople: map[int][]int{},
		Untagged:    []int{},
	}
	inRange := func(t time.Time) bool { return !t.Before(start) && t.Before(limit) }
	selected := map[int]bool{}
	for _, person := range people {
		person.Age = calculatePersonAge(person.Birthday, person.IsPregnancy)
		sources.People = append(sources.People, person)
		selected[person.Id] = true
	}

	photoIds := map[int]bool{}
	addPhoto := func(image Image) {
		if image.Id == 0 || photoIds[image.Id] || !CanAccessPhoto(tx, user, image, AccessView) {
			return
		}
		photoIds[image.Id] = true
		image.TagIds = []int{}
		tagged := []int{}
		for _, row := range GetPhotoPersonsByPhoto(tx, image.Id) {
			if selected[row.PersonId] {
				tagged = append(tagged, row.PersonId)
			}
		}
		sort.Ints(tagged)
		sources.PhotoPeople[image.Id] = tagged
		sources.Photos = append(sources.Photos, image)
	}

	for _, person := range people {
		for _, milestone := range GetPersonMilestonesTx(tx, person.Id) {
			if milestone.FamilyId != person.FamilyId || !inRange(milestone.MilestoneDate) {
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
			if inRange(image.PhotoDate) || keepPhotoIds[image.Id] {
				addPhoto(image)
			}
		}
		for _, growth := range GetPersonGrowthDataTx(tx, person.Id) {
			if inRange(growth.MeasurementDate) {
				sources.GrowthData = append(sources.GrowthData, growth)
			}
		}
	}

	if CanAccessFamily(tx, user, familyId, AccessView) {
		vbolt.IterateTerm(tx, ImageByFamilyDateIndex, familyId, func(photoId int, seconds int64) bool {
			taken := time.Unix(seconds, 0)
			if (inRange(taken) || keepPhotoIds[photoId]) && !photoIds[photoId] && len(GetPhotoPersonsByPhoto(tx, photoId)) == 0 {
				addPhoto(GetImageById(tx, photoId))
				if photoIds[photoId] {
					sources.Untagged = append(sources.Untagged, photoId)
				}
			}
			return true
		})
	}

	sort.Slice(sources.Milestones, func(i, j int) bool {
		a, b := sources.Milestones[i], sources.Milestones[j]
		if a.MilestoneDate.Equal(b.MilestoneDate) {
			return a.Id < b.Id
		}
		return a.MilestoneDate.Before(b.MilestoneDate)
	})
	sort.Slice(sources.Photos, func(i, j int) bool {
		a, b := sources.Photos[i], sources.Photos[j]
		if a.PhotoDate.Equal(b.PhotoDate) {
			return a.Id < b.Id
		}
		return a.PhotoDate.Before(b.PhotoDate)
	})
	sort.Ints(sources.Untagged)
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

// bookPeople resolves the people a new book is about: all from one household
// the user can contribute to, in the order given, without repeats.
func bookPeople(tx *vbolt.Tx, user User, personIds []int) ([]Person, error) {
	if len(personIds) == 0 {
		return nil, errors.New("Choose who the book is about")
	}
	if len(personIds) > maxBookPeople {
		return nil, errors.New("That is more people than one book can follow")
	}
	people := []Person{}
	seen := map[int]bool{}
	for _, id := range personIds {
		if seen[id] {
			continue
		}
		seen[id] = true
		person := GetPersonById(tx, id)
		if person.Id == 0 || !CanAccessFamily(tx, user, person.FamilyId, AccessContribute) {
			return nil, ErrPersonNotFound
		}
		if len(people) > 0 && person.FamilyId != people[0].FamilyId {
			return nil, errors.New("Everyone in a book must be from the same family")
		}
		people = append(people, person)
	}
	return people, nil
}

func hasBirthday(person Person) bool {
	return !person.IsPregnancy && person.Birthday.Year() >= 1000
}

func parseBookDay(value string) (time.Time, error) {
	t, err := time.Parse("2006-01-02", strings.TrimSpace(value))
	if err != nil {
		return time.Time{}, errors.New("Choose a start and end date")
	}
	return t, nil
}

// bookRange settles the concrete dates a book covers, so that a saved book
// never drifts as time passes.
func bookRange(preset string, people []Person, startDay, endDay string) (start, end time.Time, err error) {
	switch preset {
	case BookPresetFirstYear:
		if len(people) != 1 || !hasBirthday(people[0]) {
			err = errors.New("A first-year book needs one person with a birthday")
			return
		}
		start, end = firstYearRange(people[0].Birthday)
		return
	case BookPresetYear, BookPresetFamilyYear, BookPresetCustom:
	default:
		err = errors.New("Unknown kind of book")
		return
	}
	if preset == BookPresetYear && len(people) != 1 {
		err = errors.New("A year of one person needs exactly one person")
		return
	}
	if start, err = parseBookDay(startDay); err != nil {
		return
	}
	if end, err = parseBookDay(endDay); err != nil {
		return
	}
	if !end.After(start) {
		err = errors.New("The end date must be after the start date")
		return
	}
	if end.After(start.AddDate(maxBookYears, 0, 0)) {
		err = errors.New("That is a longer stretch than one book can cover")
	}
	return
}

func bookFor(tx *vbolt.Tx, user User, bookId int, need AccessLevel) (Book, []Person, error) {
	book := GetBookById(tx, bookId)
	if book.Id == 0 || !CanAccessFamily(tx, user, book.FamilyId, need) {
		return Book{}, nil, ErrBookNotFound
	}
	people := []Person{}
	for _, id := range book.PersonIds {
		person := GetPersonById(tx, id)
		if person.Id != 0 && person.FamilyId == book.FamilyId {
			people = append(people, person)
		}
	}
	if len(people) == 0 {
		return Book{}, nil, ErrBookNotFound
	}
	return book, people, nil
}

type BookSummary struct {
	Id           int       `json:"id"`
	PersonIds    []int     `json:"personIds"`
	PersonNames  []string  `json:"personNames"`
	Preset       string    `json:"preset"`
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
		summary := BookSummary{
			Id:          book.Id,
			PersonIds:   []int{},
			PersonNames: []string{},
			Preset:      book.Preset,
			Title:       book.Title,
			StartDate:   book.StartDate,
			EndDate:     book.EndDate,
			UpdatedAt:   book.UpdatedAt,
		}
		for _, id := range book.PersonIds {
			if person := GetPersonById(ctx.Tx, id); person.Id != 0 {
				summary.PersonIds = append(summary.PersonIds, id)
				summary.PersonNames = append(summary.PersonNames, person.Name)
			}
		}
		if len(summary.PersonIds) == 0 {
			continue
		}
		if book.CoverPhotoId != 0 && CanAccessPhoto(ctx.Tx, user, GetImageById(ctx.Tx, book.CoverPhotoId), AccessView) {
			summary.CoverPhotoId = book.CoverPhotoId
		}
		resp.Books = append(resp.Books, summary)
	}
	sort.Slice(resp.Books, func(i, j int) bool {
		return resp.Books[i].UpdatedAt.After(resp.Books[j].UpdatedAt)
	})
	return
}

type GetBookSourcesRequest struct {
	PersonIds []int  `json:"personIds"`
	Preset    string `json:"preset"`
	StartDate string `json:"startDate"`
	EndDate   string `json:"endDate"`
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
	people, err := bookPeople(ctx.Tx, user, req.PersonIds)
	if err != nil {
		return
	}
	resp.StartDate, resp.EndDate, err = bookRange(req.Preset, people, req.StartDate, req.EndDate)
	if err != nil {
		return
	}
	resp.Sources = bookSourcesTx(ctx.Tx, user, people[0].FamilyId, people, resp.StartDate, readLimit(req.Preset, resp.EndDate), nil)
	return
}

type GetBookRequest struct {
	Id int `json:"id"`
}

type GetBookResponse struct {
	Book    Book        `json:"book"`
	Sources BookSources `json:"sources"`
	CanEdit bool        `json:"canEdit"`
	Now     time.Time   `json:"now"`
}

func GetBook(ctx *vbeam.Context, req GetBookRequest) (resp GetBookResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}
	book, people, err := bookFor(ctx.Tx, user, req.Id, AccessView)
	if err != nil {
		return
	}
	resp.Book = book
	resp.Sources = bookSourcesTx(ctx.Tx, user, book.FamilyId, people, book.StartDate, readLimit(book.Preset, book.EndDate), bookPhotoIds(book))
	resp.CanEdit = CanAccessFamily(ctx.Tx, user, book.FamilyId, AccessContribute)
	resp.Now = time.Now()
	return
}

type BookContent struct {
	Title        string     `json:"title"`
	CoverPhotoId int        `json:"coverPhotoId"`
	Density      string     `json:"density"`
	Categories   []string   `json:"categories"`
	Match        string     `json:"match"`
	Introduction string     `json:"introduction"`
	Letter       string     `json:"letter"`
	Signature    string     `json:"signature"`
	ShowGrowth   bool       `json:"showGrowth"`
	Items        []BookItem `json:"items"`
	Excluded     []BookItem `json:"excluded"`
	ReviewedAt   time.Time  `json:"reviewedAt"`
}

type CreateBookRequest struct {
	PersonIds []int       `json:"personIds"`
	Preset    string      `json:"preset"`
	StartDate string      `json:"startDate"`
	EndDate   string      `json:"endDate"`
	Content   BookContent `json:"content"`
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

func cleanMatch(match string) string {
	if match == "all" {
		return "all"
	}
	return "any"
}

func cleanCategories(categories []string) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, c := range categories {
		if bookCategories[c] && !seen[c] {
			seen[c] = true
			out = append(out, c)
		}
	}
	sort.Strings(out)
	return out
}

func limitText(value string, max int, label string) (string, error) {
	value = strings.TrimSpace(value)
	if len([]rune(value)) > max {
		return "", errors.New(label + " is too long")
	}
	return value, nil
}

func peopleNames(people []Person) string {
	names := make([]string, len(people))
	for i, p := range people {
		names[i] = p.Name
	}
	if len(names) <= 2 {
		return strings.Join(names, " or ")
	}
	return strings.Join(names[:len(names)-1], ", ") + " or " + names[len(names)-1]
}

// applyBookContent checks every reference against the book's people and the
// editor's access before anything is stored.
func applyBookContent(tx *vbolt.Tx, user User, book *Book, people []Person, content BookContent) error {
	var err error
	if book.Title, err = limitText(content.Title, maxBookTitle, "The title"); err != nil {
		return err
	}
	if book.Title == "" {
		book.Title = people[0].Name + "'s book"
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
	book.Categories = cleanCategories(content.Categories)
	book.Match = cleanMatch(content.Match)
	book.ShowGrowth = content.ShowGrowth

	selected := map[int]bool{}
	for _, p := range people {
		selected[p.Id] = true
	}
	photoOk := map[int]bool{}
<<<<<<< HEAD
	checkedPhotos := map[int]bool{}
	allowedPhoto := func(photoId int) bool {
		if checkedPhotos[photoId] {
			return photoOk[photoId]
		}
		checkedPhotos[photoId] = true
		image := GetImageById(tx, photoId)
		if !CanAccessPhoto(tx, user, image, AccessView) {
			return false
		}
		rows := GetPhotoPersonsByPhoto(tx, photoId)
		ok := len(rows) == 0 && image.FamilyId == book.FamilyId
		for _, row := range rows {
			if selected[row.PersonId] {
				ok = true
			}
		}
=======
	allowedPhoto := func(photoId int) bool {
		if ok, checked := photoOk[photoId]; checked {
			return ok
		}
		image := GetImageById(tx, photoId)
		rows := GetPhotoPersonsByPhoto(tx, photoId)
		ok := len(rows) == 0 && image.FamilyId == book.FamilyId
		for _, row := range rows {
			ok = ok || selected[row.PersonId]
		}
		ok = ok && CanAccessPhoto(tx, user, image, AccessView)
>>>>>>> main
		photoOk[photoId] = ok
		return ok
	}
	milestonePhotos := map[int]map[int]bool{}
	checkMilestone := func(id int) bool {
		if _, ok := milestonePhotos[id]; ok {
			return true
		}
		milestone := GetMilestoneById(tx, id)
		if milestone.Id == 0 || !selected[milestone.PersonId] || milestone.FamilyId != book.FamilyId {
			return false
		}
		attached := map[int]bool{}
		for _, photoId := range GetMilestonePhotoIds(tx, id) {
			attached[photoId] = true
		}
		milestonePhotos[id] = attached
		return true
	}
<<<<<<< HEAD
	visible := func(photoId int) bool {
		return CanAccessPhoto(tx, user, GetImageById(tx, photoId), AccessView)
	}
=======
>>>>>>> main

	who := peopleNames(people)
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
				return errors.New("A milestone in this book does not belong to " + who)
			}
			if item.PhotoId != 0 {
<<<<<<< HEAD
				if !milestonePhotos[item.SourceId][item.PhotoId] || !visible(item.PhotoId) {
=======
				if !milestonePhotos[item.SourceId][item.PhotoId] || !CanAccessPhoto(tx, user, GetImageById(tx, item.PhotoId), AccessView) {
>>>>>>> main
					return errors.New("A milestone's photo is not attached to it")
				}
				clean.PhotoId = item.PhotoId
				usedPhotos[item.PhotoId] = true
			}
		case BookItemPhoto:
			if !allowedPhoto(item.SourceId) {
				return errors.New("A photo in this book is not of " + who)
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
<<<<<<< HEAD
		if !(usedPhotos[content.CoverPhotoId] && visible(content.CoverPhotoId)) && !allowedPhoto(content.CoverPhotoId) {
=======
		if !usedPhotos[content.CoverPhotoId] && !allowedPhoto(content.CoverPhotoId) {
>>>>>>> main
			return errors.New("The cover photo is not of " + who)
		}
		book.CoverPhotoId = content.CoverPhotoId
	}

	now := time.Now()
	if content.ReviewedAt.After(book.ReviewedAt) {
		book.ReviewedAt = content.ReviewedAt
	}
	if book.ReviewedAt.After(now) {
		book.ReviewedAt = now
	}
	return nil
}

func CreateBook(ctx *vbeam.Context, req CreateBookRequest) (resp BookResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}
	people, err := bookPeople(ctx.Tx, user, req.PersonIds)
	if err != nil {
		return
	}
	start, end, err := bookRange(req.Preset, people, req.StartDate, req.EndDate)
	if err != nil {
		return
	}

	vbeam.UseWriteTx(ctx)
	now := time.Now()
	book := Book{
		FamilyId:   people[0].FamilyId,
		Preset:     req.Preset,
		StartDate:  start,
		EndDate:    end,
		CreatedBy:  user.Id,
		CreatedAt:  now,
		UpdatedAt:  now,
		ReviewedAt: now,
		Revision:   1,
	}
	for _, p := range people {
		book.PersonIds = append(book.PersonIds, p.Id)
	}
	content := req.Content
	content.ReviewedAt = now
	if err = applyBookContent(ctx.Tx, user, &book, people, content); err != nil {
		return
	}
	book.Id = vbolt.NextIntId(ctx.Tx, BookBkt)
	writeBookTx(ctx.Tx, book)
	resp.Book = GetBookById(ctx.Tx, book.Id)
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
	book, people, err := bookFor(ctx.Tx, user, req.Id, AccessContribute)
	if err != nil {
		return
	}
	if req.Revision != book.Revision {
		err = ErrBookChanged
		return
	}
	if err = applyBookContent(ctx.Tx, user, &book, people, req.Content); err != nil {
		return
	}
	book.Revision++
	book.UpdatedAt = time.Now()
	writeBookTx(ctx.Tx, book)
	resp.Book = GetBookById(ctx.Tx, book.Id)
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
