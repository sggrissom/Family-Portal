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
	vbeam.RegisterProc(app, SaveAppearance)
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
	ErrInvalidOutcome         = errors.New("An outcome must be a win, a loss, or a draw")
	ErrMatchupScoreOutOfRange = errors.New("A score must be between 0 and 9999")
)

const (
	maxResultsPerAppearance = 50
	maxMatchupScore         = 9999
)

type CreateAppearanceRequest struct {
	EventId    int           `json:"eventId"`
	EntryId    int           `json:"entryId"`
	OccurredAt *string       `json:"occurredAt,omitempty"`
	Notes      string        `json:"notes"`
	Matchup    *MatchupInput `json:"matchup,omitempty"`
}

type UpdateAppearanceRequest struct {
	Id         int           `json:"id"`
	OccurredAt *string       `json:"occurredAt,omitempty"`
	Notes      string        `json:"notes"`
	Matchup    *MatchupInput `json:"matchup,omitempty"`
}

type MatchupInput struct {
	Round        string `json:"round"`
	Opponent     string `json:"opponent"`
	Outcome      string `json:"outcome"`
	ScoreFor     *int   `json:"scoreFor,omitempty"`
	ScoreAgainst *int   `json:"scoreAgainst,omitempty"`
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

func normalizeOutcome(outcome string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(outcome)) {
	case "":
		return "", true
	case OutcomeWin:
		return OutcomeWin, true
	case OutcomeLoss:
		return OutcomeLoss, true
	case OutcomeDraw:
		return OutcomeDraw, true
	default:
		return "", false
	}
}

func validMatchupScore(score *int) bool {
	return score == nil || (*score >= 0 && *score <= maxMatchupScore)
}

