package backend

import (
	"encoding/json"
	"family/cfg"
	"testing"
	"time"

	"go.hasen.dev/vbeam"
	"go.hasen.dev/vbolt"
	"go.hasen.dev/vpack"
	"golang.org/x/crypto/bcrypt"
)

type bookFixture struct {
	db          *vbolt.DB
	owner       User
	outsider    User
	baby        Person
	sibling     Person
	stranger    Person
	steps       Milestone
	smile       Milestone
	tooth       Milestone
	early       Image
	attached    Image
	untagged    Image
	siblingOnly Image
	together    Image
	otherBaby   Image
}

func setupBookFixture(t *testing.T) bookFixture {
	t.Helper()
	db := vbolt.Open(t.TempDir() + "/books.db")
	vbolt.InitBuckets(db, &cfg.Info)
	t.Cleanup(func() { _ = db.Close() })
	appDb = db
	jwtKey = []byte("books-test-secret-key-that-is-at-least-32")

	fx := bookFixture{db: db}
	hash, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.MinCost)
	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		fx.owner = AddUserTx(tx, CreateAccountRequest{Name: "Owner", Email: "owner@example.com"}, hash)
		fx.outsider = AddUserTx(tx, CreateAccountRequest{Name: "Outsider", Email: "out@example.com"}, hash)
		fx.baby, _ = AddPersonTx(tx, AddPersonRequest{Name: "June", Birthdate: "2024-03-14"}, fx.owner.FamilyId)
		fx.sibling, _ = AddPersonTx(tx, AddPersonRequest{Name: "Theo", Birthdate: "2021-06-01"}, fx.owner.FamilyId)
		fx.stranger, _ = AddPersonTx(tx, AddPersonRequest{Name: "Rosie", Birthdate: "2024-01-01"}, fx.outsider.FamilyId)

		photo := func(familyId int, date string, tagged *Person) Image {
			image := Image{
				Id: vbolt.NextIntId(tx, ImagesBkt), FamilyId: familyId, OwnerUserId: fx.owner.Id,
				OriginalFilename: date + ".jpg", PhotoDate: day(date), Width: 1200, Height: 800,
				CreatedAt: time.Now(),
			}
			vbolt.Write(tx, ImagesBkt, image.Id, &image)
			vbolt.SetTargetSingleTerm(tx, ImageByFamilyIndex, image.Id, familyId)
			if tagged != nil {
				tagPersonInPhoto(tx, image.Id, tagged.Id, familyId)
			}
			return image
		}
		fx.early = photo(fx.owner.FamilyId, "2024-04-02", &fx.baby)
		fx.attached = photo(fx.owner.FamilyId, "2024-05-01", &fx.baby)
		fx.untagged = photo(fx.owner.FamilyId, "2024-05-02", nil)
		fx.otherBaby = photo(fx.outsider.FamilyId, "2024-05-03", &fx.stranger)
		fx.siblingOnly = photo(fx.owner.FamilyId, "2024-05-04", &fx.sibling)
		fx.together = photo(fx.owner.FamilyId, "2024-06-01", &fx.baby)
		tagPersonInPhoto(tx, fx.together.Id, fx.sibling.Id, fx.owner.FamilyId)
		for _, image := range []Image{fx.early, fx.attached, fx.untagged, fx.otherBaby, fx.siblingOnly, fx.together} {
			ReindexPhotoDates(tx, image.Id)
		}
		photo(fx.owner.FamilyId, "2026-01-01", &fx.baby)

		milestone := func(person Person, date, text string, photoIds ...int) Milestone {
			m, err := AddMilestoneTx(tx, AddMilestoneRequest{
				PersonId: person.Id, Description: text, Category: "first",
				InputType: "date", MilestoneDate: &date, PhotoIds: photoIds,
			}, person.FamilyId)
			if err != nil {
				t.Fatalf("AddMilestoneTx(%s) = %v", text, err)
			}
			return m
		}
		fx.smile = milestone(fx.baby, "2024-05-01", "First smile", fx.attached.Id)
		fx.steps = milestone(fx.baby, "2025-03-14", "First steps on her birthday")
		fx.tooth = milestone(fx.sibling, "2024-05-05", "Theo lost a tooth")
		milestone(fx.baby, "2025-03-15", "The day after")
		vbolt.TxCommit(tx)
	})
	return fx
}

