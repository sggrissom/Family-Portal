// Photos on performances and competitions.
//
// Two join tables rather than one polymorphic table keyed by (subjectKind,
// subjectId): vbolt indexes are typed term→target pairs, so a composite term
// would have to be encoded by hand and every read would need a kind filter. Two
// tables is more lines and less cleverness. See docs/activities-plan.md.
//
// The split is a real one, not an artifact. AppearancePhoto is a photo of one
// routine at one competition and travels with that performance; EventPhoto is
// the rest of the weekend — the venue, the group in the lobby — and belongs to
// the competition itself.
package backend

import (
	"errors"
	"time"

	"go.hasen.dev/vbeam"
	"go.hasen.dev/vbolt"
)

func RegisterActivityPhotoMethods(app *vbeam.Application) {
	vbeam.RegisterProc(app, SetAppearancePhotos)
	vbeam.RegisterProc(app, SetEventPhotos)
}

var ErrTooManyPhotos = errors.New("That is more photos than one record can hold")

// maxPhotosPerSubject is a sanity bound on a single request, not a limit on how
// many photos a weekend produces — those live in the photo library and a
// competition can point at as many batches as it likes across calls.
const maxPhotosPerSubject = 200

// ── reads ─────────────────────────────────────────────────────────────────────

func GetAppearancePhotoIds(tx *vbolt.Tx, appearanceId int) []int {
	joins := GetAppearancePhotoJoins(tx, appearanceId)
	photoIds := make([]int, 0, len(joins))
	for _, join := range joins {
		photoIds = append(photoIds, join.PhotoId)
	}
	return photoIds
}

func GetEventPhotoIds(tx *vbolt.Tx, eventId int) []int {
	joins := GetEventPhotoJoins(tx, eventId)
	photoIds := make([]int, 0, len(joins))
	for _, join := range joins {
		photoIds = append(photoIds, join.PhotoId)
	}
	return photoIds
}

// visiblePhotoIds drops the ids this caller could not fetch anyway.
//
// A photo is owned by a family and reachable through a link only if someone
// tagged in it is shared under ScopePhotos, which is a stricter test than the
// one that got the caller to the performance. Without this filter a linked
// household would receive ids for photos it cannot load — broken images rather
// than a leak, but a photo id it can do nothing with is still not its to have.
func visiblePhotoIds(tx *vbolt.Tx, user User, photoIds []int) []int {
	visible := make([]int, 0, len(photoIds))
	for _, photoId := range photoIds {
		if CanAccessPhoto(tx, user, GetImageById(tx, photoId), AccessView) {
			visible = append(visible, photoId)
		}
	}
	return visible
}

// ── writes ────────────────────────────────────────────────────────────────────

type SetAppearancePhotosRequest struct {
	AppearanceId int   `json:"appearanceId"`
	PhotoIds     []int `json:"photoIds"`
}

type SetEventPhotosRequest struct {
	EventId  int   `json:"eventId"`
	PhotoIds []int `json:"photoIds"`
}

type SetEventPhotosResponse struct {
	EventId  int   `json:"eventId"`
	PhotoIds []int `json:"photoIds"`
}

// resolveActivityPhotoIds reuses milestone.go's normalizePhotoIds for the
// dedupe-and-drop-junk half, then checks every survivor against the owning
// family. Attaching a photo is not a way to reach one: the caller must already
// be able to contribute to the family that owns it, which is the same bar
// milestones set.
//
// The whole list is checked before anything is written, so one unreachable id
// leaves the existing set alone rather than half-replacing it.
func resolveActivityPhotoIds(tx *vbolt.Tx, photoIds []int, familyId int) ([]int, error) {
	if len(photoIds) > maxPhotosPerSubject {
		return nil, ErrTooManyPhotos
	}
	ordered := normalizePhotoIds(photoIds)
	for _, photoId := range ordered {
		if err := validatePhotoAccess(tx, photoId, familyId); err != nil {
			return nil, err
		}
	}
	return ordered, nil
}

