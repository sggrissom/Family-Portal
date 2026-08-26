package backend

import (
	"context"
	"family/cfg"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"go.hasen.dev/vbeam"
	"go.hasen.dev/vbolt"
)

// Actions the panel is worth having a button for: things that currently require
// direct database access or an SSH session, and that one operator would
// plausibly reach for more than once.

func RegisterAdminActionMethods(app *vbeam.Application) {
	vbeam.RegisterProc(app, RequeueStuckPhotos)
	vbeam.RegisterProc(app, RevokeUserSessions)
	vbeam.RegisterProc(app, VerifyBackupPath)
}

type RequeueStuckPhotosRequest struct{}

type RequeueStuckPhotosResponse struct {
	Queued int `json:"queued"`
}

// RequeueStuckPhotos rescues rows stranded in Processing.
//
// The worker sets Status = 1 when it picks a job up, so a row still in it an
// hour later belongs to a job that was interrupted — a restart mid-processing,
// or an abandoned queue on shutdown. Nothing retries them: the queue is
// in-memory, so the job that owned the row died with the process, and the row
// is left claiming to be in progress forever. Until now the only fix was
// editing the database.
//
// They are requeued rather than marked failed, because the original is still on
// disk and reprocessing is exactly what they need. A photo whose original is
// genuinely gone will fail once, visibly, and land in the failed count where it
// belongs — which is a better resting place than a permanent lie.
func RequeueStuckPhotos(ctx *vbeam.Context, req RequeueStuckPhotosRequest) (resp RequeueStuckPhotosResponse, err error) {
	if err = requireAdminAccess(ctx); err != nil {
		return
	}

	pw := activePhotoWorker()
	if pw == nil {
		err = ErrPhotoWorkerUnavailable
		return
	}

	stuckBefore := time.Now().Add(-stuckPhotoAge)
	var toQueue []PhotoProcessingJob
	vbolt.IterateAll(ctx.Tx, ImagesBkt, func(key int, image Image) bool {
		if image.Status == 1 && image.CreatedAt.Before(stuckBefore) {
			toQueue = append(toQueue, PhotoProcessingJob{
				ImageId:   image.Id,
				FamilyId:  image.FamilyId,
				FilePath:  image.FilePath,
				MimeType:  image.MimeType,
				Reprocess: true,
			})
		}
		return true
	})

	queueBacklog(pw, toQueue,
		"Failed to requeue a stuck photo",
		"Stuck-photo backlog fully queued")

	LogInfo(LogCategoryAdmin, "Requeued photos stranded in processing", map[string]interface{}{
		"count": len(toQueue),
	})

	resp.Queued = len(toQueue)
	return
}

type RevokeUserSessionsRequest struct {
	UserId int `json:"userId"`
}

type RevokeUserSessionsResponse struct {
	// Revoked is how many refresh tokens were deleted. A single login issues
	// several over its life — every rotation keeps its predecessor as replay
	// evidence — so this is larger than the number of devices.
	Revoked int `json:"revoked"`
}

// RevokeUserSessions signs a user out everywhere.
//
// Worth a button for a lost phone, or for a stale session that survived a JWT
// secret change. The access token is a short-lived JWT this cannot recall, so
// the effect lands when it expires; deleting the refresh tokens is what stops
// it being renewed.
func RevokeUserSessions(ctx *vbeam.Context, req RevokeUserSessionsRequest) (resp RevokeUserSessionsResponse, err error) {
	if err = requireAdminAccess(ctx); err != nil {
		return
	}
	if req.UserId <= 0 {
		err = ErrUserNotFound
		return
	}

	user := GetUser(ctx.Tx, req.UserId)
	if user.Id == 0 {
		err = ErrUserNotFound
		return
	}

	var tokenIds []int
	vbolt.ReadTermTargets(ctx.Tx, RefreshTokenByUserIndex, req.UserId, &tokenIds, vbolt.Window{})
	resp.Revoked = len(tokenIds)

	if resp.Revoked > 0 {
		vbeam.UseWriteTx(ctx)
		DeleteUserRefreshTokens(ctx.Tx, req.UserId)
		vbolt.TxCommit(ctx.Tx)
	}

	LogInfo(LogCategoryAdmin, "Admin revoked a user's sessions", map[string]interface{}{
		"userId":  req.UserId,
		"revoked": resp.Revoked,
	})
	return
}

