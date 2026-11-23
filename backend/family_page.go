package backend

import (
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode"

	"go.hasen.dev/vbeam"
	"go.hasen.dev/vbolt"
)

func RegisterFamilyPageMethods(app *vbeam.Application) {
	vbeam.RegisterProc(app, GetFamilyPageSettings)
	vbeam.RegisterProc(app, PublishFamilyPage)
	vbeam.RegisterProc(app, UnpublishFamilyPage)
	vbeam.RegisterProc(app, GetPublicFamilyPage)
	app.HandleFunc(FamilyPagePhotoPathPrefix, serveFamilyPagePhoto)
}

func capText(value string, limit int) string {
	value = strings.TrimSpace(value)
	if runes := []rune(value); len(runes) > limit {
		value = strings.TrimSpace(string(runes[:limit]))
	}
	return value
}

func normalizeFamilyPageSlug(slug string) string {
	return strings.ToLower(strings.TrimSpace(slug))
}

func validFamilyPageSlug(slug string) bool {
	return len(slug) >= minFamilyPageSlug && len(slug) <= maxFamilyPageSlug &&
		familyPageSlugPattern.MatchString(slug) && !reservedFamilyPageSlugs[slug]
}

func suggestFamilyPageSlug(name string) string {
	var b strings.Builder
	hyphen := false
	for _, r := range strings.ToLower(name) {
		switch {
		case r < unicode.MaxASCII && (unicode.IsLetter(r) || unicode.IsDigit(r)):
			b.WriteRune(r)
			hyphen = false
		case b.Len() > 0 && !hyphen:
			b.WriteByte('-')
			hyphen = true
		}
	}
	slug := strings.Trim(b.String(), "-")
	if len(slug) > maxFamilyPageSlug {
		slug = strings.Trim(slug[:maxFamilyPageSlug], "-")
	}
	return slug
}

func familyPagePhotoOk(tx *vbolt.Tx, familyId int, imageId int) bool {
	if imageId <= 0 {
		return false
	}
	image := GetImageById(tx, imageId)
	return image.Id != 0 && image.FamilyId == familyId && image.Status == PhotoReady && !image.IsVideo()
}

func familyPageAge(birthday, now time.Time) *int {
	if birthday.IsZero() {
		return nil
	}
	born, today := birthday.UTC(), now.UTC()
	if today.Before(born) {
		return nil
	}
	years := today.Year() - born.Year()
	if today.Month() < born.Month() || (today.Month() == born.Month() && today.Day() < born.Day()) {
		years--
	}
	return &years
}

func resolveFamilyPageFamily(user User, familyId int) int {
	if familyId == 0 {
		return user.FamilyId
	}
	return familyId
}

func buildFamilyPage(tx *vbolt.Tx, familyId int, in FamilyPageSettings, now time.Time) (FamilyPage, error) {
	page := FamilyPage{
		FamilyId:  familyId,
		Slug:      normalizeFamilyPageSlug(in.Slug),
		Indexable: in.Indexable,
		Title:     capText(in.Title, maxFamilyPageTitle),
		Intro:     capText(in.Intro, maxFamilyPageIntro),
	}
	if !validFamilyPageSlug(page.Slug) {
		return FamilyPage{}, ErrFamilyPageSlugInvalid
	}
	if taken, found := findFamilyPageBySlug(tx, page.Slug); found && taken.FamilyId != familyId {
		return FamilyPage{}, ErrFamilyPageSlugTaken
	}
	if page.Title == "" {
		page.Title = GetFamily(tx, familyId).Name
	}
	if len(in.Members) > maxFamilyPageMembers || len(in.PhotoIds) > maxFamilyPagePhotos {
		return FamilyPage{}, ErrFamilyPageTooLarge
	}

	seenPeople := make(map[int]bool)
	for _, memberIn := range in.Members {
		if _, onRoster := FindPersonFamily(tx, memberIn.PersonId, familyId); !onRoster || seenPeople[memberIn.PersonId] {
			return FamilyPage{}, ErrFamilyPagePerson
		}
		seenPeople[memberIn.PersonId] = true
		person := GetPersonById(tx, memberIn.PersonId)
		if person.Id == 0 {
			return FamilyPage{}, ErrFamilyPagePerson
		}

		member := FamilyPageMember{
			PersonId: person.Id,
			Name:     capText(memberIn.Name, maxFamilyPageName),
			Line:     capText(memberIn.Line, maxFamilyPageLine),
		}
		if member.Name == "" {
			member.Name = firstName(person.Name)
		}
		if memberIn.ShowAge && !person.IsPregnancy && familyPageAge(person.Birthday, now) != nil {
			member.Birthday = person.Birthday
		}
		if memberIn.ShowPhoto && familyPagePhotoOk(tx, familyId, person.ProfilePhotoId) {
			member.PhotoId = person.ProfilePhotoId
			member.CropX = person.ProfileCropX
			member.CropY = person.ProfileCropY
			member.CropScale = person.ProfileCropScale
		}
		page.Members = append(page.Members, member)
	}

	seenPhotos := make(map[int]bool)
	for _, photoId := range in.PhotoIds {
		if seenPhotos[photoId] || !familyPagePhotoOk(tx, familyId, photoId) {
			return FamilyPage{}, ErrFamilyPagePhoto
		}
		seenPhotos[photoId] = true
		page.PhotoIds = append(page.PhotoIds, photoId)
	}

	if page.Intro == "" && len(page.Members) == 0 && len(page.PhotoIds) == 0 {
		return FamilyPage{}, ErrFamilyPageEmpty
	}
	return page, nil
}