// setAppearancePhotosTx replaces the whole set, matching every other set-* proc
// in this feature. The existing joins are dropped and rewritten in the order
// asked for, so the ids ascend the way the caller listed them and a read comes
// back in that order. A join row holds nothing but the pair, so there is nothing
// for the churn to lose.
func setAppearancePhotosTx(tx *vbolt.Tx, appearance Appearance, photoIds []int) error {
	ordered, err := resolveActivityPhotoIds(tx, photoIds, appearance.FamilyId)
	if err != nil {
		return err
	}
	for _, join := range GetAppearancePhotoJoins(tx, appearance.Id) {
		deleteAppearancePhotoRowTx(tx, join.Id)
	}
	now := time.Now()
	for _, photoId := range ordered {
		join := AppearancePhoto{
			Id:           vbolt.NextIntId(tx, AppearancePhotoBkt),
			AppearanceId: appearance.Id,
			PhotoId:      photoId,
			FamilyId:     appearance.FamilyId,
			CreatedAt:    now,
		}
		writeAppearancePhotoTx(tx, &join)
	}
	return nil
}

func setEventPhotosTx(tx *vbolt.Tx, event Event, photoIds []int) error {
	ordered, err := resolveActivityPhotoIds(tx, photoIds, event.FamilyId)
	if err != nil {
		return err
	}
	for _, join := range GetEventPhotoJoins(tx, event.Id) {
		deleteEventPhotoRowTx(tx, join.Id)
	}
	now := time.Now()
	for _, photoId := range ordered {
		join := EventPhoto{
			Id:        vbolt.NextIntId(tx, EventPhotoBkt),
			EventId:   event.Id,
			PhotoId:   photoId,
			FamilyId:  event.FamilyId,
			CreatedAt: now,
		}
		writeEventPhotoTx(tx, &join)
	}
	return nil
}

func SetAppearancePhotos(ctx *vbeam.Context, req SetAppearancePhotosRequest) (resp AppearanceResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}

	appearance, err := getAppearanceForUser(ctx.Tx, req.AppearanceId, user, AccessContribute)
	if err != nil {
		return
	}

	vbeam.UseWriteTx(ctx)
	if err = setAppearancePhotosTx(ctx.Tx, appearance, req.PhotoIds); err != nil {
		return
	}
	resp.Appearance = appearanceView(ctx.Tx, user, appearance)
	vbolt.TxCommit(ctx.Tx)
	return
}

func SetEventPhotos(ctx *vbeam.Context, req SetEventPhotosRequest) (resp SetEventPhotosResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}

	event, err := getEventForUser(ctx.Tx, req.EventId, user, AccessContribute)
	if err != nil {
		return
	}

	vbeam.UseWriteTx(ctx)
	if err = setEventPhotosTx(ctx.Tx, event, req.PhotoIds); err != nil {
		return
	}
	resp.EventId = event.Id
	resp.PhotoIds = GetEventPhotoIds(ctx.Tx, event.Id)
	vbolt.TxCommit(ctx.Tx)
	return
}

// ── photo deletion ────────────────────────────────────────────────────────────

// removePhotoFromActivities clears both join tables when a photo goes, the same
// way removePhotoFromMilestones does. It is called from deletePhotoRecordTx, and
// the by-photo indexes exist for exactly this — a join whose photo is gone is a
// row nothing can reach and nothing will ever clean up.
func removePhotoFromActivities(tx *vbolt.Tx, photoId int) {
	var joinIds []int

	vbolt.ReadTermTargets(tx, AppearancePhotoByPhotoIndex, photoId, &joinIds, vbolt.Window{})
	for _, joinId := range joinIds {
		deleteAppearancePhotoRowTx(tx, joinId)
	}

	joinIds = joinIds[:0]
	vbolt.ReadTermTargets(tx, EventPhotoByPhotoIndex, photoId, &joinIds, vbolt.Window{})
	for _, joinId := range joinIds {
		deleteEventPhotoRowTx(tx, joinId)
	}
}
