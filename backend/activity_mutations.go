package backend

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"family/cfg"

	"go.hasen.dev/vbeam"
	"go.hasen.dev/vbolt"
	"go.hasen.dev/vpack"
)

// Writes a phone can queue while offline and replay later, possibly more than
// once and possibly long after the website has changed the same performance.
//
// Each request carries a client-chosen mutation id. The first time one is
// applied, a receipt is written in the same transaction as the change; a retry
// with the same id (a lost response, an app killed mid-request) answers from
// the receipt instead of writing again, so a create cannot happen twice and an
// old edit cannot be reapplied over a newer one. A receipt is bound to the
// request's contents: the same id with a different payload is refused.
//
// Edits and deletes name the revision they were made against and are refused
// with a "conflict" outcome, carrying the server's current copy, when the
// performance has moved on. Every refusal the phone has to show someone comes
// back as an outcome in a successful response rather than as an error string,
// so the app never has to match message text. Errors are left for what the
// phone should simply retry later, or for a malformed request.
//
// Receipts are kept for as long as the account and family exist: a phone may
// stay offline indefinitely, and an expired receipt would turn its retry into a
// second create.

func RegisterActivityMutationMethods(app *vbeam.Application) {
	vbeam.RegisterProc(app, GetActivitySyncSupport)
	vbeam.RegisterProc(app, SyncSaveAppearance)
	vbeam.RegisterProc(app, SyncDeleteAppearance)
}

// ActivityMutationVersion is the contract above. A client sends queued writes
// only to a server reporting the version it was built for; an older server has
// no GetActivitySyncSupport at all, and the client must keep its queue rather
// than fall back to SaveAppearance.
const ActivityMutationVersion = 1

const (
	MutationApplied   = "applied"   // written now
	MutationReplayed  = "replayed"  // written by an earlier request with this mutation id
	MutationConflict  = "conflict"  // the performance changed since ExpectedRevision
	MutationDeleted   = "deleted"   // the performance has been deleted
	MutationNotFound  = "notFound"  // no such performance, or none this account can see
	MutationForbidden = "forbidden" // visible to this account, but not changeable by it
	MutationInvalid   = "invalid"   // refused on its contents; see Code
)

const (
	InvalidDate             = "date"
	InvalidParent           = "parentMissing"
	InvalidEntryNotInSeason = "entryNotInSeason"
	InvalidResult           = "result"
	InvalidPersonNotOnEntry = "personNotOnEntry"
	InvalidMatchup          = "matchup"
	InvalidPhoto            = "photo"
)

const maxMutationIdLength = 64

var (
	ErrMutationIdInvalid        = errors.New("A sync request needs a mutation id of letters, digits and dashes")
	ErrMutationIdReused         = errors.New("That mutation id was already used for a different change")
	ErrExpectedRevisionRequired = errors.New("A synced edit or delete must name the revision it was made against")
)

const (
	mutationKindSave   = "save"
	mutationKindDelete = "delete"
)

// AppearanceMutationReceipt records that a mutation id was applied, and what
// it produced.
type AppearanceMutationReceipt struct {
	Key          string
	UserId       int
	FamilyId     int
	AppearanceId int
	Kind         string
	Fingerprint  string
	Revision     int
	CreatedAt    time.Time
}

func PackAppearanceMutationReceipt(self *AppearanceMutationReceipt, buf *vpack.Buffer) {
	vpack.Version(1, buf)
	vpack.String(&self.Key, buf)
	vpack.Int(&self.UserId, buf)
	vpack.Int(&self.FamilyId, buf)
	vpack.Int(&self.AppearanceId, buf)
	vpack.String(&self.Kind, buf)
	vpack.String(&self.Fingerprint, buf)
	vpack.Int(&self.Revision, buf)
	vpack.Time(&self.CreatedAt, buf)
}

var AppearanceMutationBkt = vbolt.Bucket(&cfg.Info, "appearance_mutations", vpack.StringZ, PackAppearanceMutationReceipt)

var AppearanceMutationByUserIndex = vbolt.Index(&cfg.Info, "appearance_mutation_by_user", vpack.FInt, vpack.StringZ)

var AppearanceMutationByFamilyIndex = vbolt.Index(&cfg.Info, "appearance_mutation_by_family", vpack.FInt, vpack.StringZ)

// Requests and responses

type GetActivitySyncSupportRequest struct{}

