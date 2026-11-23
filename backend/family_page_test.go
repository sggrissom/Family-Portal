package backend

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.hasen.dev/vbeam"
	"go.hasen.dev/vbolt"
	"golang.org/x/crypto/bcrypt"
)

func (fx resultsFixture) addFamilyPhoto(t *testing.T, familyId int, mutate func(*Image)) Image {
	t.Helper()

	var image Image
	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		image = Image{
			Id:        vbolt.NextIntId(tx, ImagesBkt),
			FamilyId:  familyId,
			MimeType:  "image/jpeg",
			Status:    PhotoReady,
			PhotoDate: time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC),
			CreatedAt: time.Now(),
		}
		image.FilePath = "photos/family-page-test-" + time.Now().Format("150405.000000000") + ".jpg"
		if mutate != nil {
			mutate(&image)
		}
		writeImage(tx, &image)
		vbolt.TxCommit(tx)
	})
	return image
}

func writeVariantFiles(t *testing.T, image Image, contents map[string]string) {
	t.Helper()
	for name, body := range contents {
		var path string
		if name == "original" {
			path = photoOriginalPath(image.FilePath)
		} else {
			size, format, _ := strings.Cut(name, "_")
			path = photoVariantPath(image.FilePath, size, format)
		}
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Remove(path) })
	}
}

func (fx resultsFixture) setProfilePhoto(t *testing.T, person Person, photoId int) {
	t.Helper()
	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		p := GetPersonById(tx, person.Id)
		p.ProfilePhotoId = photoId
		p.ProfileCropX, p.ProfileCropY, p.ProfileCropScale = 40, 30, 1.5
		vbolt.Write(tx, PeopleBkt, p.Id, &p)
		vbolt.TxCommit(tx)
	})
}

func (fx resultsFixture) publishFamilyPage(t *testing.T, user User, settings FamilyPageSettings) (FamilyPageSettingsResponse, error) {
	t.Helper()
	var resp FamilyPageSettingsResponse
	var err error
	fx.asUser(t, user, func(ctx *vbeam.Context) {
		resp, err = PublishFamilyPage(ctx, PublishFamilyPageRequest{Settings: settings})
	})
	return resp, err
}

func (fx resultsFixture) readFamilyPage(slug string) (resp GetPublicFamilyPageResponse, err error) {
	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		resp, err = GetPublicFamilyPage(&vbeam.Context{Tx: tx}, GetPublicFamilyPageRequest{Slug: slug})
	})
	return
}

func (fx resultsFixture) outsiderUser(t *testing.T) User {
	t.Helper()
	hash, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.MinCost)
	var user User
	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		user = AddUserTx(tx, CreateAccountRequest{Name: "Outsider", Email: "outsider@example.com"}, hash)
		vbolt.TxCommit(tx)
	})
	return user
}

func basicFamilyPage(fx resultsFixture) FamilyPageSettings {
	return FamilyPageSettings{
		Slug:  "the-owners",
		Title: "The Owners",
		Intro: "Hello from us.",
		Members: []FamilyPageMemberSettings{
			{PersonId: fx.alice.Id, Name: "Ali", Line: "Loves dance", ShowAge: true},
			{PersonId: fx.bob.Id},
		},
	}
}

func TestFamilyPageServesOnlyWhatWasMarkedPublic(t *testing.T) {
	fx := setupResultsFixture(t)
	photo := fx.addFamilyPhoto(t, fx.familyId, nil)
	avatar := fx.addFamilyPhoto(t, fx.familyId, nil)
	fx.setProfilePhoto(t, fx.alice, avatar.Id)

	settings := basicFamilyPage(fx)
	settings.Members[0].ShowPhoto = true
	settings.PhotoIds = []int{photo.Id}
	resp, err := fx.publishFamilyPage(t, fx.owner, settings)
	if err != nil {
		t.Fatalf("PublishFamilyPage() error = %v", err)
	}
	if !resp.Published || !strings.HasSuffix(resp.URL, "/f/the-owners") {
		t.Fatalf("published = %v at %q", resp.Published, resp.URL)
	}

	public, err := fx.readFamilyPage("the-owners")
	if err != nil {
		t.Fatalf("GetPublicFamilyPage() without a session: %v", err)
	}
	body, _ := json.Marshal(public.Page)
	age := familyPageAge(fx.alice.Birthday, time.Now())
	want := `{"title":"The Owners","intro":"Hello from us.","members":[` +
		`{"name":"Ali","line":"Loves dance","ageYears":` + jsonInt(*age) + `,"photoUrl":"/api/family-page/the-owners/person/0","cropX":40,"cropY":30,"cropScale":1.5},` +
		`{"name":"Bob","line":"","photoUrl":"","cropX":0,"cropY":0,"cropScale":0}],` +
		`"photoUrls":["/api/family-page/the-owners/photo/0"]}`
	if string(body) != want {
		t.Errorf("public page =\n%s\nwant\n%s", body, want)
	}
	for _, private := range []string{"2014", "Carol", `"id"`, "personId", "birthday"} {
		if strings.Contains(string(body), private) {
			t.Errorf("public page leaks %q: %s", private, body)
		}
	}
}

