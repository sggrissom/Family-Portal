package backend

import (
	"errors"
	"family/cfg"
	"regexp"
	"time"

	"go.hasen.dev/vbolt"
	"go.hasen.dev/vpack"
)

// A FamilyPage is the one public page a family can publish at /f/<slug>. It
// holds the snapshot the owner approved, never live records, so editing a
// person leaves it as it was. Unpublishing deletes it outright.
type FamilyPage struct {
	FamilyId    int
	Slug        string
	PublishedBy int
	PublishedAt time.Time
	Indexable   bool
	Title       string
	Intro       string
	Members     []FamilyPageMember
	PhotoIds    []int
}

// Birthday is set only when the age is shown, and PhotoId only when the
// avatar is. PersonId lets the editor reopen the page and is never sent to
// visitors.
type FamilyPageMember struct {
	PersonId  int
	Name      string
	Line      string
	Birthday  time.Time
	PhotoId   int
	CropX     float64
	CropY     float64
	CropScale float64
}

func PackFamilyPageMember(self *FamilyPageMember, buf *vpack.Buffer) {
	vpack.Version(1, buf)
	vpack.Int(&self.PersonId, buf)
	vpack.String(&self.Name, buf)
	vpack.String(&self.Line, buf)
	vpack.Time(&self.Birthday, buf)
	vpack.Int(&self.PhotoId, buf)
	vpack.Float64(&self.CropX, buf)
	vpack.Float64(&self.CropY, buf)
	vpack.Float64(&self.CropScale, buf)
}

func PackFamilyPage(self *FamilyPage, buf *vpack.Buffer) {
	vpack.Version(1, buf)
	vpack.Int(&self.FamilyId, buf)
	vpack.String(&self.Slug, buf)
	vpack.Int(&self.PublishedBy, buf)
	vpack.Time(&self.PublishedAt, buf)
	vpack.Bool(&self.Indexable, buf)
	vpack.String(&self.Title, buf)
	vpack.String(&self.Intro, buf)
	vpack.Slice(&self.Members, PackFamilyPageMember, buf)
	vpack.Slice(&self.PhotoIds, vpack.Int, buf)
}

var FamilyPageBkt = vbolt.Bucket(&cfg.Info, "family_pages", vpack.FInt, PackFamilyPage)

var FamilyPageBySlugBkt = vbolt.Bucket(&cfg.Info, "family_pages_by_slug", vpack.StringZ, vpack.Int)

const (
	FamilyPagePathPrefix      = "/f/"
	FamilyPagePhotoPathPrefix = "/api/family-page/"
	minFamilyPageSlug         = 3
	maxFamilyPageSlug         = 40
	maxFamilyPageMembers      = 20
	maxFamilyPagePhotos       = 6
	maxFamilyPageTitle        = 80
	maxFamilyPageIntro        = 1000
	maxFamilyPageName         = 60
	maxFamilyPageLine         = 160
)

var familyPageSlugPattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

var reservedFamilyPageSlugs = map[string]bool{
	"admin": true, "administrator": true, "api": true, "app": true, "family": true,
	"familyrecord": true, "family-record": true, "help": true, "login": true,
	"logout": true, "official": true, "privacy": true, "root": true, "security": true,
	"settings": true, "signup": true, "staff": true, "support": true, "system": true,
	"team": true, "terms": true, "www": true,
}

var (
	ErrFamilyPageSlugInvalid = errors.New("Use 3 to 40 lowercase letters, numbers and single hyphens")
	ErrFamilyPageSlugTaken   = errors.New("That address is taken. Try another.")
	ErrFamilyPageNotAllowed  = errors.New("Only a family admin can publish or change the family page")
	ErrFamilyPageTooLarge    = errors.New("That's more than a family page can hold")
	ErrFamilyPagePerson      = errors.New("Everyone on the page has to be in this family")
	ErrFamilyPagePhoto       = errors.New("Choose finished photos from this family's library")
	ErrFamilyPageEmpty       = errors.New("Add a title, an intro, a person or a photo before publishing")
	// One answer for unknown and unpublished slugs.
	ErrFamilyPageUnavailable = errors.New("This page isn't available")
)

func familyPageURL(slug string) string {
	return siteRoot() + FamilyPagePathPrefix + slug
}