func bookCall[Req any, Resp any](t *testing.T, fx bookFixture, as User, proc func(*vbeam.Context, Req) (Resp, error), req Req) (Resp, error) {
	t.Helper()
	token, err := generateJwtTokenString(as)
	if err != nil {
		t.Fatal(err)
	}
	var resp Resp
	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		resp, err = proc(&vbeam.Context{Tx: tx, Token: token}, req)
	})
	return resp, err
}

func (fx bookFixture) content() BookContent {
	return BookContent{
		Title:        "June's first year",
		CoverPhotoId: fx.early.Id,
		Density:      "balanced",
		Introduction: "We waited a long time for you.",
		ShowGrowth:   true,
		Items: []BookItem{
			{Kind: BookItemPhoto, SourceId: fx.early.Id, Caption: "Home at last"},
			{Kind: BookItemMilestone, SourceId: fx.smile.Id, PhotoId: fx.attached.Id, Pinned: true},
			{Kind: BookItemMilestone, SourceId: fx.steps.Id},
		},
	}
}

func (fx bookFixture) firstYear(content BookContent) CreateBookRequest {
	return CreateBookRequest{PersonIds: []int{fx.baby.Id}, Preset: BookPresetFirstYear, Content: content}
}

func (fx bookFixture) createBook(t *testing.T) Book {
	t.Helper()
	resp, err := bookCall(t, fx, fx.owner, CreateBook, fx.firstYear(fx.content()))
	if err != nil {
		t.Fatalf("CreateBook() = %v", err)
	}
	return resp.Book
}

func TestBookSourcesCoverTheFirstYearAndItsBirthday(t *testing.T) {
	fx := setupBookFixture(t)
	resp, err := bookCall(t, fx, fx.owner, GetBookSources, GetBookSourcesRequest{PersonIds: []int{fx.baby.Id}, Preset: BookPresetFirstYear})
	if err != nil {
		t.Fatal(err)
	}
	if !resp.StartDate.Equal(day("2024-03-14")) || !resp.EndDate.Equal(day("2025-03-14")) {
		t.Errorf("range = %v – %v", resp.StartDate, resp.EndDate)
	}
	var texts []string
	for _, m := range resp.Sources.Milestones {
		texts = append(texts, m.Description)
	}
	if len(texts) != 2 || texts[0] != "First smile" || texts[1] != "First steps on her birthday" {
		t.Errorf("milestones = %v, want the smile and the birthday, not the day after or Theo's", texts)
	}
	if len(resp.Sources.Photos) != 4 {
		t.Errorf("photos = %d, want three of June and one of nobody", len(resp.Sources.Photos))
	}
	if len(resp.Sources.Untagged) != 1 || resp.Sources.Untagged[0] != fx.untagged.Id {
		t.Errorf("untagged = %v, want the household photo with nobody in it", resp.Sources.Untagged)
	}
	if got := resp.Sources.PhotoPeople[fx.together.Id]; len(got) != 1 || got[0] != fx.baby.Id {
		t.Errorf("photo people = %v, want only the book's own people", got)
	}
}

func TestBookSavesAndReopens(t *testing.T) {
	fx := setupBookFixture(t)
	book := fx.createBook(t)
	if book.Revision != 1 || book.PersonId != fx.baby.Id || len(book.Items) != 3 {
		t.Fatalf("created = %+v", book)
	}

	got, err := bookCall(t, fx, fx.owner, GetBook, GetBookRequest{Id: book.Id})
	if err != nil {
		t.Fatal(err)
	}
	if !got.CanEdit || got.Book.Items[0].Caption != "Home at last" || !got.Book.Items[1].Pinned {
		t.Errorf("reopened = %+v", got.Book)
	}
	if got.Book.Introduction != "We waited a long time for you." || got.Book.CoverPhotoId != fx.early.Id {
		t.Errorf("writing or cover lost: %+v", got.Book)
	}

	content := fx.content()
	content.Items = content.Items[1:]
	content.Excluded = []BookItem{{Kind: BookItemPhoto, SourceId: fx.early.Id, Caption: "dropped"}}
	updated, err := bookCall(t, fx, fx.owner, UpdateBook, UpdateBookRequest{Id: book.Id, Revision: 1, Content: content})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Book.Revision != 2 || len(updated.Book.Items) != 2 || len(updated.Book.Excluded) != 1 {
		t.Errorf("updated = %+v", updated.Book)
	}
	if updated.Book.Excluded[0].Caption != "" {
		t.Errorf("excluded items keep only their reference, got %+v", updated.Book.Excluded[0])
	}

	if _, err := bookCall(t, fx, fx.owner, UpdateBook, UpdateBookRequest{Id: book.Id, Revision: 1, Content: content}); err != ErrBookChanged {
		t.Errorf("stale save = %v, want ErrBookChanged", err)
	}
}

