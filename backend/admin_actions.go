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

func RegisterAdminActionMethods(app *vbeam.Application) {
	vbeam.RegisterProc(app, RequeueStuckPhotos)
	vbeam.RegisterProc(app, RevokeUserSessions)
	vbeam.RegisterProc(app, VerifyBackupPath)
}

type RequeueStuckPhotosRequest struct{}

type RequeueStuckPhotosResponse struct {
	Queued int `json:"queued"`
}

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
	Revoked int `json:"revoked"`
}

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

const backupVerifyTimeout = 60 * time.Second

const backupVerifyMinInterval = 10 * time.Minute

var backupVerify struct {
	mu   sync.Mutex
	last VerifyBackupPathResponse
}

type VerifyBackupPathRequest struct{}

type VerifyBackupPathResponse struct {
	OK            bool      `json:"ok"`
	Detail        string    `json:"detail"`
	Status        int       `json:"status"`
	DeclaredBytes int64     `json:"declaredBytes"`
	ReceivedBytes int64     `json:"receivedBytes"`
	DurationMs    int64     `json:"durationMs"`
	CheckedAt     time.Time `json:"checkedAt"`
	Cached        bool      `json:"cached"`
}

func VerifyBackupPath(ctx *vbeam.Context, req VerifyBackupPathRequest) (resp VerifyBackupPathResponse, err error) {
	if err = requireAdminAccess(ctx); err != nil {
		return
	}
	return verifyBackupPath(), nil
}

func verifyBackupPath() VerifyBackupPathResponse {
	backupVerify.mu.Lock()
	defer backupVerify.mu.Unlock()

	if !backupVerify.last.CheckedAt.IsZero() && time.Since(backupVerify.last.CheckedAt) < backupVerifyMinInterval {
		cached := backupVerify.last
		cached.Cached = true
		return cached
	}

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

func loopbackBaseURL() string {
	return fmt.Sprintf("http://127.0.0.1:%d", cfg.Port)
}

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
		result.Detail = "the endpoint answered 404, the same answer it gives an unauthorized caller. " +
			"Either BACKUP_TOKEN was edited since this process started, so the running server is still " +
			"checking against the old value, or the snapshot budget of ten requests an hour is spent — " +
			"a rate-limited caller is turned away with a 404 too."
		return result
	case http.StatusConflict:
		result.Detail = "a snapshot was already running, so this proves nothing either way. Try again in a minute."
		return result
	case http.StatusServiceUnavailable:
		result.Detail = "the endpoint answered 503: it could not open a read transaction on the database."
		return result
	default:
		result.Detail = fmt.Sprintf("the endpoint answered %d, which it should never do.", response.StatusCode)
		return result
	}

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
