package backend

import (
	"errors"
	"sort"
	"strings"
	"time"

	"go.hasen.dev/vbeam"
	"go.hasen.dev/vbolt"
)

func RegisterActivityResultMethods(app *vbeam.Application) {
	vbeam.RegisterProc(app, CreateAppearance)
	vbeam.RegisterProc(app, UpdateAppearance)
	vbeam.RegisterProc(app, DeleteAppearance)
	vbeam.RegisterProc(app, SetAppearanceResults)
}

var (
	ErrAppearanceNotFound     = errors.New("Performance not found")
	ErrEntryNotInSeason       = errors.New("That entry is not in the same season as this competition")
	ErrInvalidResultKind      = errors.New("A result must be an adjudication, placement, award, or score")
	ErrResultLabelRequired    = errors.New("An adjudication or award needs a label")
	ErrResultRankRequired     = errors.New("A placement needs a rank")
	ErrResultScoreRequired    = errors.New("A score result needs a score")
	ErrResultRankOutOfRange   = errors.New("A rank must be 1 or greater, and no greater than the field size")
	ErrResultPersonNotOnEntry = errors.New("A result can only name someone on this entry's roster")
	ErrTooManyResults         = errors.New("That is more results than one performance can hold")
)

const maxResultsPerAppearance = 50

type CreateAppearanceRequest struct {
	EventId    int     `json:"eventId"`
	EntryId    int     `json:"entryId"`
	OccurredAt *string `json:"occurredAt,omitempty"`
	Notes      string  `json:"notes"`
}

type UpdateAppearanceRequest struct {
	Id         int     `json:"id"`
	OccurredAt *string `json:"occurredAt,omitempty"`
	Notes      string  `json:"notes"`
}

type AppearanceIdRequest struct {
	Id int `json:"id"`
}

type AppearanceView struct {
	Appearance Appearance `json:"appearance"`
	Results    []Result   `json:"results"`
	PhotoIds   []int      `json:"photoIds"`
}

type AppearanceResponse struct {
	Appearance AppearanceView `json:"appearance"`
}

type SetAppearanceResultsRequest struct {
	AppearanceId int           `json:"appearanceId"`
	Results      []ResultInput `json:"results"`
}

type ResultInput struct {
	Kind     string   `json:"kind"`
	Label    string   `json:"label"`
	Rank     *int     `json:"rank,omitempty"`
	OutOf    *int     `json:"outOf,omitempty"`
	Category string   `json:"category"`
	Score    *float64 `json:"score,omitempty"`
	PersonId *int     `json:"personId,omitempty"`
	Notes    string   `json:"notes"`
}

func getAppearanceForUser(tx *vbolt.Tx, id int, user User, need AccessLevel) (Appearance, error) {
	appearance := GetAppearanceById(tx, id)
	if appearance.Id == 0 {
		return appearance, ErrAppearanceNotFound
	}
	if !canAccessAppearance(tx, user, appearance, need) {
		return Appearance{}, ErrAppearanceNotFound
	}
	return appearance, nil
}

func sortResults(results []Result) []Result {
	sort.Slice(results, func(i, j int) bool {
		if results[i].SortOrder != results[j].SortOrder {
			return results[i].SortOrder < results[j].SortOrder
		}
		return results[i].Id < results[j].Id
	})
	return results
}

func appearanceView(tx *vbolt.Tx, user User, appearance Appearance) AppearanceView {
	return AppearanceView{
		Appearance: appearance,
		Results:    sortResults(GetAppearanceResults(tx, appearance.Id)),
		PhotoIds:   visiblePhotoIds(tx, user, GetAppearancePhotoIds(tx, appearance.Id)),
	}
}

func CreateAppearance(ctx *vbeam.Context, req CreateAppearanceRequest) (resp AppearanceResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}

	occurredAt, err := parseActivityDate(req.OccurredAt)
	if err != nil {
		return
	}

	event, err := getEventForUser(ctx.Tx, req.EventId, user, AccessContribute)
	if err != nil {
		return
	}
	entry, err := getEntryForUser(ctx.Tx, req.EntryId, user, AccessContribute)
	if err != nil {
		return
	}
	if entry.SeasonId != event.SeasonId {
		err = ErrEntryNotInSeason
		return
	}

	vbeam.UseWriteTx(ctx)
	appearance := Appearance{
		Id:         vbolt.NextIntId(ctx.Tx, AppearanceBkt),
		EventId:    event.Id,
		EntryId:    entry.Id,
		FamilyId:   event.FamilyId,
		OccurredAt: occurredAt,
		Notes:      trimField(req.Notes, maxNotesLength),
		CreatedAt:  time.Now(),
	}
	writeAppearanceTx(ctx.Tx, &appearance)
	resp.Appearance = appearanceView(ctx.Tx, user, appearance)
	// Build the response before committing: TxCommit closes the tx, and reading a
	// closed one panics.
	vbolt.TxCommit(ctx.Tx)
	return
}