func TestBookRefusesRecordsThatAreNotTheSubjects(t *testing.T) {
	fx := setupBookFixture(t)
	for name, items := range map[string][]BookItem{
		"sibling's milestone":    {{Kind: BookItemMilestone, SourceId: fx.tooth.Id}},
		"sibling's photo":        {{Kind: BookItemPhoto, SourceId: fx.siblingOnly.Id}},
		"other family's photo":   {{Kind: BookItemPhoto, SourceId: fx.otherBaby.Id}},
		"photo not on milestone": {{Kind: BookItemMilestone, SourceId: fx.smile.Id, PhotoId: fx.early.Id}},
		"missing milestone":      {{Kind: BookItemMilestone, SourceId: 9999}},
	} {
		content := fx.content()
		content.Items = items
		if _, err := bookCall(t, fx, fx.owner, CreateBook, fx.firstYear(content)); err == nil {
			t.Errorf("%s: CreateBook succeeded", name)
		}
	}
	content := fx.content()
	content.CoverPhotoId = fx.otherBaby.Id
	if _, err := bookCall(t, fx, fx.owner, CreateBook, fx.firstYear(content)); err == nil {
		t.Error("another family's cover photo was accepted")
	}
}

func TestBooksStayInTheirHousehold(t *testing.T) {
	fx := setupBookFixture(t)
	book := fx.createBook(t)

	list, err := bookCall(t, fx, fx.outsider, ListBooks, ListBooksRequest{})
	if err != nil || len(list.Books) != 0 {
		t.Errorf("outsider list = %+v, %v", list.Books, err)
	}
	if _, err := bookCall(t, fx, fx.outsider, GetBook, GetBookRequest{Id: book.Id}); err != ErrBookNotFound {
		t.Errorf("outsider GetBook = %v", err)
	}
	if _, err := bookCall(t, fx, fx.outsider, GetBookSources, GetBookSourcesRequest{PersonIds: []int{fx.baby.Id}, Preset: BookPresetFirstYear}); err == nil {
		t.Error("outsider read the candidates")
	}
	if _, err := bookCall(t, fx, fx.outsider, UpdateBook, UpdateBookRequest{Id: book.Id, Revision: 1, Content: fx.content()}); err != ErrBookNotFound {
		t.Errorf("outsider UpdateBook = %v", err)
	}
	if _, err := bookCall(t, fx, fx.outsider, DeleteBook, DeleteBookRequest{Id: book.Id}); err != ErrBookNotFound {
		t.Errorf("outsider DeleteBook = %v", err)
	}
	if _, err := bookCall(t, fx, fx.outsider, CreateBook, fx.firstYear(fx.content())); err == nil {
		t.Error("outsider created a book of someone else's child")
	}

	mine, _ := bookCall(t, fx, fx.owner, ListBooks, ListBooksRequest{})
	if len(mine.Books) != 1 || mine.Books[0].PersonNames[0] != "June" {
		t.Errorf("owner list = %+v", mine.Books)
	}
}

func TestDeletedSourcesLeaveTheBookButNotTheReader(t *testing.T) {
	fx := setupBookFixture(t)
	book := fx.createBook(t)
	if _, err := bookCall(t, fx, fx.owner, DeleteMilestone, DeleteMilestoneRequest{Id: fx.steps.Id}); err != nil {
		t.Fatal(err)
	}
	got, err := bookCall(t, fx, fx.owner, GetBook, GetBookRequest{Id: book.Id})
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range got.Sources.Milestones {
		if m.Id == fx.steps.Id {
			t.Error("deleted milestone is still served")
		}
	}
	if len(got.Book.Items) != 3 {
		t.Errorf("the saved selection should be untouched until the editor saves, got %d items", len(got.Book.Items))
	}
}