func jsonInt(n int) string {
	b, _ := json.Marshal(n)
	return string(b)
}

func TestFamilyPageIsASnapshot(t *testing.T) {
	fx := setupResultsFixture(t)
	if _, err := fx.publishFamilyPage(t, fx.owner, basicFamilyPage(fx)); err != nil {
		t.Fatal(err)
	}

	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		alice := GetPersonById(tx, fx.alice.Id)
		alice.Name = "Alice Renamed"
		alice.Birthday = time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
		vbolt.Write(tx, PeopleBkt, alice.Id, &alice)
		vbolt.TxCommit(tx)
	})
	public, err := fx.readFamilyPage("the-owners")
	if err != nil {
		t.Fatal(err)
	}
	if got := public.Page.Members[0]; got.Name != "Ali" || *got.AgeYears != *familyPageAge(fx.alice.Birthday, time.Now()) {
		t.Errorf("editing a person changed the public page: %+v", got)
	}

	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		row, _ := FindPersonFamily(tx, fx.bob.Id, fx.familyId)
		deletePersonFamilyTx(tx, row)
		vbolt.TxCommit(tx)
	})
	public, _ = fx.readFamilyPage("the-owners")
	if len(public.Page.Members) != 1 {
		t.Errorf("someone taken off the roster still shows: %+v", public.Page.Members)
	}
}

func TestUnpublishTakesThePageAndPhotosDown(t *testing.T) {
	fx := setupResultsFixture(t)
	photo := fx.addFamilyPhoto(t, fx.familyId, nil)
	writeVariantFiles(t, photo, map[string]string{"xlarge_jpeg": "xlarge"})
	settings := basicFamilyPage(fx)
	settings.PhotoIds = []int{photo.Id}
	if _, err := fx.publishFamilyPage(t, fx.owner, settings); err != nil {
		t.Fatal(err)
	}
	if rec := getFamilyPagePhoto("/api/family-page/the-owners/photo/0"); rec.Code != http.StatusOK {
		t.Fatalf("photo before unpublish: %d", rec.Code)
	}

	fx.asUser(t, fx.owner, func(ctx *vbeam.Context) {
		resp, err := UnpublishFamilyPage(ctx, UnpublishFamilyPageRequest{})
		if err != nil || resp.Published {
			t.Fatalf("UnpublishFamilyPage() = %+v, %v", resp, err)
		}
	})
	if _, err := fx.readFamilyPage("the-owners"); err != ErrFamilyPageUnavailable {
		t.Errorf("page after unpublish: err = %v", err)
	}
	if rec := getFamilyPagePhoto("/api/family-page/the-owners/photo/0"); rec.Code != http.StatusNotFound {
		t.Errorf("photo after unpublish: %d", rec.Code)
	}
}

func getFamilyPagePhoto(path string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Header.Set("Accept", "image/avif,image/webp,*/*")
	rec := httptest.NewRecorder()
	serveFamilyPagePhoto(rec, req)
	return rec
}

