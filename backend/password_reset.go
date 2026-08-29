package backend

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"family/cfg"
	"fmt"
	"os"
	"strings"
	"time"

	"go.hasen.dev/vbeam"
	"go.hasen.dev/vbolt"
	"go.hasen.dev/vpack"
	"golang.org/x/crypto/bcrypt"
)

const (
	passwordResetTokenLifetime = time.Hour

	passwordResetRequestInterval = time.Minute
)

func RegisterPasswordResetMethods(app *vbeam.Application) {
	vbeam.RegisterProc(app, RequestPasswordReset)
	vbeam.RegisterProc(app, ValidatePasswordResetToken)
	vbeam.RegisterProc(app, ResetPassword)
}

type PasswordResetToken struct {
	Id        int       `json:"id"`
	UserId    int       `json:"userId"`
	TokenHash string    `json:"tokenHash"`
	ExpiresAt time.Time `json:"expiresAt"`
	CreatedAt time.Time `json:"createdAt"`
}

func PackPasswordResetToken(self *PasswordResetToken, buf *vpack.Buffer) {
	vpack.Version(1, buf)
	vpack.Int(&self.Id, buf)
	vpack.Int(&self.UserId, buf)
	vpack.String(&self.TokenHash, buf)
	vpack.Time(&self.ExpiresAt, buf)
	vpack.Time(&self.CreatedAt, buf)
}

var PasswordResetBkt = vbolt.Bucket(&cfg.Info, "password_resets", vpack.FInt, PackPasswordResetToken)

var PasswordResetByHashBkt = vbolt.Bucket(&cfg.Info, "password_resets_by_hash", vpack.StringZ, vpack.Int)

var PasswordResetByUserIndex = vbolt.Index(&cfg.Info, "password_resets_by_user", vpack.FInt, vpack.FInt)

type RequestPasswordResetRequest struct {
	Email string `json:"email"`
}

type RequestPasswordResetResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

type ValidatePasswordResetTokenRequest struct {
	Token string `json:"token"`
}

type ValidatePasswordResetTokenResponse struct {
	Valid bool `json:"valid"`
}

type ResetPasswordRequest struct {
	Token           string `json:"token"`
	Password        string `json:"password"`
	ConfirmPassword string `json:"confirmPassword"`
}

type ResetPasswordResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

func hashResetToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func createPasswordResetTokenTx(tx *vbolt.Tx, userId int, now time.Time) (string, error) {
	tokenString, err := generateToken(32)
	if err != nil {
		return "", err
	}

	deleteUserPasswordResetTokensTx(tx, userId)

	token := PasswordResetToken{
		Id:        vbolt.NextIntId(tx, PasswordResetBkt),
		UserId:    userId,
		TokenHash: hashResetToken(tokenString),
		ExpiresAt: now.Add(passwordResetTokenLifetime),
		CreatedAt: now,
	}

	vbolt.Write(tx, PasswordResetBkt, token.Id, &token)
	vbolt.Write(tx, PasswordResetByHashBkt, token.TokenHash, &token.Id)
	vbolt.SetTargetSingleTerm(tx, PasswordResetByUserIndex, token.Id, token.UserId)

	return tokenString, nil
}

func getPasswordResetTokenTx(tx *vbolt.Tx, tokenString string) (PasswordResetToken, bool) {
	var tokenId int
	vbolt.Read(tx, PasswordResetByHashBkt, hashResetToken(tokenString), &tokenId)
	if tokenId == 0 {
		return PasswordResetToken{}, false
	}

	var token PasswordResetToken
	vbolt.Read(tx, PasswordResetBkt, tokenId, &token)
	return token, token.Id != 0
}

func validatePasswordResetTokenTx(tx *vbolt.Tx, tokenString string, now time.Time) (PasswordResetToken, bool) {
	if tokenString == "" {
		return PasswordResetToken{}, false
	}

	token, found := getPasswordResetTokenTx(tx, tokenString)
	if !found || !token.ExpiresAt.After(now) {
		return PasswordResetToken{}, false
	}
	return token, true
}