func TestMergingAndDeletingPeopleCarryTheirBooks(t *testing.T) {
	fx := setupBookFixture(t)
	book := fx.createBook(t)

	summary, _ := bookCall(t, fx, fx.owner, GetPersonDeletionSummary, PersonDeletionRequest{PersonId: fx.baby.Id})
	if summary.Books != 1 {
		t.Errorf("deletion summary books = %d", summary.Books)
	}

	if _, err := bookCall(t, fx, fx.owner, MergePeople, MergePeopleRequest{SourcePersonId: fx.baby.Id, TargetPersonId: fx.sibling.Id}); err != nil {
		t.Fatal(err)
	}
	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		if got := GetBookById(tx, book.Id); got.PersonId != fx.sibling.Id {
			t.Errorf("book person after merge = %d, want %d", got.PersonId, fx.sibling.Id)
		}
	})

	if _, err := bookCall(t, fx, fx.owner, DeletePerson, PersonDeletionRequest{PersonId: fx.sibling.Id}); err != nil {
		t.Fatal(err)
	}
	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		if GetBookById(tx, book.Id).Id != 0 || len(GetFamilyBooks(tx, fx.owner.FamilyId)) != 0 {
			t.Error("book survived its person")
		}
	})
}

func TestFamilyDeletionRemovesBooks(t *testing.T) {
	fx := setupBookFixture(t)
	book := fx.createBook(t)
	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		deleteFamilyContentTx(tx, fx.owner.FamilyId)
		vbolt.TxCommit(tx)
	})
	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		if GetBookById(tx, book.Id).Id != 0 {
			t.Error("book survived its family")
		}
	})
}

func TestBookExportImportRemapsReferences(t *testing.T) {
	fx := setupBookFixture(t)
	fx.createBook(t)

	var bundle ExportDataStructure
	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		bundle, _ = buildExportData(tx, fx.owner.FamilyId)
	})
	if len(bundle.Books) != 1 || bundle.Books[0].Items[1].Kind != "milestone" {
		t.Fatalf("exported books = %+v", bundle.Books)
	}
	data, _ := json.Marshal(bundle)

	resp, err := bookCall(t, fx, fx.outsider, ImportData, ImportDataRequest{JsonData: string(data), ImportMilestones: true})
	if err != nil {
		t.Fatal(err)
	}
	if resp.ImportedBooks != 1 || len(resp.Warnings) == 0 {
		t.Fatalf("imported = %d, warnings = %v; photos are not in a data-only import", resp.ImportedBooks, resp.Warnings)
	}

	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		books := GetFamilyBooks(tx, fx.outsider.FamilyId)
		if len(books) != 1 {
			t.Fatalf("books in new family = %d", len(books))
		}
		imported := books[0]
		person := GetPersonById(tx, imported.PersonId)
		if person.FamilyId != fx.outsider.FamilyId || person.Name != "June" {
			t.Errorf("imported book points at %+v", person)
		}
		if len(imported.Items) != 2 || imported.CoverPhotoId != 0 {
			t.Fatalf("imported items = %+v, cover = %d", imported.Items, imported.CoverPhotoId)
		}
		for _, item := range imported.Items {
			m := GetMilestoneById(tx, item.SourceId)
			if m.FamilyId != fx.outsider.FamilyId || m.PersonId != person.Id {
				t.Errorf("item %+v points at %+v", item, m)
			}
			if item.PhotoId != 0 {
				t.Errorf("photo reference survived without its photo: %+v", item)
			}
		}
	})

	again, _ := bookCall(t, fx, fx.outsider, ImportData, ImportDataRequest{JsonData: string(data), ImportMilestones: true, MergeStrategy: "merge_people"})
	if again.ImportedBooks != 0 || again.SkippedBooks != 1 {
		t.Errorf("re-import imported %d, skipped %d", again.ImportedBooks, again.SkippedBooks)
	}
}

func TestBookImportMapsPhotos(t *testing.T) {
	fx := setupBookFixture(t)
	fx.createBook(t)
	var bundle ExportDataStructure
	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		bundle, _ = buildExportData(tx, fx.owner.FamilyId)
	})
	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		imported, _, warnings := importBooks(tx, bundle.Books, bookImport{
			familyId:        fx.owner.FamilyId,
			personIdMapping: map[int]int{fx.baby.Id: fx.sibling.Id},
			milestoneIds:    map[int]int{fx.smile.Id: fx.smile.Id},
			photoIds:        map[int]int{fx.early.Id: fx.early.Id, fx.attached.Id: fx.attached.Id},
		})
		if imported != 1 || len(warnings) != 1 {
			t.Errorf("imported = %d, warnings = %v (the birthday milestone was not mapped)", imported, warnings)
		}
		books := GetPersonBooks(tx, fx.sibling.Id)
		if len(books) != 1 || books[0].CoverPhotoId != fx.early.Id || len(books[0].Items) != 2 || books[0].Items[1].PhotoId != fx.attached.Id {
			t.Errorf("imported = %+v", books)
		}
	})
}

