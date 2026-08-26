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

const maxPhotosPerSubject = 200

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

func visiblePhotoIds(tx *vbolt.Tx, user User, photoIds []int) []int {
	visible := make([]int, 0, len(photoIds))
	for _, photoId := range photoIds {
		if CanAccessPhoto(tx, user, GetImageById(tx, photoId), AccessView) {
			visible = append(visible, photoId)
		}
	}
	return visible
}

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