func familyPageSettingsView(tx *vbolt.Tx, user User, familyId int) FamilyPageSettingsResponse {
	resp := FamilyPageSettingsResponse{
		FamilyId: familyId,
		CanEdit:  CanAccessFamily(tx, user, familyId, AccessAdmin),
		Settings: FamilyPageSettings{Members: []FamilyPageMemberSettings{}, PhotoIds: []int{}},
		People:   []FamilyPagePerson{},
	}
	now := time.Now()
	for _, person := range GetFamilyPeople(tx, familyId) {
		candidate := FamilyPagePerson{
			Id:   person.Id,
			Name: person.Name,
		}
		if !person.IsPregnancy {
			candidate.AgeYears = familyPageAge(person.Birthday, now)
		}
		if familyPagePhotoOk(tx, familyId, person.ProfilePhotoId) {
			candidate.ProfilePhotoId = person.ProfilePhotoId
			candidate.ProfileCropX = person.ProfileCropX
			candidate.ProfileCropY = person.ProfileCropY
			candidate.ProfileCropScale = person.ProfileCropScale
		}
		resp.People = append(resp.People, candidate)
	}
	page, found := GetFamilyPage(tx, familyId)
	if !found {
		name := GetFamily(tx, familyId).Name
		resp.Settings.Title = name
		if slug := suggestFamilyPageSlug(name); validFamilyPageSlug(slug) {
			if _, taken := findFamilyPageBySlug(tx, slug); !taken {
				resp.Settings.Slug = slug
			}
		}
		return resp
	}

	resp.Published = true
	resp.URL = familyPageURL(page.Slug)
	publishedAt := page.PublishedAt
	resp.PublishedAt = &publishedAt
	if publisher := GetUser(tx, page.PublishedBy); publisher.Id != 0 {
		resp.PublishedByName = publisher.Name
	}
	resp.Settings.Slug = page.Slug
	resp.Settings.Indexable = page.Indexable
	resp.Settings.Title = page.Title
	resp.Settings.Intro = page.Intro
	for _, member := range page.Members {
		resp.Settings.Members = append(resp.Settings.Members, FamilyPageMemberSettings{
			PersonId:  member.PersonId,
			Name:      member.Name,
			Line:      member.Line,
			ShowAge:   !member.Birthday.IsZero(),
			ShowPhoto: member.PhotoId != 0,
		})
	}
	resp.Settings.PhotoIds = append(resp.Settings.PhotoIds, page.PhotoIds...)
	return resp
}

func GetFamilyPageSettings(ctx *vbeam.Context, req GetFamilyPageSettingsRequest) (resp FamilyPageSettingsResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}
	familyId := resolveFamilyPageFamily(user, req.FamilyId)
	if !CanAccessFamily(ctx.Tx, user, familyId, AccessContribute) {
		err = ErrFamilyAccessDenied
		return
	}
	resp = familyPageSettingsView(ctx.Tx, user, familyId)
	return
}

func PublishFamilyPage(ctx *vbeam.Context, req PublishFamilyPageRequest) (resp FamilyPageSettingsResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}
	familyId := resolveFamilyPageFamily(user, req.FamilyId)
	if !CanAccessFamily(ctx.Tx, user, familyId, AccessAdmin) {
		err = ErrFamilyPageNotAllowed
		return
	}

	now := time.Now()
	page, err := buildFamilyPage(ctx.Tx, familyId, req.Settings, now)
	if err != nil {
		return
	}
	page.PublishedBy = user.Id
	page.PublishedAt = now

	vbeam.UseWriteTx(ctx)
	writeFamilyPageTx(ctx.Tx, &page)
	resp = familyPageSettingsView(ctx.Tx, user, familyId)
	vbolt.TxCommit(ctx.Tx)
	return
}

func UnpublishFamilyPage(ctx *vbeam.Context, req UnpublishFamilyPageRequest) (resp FamilyPageSettingsResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}
	familyId := resolveFamilyPageFamily(user, req.FamilyId)
	if !CanAccessFamily(ctx.Tx, user, familyId, AccessAdmin) {
		err = ErrFamilyPageNotAllowed
		return
	}

	vbeam.UseWriteTx(ctx)
	deleteFamilyPageTx(ctx.Tx, familyId)
	resp = familyPageSettingsView(ctx.Tx, user, familyId)
	vbolt.TxCommit(ctx.Tx)
	return
}

