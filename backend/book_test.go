package backend

import (
	"encoding/json"
	"family/cfg"
	"testing"
	"time"

	"go.hasen.dev/vbeam"
	"go.hasen.dev/vbolt"
	"golang.org/x/crypto/bcrypt"
)

type bookFixture struct {
	db        *vbolt.DB
	owner     User
	outsider  User
	baby      Person
	sibling   Person
	stranger  Person
	steps     Milestone
	smile     Milestone
	tooth     Milestone
	early     Image
	attached  Image
	untagged  Image
	otherBaby Image
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

func (fx bookFixture) createBook(t *testing.T) Book {
	t.Helper()
	resp, err := bookCall(t, fx, fx.owner, CreateBook, CreateBookRequest{PersonId: fx.baby.Id, Content: fx.content()})
	if err != nil {
		t.Fatalf("CreateBook() = %v", err)
	}
	return resp.Book
}

func TestBookSourcesCoverTheFirstYearAndItsBirthday(t *testing.T) {
	fx := setupBookFixture(t)
	resp, err := bookCall(t, fx, fx.owner, GetBookSources, GetBookSourcesRequest{PersonId: fx.baby.Id})
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
	if len(resp.Sources.Photos) != 2 {
		t.Errorf("photos = %d, want the two tagged photos from the year", len(resp.Sources.Photos))
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
		"untagged photo":         {{Kind: BookItemPhoto, SourceId: fx.untagged.Id}},
		"other family's photo":   {{Kind: BookItemPhoto, SourceId: fx.otherBaby.Id}},
		"photo not on milestone": {{Kind: BookItemMilestone, SourceId: fx.smile.Id, PhotoId: fx.early.Id}},
		"missing milestone":      {{Kind: BookItemMilestone, SourceId: 9999}},
	} {
		content := fx.content()
		content.Items = items
		if _, err := bookCall(t, fx, fx.owner, CreateBook, CreateBookRequest{PersonId: fx.baby.Id, Content: content}); err == nil {
			t.Errorf("%s: CreateBook succeeded", name)
		}
	}
	content := fx.content()
	content.CoverPhotoId = fx.otherBaby.Id
	if _, err := bookCall(t, fx, fx.owner, CreateBook, CreateBookRequest{PersonId: fx.baby.Id, Content: content}); err == nil {
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
	if _, err := bookCall(t, fx, fx.outsider, GetBookSources, GetBookSourcesRequest{PersonId: fx.baby.Id}); err == nil {
		t.Error("outsider read the candidates")
	}
	if _, err := bookCall(t, fx, fx.outsider, UpdateBook, UpdateBookRequest{Id: book.Id, Revision: 1, Content: fx.content()}); err != ErrBookNotFound {
		t.Errorf("outsider UpdateBook = %v", err)
	}
	if _, err := bookCall(t, fx, fx.outsider, DeleteBook, DeleteBookRequest{Id: book.Id}); err != ErrBookNotFound {
		t.Errorf("outsider DeleteBook = %v", err)
	}
	if _, err := bookCall(t, fx, fx.outsider, CreateBook, CreateBookRequest{PersonId: fx.baby.Id, Content: fx.content()}); err == nil {
		t.Error("outsider created a book of someone else's child")
	}

	mine, _ := bookCall(t, fx, fx.owner, ListBooks, ListBooksRequest{})
	if len(mine.Books) != 1 || mine.Books[0].PersonName != "June" {
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