// The backup path, proven rather than assumed.
//
// Nothing exercises the snapshot endpoint from this side. backupctl fetches it
// at night and the only evidence it worked is a file on a box this application
// cannot read, and restore.md's drill proves that an archive restores — not
// that a snapshot can still be taken today, after the token was retyped or the
// service moved. When it does fail, backupctl reports a 404, and the endpoint
// answers 404 to every caller it will not authorize: a token that does not
// match, an unset BACKUP_TOKEN, and a spent rate limit are the same three
// digits. Telling them apart means an SSH session.
//
// So the check sends this application the same request backupctl sends, over
// the same loopback URL, with the token this process would accept now, and
// reads the whole body to be sure the stream finishes. One press covers the
// token, the route, and a full read of the database.

// backupVerifyTimeout bounds the whole check. It has to stay comfortably under
// the RPC response deadline (two minutes, request_timeouts.go) so a slow
// snapshot fails as a stated result rather than as a severed reply.
const backupVerifyTimeout = 60 * time.Second

// backupVerifyMinInterval is how often the check may actually run.
//
// The snapshot endpoint allows ten requests an hour per caller (rate_limit.go),
// and backupctl fetches from 127.0.0.1 — the same address this check calls
// from, so the two share one budget. Worse, an exhausted budget answers 404,
// the exact symptom the check exists to diagnose. A cooldown keeps a row of
// impatient presses from spending the nightly backup's attempts on a question
// it has already answered.
const backupVerifyMinInterval = 10 * time.Minute

var backupVerify struct {
	mu   sync.Mutex
	last VerifyBackupPathResponse
}

type VerifyBackupPathRequest struct{}

type VerifyBackupPathResponse struct {
	// OK is the whole point: a complete snapshot came back over the same path
	// the nightly backup uses.
	OK bool `json:"ok"`
	// Detail is a sentence about what happened, in both outcomes. The failures
	// are the reason this exists, so each names what to go and fix.
	Detail string `json:"detail"`
	// Status is what the endpoint answered, carried separately because 404 and
	// 409 mean very different things here.
	Status        int   `json:"status"`
	DeclaredBytes int64 `json:"declaredBytes"`
	ReceivedBytes int64 `json:"receivedBytes"`
	DurationMs    int64 `json:"durationMs"`
	// CheckedAt is when the result was produced, which is not when it was
	// asked for — see Cached.
	CheckedAt time.Time `json:"checkedAt"`
	// Cached says this is an earlier result replayed because the cooldown has
	// not elapsed. Stated rather than hidden: a stale pass after an .env edit
	// would otherwise be the most misleading thing on the page.
	Cached bool `json:"cached"`
}

// VerifyBackupPath fetches a snapshot from this application and throws it away.
func VerifyBackupPath(ctx *vbeam.Context, req VerifyBackupPathRequest) (resp VerifyBackupPathResponse, err error) {
	if err = requireAdminAccess(ctx); err != nil {
		return
	}
	return verifyBackupPath(), nil
}

// verifyBackupPath is the cooldown wrapper. Like the host metrics fetch, it
// never returns an error: every outcome here is a state to report, including
// the ones that mean the backup is broken.
func verifyBackupPath() VerifyBackupPathResponse {
	backupVerify.mu.Lock()
	defer backupVerify.mu.Unlock()

	if !backupVerify.last.CheckedAt.IsZero() && time.Since(backupVerify.last.CheckedAt) < backupVerifyMinInterval {
		cached := backupVerify.last
		cached.Cached = true
		return cached
	}

	// The token is read now rather than at startup, deliberately. The endpoint
	// captured its copy when the process started, so a .env edited since then
	// makes these two disagree — and that disagreement is exactly the state
	// that breaks the nightly backup while everything looks configured.
	token, tokenErr := resolveBackupToken()
	if tokenErr != nil {
		token = ""
	}

	result := runBackupVerification(loopbackBaseURL(), token)
	if tokenErr != nil {
		result.Detail = tokenErr.Error()
	}

	if result.OK {
		LogInfo(LogCategoryAdmin, "Backup path verified", map[string]interface{}{
			"bytes":      result.ReceivedBytes,
			"durationMs": result.DurationMs,
		})
	} else {
		LogWarn(LogCategoryAdmin, "Backup path verification failed", map[string]interface{}{
			"status": result.Status,
			"detail": result.Detail,
		})
	}

	backupVerify.last = result
	return result
}