func applyMatchup(appearance *Appearance, in *MatchupInput) error {
	if in == nil {
		return nil
	}
	outcome, ok := normalizeOutcome(in.Outcome)
	if !ok {
		return ErrInvalidOutcome
	}
	if !validMatchupScore(in.ScoreFor) || !validMatchupScore(in.ScoreAgainst) {
		return ErrMatchupScoreOutOfRange
	}
	appearance.Round = trimField(in.Round, maxLabelLength)
	appearance.Opponent = trimField(in.Opponent, maxLabelLength)
	appearance.Outcome = outcome
	appearance.ScoreFor = in.ScoreFor
	appearance.ScoreAgainst = in.ScoreAgainst
	return nil
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

	appearance := Appearance{
		EventId:    event.Id,
		EntryId:    entry.Id,
		FamilyId:   event.FamilyId,
		OccurredAt: occurredAt,
		Notes:      trimField(req.Notes, maxNotesLength),
		CreatedAt:  time.Now(),
	}
	if err = applyMatchup(&appearance, req.Matchup); err != nil {
		return
	}

	vbeam.UseWriteTx(ctx)
	appearance.Id = vbolt.NextIntId(ctx.Tx, AppearanceBkt)
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

	if err = applyMatchup(&appearance, req.Matchup); err != nil {
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

func prepareResults(inputs []ResultInput, familyId int, roster []int) ([]Result, error) {
	if len(inputs) > maxResultsPerAppearance {
		return nil, ErrTooManyResults
	}
	prepared := make([]Result, 0, len(inputs))
	for i, in := range inputs {
		kind, ok := normalizeResultKind(in.Kind)
		if !ok {
			return nil, ErrInvalidResultKind
		}
		label := trimField(in.Label, maxLabelLength)
		if err := validateResultInput(kind, in, label); err != nil {
			return nil, err
		}
		personId, err := resultPersonId(in, roster)
		if err != nil {
			return nil, err
		}
		prepared = append(prepared, Result{
			FamilyId:  familyId,
			Kind:      kind,
			Label:     label,
			Rank:      in.Rank,
			OutOf:     in.OutOf,
			Category:  trimField(in.Category, maxLabelLength),
			Score:     in.Score,
			PersonId:  personId,
			Notes:     trimField(in.Notes, maxNotesLength),
			SortOrder: i,
		})
	}
	return prepared, nil
}

func replaceResultsTx(tx *vbolt.Tx, appearanceId int, prepared []Result) {
	for _, existing := range GetAppearanceResults(tx, appearanceId) {
		deleteResultRowTx(tx, existing.Id)
	}
	now := time.Now()
	for i := range prepared {
		prepared[i].Id = vbolt.NextIntId(tx, ResultBkt)
		prepared[i].AppearanceId = appearanceId
		prepared[i].CreatedAt = now
		writeResultTx(tx, &prepared[i])
	}
}

func SetAppearanceResults(ctx *vbeam.Context, req SetAppearanceResultsRequest) (resp AppearanceResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}

	appearance, err := getAppearanceForUser(ctx.Tx, req.AppearanceId, user, AccessContribute)
	if err != nil {
		return
	}
	roster := GetEntryPersonIds(ctx.Tx, appearance.EntryId)

	prepared, err := prepareResults(req.Results, appearance.FamilyId, roster)
	if err != nil {
		return
	}

	vbeam.UseWriteTx(ctx)
	replaceResultsTx(ctx.Tx, appearance.Id, prepared)
	resp.Appearance = appearanceView(ctx.Tx, user, appearance)
	vbolt.TxCommit(ctx.Tx)
	return
}

type SaveAppearanceRequest struct {
	Id         int           `json:"id"`
	EventId    int           `json:"eventId"`
	EntryId    int           `json:"entryId"`
	OccurredAt *string       `json:"occurredAt,omitempty"`
	Notes      string        `json:"notes"`
	Matchup    *MatchupInput `json:"matchup,omitempty"`
	Results    []ResultInput `json:"results"`
	PhotoIds   []int         `json:"photoIds"`
}

// SaveAppearance writes a whole performance in one transaction: the appearance
// itself (created when Id is 0), its full results sheet, and its photos.
func SaveAppearance(ctx *vbeam.Context, req SaveAppearanceRequest) (resp AppearanceResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}

	occurredAt, err := parseActivityDate(req.OccurredAt)
	if err != nil {
		return
	}

	var appearance Appearance
	if req.Id != 0 {
		if appearance, err = getAppearanceForUser(ctx.Tx, req.Id, user, AccessContribute); err != nil {
			return
		}
	} else {
		var event Event
		var entry Entry
		if event, err = getEventForUser(ctx.Tx, req.EventId, user, AccessContribute); err != nil {
			return
		}
		if entry, err = getEntryForUser(ctx.Tx, req.EntryId, user, AccessContribute); err != nil {
			return
		}
		if entry.SeasonId != event.SeasonId {
			err = ErrEntryNotInSeason
			return
		}
		appearance = Appearance{
			EventId:   event.Id,
			EntryId:   entry.Id,
			FamilyId:  event.FamilyId,
			CreatedAt: time.Now(),
		}
	}

	appearance.OccurredAt = occurredAt
	appearance.Notes = trimField(req.Notes, maxNotesLength)
	if err = applyMatchup(&appearance, req.Matchup); err != nil {
		return
	}

	roster := GetEntryPersonIds(ctx.Tx, appearance.EntryId)
	prepared, err := prepareResults(req.Results, appearance.FamilyId, roster)
	if err != nil {
		return
	}
	if _, err = resolveActivityPhotoIds(ctx.Tx, req.PhotoIds, appearance.FamilyId); err != nil {
		return
	}

	vbeam.UseWriteTx(ctx)
	if appearance.Id == 0 {
		appearance.Id = vbolt.NextIntId(ctx.Tx, AppearanceBkt)
	}
	writeAppearanceTx(ctx.Tx, &appearance)
	replaceResultsTx(ctx.Tx, appearance.Id, prepared)
	if err = setAppearancePhotosTx(ctx.Tx, appearance, req.PhotoIds); err != nil {
		return
	}
	resp.Appearance = appearanceView(ctx.Tx, user, appearance)
	vbolt.TxCommit(ctx.Tx)
	return
}