func deletePasswordResetTokenTx(tx *vbolt.Tx, token PasswordResetToken) {
	vbolt.Delete(tx, PasswordResetBkt, token.Id)
	vbolt.Delete(tx, PasswordResetByHashBkt, token.TokenHash)
	vbolt.SetTargetSingleTerm(tx, PasswordResetByUserIndex, token.Id, -1)
}

func deleteUserPasswordResetTokensTx(tx *vbolt.Tx, userId int) {
	var tokenIds []int
	vbolt.ReadTermTargets(tx, PasswordResetByUserIndex, userId, &tokenIds, vbolt.Window{})

	for _, tokenId := range tokenIds {
		var token PasswordResetToken
		vbolt.Read(tx, PasswordResetBkt, tokenId, &token)
		if token.Id == 0 {
			continue
		}
		deletePasswordResetTokenTx(tx, token)
	}
}

func latestPasswordResetTokenTx(tx *vbolt.Tx, userId int) (PasswordResetToken, bool) {
	var tokenIds []int
	vbolt.ReadTermTargets(tx, PasswordResetByUserIndex, userId, &tokenIds, vbolt.Window{})

	var newest PasswordResetToken
	for _, tokenId := range tokenIds {
		var token PasswordResetToken
		vbolt.Read(tx, PasswordResetBkt, tokenId, &token)
		if token.Id == 0 {
			continue
		}
		if newest.Id == 0 || token.CreatedAt.After(newest.CreatedAt) {
			newest = token
		}
	}
	return newest, newest.Id != 0
}

func CleanupExpiredPasswordResetTokens(tx *vbolt.Tx, now time.Time) int {
	var expired []PasswordResetToken
	vbolt.IterateAll(tx, PasswordResetBkt, func(_ int, token PasswordResetToken) bool {
		if !token.ExpiresAt.After(now) {
			expired = append(expired, token)
		}
		return true
	})

	for _, token := range expired {
		deletePasswordResetTokenTx(tx, token)
	}
	return len(expired)
}

var passwordResetSender = deliverPasswordResetEmail

func passwordResetLink(token string) string {
	siteRoot := os.Getenv("SITE_ROOT")
	if siteRoot == "" {
		siteRoot = cfg.SiteURL
	}
	return strings.TrimRight(siteRoot, "/") + "/reset-password?token=" + token
}

func passwordResetBody(name, link string) string {
	greeting := "Hello,"
	if name != "" {
		greeting = "Hello " + name + ","
	}
	return fmt.Sprintf(`%s

Somebody asked to reset the password for your Family Record account. Open the
link below to choose a new one:

%s

This link expires in one hour and can only be used once. Resetting your password
signs you out everywhere else.

If you did not ask for this, you can ignore this email.
`, greeting, link)
}

func deliverPasswordResetEmail(user User, token string) error {
	return QueueMail(MailJob{
		To:      user.Email,
		Subject: "Reset your Family Record password",
		Body:    passwordResetBody(user.Name, passwordResetLink(token)),
		Kind:    "password_reset",
	})
}

var passwordChangedSender = deliverPasswordChangedEmail

func passwordChangedBody(name string) string {
	greeting := "Hello,"
	if name != "" {
		greeting = "Hello " + name + ","
	}
	return fmt.Sprintf(`%s

The password for your Family Record account was just changed, and every other
device signed in to the account has been signed out.
`, greeting)
}

func deliverPasswordChangedEmail(user User) error {
	return QueueMail(MailJob{
		To:      user.Email,
		Subject: "Your Family Record password was changed",
		Body:    passwordChangedBody(user.Name),
		Kind:    "password_changed",
	})
}