func familyPageMemberLive(tx *vbolt.Tx, page FamilyPage, member FamilyPageMember) bool {
	_, onRoster := FindPersonFamily(tx, member.PersonId, page.FamilyId)
	return onRoster
}

func familyPageAssetURL(slug, kind string, index int) string {
	return fmt.Sprintf("%s%s/%s/%d", FamilyPagePhotoPathPrefix, slug, kind, index)
}

func GetPublicFamilyPage(ctx *vbeam.Context, req GetPublicFamilyPageRequest) (resp GetPublicFamilyPageResponse, err error) {
	page, found := findFamilyPageBySlug(ctx.Tx, normalizeFamilyPageSlug(req.Slug))
	if !found {
		err = ErrFamilyPageUnavailable
		return
	}

	now := time.Now()
	public := PublicFamilyPage{
		Title:     page.Title,
		Intro:     page.Intro,
		Members:   []PublicFamilyPageMember{},
		PhotoUrls: []string{},
	}
	for i, member := range page.Members {
		if !familyPageMemberLive(ctx.Tx, page, member) {
			continue
		}
		out := PublicFamilyPageMember{
			Name:     member.Name,
			Line:     member.Line,
			AgeYears: familyPageAge(member.Birthday, now),
		}
		if familyPagePhotoOk(ctx.Tx, page.FamilyId, member.PhotoId) {
			out.PhotoUrl = familyPageAssetURL(page.Slug, "person", i)
			out.CropX = member.CropX
			out.CropY = member.CropY
			out.CropScale = member.CropScale
		}
		public.Members = append(public.Members, out)
	}
	for i, photoId := range page.PhotoIds {
		if familyPagePhotoOk(ctx.Tx, page.FamilyId, photoId) {
			public.PhotoUrls = append(public.PhotoUrls, familyPageAssetURL(page.Slug, "photo", i))
		}
	}
	resp.Page = public
	return
}

func familyPageAsset(tx *vbolt.Tx, slug, kind, indexText string) (image Image, size ImageSize, page FamilyPage, ok bool) {
	page, found := findFamilyPageBySlug(tx, slug)
	if !found {
		return
	}
	index, err := strconv.Atoi(indexText)
	if err != nil || index < 0 || strconv.Itoa(index) != indexText {
		return
	}
	photoId := 0
	switch kind {
	case "photo":
		if index < len(page.PhotoIds) {
			photoId, size = page.PhotoIds[index], XLargeSize
		}
	case "person":
		if index < len(page.Members) && familyPageMemberLive(tx, page, page.Members[index]) {
			photoId, size = page.Members[index].PhotoId, MediumSize
		}
	}
	if !familyPagePhotoOk(tx, page.FamilyId, photoId) {
		return
	}
	return GetImageById(tx, photoId), size, page, true
}

// serveFamilyPagePhoto serves only re-encoded variants, never an original, so
// no camera metadata reaches the public page.
func serveFamilyPagePhoto(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, FamilyPagePhotoPathPrefix), "/")
	if len(parts) != 3 {
		http.NotFound(w, r)
		return
	}

	var image Image
	var size ImageSize
	var page FamilyPage
	var ok bool
	vbolt.WithReadTx(appDb, func(tx *vbolt.Tx) {
		image, size, page, ok = familyPageAsset(tx, normalizeFamilyPageSlug(parts[0]), parts[1], parts[2])
	})
	setFamilyPageHeaders(w, page.Indexable)
	if !ok {
		http.NotFound(w, r)
		return
	}

	for _, format := range []string{GetOptimalImageFormat(r.Header.Get("Accept")), "jpeg"} {
		path := photoVariantPath(image.FilePath, size.Name, format)
		if _, err := os.Stat(path); err == nil {
			_, contentType := photoFormatFile(format)
			w.Header().Set("Content-Type", contentType)
			w.Header().Set("Vary", "Accept")
			http.ServeFile(w, r, path)
			return
		}
	}
	http.NotFound(w, r)
}

func setFamilyPageHeaders(w http.ResponseWriter, indexable bool) {
	h := w.Header()
	h.Set("Cache-Control", "no-store")
	h.Set("Referrer-Policy", "no-referrer")
	if !indexable {
		h.Set("X-Robots-Tag", "noindex, nofollow, noarchive")
	}
}

// addFamilyPageHeaders covers the app shell served at /f/<slug>. The shell
// holds no page; GetPublicFamilyPage checks the publication on every load.
func addFamilyPageHeaders(w http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(r.URL.Path, FamilyPagePathPrefix) {
		return
	}
	indexable := false
	if appDb != nil {
		slug := normalizeFamilyPageSlug(strings.TrimPrefix(r.URL.Path, FamilyPagePathPrefix))
		vbolt.WithReadTx(appDb, func(tx *vbolt.Tx) {
			page, found := findFamilyPageBySlug(tx, slug)
			indexable = found && page.Indexable
		})
	}
	setFamilyPageHeaders(w, indexable)
}