type GetActivitySyncSupportResponse struct {
	MutationVersion int `json:"mutationVersion"`
}

type SyncSaveAppearanceRequest struct {
	MutationId string `json:"mutationId"`
	// Required when Appearance.Id is set; ignored on a create.
	ExpectedRevision *int                  `json:"expectedRevision,omitempty"`
	Appearance       SaveAppearanceRequest `json:"appearance"`
}

type SyncDeleteAppearanceRequest struct {
	MutationId       string `json:"mutationId"`
	Id               int    `json:"id"`
	ExpectedRevision *int   `json:"expectedRevision,omitempty"`
}

type AppearanceMutationResponse struct {
	Status string `json:"status"`
	// The performance's server id: on a create, the one it was given.
	AppearanceId int `json:"appearanceId"`
	// Applied or replayed: the revision this mutation produced. Conflict,
	// deleted and forbidden: the server's current revision.
	Revision int `json:"revision"`
	// The server's current copy, whenever there is one this account can see. On
	// a replay it may be newer than Revision, if something changed it since.
	Appearance *AppearanceView `json:"appearance,omitempty"`
	// Invalid only: which kind of refusal (Invalid* above) and its wording.
	Code    string `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}

// Procs

func GetActivitySyncSupport(ctx *vbeam.Context, req GetActivitySyncSupportRequest) (resp GetActivitySyncSupportResponse, err error) {
	if _, authErr := GetAuthUser(ctx); authErr != nil {
		err = ErrAuthFailure
		return
	}
	resp.MutationVersion = ActivityMutationVersion
	return
}

func SyncSaveAppearance(ctx *vbeam.Context, req SyncSaveAppearanceRequest) (resp AppearanceMutationResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}
	if !validMutationId(req.MutationId) {
		err = ErrMutationIdInvalid
		return
	}
	editing := req.Appearance.Id != 0
	if editing && req.ExpectedRevision == nil {
		err = ErrExpectedRevisionRequired
		return
	}

	// Everything from the receipt lookup to the receipt write happens in one
	// write transaction, so the revision check and the write cannot be split by
	// another request.
	vbeam.UseWriteTx(ctx)
	tx := ctx.Tx

	fingerprint := mutationFingerprint(req)
	key := mutationKey(user.Id, req.MutationId)
	if receipt, found := getMutationReceipt(tx, key); found {
		resp, err = replayMutation(tx, user, receipt, mutationKindSave, fingerprint)
		return
	}

	if editing {
		var current Appearance
		var refused bool
		if current, resp, refused = mutationTarget(tx, user, req.Appearance.Id); refused {
			return
		}
		if current.Revision != *req.ExpectedRevision {
			resp = outcomeWithCurrent(tx, user, MutationConflict, current)
			return
		}
	} else if refusal, refused := createRefusal(tx, user, req.Appearance); refused {
		resp = refusal
		return
	}

	save, prepareErr := prepareAppearanceSave(tx, user, req.Appearance)
	if prepareErr == nil {
		prepareErr = commitAppearanceSaveTx(tx, &save)
	}
	if prepareErr != nil {
		// Nothing is committed: closing the transaction drops any partial write.
		if code, ok := invalidCode(prepareErr); ok {
			resp = AppearanceMutationResponse{
				Status: MutationInvalid, AppearanceId: req.Appearance.Id,
				Code: code, Message: prepareErr.Error(),
			}
			return
		}
		err = prepareErr
		return
	}

	saved := save.appearance
	writeMutationReceiptTx(tx, AppearanceMutationReceipt{
		Key: key, UserId: user.Id, FamilyId: saved.FamilyId, AppearanceId: saved.Id,
		Kind: mutationKindSave, Fingerprint: fingerprint, Revision: saved.Revision,
		CreatedAt: time.Now(),
	})
	view := appearanceView(tx, user, saved)
	resp = AppearanceMutationResponse{
		Status: MutationApplied, AppearanceId: saved.Id, Revision: saved.Revision, Appearance: &view,
	}
	vbolt.TxCommit(tx)
	return
}

func SyncDeleteAppearance(ctx *vbeam.Context, req SyncDeleteAppearanceRequest) (resp AppearanceMutationResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}
	if !validMutationId(req.MutationId) {
		err = ErrMutationIdInvalid
		return
	}
	if req.ExpectedRevision == nil {
		err = ErrExpectedRevisionRequired
		return
	}

	vbeam.UseWriteTx(ctx)
	tx := ctx.Tx

	fingerprint := mutationFingerprint(req)
	key := mutationKey(user.Id, req.MutationId)
	if receipt, found := getMutationReceipt(tx, key); found {
		resp, err = replayMutation(tx, user, receipt, mutationKindDelete, fingerprint)
		return
	}

	current, refusal, refused := mutationTarget(tx, user, req.Id)
	if refused {
		resp = refusal
		return
	}
	if current.Revision != *req.ExpectedRevision {
		resp = outcomeWithCurrent(tx, user, MutationConflict, current)
		return
	}

	deleteAppearanceTx(tx, current.Id)
	tombstone := GetAppearanceTombstone(tx, current.Id)
	writeMutationReceiptTx(tx, AppearanceMutationReceipt{
		Key: key, UserId: user.Id, FamilyId: current.FamilyId, AppearanceId: current.Id,
		Kind: mutationKindDelete, Fingerprint: fingerprint, Revision: tombstone.Revision,
		CreatedAt: time.Now(),
	})
	resp = AppearanceMutationResponse{
		Status: MutationApplied, AppearanceId: current.Id, Revision: tombstone.Revision,
	}
	vbolt.TxCommit(tx)
	return
}

// Targets and outcomes

// mutationTarget finds the performance an edit or delete names, or the outcome
// to answer with instead. A performance in a family this account has no part
// in reads as not found, as it does everywhere else.
func mutationTarget(tx *vbolt.Tx, user User, id int) (Appearance, AppearanceMutationResponse, bool) {
	appearance := GetAppearanceById(tx, id)
	if appearance.Id == 0 {
		tombstone := GetAppearanceTombstone(tx, id)
		if tombstone.Id != 0 && CanAccessFamily(tx, user, tombstone.FamilyId, AccessView) {
			return Appearance{}, AppearanceMutationResponse{
				Status: MutationDeleted, AppearanceId: id, Revision: tombstone.Revision,
			}, true
		}
		return Appearance{}, AppearanceMutationResponse{Status: MutationNotFound, AppearanceId: id}, true
	}
	if canAccessAppearance(tx, user, appearance, AccessContribute) {
		return appearance, AppearanceMutationResponse{}, false
	}
	if canAccessAppearance(tx, user, appearance, AccessView) {
		return Appearance{}, outcomeWithCurrent(tx, user, MutationForbidden, appearance), true
	}
	return Appearance{}, AppearanceMutationResponse{Status: MutationNotFound, AppearanceId: id}, true
}

// createRefusal answers a create whose competition or routine this account
// can see but not add to; prepareAppearanceSave reports one it cannot see, or
// one since deleted, as a missing parent.
func createRefusal(tx *vbolt.Tx, user User, req SaveAppearanceRequest) (AppearanceMutationResponse, bool) {
	event := GetEventById(tx, req.EventId)
	entry := GetEntryById(tx, req.EntryId)
	viewOnly := (canAccessEvent(tx, user, event, AccessView) && !canAccessEvent(tx, user, event, AccessContribute)) ||
		(canAccessEntry(tx, user, entry, AccessView) && !canAccessEntry(tx, user, entry, AccessContribute))
	if viewOnly {
		return AppearanceMutationResponse{Status: MutationForbidden}, true
	}
	return AppearanceMutationResponse{}, false
}

func outcomeWithCurrent(tx *vbolt.Tx, user User, status string, current Appearance) AppearanceMutationResponse {
	view := appearanceView(tx, user, current)
	return AppearanceMutationResponse{
		Status: status, AppearanceId: current.Id, Revision: current.Revision, Appearance: &view,
	}
}

// replayMutation answers a mutation id seen before, without writing.
func replayMutation(tx *vbolt.Tx, user User, receipt AppearanceMutationReceipt, kind string, fingerprint string) (AppearanceMutationResponse, error) {
	if receipt.Kind != kind || receipt.Fingerprint != fingerprint {
		return AppearanceMutationResponse{}, ErrMutationIdReused
	}
	if !CanAccessFamily(tx, user, receipt.FamilyId, AccessView) {
		return AppearanceMutationResponse{Status: MutationNotFound, AppearanceId: receipt.AppearanceId}, nil
	}
	resp := AppearanceMutationResponse{
		Status: MutationReplayed, AppearanceId: receipt.AppearanceId, Revision: receipt.Revision,
	}
	if current := GetAppearanceById(tx, receipt.AppearanceId); current.Id != 0 && canAccessAppearance(tx, user, current, AccessView) {
		view := appearanceView(tx, user, current)
		resp.Appearance = &view
	}
	return resp, nil
}

func invalidCode(err error) (string, bool) {
	switch {
	case errors.Is(err, ErrInvalidActivityDate):
		return InvalidDate, true
	case errors.Is(err, ErrEventNotFound), errors.Is(err, ErrEntryNotFound), errors.Is(err, ErrAppearanceNotFound):
		return InvalidParent, true
	case errors.Is(err, ErrEntryNotInSeason):
		return InvalidEntryNotInSeason, true
	case errors.Is(err, ErrResultPersonNotOnEntry):
		return InvalidPersonNotOnEntry, true
	case errors.Is(err, ErrInvalidResultKind), errors.Is(err, ErrResultLabelRequired),
		errors.Is(err, ErrResultRankRequired), errors.Is(err, ErrResultScoreRequired),
		errors.Is(err, ErrResultRankOutOfRange), errors.Is(err, ErrTooManyResults):
		return InvalidResult, true
	case errors.Is(err, ErrInvalidOutcome), errors.Is(err, ErrMatchupScoreOutOfRange),
		errors.Is(err, ErrOutcomeContradictsScore):
		return InvalidMatchup, true
	case errors.Is(err, ErrTooManyPhotos), errors.Is(err, ErrPhotoNotFoundOrDenied):
		return InvalidPhoto, true
	}
	return "", false
}

// Receipts

func validMutationId(id string) bool {
	if id == "" || len(id) > maxMutationIdLength {
		return false
	}
	for _, c := range id {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-') {
			return false
		}
	}
	return true
}

// mutationKey scopes an id to the account that sent it: two accounts can never
// answer from each other's receipts.
func mutationKey(userId int, mutationId string) string {
	return fmt.Sprintf("%d:%s", userId, mutationId)
}

// mutationFingerprint hashes the whole request but its id, so a retry matches
// only if it asks for exactly the same change, expected revision included.
func mutationFingerprint[Req SyncSaveAppearanceRequest | SyncDeleteAppearanceRequest](req Req) string {
	switch r := any(&req).(type) {
	case *SyncSaveAppearanceRequest:
		r.MutationId = ""
	case *SyncDeleteAppearanceRequest:
		r.MutationId = ""
	}
	encoded, _ := json.Marshal(req)
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:])
}

func getMutationReceipt(tx *vbolt.Tx, key string) (receipt AppearanceMutationReceipt, found bool) {
	found = vbolt.Read(tx, AppearanceMutationBkt, key, &receipt)
	return
}

func writeMutationReceiptTx(tx *vbolt.Tx, receipt AppearanceMutationReceipt) {
	vbolt.Write(tx, AppearanceMutationBkt, receipt.Key, &receipt)
	vbolt.SetTargetSingleTerm(tx, AppearanceMutationByUserIndex, receipt.Key, receipt.UserId)
	vbolt.SetTargetSingleTerm(tx, AppearanceMutationByFamilyIndex, receipt.Key, receipt.FamilyId)
}

func deleteMutationReceiptTx(tx *vbolt.Tx, key string) {
	vbolt.Delete(tx, AppearanceMutationBkt, key)
	vbolt.SetTargetSingleTerm(tx, AppearanceMutationByUserIndex, key, -1)
	vbolt.SetTargetSingleTerm(tx, AppearanceMutationByFamilyIndex, key, -1)
}

func deleteUserAppearanceMutationsTx(tx *vbolt.Tx, userId int) {
	var keys []string
	vbolt.ReadTermTargets(tx, AppearanceMutationByUserIndex, userId, &keys, vbolt.Window{})
	for _, key := range keys {
		deleteMutationReceiptTx(tx, key)
	}
}

func deleteFamilyAppearanceMutationsTx(tx *vbolt.Tx, familyId int) {
	var keys []string
	vbolt.ReadTermTargets(tx, AppearanceMutationByFamilyIndex, familyId, &keys, vbolt.Window{})
	for _, key := range keys {
		deleteMutationReceiptTx(tx, key)
	}
}