func RequestPasswordReset(ctx *vbeam.Context, req RequestPasswordResetRequest) (resp RequestPasswordResetResponse, err error) {
	email := strings.TrimSpace(req.Email)
	if email == "" {
		resp.Error = "Email is required"
		return
	}

	now := time.Now()

	var user User
	var token string
	vbeam.UseWriteTx(ctx)

	userId := GetUserId(ctx.Tx, email)
	if userId != 0 {
		user = GetUser(ctx.Tx, userId)
	}

	if user.Id != 0 {
		latest, found := latestPasswordResetTokenTx(ctx.Tx, user.Id)
		throttled := found && now.Sub(latest.CreatedAt) < passwordResetRequestInterval

		if !throttled {
			token, err = createPasswordResetTokenTx(ctx.Tx, user.Id, now)
			if err != nil {
				LogErrorSimple(LogCategoryAuth, "Failed to create password reset token", map[string]interface{}{
					"userId": user.Id,
					"error":  err.Error(),
				})
				err = nil
				token = ""
			} else {
				vbolt.TxCommit(ctx.Tx)
			}
		}
	}

	if token != "" {
		if sendErr := passwordResetSender(user, token); sendErr != nil {
			LogErrorSimple(LogCategoryAuth, "Failed to queue password reset email", map[string]interface{}{
				"userId": user.Id,
				"error":  sendErr.Error(),
			})
		} else {
			LogInfo(LogCategoryAuth, "Password reset email queued", map[string]interface{}{
				"userId": user.Id,
			})
		}
	} else if user.Id == 0 {
		LogInfo(LogCategoryAuth, "Password reset requested for unknown email", nil)
	}

	resp.Success = true
	return
}

func ValidatePasswordResetToken(ctx *vbeam.Context, req ValidatePasswordResetTokenRequest) (resp ValidatePasswordResetTokenResponse, err error) {
	_, resp.Valid = validatePasswordResetTokenTx(ctx.Tx, req.Token, time.Now())
	return
}

func ResetPassword(ctx *vbeam.Context, req ResetPasswordRequest) (resp ResetPasswordResponse, err error) {
	if err := validateNewPassword(req.Password, req.ConfirmPassword); err != nil {
		resp.Error = err.Error()
		return resp, nil
	}

	vbeam.UseWriteTx(ctx)

	token, valid := validatePasswordResetTokenTx(ctx.Tx, req.Token, time.Now())
	if !valid {
		resp.Error = "This reset link is invalid or has expired. Please request a new one."
		return
	}

	user := GetUser(ctx.Tx, token.UserId)
	if user.Id == 0 {
		deletePasswordResetTokenTx(ctx.Tx, token)
		vbolt.TxCommit(ctx.Tx)
		resp.Error = "This reset link is invalid or has expired. Please request a new one."
		return
	}

	hash, hashErr := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if hashErr != nil {
		resp.Error = "Failed to process password"
		return
	}

	vbolt.Write(ctx.Tx, PasswdBkt, user.Id, &hash)
	deleteUserPasswordResetTokensTx(ctx.Tx, user.Id)
	DeleteUserRefreshTokens(ctx.Tx, user.Id)
	vbolt.TxCommit(ctx.Tx)

	LogInfo(LogCategoryAuth, "Password reset completed", map[string]interface{}{
		"userId": user.Id,
		"email":  redactEmail(user.Email),
	})

	if sendErr := passwordChangedSender(user); sendErr != nil {
		LogErrorSimple(LogCategoryAuth, "Failed to queue password changed notice", map[string]interface{}{
			"userId": user.Id,
			"error":  sendErr.Error(),
		})
	}

	resp.Success = true
	return
}

func validateNewPassword(password, confirm string) error {
	if password == "" {
		return errors.New("Password is required")
	}
	if len(password) < 8 {
		return errors.New("Password must be at least 8 characters")
	}
	if password != confirm {
		return errors.New("Passwords do not match")
	}
	return nil
}
