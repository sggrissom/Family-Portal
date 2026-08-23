package backend

import (
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