func TestFamilyPagePhotosServeResizedVariantsOnly(t *testing.T) {
	fx := setupResultsFixture(t)
	photo := fx.addFamilyPhoto(t, fx.familyId, nil)
	avatar := fx.addFamilyPhoto(t, fx.familyId, nil)
	bare := fx.addFamilyPhoto(t, fx.familyId, nil)
	writeVariantFiles(t, photo, map[string]string{"xlarge_avif": "xlarge avif", "original": "camera bytes"})
	writeVariantFiles(t, avatar, map[string]string{"medium_jpeg": "medium jpeg"})
	writeVariantFiles(t, bare, map[string]string{"original": "camera bytes"})
	fx.setProfilePhoto(t, fx.alice, avatar.Id)

	settings := basicFamilyPage(fx)
	settings.Members[0].ShowPhoto = true
	settings.PhotoIds = []int{photo.Id, bare.Id}
	if _, err := fx.publishFamilyPage(t, fx.owner, settings); err != nil {
		t.Fatal(err)
	}

	rec := getFamilyPagePhoto("/api/family-page/the-owners/photo/0")
	if rec.Code != http.StatusOK || rec.Body.String() != "xlarge avif" || rec.Header().Get("Content-Type") != "image/avif" {
		t.Errorf("photo = %d %q %q", rec.Code, rec.Header().Get("Content-Type"), rec.Body.String())
	}
	for header, want := range map[string]string{
		"Cache-Control":   "no-store",
		"Referrer-Policy": "no-referrer",
		"X-Robots-Tag":    "noindex, nofollow, noarchive",
	} {
		if got := rec.Header().Get(header); got != want {
			t.Errorf("%s = %q, want %q", header, got, want)
		}
	}
	if rec := getFamilyPagePhoto("/api/family-page/the-owners/person/0"); rec.Body.String() != "medium jpeg" {
		t.Errorf("avatar = %d %q", rec.Code, rec.Body.String())
	}

	for _, path := range []string{
		"/api/family-page/the-owners/photo/1",
		"/api/family-page/the-owners/photo/2",
		"/api/family-page/the-owners/photo/01",
		"/api/family-page/the-owners/person/1",
		"/api/family-page/the-owners/original/0",
		"/api/family-page/someone-else/photo/0",
		"/api/family-page/the-owners/photo",
	} {
		if rec := getFamilyPagePhoto(path); rec.Code != http.StatusNotFound {
			t.Errorf("%s = %d %q, want 404", path, rec.Code, rec.Body.String())
		}
	}
}

func TestFamilyPageRejectsPhotosItCannotPublish(t *testing.T) {
	fx := setupResultsFixture(t)
	outsider := fx.outsiderUser(t)
	cases := map[string]Image{
		"another family's": fx.addFamilyPhoto(t, outsider.FamilyId, nil),
		"a video":          fx.addFamilyPhoto(t, fx.familyId, func(i *Image) { i.MediaKind = MediaVideo }),
		"a processing":     fx.addFamilyPhoto(t, fx.familyId, func(i *Image) { i.Status = PhotoProcessing }),
	}
	for name, image := range cases {
		settings := basicFamilyPage(fx)
		settings.PhotoIds = []int{image.Id}
		if _, err := fx.publishFamilyPage(t, fx.owner, settings); err != ErrFamilyPagePhoto {
			t.Errorf("publishing %s photo: err = %v", name, err)
		}
	}

	settings := basicFamilyPage(fx)
	other := fx.addFamilyPhoto(t, fx.familyId, nil)
	settings.PhotoIds = []int{other.Id, other.Id}
	if _, err := fx.publishFamilyPage(t, fx.owner, settings); err != ErrFamilyPagePhoto {
		t.Errorf("duplicate photo: err = %v", err)
	}
}

func TestFamilyPageRejectsPeopleOutsideTheFamily(t *testing.T) {
	fx := setupResultsFixture(t)
	outsider := fx.outsiderUser(t)
	var stranger Person
	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		stranger, _ = AddPersonTx(tx, AddPersonRequest{Name: "Stranger", Birthdate: "2015-01-01"}, outsider.FamilyId)
		vbolt.TxCommit(tx)
	})

	settings := basicFamilyPage(fx)
	settings.Members = append(settings.Members, FamilyPageMemberSettings{PersonId: stranger.Id})
	if _, err := fx.publishFamilyPage(t, fx.owner, settings); err != ErrFamilyPagePerson {
		t.Errorf("person from another family: err = %v", err)
	}
	settings = basicFamilyPage(fx)
	settings.Members = append(settings.Members, settings.Members[0])
	if _, err := fx.publishFamilyPage(t, fx.owner, settings); err != ErrFamilyPagePerson {
		t.Errorf("duplicate person: err = %v", err)
	}
}