func TestYearBooksUseTheDatesTheyWereGiven(t *testing.T) {
	fx := setupBookFixture(t)
	req := CreateBookRequest{
		PersonIds: []int{fx.baby.Id}, Preset: BookPresetYear,
		StartDate: "2024-05-01", EndDate: "2024-06-01",
		Content: BookContent{Items: []BookItem{{Kind: BookItemMilestone, SourceId: fx.smile.Id}}},
	}
	created, err := bookCall(t, fx, fx.owner, CreateBook, req)
	if err != nil {
		t.Fatal(err)
	}
	if !created.Book.StartDate.Equal(day("2024-05-01")) || !created.Book.EndDate.Equal(day("2024-06-01")) {
		t.Errorf("range = %v – %v", created.Book.StartDate, created.Book.EndDate)
	}
	got, _ := bookCall(t, fx, fx.owner, GetBook, GetBookRequest{Id: created.Book.Id})
	for _, p := range got.Sources.Photos {
		if p.Id == fx.together.Id {
			t.Error("June 1 is the exclusive end of a May book")
		}
	}

	for name, bad := range map[string]CreateBookRequest{
		"backwards":       {PersonIds: []int{fx.baby.Id}, Preset: BookPresetYear, StartDate: "2024-06-01", EndDate: "2024-05-01"},
		"no dates":        {PersonIds: []int{fx.baby.Id}, Preset: BookPresetCustom},
		"two in a year":   {PersonIds: []int{fx.baby.Id, fx.sibling.Id}, Preset: BookPresetYear, StartDate: "2024-01-01", EndDate: "2025-01-01"},
		"two first years": {PersonIds: []int{fx.baby.Id, fx.sibling.Id}, Preset: BookPresetFirstYear},
		"unknown preset":  {PersonIds: []int{fx.baby.Id}, Preset: "novel", StartDate: "2024-01-01", EndDate: "2025-01-01"},
		"someone else's":  {PersonIds: []int{fx.baby.Id, fx.stranger.Id}, Preset: BookPresetCustom, StartDate: "2024-01-01", EndDate: "2025-01-01"},
		"nobody":          {Preset: BookPresetCustom, StartDate: "2024-01-01", EndDate: "2025-01-01"},
		"half a lifetime": {PersonIds: []int{fx.baby.Id}, Preset: BookPresetCustom, StartDate: "1980-01-01", EndDate: "2024-01-01"},
	} {
		if _, err := bookCall(t, fx, fx.owner, CreateBook, bad); err == nil {
			t.Errorf("%s: CreateBook succeeded", name)
		}
	}
}

