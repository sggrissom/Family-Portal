package backend

import (
	"errors"
	"family/cfg"
	"time"

	"go.hasen.dev/vbeam"
	"go.hasen.dev/vbolt"
	"go.hasen.dev/vpack"
)

func RegisterPushNotificationMethods(app *vbeam.Application) {
	vbeam.RegisterProc(app, RegisterPushDevice)
	vbeam.RegisterProc(app, UnregisterPushDevice)
}

// Request/Response types
type RegisterPushDeviceRequest struct {
	Token       string `json:"token"`
	Platform    string `json:"platform"`    // "ios" or "android"
	Environment string `json:"environment"` // "sandbox" or "production"
	BundleId    string `json:"bundleId"`
}

type RegisterPushDeviceResponse struct {
	Success bool `json:"success"`
}

type UnregisterPushDeviceRequest struct {
	Token string `json:"token"`
}

type UnregisterPushDeviceResponse struct {
	Success bool `json:"success"`
}

// Database types
type PushDeviceToken struct {
	Id          int       `json:"id"`
	UserId      int       `json:"userId"`
	Token       string    `json:"token"`
	Platform    string    `json:"platform"`    // "ios" or "android"
	Environment string    `json:"environment"` // "sandbox" or "production"
	BundleId    string    `json:"bundleId"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
	IsActive    bool      `json:"isActive"`
}

// Packing function for vbolt serialization
func PackPushDeviceToken(self *PushDeviceToken, buf *vpack.Buffer) {
	vpack.Version(1, buf)
	vpack.Int(&self.Id, buf)
	vpack.Int(&self.UserId, buf)
	vpack.String(&self.Token, buf)
	vpack.String(&self.Platform, buf)
	vpack.String(&self.Environment, buf)
	vpack.String(&self.BundleId, buf)
	vpack.Time(&self.CreatedAt, buf)
	vpack.Time(&self.UpdatedAt, buf)
	vpack.Bool(&self.IsActive, buf)
}

// Buckets for vbolt database storage
var PushDeviceTokenBkt = vbolt.Bucket(&cfg.Info, "push_device_tokens", vpack.FInt, PackPushDeviceToken)

// PushDeviceTokenByTokenBkt: token string => device token id (for uniqueness lookup)
var PushDeviceTokenByTokenBkt = vbolt.Bucket(&cfg.Info, "push_device_token_by_token", vpack.StringZ, vpack.Int)

// PushDeviceTokenByUserIndex: term = user_id, target = device_token_id
var PushDeviceTokenByUserIndex = vbolt.Index(&cfg.Info, "push_device_token_by_user", vpack.FInt, vpack.FInt)

// Database helper functions
func GetPushDeviceTokenById(tx *vbolt.Tx, tokenId int) (token PushDeviceToken) {
	vbolt.Read(tx, PushDeviceTokenBkt, tokenId, &token)
	return
}

func GetPushDeviceTokenByToken(tx *vbolt.Tx, tokenStr string) (token PushDeviceToken) {
	var tokenId int
	vbolt.Read(tx, PushDeviceTokenByTokenBkt, tokenStr, &tokenId)
	if tokenId != 0 {
		token = GetPushDeviceTokenById(tx, tokenId)
	}
	return
}

// GetActiveDeviceTokensForUser returns all active device tokens for a user
func GetActiveDeviceTokensForUser(tx *vbolt.Tx, userId int) (tokens []PushDeviceToken) {
	var tokenIds []int
	vbolt.ReadTermTargets(tx, PushDeviceTokenByUserIndex, userId, &tokenIds, vbolt.Window{})

	for _, tokenId := range tokenIds {
		token := GetPushDeviceTokenById(tx, tokenId)
		if token.Id != 0 && token.IsActive {
			tokens = append(tokens, token)
		}
	}
	return
}

// upsertPushDeviceToken creates or updates a device token
func upsertPushDeviceToken(tx *vbolt.Tx, userId int, req RegisterPushDeviceRequest) (PushDeviceToken, error) {
	now := time.Now()

	// Check if token already exists
	existingToken := GetPushDeviceTokenByToken(tx, req.Token)

	if existingToken.Id != 0 {
		// Update existing token
		existingToken.UserId = userId
		existingToken.Platform = req.Platform
		existingToken.Environment = req.Environment
		existingToken.BundleId = req.BundleId
		existingToken.UpdatedAt = now
		existingToken.IsActive = true

		vbolt.Write(tx, PushDeviceTokenBkt, existingToken.Id, &existingToken)
		// Update user index (in case user changed)
		vbolt.SetTargetSingleTerm(tx, PushDeviceTokenByUserIndex, existingToken.Id, userId)

		return existingToken, nil
	}

	// Create new token
	token := PushDeviceToken{
		Id:          vbolt.NextIntId(tx, PushDeviceTokenBkt),
		UserId:      userId,
		Token:       req.Token,
		Platform:    req.Platform,
		Environment: req.Environment,
		BundleId:    req.BundleId,
		CreatedAt:   now,
		UpdatedAt:   now,
		IsActive:    true,
	}

	vbolt.Write(tx, PushDeviceTokenBkt, token.Id, &token)
	vbolt.Write(tx, PushDeviceTokenByTokenBkt, token.Token, &token.Id)
	vbolt.SetTargetSingleTerm(tx, PushDeviceTokenByUserIndex, token.Id, userId)

	return token, nil
}

// deactivatePushDeviceToken soft-deletes a device token by setting IsActive=false
func deactivatePushDeviceToken(tx *vbolt.Tx, tokenStr string) error {
	token := GetPushDeviceTokenByToken(tx, tokenStr)
	if token.Id == 0 {
		return errors.New("token not found")
	}

	token.IsActive = false
	token.UpdatedAt = time.Now()
	vbolt.Write(tx, PushDeviceTokenBkt, token.Id, &token)

	return nil
}

// DeactivatePushDeviceTokenById deactivates a token by its database ID
// Used by the push worker when APNs reports an invalid token
func DeactivatePushDeviceTokenById(db *vbolt.DB, tokenId int) error {
	var updateError error
	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		token := GetPushDeviceTokenById(tx, tokenId)
		if token.Id == 0 {
			updateError = errors.New("token not found")
			return
		}

		token.IsActive = false
		token.UpdatedAt = time.Now()
		vbolt.Write(tx, PushDeviceTokenBkt, token.Id, &token)
		vbolt.TxCommit(tx)
	})
	return updateError
}

// Validation
func validateRegisterPushDeviceRequest(req RegisterPushDeviceRequest) error {
	if req.Token == "" {
		return errors.New("device token is required")
	}
	if req.Platform != "ios" && req.Platform != "android" {
		return errors.New("platform must be 'ios' or 'android'")
	}
	if req.Environment != "sandbox" && req.Environment != "production" {
		return errors.New("environment must be 'sandbox' or 'production'")
	}
	if req.BundleId == "" {
		return errors.New("bundle ID is required")
	}
	return nil
}

// vbeam procedures
func RegisterPushDevice(ctx *vbeam.Context, req RegisterPushDeviceRequest) (resp RegisterPushDeviceResponse, err error) {
	// Get authenticated user
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}

	// Validate request
	if err = validateRegisterPushDeviceRequest(req); err != nil {
		return
	}

	// Upsert device token
	vbeam.UseWriteTx(ctx)
	_, err = upsertPushDeviceToken(ctx.Tx, user.Id, req)
	if err != nil {
		return
	}

	vbolt.TxCommit(ctx.Tx)

	LogInfo(LogCategoryAPI, "Push device registered", map[string]interface{}{
		"userId":      user.Id,
		"platform":    req.Platform,
		"environment": req.Environment,
		"bundleId":    req.BundleId,
	})

	resp.Success = true
	return
}

func UnregisterPushDevice(ctx *vbeam.Context, req UnregisterPushDeviceRequest) (resp UnregisterPushDeviceResponse, err error) {
	// Get authenticated user
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}

	if req.Token == "" {
		err = errors.New("device token is required")
		return
	}

	// Deactivate device token
	vbeam.UseWriteTx(ctx)
	err = deactivatePushDeviceToken(ctx.Tx, req.Token)
	if err != nil {
		return
	}

	vbolt.TxCommit(ctx.Tx)

	LogInfo(LogCategoryAPI, "Push device unregistered", map[string]interface{}{
		"userId": user.Id,
	})

	resp.Success = true
	return
}