func TestFamilyPageSlugs(t *testing.T) {
	fx := setupResultsFixture(t)
	for _, slug := range []string{"ab", "Has Space", "-lead", "trail-", "two--hyphens", "admin", "support", "ünï", strings.Repeat("a", 41)} {
		settings := basicFamilyPage(fx)
		settings.Slug = slug
		if _, err := fx.publishFamilyPage(t, fx.owner, settings); err != ErrFamilyPageSlugInvalid {
			t.Errorf("slug %q: err = %v", slug, err)
		}
	}

	settings := basicFamilyPage(fx)
	settings.Slug = "  The-Owners "
	if resp, err := fx.publishFamilyPage(t, fx.owner, settings); err != nil || resp.Settings.Slug != "the-owners" {
		t.Fatalf("mixed-case slug: %+v, %v", resp.Settings.Slug, err)
	}

	outsider := fx.outsiderUser(t)
	if _, err := fx.publishFamilyPage(t, outsider, FamilyPageSettings{Slug: "the-owners", Intro: "Us"}); err != ErrFamilyPageSlugTaken {
		t.Errorf("another family's slug: err = %v", err)
	}

	settings.Slug = "owners-two"
	if _, err := fx.publishFamilyPage(t, fx.owner, settings); err != nil {
		t.Fatal(err)
	}
	if _, err := fx.readFamilyPage("the-owners"); err != ErrFamilyPageUnavailable {
		t.Errorf("old slug still serves after a rename: %v", err)
	}
	if _, err := fx.publishFamilyPage(t, outsider, FamilyPageSettings{Slug: "the-owners", Intro: "Us"}); err != nil {
		t.Errorf("a freed slug can't be claimed: %v", err)
	}
	if _, err := fx.readFamilyPage("owners-two"); err != nil {
		t.Errorf("new slug: %v", err)
	}
}

func TestOnlyAdminsPublishTheFamilyPage(t *testing.T) {
	fx := setupResultsFixture(t)
	contributor := fx.addMember(t, "contributor@example.com", AccessContribute)

	fx.asUser(t, contributor, func(ctx *vbeam.Context) {
		if _, err := PublishFamilyPage(ctx, PublishFamilyPageRequest{FamilyId: fx.familyId, Settings: basicFamilyPage(fx)}); err != ErrFamilyPageNotAllowed {
			t.Errorf("contributor publish: err = %v", err)
		}
		resp, err := GetFamilyPageSettings(ctx, GetFamilyPageSettingsRequest{FamilyId: fx.familyId})
		if err != nil || resp.CanEdit {
			t.Errorf("contributor settings = canEdit %v, %v", resp.CanEdit, err)
		}
		if _, err := UnpublishFamilyPage(ctx, UnpublishFamilyPageRequest{FamilyId: fx.familyId}); err != ErrFamilyPageNotAllowed {
			t.Errorf("contributor unpublish: err = %v", err)
		}
	})

	outsider := fx.outsiderUser(t)
	fx.asUser(t, outsider, func(ctx *vbeam.Context) {
		if _, err := GetFamilyPageSettings(ctx, GetFamilyPageSettingsRequest{FamilyId: fx.familyId}); err != ErrFamilyAccessDenied {
			t.Errorf("outsider settings: err = %v", err)
		}
		if _, err := PublishFamilyPage(ctx, PublishFamilyPageRequest{FamilyId: fx.familyId, Settings: basicFamilyPage(fx)}); err != ErrFamilyPageNotAllowed {
			t.Errorf("outsider publish: err = %v", err)
		}
	})
}

func TestFamilyPageSettingsDefaultsAndReopen(t *testing.T) {
	fx := setupResultsFixture(t)
	fx.asUser(t, fx.owner, func(ctx *vbeam.Context) {
		resp, err := GetFamilyPageSettings(ctx, GetFamilyPageSettingsRequest{})
		if err != nil {
			t.Fatal(err)
		}
		family := GetFamily(ctx.Tx, fx.familyId)
		if resp.Published || resp.Settings.Title != family.Name || resp.Settings.Slug != suggestFamilyPageSlug(family.Name) {
			t.Errorf("defaults = %+v", resp)
		}
		if len(resp.People) != 3 || len(resp.Settings.Members) != 0 {
			t.Errorf("defaults list %d people with %d preselected", len(resp.People), len(resp.Settings.Members))
		}
	})

	if _, err := fx.publishFamilyPage(t, fx.owner, basicFamilyPage(fx)); err != nil {
		t.Fatal(err)
	}
	fx.asUser(t, fx.owner, func(ctx *vbeam.Context) {
		resp, _ := GetFamilyPageSettings(ctx, GetFamilyPageSettingsRequest{})
		got := resp.Settings.Members
		if len(got) != 2 || got[0] != (FamilyPageMemberSettings{PersonId: fx.alice.Id, Name: "Ali", Line: "Loves dance", ShowAge: true}) ||
			got[1] != (FamilyPageMemberSettings{PersonId: fx.bob.Id, Name: "Bob"}) {
			t.Errorf("reopened members = %+v", got)
		}
		if resp.PublishedByName != fx.owner.Name || resp.PublishedAt == nil {
			t.Errorf("publisher = %q at %v", resp.PublishedByName, resp.PublishedAt)
		}
	})
}