func UpdateAppearance(ctx *vbeam.Context, req UpdateAppearanceRequest) (resp AppearanceResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}

	occurredAt, err := parseActivityDate(req.OccurredAt)
	if err != nil {
		return
	}

	appearance, err := getAppearanceForUser(ctx.Tx, req.Id, user, AccessContribute)
	if err != nil {
		return
	}

	vbeam.UseWriteTx(ctx)
	appearance.OccurredAt = occurredAt
	appearance.Notes = trimField(req.Notes, maxNotesLength)
	writeAppearanceTx(ctx.Tx, &appearance)
	resp.Appearance = appearanceView(ctx.Tx, user, appearance)
	vbolt.TxCommit(ctx.Tx)
	return
}

func DeleteAppearance(ctx *vbeam.Context, req AppearanceIdRequest) (resp DeleteResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}

	appearance, err := getAppearanceForUser(ctx.Tx, req.Id, user, AccessContribute)
	if err != nil {
		return
	}

	vbeam.UseWriteTx(ctx)
	deleteAppearanceTx(ctx.Tx, appearance.Id)
	vbolt.TxCommit(ctx.Tx)

	resp.Success = true
	return
}

func normalizeResultKind(kind string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case ResultKindAdjudication:
		return ResultKindAdjudication, true
	case ResultKindPlacement:
		return ResultKindPlacement, true
	case ResultKindAward:
		return ResultKindAward, true
	case ResultKindScore:
		return ResultKindScore, true
	default:
		return "", false
	}
}

func validateResultInput(kind string, in ResultInput, label string) error {
	switch kind {
	case ResultKindAdjudication, ResultKindAward:
		if label == "" {
			return ErrResultLabelRequired
		}
	case ResultKindPlacement:
		if in.Rank == nil {
			return ErrResultRankRequired
		}
	case ResultKindScore:
		if in.Score == nil {
			return ErrResultScoreRequired
		}
	}
	if in.Rank != nil && *in.Rank < 1 {
		return ErrResultRankOutOfRange
	}
	if in.OutOf != nil && *in.OutOf < 1 {
		return ErrResultRankOutOfRange
	}
	if in.Rank != nil && in.OutOf != nil && *in.Rank > *in.OutOf {
		return ErrResultRankOutOfRange
	}
	return nil
}

func resultPersonId(in ResultInput, roster []int) (*int, error) {
	if in.PersonId == nil || *in.PersonId <= 0 {
		return nil, nil
	}
	for _, personId := range roster {
		if personId == *in.PersonId {
			return in.PersonId, nil
		}
	}
	return nil, ErrResultPersonNotOnEntry
}

func SetAppearanceResults(ctx *vbeam.Context, req SetAppearanceResultsRequest) (resp AppearanceResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}

	if len(req.Results) > maxResultsPerAppearance {
		err = ErrTooManyResults
		return
	}

	appearance, err := getAppearanceForUser(ctx.Tx, req.AppearanceId, user, AccessContribute)
	if err != nil {
		return
	}
	roster := GetEntryPersonIds(ctx.Tx, appearance.EntryId)

	prepared := make([]Result, 0, len(req.Results))
	for i, in := range req.Results {
		kind, ok := normalizeResultKind(in.Kind)
		if !ok {
			err = ErrInvalidResultKind
			return
		}
		label := trimField(in.Label, maxLabelLength)
		if err = validateResultInput(kind, in, label); err != nil {
			return
		}
		var personId *int
		if personId, err = resultPersonId(in, roster); err != nil {
			return
		}
		prepared = append(prepared, Result{
			AppearanceId: appearance.Id,
			FamilyId:     appearance.FamilyId,
			Kind:         kind,
			Label:        label,
			Rank:         in.Rank,
			OutOf:        in.OutOf,
			Category:     trimField(in.Category, maxLabelLength),
			Score:        in.Score,
			PersonId:     personId,
			Notes:        trimField(in.Notes, maxNotesLength),
			SortOrder:    i,
		})
	}

	vbeam.UseWriteTx(ctx)
	for _, existing := range GetAppearanceResults(ctx.Tx, appearance.Id) {
		deleteResultRowTx(ctx.Tx, existing.Id)
	}
	now := time.Now()
	for i := range prepared {
		prepared[i].Id = vbolt.NextIntId(ctx.Tx, ResultBkt)
		prepared[i].CreatedAt = now
		writeResultTx(ctx.Tx, &prepared[i])
	}
	resp.Appearance = appearanceView(ctx.Tx, user, appearance)
	vbolt.TxCommit(ctx.Tx)
	return
}