func GetFamilyPage(tx *vbolt.Tx, familyId int) (page FamilyPage, found bool) {
	found = vbolt.Read(tx, FamilyPageBkt, familyId, &page)
	return
}

func findFamilyPageBySlug(tx *vbolt.Tx, slug string) (FamilyPage, bool) {
	var familyId int
	vbolt.Read(tx, FamilyPageBySlugBkt, slug, &familyId)
	if familyId == 0 {
		return FamilyPage{}, false
	}
	page, found := GetFamilyPage(tx, familyId)
	return page, found && page.Slug == slug
}

func writeFamilyPageTx(tx *vbolt.Tx, page *FamilyPage) {
	if old, found := GetFamilyPage(tx, page.FamilyId); found && old.Slug != page.Slug {
		vbolt.Delete(tx, FamilyPageBySlugBkt, old.Slug)
	}
	vbolt.Write(tx, FamilyPageBkt, page.FamilyId, page)
	vbolt.Write(tx, FamilyPageBySlugBkt, page.Slug, &page.FamilyId)
}

func deleteFamilyPageTx(tx *vbolt.Tx, familyId int) {
	page, found := GetFamilyPage(tx, familyId)
	if !found {
		return
	}
	vbolt.Delete(tx, FamilyPageBySlugBkt, page.Slug)
	vbolt.Delete(tx, FamilyPageBkt, familyId)
}

// FamilyPageSettings is what the editor shows and sends back to publish.
type FamilyPageSettings struct {
	Slug      string                     `json:"slug"`
	Indexable bool                       `json:"indexable"`
	Title     string                     `json:"title"`
	Intro     string                     `json:"intro"`
	Members   []FamilyPageMemberSettings `json:"members"`
	PhotoIds  []int                      `json:"photoIds"`
}

type FamilyPageMemberSettings struct {
	PersonId  int    `json:"personId"`
	Name      string `json:"name"`
	Line      string `json:"line"`
	ShowAge   bool   `json:"showAge"`
	ShowPhoto bool   `json:"showPhoto"`
}

type GetFamilyPageSettingsRequest struct {
	FamilyId int `json:"familyId"`
}

type FamilyPageSettingsResponse struct {
	FamilyId        int                `json:"familyId"`
	CanEdit         bool               `json:"canEdit"`
	Published       bool               `json:"published"`
	URL             string             `json:"url"`
	PublishedAt     *time.Time         `json:"publishedAt,omitempty"`
	PublishedByName string             `json:"publishedByName"`
	Settings        FamilyPageSettings `json:"settings"`
	People          []FamilyPagePerson `json:"people"`
}

// FamilyPagePerson is someone on the family's roster the editor can add.
type FamilyPagePerson struct {
	Id               int     `json:"id"`
	Name             string  `json:"name"`
	AgeYears         *int    `json:"ageYears,omitempty"`
	ProfilePhotoId   int     `json:"profilePhotoId"`
	ProfileCropX     float64 `json:"profileCropX"`
	ProfileCropY     float64 `json:"profileCropY"`
	ProfileCropScale float64 `json:"profileCropScale"`
}

type PublishFamilyPageRequest struct {
	FamilyId int                `json:"familyId"`
	Settings FamilyPageSettings `json:"settings"`
}

type UnpublishFamilyPageRequest struct {
	FamilyId int `json:"familyId"`
}

// GetPublicFamilyPageRequest is the public read behind /f/<slug>.
type GetPublicFamilyPageRequest struct {
	Slug string `json:"slug"`
}

type PublicFamilyPage struct {
	Title     string                   `json:"title"`
	Intro     string                   `json:"intro"`
	Members   []PublicFamilyPageMember `json:"members"`
	PhotoUrls []string                 `json:"photoUrls"`
}

type PublicFamilyPageMember struct {
	Name      string  `json:"name"`
	Line      string  `json:"line"`
	AgeYears  *int    `json:"ageYears,omitempty"`
	PhotoUrl  string  `json:"photoUrl"`
	CropX     float64 `json:"cropX"`
	CropY     float64 `json:"cropY"`
	CropScale float64 `json:"cropScale"`
}

type GetPublicFamilyPageResponse struct {
	Page PublicFamilyPage `json:"page"`
}