func TestFamilyPageNeedsSomethingToShow(t *testing.T) {
	fx := setupResultsFixture(t)
	if _, err := fx.publishFamilyPage(t, fx.owner, FamilyPageSettings{Slug: "empty-page", Title: "Just a title"}); err != ErrFamilyPageEmpty {
		t.Errorf("title-only page: err = %v", err)
	}
	settings := basicFamilyPage(fx)
	settings.Members = make([]FamilyPageMemberSettings, maxFamilyPageMembers+1)
	if _, err := fx.publishFamilyPage(t, fx.owner, settings); err != ErrFamilyPageTooLarge {
		t.Errorf("too many people: err = %v", err)
	}
}

func TestDeletingAFamilyDeletesItsPage(t *testing.T) {
	fx := setupResultsFixture(t)
	if _, err := fx.publishFamilyPage(t, fx.owner, basicFamilyPage(fx)); err != nil {
		t.Fatal(err)
	}
	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		deleteFamilyContentTx(tx, fx.familyId)
		vbolt.TxCommit(tx)
	})
	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		if _, found := GetFamilyPage(tx, fx.familyId); found {
			t.Error("page survived family deletion")
		}
		var familyId int
		if vbolt.Read(tx, FamilyPageBySlugBkt, "the-owners", &familyId) {
			t.Error("slug survived family deletion")
		}
	})
}

func TestFamilyPageShellHeaders(t *testing.T) {
	fx := setupResultsFixture(t)
	settings := basicFamilyPage(fx)
	if _, err := fx.publishFamilyPage(t, fx.owner, settings); err != nil {
		t.Fatal(err)
	}
	shell := func(path string) http.Header {
		rec := httptest.NewRecorder()
		addFamilyPageHeaders(rec, httptest.NewRequest(http.MethodGet, path, nil))
		return rec.Header()
	}

	h := shell("/f/the-owners")
	if h.Get("Cache-Control") != "no-store" || h.Get("Referrer-Policy") != "no-referrer" || h.Get("X-Robots-Tag") == "" {
		t.Errorf("unlisted page headers = %v", h)
	}
	if h := shell("/f/nobody-here"); h.Get("X-Robots-Tag") == "" {
		t.Errorf("unknown slug is indexable: %v", h)
	}
	if h := shell("/settings"); len(h) != 0 {
		t.Errorf("headers set outside /f/: %v", h)
	}

	settings.Indexable = true
	if _, err := fx.publishFamilyPage(t, fx.owner, settings); err != nil {
		t.Fatal(err)
	}
	if h := shell("/f/the-owners"); h.Get("X-Robots-Tag") != "" || h.Get("Cache-Control") != "no-store" {
		t.Errorf("indexable page headers = %v", h)
	}
}

func TestFamilyPageAge(t *testing.T) {
	born := time.Date(2020, 3, 15, 0, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		today string
		want  int
	}{{"2020-03-15", 0}, {"2021-03-14", 0}, {"2021-03-15", 1}, {"2026-11-22", 6}} {
		today, _ := time.Parse("2006-01-02", tc.today)
		if got := familyPageAge(born, today); got == nil || *got != tc.want {
			t.Errorf("age on %s = %v, want %d", tc.today, got, tc.want)
		}
	}
	if familyPageAge(born, born.AddDate(0, 0, -1)) != nil || familyPageAge(time.Time{}, born) != nil {
		t.Error("an unborn or undated person got an age")
	}
}

func TestSuggestFamilyPageSlug(t *testing.T) {
	for name, want := range map[string]string{
		"The Grissom Family": "the-grissom-family",
		"O'Brien & Co.":      "o-brien-co",
		"  Ünïcode  Smiths ": "n-code-smiths",
		"":                   "",
	} {
		if got := suggestFamilyPageSlug(name); got != want {
			t.Errorf("suggestFamilyPageSlug(%q) = %q, want %q", name, got, want)
		}
	}
}

func TestFamilyPageRequestsAreRateLimited(t *testing.T) {
	for _, path := range []string{"/f/the-owners", "/rpc/GetPublicFamilyPage", "/api/family-page/the-owners/photo/0"} {
		if rule, ok := ruleForPath(path); !ok || rule.Name != rateRuleFamilyPage.Name {
			t.Errorf("%s rate rule = %q", path, rule.Name)
		}
	}
}
