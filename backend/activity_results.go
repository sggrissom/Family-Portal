// Appearances and results: a routine at a competition, and how it did there.
//
// Appearance is the hinge of the whole schema — one Entry at one Event — so
// these procs are what turn the structure phase 2 built into a season anybody
// would want to look at. Results hang off an appearance and are always edited as
// a set, because that is how they arrive: one results sheet, read off in one
// sitting. See docs/activities-plan.md.
//
// Access follows the entry, not the event. An appearance carries no roster of
// its own, so canAccessAppearance resolves to the entry that names the people —
// which is what lets a linked household read the group routine its child is in.
// Writes ask for AccessContribute, and a link is capped at AccessView, so every
// mutation here stays membership-only.
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

// maxResultsPerAppearance is a sanity bound rather than a domain limit. A
// routine collects an adjudication, a placement or two, and a handful of special
// awards; anything past this is a client bug or a paste accident, and the write
// is unbounded without it.
const maxResultsPerAppearance = 50

// ── appearances ───────────────────────────────────────────────────────────────

type CreateAppearanceRequest struct {
	EventId    int     `json:"eventId"`
	EntryId    int     `json:"entryId"`
	OccurredAt *string `json:"occurredAt,omitempty"` // YYYY-MM-DD; absent means "sometime that weekend"
	Notes      string  `json:"notes"`
}

// UpdateAppearanceRequest deliberately cannot move an appearance to a different
// event or entry. Which routine performed at which competition is the identity
// of the record, not a field on it; a misfiled appearance is deleted and
// re-entered, which also makes it obvious that its results went with it.
type UpdateAppearanceRequest struct {
	Id         int     `json:"id"`
	OccurredAt *string `json:"occurredAt,omitempty"`
	Notes      string  `json:"notes"`
}

type AppearanceIdRequest struct {
	Id int `json:"id"`
}

// AppearanceView is an appearance with its results, which is the only useful
// shape: an appearance on its own says a routine turned up and nothing about
// how it went.
type AppearanceView struct {
	Appearance Appearance `json:"appearance"`
	Results    []Result   `json:"results"`
}

type AppearanceResponse struct {
	Appearance AppearanceView `json:"appearance"`
}

type SetAppearanceResultsRequest struct {
	AppearanceId int           `json:"appearanceId"`
	Results      []ResultInput `json:"results"`
}

// ResultInput is a Result without the fields the server owns. SortOrder is
// absent on purpose — position in the array is the order, so reordering a
// results sheet is a reordered array rather than a second field to keep in sync.
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

// sortResults puts results in the order the client asked for, falling back to id
// so a set written before SortOrder mattered still comes back stably.
func sortResults(results []Result) []Result {
	sort.Slice(results, func(i, j int) bool {
		if results[i].SortOrder != results[j].SortOrder {
			return results[i].SortOrder < results[j].SortOrder
		}
		return results[i].Id < results[j].Id
	})
	return results
}

func appearanceView(tx *vbolt.Tx, appearance Appearance) AppearanceView {
	return AppearanceView{
		Appearance: appearance,
		Results:    sortResults(GetAppearanceResults(tx, appearance.Id)),
	}
}

// CreateAppearance takes both parents by id and checks that they agree. The
// event names the season and the entry names the season, so an entry from
// another season — or another family — is rejected here rather than becoming a
// row that neither the competition view nor the routine view can explain.
//
// Two appearances of the same entry at the same event are allowed. A routine
// that dances in its category and again in the overall round is two
// performances with two sets of results, and collapsing them would lose the
// second.
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
	// The view is built before the commit: TxCommit closes the transaction, and
	// reading a closed one panics rather than returning stale data.
	resp.Appearance = appearanceView(ctx.Tx, appearance)
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
	resp.Appearance = appearanceView(ctx.Tx, appearance)
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

// ── results ───────────────────────────────────────────────────────────────────

// normalizeResultKind rejects what it does not recognize, unlike
// normalizeActivityKind, which degrades to generic. Activity.Kind only picks
// vocabulary, so guessing wrong is a cosmetic problem; Result.Kind decides which
// fields carry the meaning, so a typo would file a placement under a label
// nothing reads and silently lose it.
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

// validateResultInput enforces that each kind carries the field it exists for.
// The rest stays free text — competitions do not agree on what a level or an
// adjudication is called, and the plan is explicit that nothing here normalizes
// those. What it will not accept is a result with no content at all, which is
// what an accidental extra row in a form looks like.
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

// resultPersonId resolves the optional person a result names. It must be someone
// on the entry's roster: PersonId narrows an award to one dancer inside a group,
// and without this check any person id at all would land in ResultByPersonIndex,
// where another family's "a kid's individual awards" view would find it.
//
// A zero or negative id is read as "names nobody" rather than an error, so a
// form that sends 0 for an unset select does the harmless thing.
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

// SetAppearanceResults replaces the whole set. Results arrive together off one
// sheet and are entered together, so replace-all is the honest shape — and
// per-result CRUD would triple the proc count to let somebody edit a placement
// without looking at the adjudication next to it.
//
// The existing rows are deleted rather than reused, so ids and CreatedAt change
// on every edit. Nothing holds a Result id — photos attach to appearances, not
// results — so there is nothing for the churn to break, and matching old rows to
// new ones would need an identity the input does not have.
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

	// Everything is validated before anything is written, so a bad row partway
	// down a results sheet leaves the appearance holding what it held before
	// rather than half a new set.
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
	resp.Appearance = appearanceView(ctx.Tx, appearance)
	vbolt.TxCommit(ctx.Tx)
	return
}