// loopbackBaseURL is where this application answers itself. Loopback rather
// than SiteURL: the public name would take the request out through DNS, TLS,
// and Caddy, testing three things that are not the backup path and that a
// nightly backup does not use either.
func loopbackBaseURL() string {
	return fmt.Sprintf("http://127.0.0.1:%d", cfg.Port)
}

// runBackupVerification performs one fetch. Split out from the caller so the
// tests can point it at a server whose failures they choose.
func runBackupVerification(baseURL, token string) VerifyBackupPathResponse {
	result := VerifyBackupPathResponse{CheckedAt: time.Now()}

	if token == "" {
		result.Detail = "BACKUP_TOKEN is not set, so there is no snapshot endpoint to verify. " +
			"A release build refuses to start without one; a development build simply has no backup path."
		return result
	}

	ctx, cancel := context.WithTimeout(context.Background(), backupVerifyTimeout)
	defer cancel()

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+SnapshotPath, nil)
	if err != nil {
		result.Detail = fmt.Sprintf("the snapshot URL is not usable: %v", err)
		return result
	}
	request.Header.Set("Authorization", "Bearer "+token)

	start := time.Now()
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		result.DurationMs = time.Since(start).Milliseconds()
		result.Detail = fmt.Sprintf("nothing answered at %s: %v", baseURL+SnapshotPath, err)
		return result
	}
	defer response.Body.Close()
	result.Status = response.StatusCode

	switch response.StatusCode {
	case http.StatusOK:
	case http.StatusNotFound:
		// This is backupctl's ambiguous failure, reproduced on demand. Both
		// causes are named because the fixes are different and the response
		// deliberately does not distinguish them.
		result.Detail = "the endpoint answered 404, the same answer it gives an unauthorized caller. " +
			"Either BACKUP_TOKEN was edited since this process started, so the running server is still " +
			"checking against the old value, or the snapshot budget of ten requests an hour is spent — " +
			"a rate-limited caller is turned away with a 404 too."
		return result
	case http.StatusConflict:
		// Two overlapping snapshots are how the database file doubles, so the
		// endpoint refuses the second caller. Nothing is wrong.
		result.Detail = "a snapshot was already running, so this proves nothing either way. Try again in a minute."
		return result
	case http.StatusServiceUnavailable:
		result.Detail = "the endpoint answered 503: it could not open a read transaction on the database."
		return result
	default:
		result.Detail = fmt.Sprintf("the endpoint answered %d, which it should never do.", response.StatusCode)
		return result
	}

	// Content-Length is the endpoint's declaration of tx.Size(). backupctl
	// stores whatever body it receives, so a stream that stops short of this
	// number is a truncated backup that looks like a successful one.
	result.DeclaredBytes = response.ContentLength

	received, copyErr := io.Copy(io.Discard, response.Body)
	result.ReceivedBytes = received
	result.DurationMs = time.Since(start).Milliseconds()

	switch {
	case copyErr != nil:
		result.Detail = fmt.Sprintf("the stream broke after %d of %d bytes: %v", received, result.DeclaredBytes, copyErr)
	case result.DeclaredBytes < 0:
		result.Detail = "the endpoint sent no Content-Length, so a backup could not tell a complete snapshot from a truncated one."
	case received != result.DeclaredBytes:
		result.Detail = fmt.Sprintf("the endpoint declared %d bytes and sent %d. A backup taken now would be short.", result.DeclaredBytes, received)
	default:
		result.OK = true
		result.Detail = "a complete snapshot came back over the path the nightly backup uses."
	}
	return result
}