func TestFamilyBooksTakeEveryonesRecordsAndHouseholdPhotos(t *testing.T) {
	fx := setupBookFixture(t)
	req := CreateBookRequest{
		PersonIds: []int{fx.baby.Id, fx.sibling.Id}, Preset: BookPresetFamilyYear,
		StartDate: "2024-01-01", EndDate: "2025-01-01",
		Content: BookContent{
			Categories: []string{"photos", "milestones", "bogus"},
			Match:      "all",
			Items: []BookItem{
				{Kind: BookItemMilestone, SourceId: fx.tooth.Id},
				{Kind: BookItemMilestone, SourceId: fx.smile.Id},
				{Kind: BookItemPhoto, SourceId: fx.siblingOnly.Id},
				{Kind: BookItemPhoto, SourceId: fx.together.Id},
				{Kind: BookItemPhoto, SourceId: fx.untagged.Id},
			},
			CoverPhotoId: fx.together.Id,
		},
	}
	created, err := bookCall(t, fx, fx.owner, CreateBook, req)
	if err != nil {
		t.Fatal(err)
	}
	book := created.Book
	if len(book.PersonIds) != 2 || book.PersonId != fx.baby.Id || len(book.Items) != 5 {
		t.Errorf("created = %+v", book)
	}
	if len(book.Categories) != 2 || book.Match != "all" {
		t.Errorf("categories = %v, match = %q", book.Categories, book.Match)
	}

	got, _ := bookCall(t, fx, fx.owner, GetBook, GetBookRequest{Id: book.Id})
	if len(got.Sources.People) != 2 || got.Sources.PhotoPeople[fx.together.Id] == nil || len(got.Sources.PhotoPeople[fx.together.Id]) != 2 {
		t.Errorf("sources people = %v, photo people = %v", got.Sources.People, got.Sources.PhotoPeople)
	}
	if got.Now.IsZero() {
		t.Error("GetBook should say what time it is for the additions tray")
	}

	other := req
	other.Content.Items = []BookItem{{Kind: BookItemPhoto, SourceId: fx.otherBaby.Id}}
	other.Content.CoverPhotoId = 0
	if _, err := bookCall(t, fx, fx.owner, CreateBook, other); err == nil {
		t.Error("another family's photo was accepted in a family book")
	}

	for _, b := range []int{fx.baby.Id, fx.sibling.Id} {
		vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
			if len(GetPersonBooks(tx, b)) != 1 {
				t.Errorf("book is not indexed under person %d", b)
			}
		})
	}

	if _, err := bookCall(t, fx, fx.owner, DeletePerson, PersonDeletionRequest{PersonId: fx.sibling.Id}); err != nil {
		t.Fatal(err)
	}
	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		kept := GetBookById(tx, book.Id)
		if kept.Id == 0 || len(kept.PersonIds) != 1 || kept.PersonIds[0] != fx.baby.Id {
			t.Errorf("after deleting Theo the book is %+v, want June's alone", kept)
		}
	})
}

func TestMergingTwoPeopleInOneBookLeavesOne(t *testing.T) {
	fx := setupBookFixture(t)
	created, err := bookCall(t, fx, fx.owner, CreateBook, CreateBookRequest{
		PersonIds: []int{fx.baby.Id, fx.sibling.Id}, Preset: BookPresetCustom,
		StartDate: "2024-01-01", EndDate: "2025-01-01",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := bookCall(t, fx, fx.owner, MergePeople, MergePeopleRequest{SourcePersonId: fx.sibling.Id, TargetPersonId: fx.baby.Id}); err != nil {
		t.Fatal(err)
	}
	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		if got := GetBookById(tx, created.Book.Id); len(got.PersonIds) != 1 || got.PersonIds[0] != fx.baby.Id {
			t.Errorf("person ids after merge = %v", got.PersonIds)
		}
	})
}

func TestReviewedAtOnlyMovesForward(t *testing.T) {
	fx := setupBookFixture(t)
	book := fx.createBook(t)
	content := fx.content()
	content.ReviewedAt = book.ReviewedAt.Add(-time.Hour)
	updated, _ := bookCall(t, fx, fx.owner, UpdateBook, UpdateBookRequest{Id: book.Id, Revision: 1, Content: content})
	if !updated.Book.ReviewedAt.Equal(book.ReviewedAt) {
		t.Errorf("reviewedAt moved back to %v", updated.Book.ReviewedAt)
	}
	content.ReviewedAt = time.Now().Add(time.Hour)
	updated, _ = bookCall(t, fx, fx.owner, UpdateBook, UpdateBookRequest{Id: book.Id, Revision: 2, Content: content})
	if updated.Book.ReviewedAt.After(time.Now()) {
		t.Errorf("reviewedAt jumped into the future: %v", updated.Book.ReviewedAt)
	}
}

func packBookV1(self *Book, buf *vpack.Buffer) {
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

func TestFirstVersionBooksReadAsOnePersonBooks(t *testing.T) {
	saved := Book{Id: 7, FamilyId: 3, PersonId: 42, Preset: BookPresetFirstYear, Title: "Old", UpdatedAt: day("2026-10-01")}
	got := vpack.FromBytes(vpack.ToBytes(&saved, packBookV1), PackBook)
	if len(got.PersonIds) != 1 || got.PersonIds[0] != 42 || got.Match != "any" || !got.ReviewedAt.Equal(saved.UpdatedAt) {
		t.Errorf("v1 book read as %+v", got)
	}
	again := vpack.FromBytes(vpack.ToBytes(got, PackBook), PackBook)
	if len(again.PersonIds) != 1 || again.Title != "Old" {
		t.Errorf("v2 round trip = %+v", again)
	}
}
