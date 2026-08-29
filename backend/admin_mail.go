package backend

import (
	"time"

	"go.hasen.dev/vbeam"
	"go.hasen.dev/vbolt"
)

func RegisterAdminMailMethods(app *vbeam.Application) {
	vbeam.RegisterProc(app, GetMailStats)
	vbeam.RegisterProc(app, ResendPasswordReset)
}

type GetMailStatsRequest struct{}

func GetMailStats(ctx *vbeam.Context, req GetMailStatsRequest) (resp MailWorkerStats, err error) {
	if err = requireAdminAccess(ctx); err != nil {
		return
	}
	return GetMailWorkerStats(), nil
}

type ResendPasswordResetRequest struct {
	UserId int `json:"userId"`
}

type ResendPasswordResetResponse struct {
	Email               string    `json:"email"`
	Queued              bool      `json:"queued"`
	Detail              string    `json:"detail"`
	InvalidatedPrevious bool      `json:"invalidatedPrevious"`
	ExpiresAt           time.Time `json:"expiresAt"`
}

func ResendPasswordReset(ctx *vbeam.Context, req ResendPasswordResetRequest) (resp ResendPasswordResetResponse, err error) {
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

	now := time.Now()
	vbeam.UseWriteTx(ctx)

	previous, hadPrevious := latestPasswordResetTokenTx(ctx.Tx, user.Id)
	resp.InvalidatedPrevious = hadPrevious && previous.ExpiresAt.After(now)

	token, tokenErr := createPasswordResetTokenTx(ctx.Tx, user.Id, now)
	if tokenErr != nil {
		err = tokenErr
		return
	}

	resp.Email = user.Email
	resp.ExpiresAt = now.Add(passwordResetTokenLifetime)

	vbolt.TxCommit(ctx.Tx)

	if sendErr := passwordResetSender(user, token); sendErr != nil {
		LogErrorSimple(LogCategoryAdmin, "Admin resend could not queue a password reset email", map[string]interface{}{
			"userId": user.Id,
			"error":  sendErr.Error(),
		})
		resp.Detail = "The link was created but the mail worker would not take it: " + sendErr.Error()
		return
	}

	LogInfo(LogCategoryAdmin, "Admin resent a password reset email", map[string]interface{}{
		"userId": user.Id,
	})

	resp.Queued = true
	resp.Detail = "Handed to the mail worker. Delivery is what the log below reports; queued is not sent."
	return
}
