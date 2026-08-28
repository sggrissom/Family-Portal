package backend

import (
	"crypto/sha256"
	"encoding/hex"
	"family/cfg"
	"fmt"
	"os"
	"strings"
	"time"

	"go.hasen.dev/vbeam"
	"go.hasen.dev/vbolt"
	"go.hasen.dev/vpack"
)

// Verification is advisory: nothing in the app is gated on it. It exists so a
// mistyped address is discoverable before the owner needs a password reset, and
// so an address that never verifies is visible as such.
const emailVerificationLifetime = 7 * 24 * time.Hour

type EmailVerificationToken struct {
	Id        int       `json:"id"`
	UserId    int       `json:"userId"`
	TokenHash string    `json:"tokenHash"`
	ExpiresAt time.Time `json:"expiresAt"`
	CreatedAt time.Time `json:"createdAt"`
}

func PackEmailVerificationToken(self *EmailVerificationToken, buf *vpack.Buffer) {
	vpack.Version(1, buf)
	vpack.Int(&self.Id, buf)
	vpack.Int(&self.UserId, buf)
	vpack.String(&self.TokenHash, buf)
	vpack.Time(&self.ExpiresAt, buf)
	vpack.Time(&self.CreatedAt, buf)
}

var EmailVerificationBkt = vbolt.Bucket(&cfg.Info, "email_verifications", vpack.FInt, PackEmailVerificationToken)

var EmailVerificationByHashBkt = vbolt.Bucket(&cfg.Info, "email_verifications_by_hash", vpack.StringZ, vpack.Int)

var EmailVerificationByUserIndex = vbolt.Index(&cfg.Info, "email_verifications_by_user", vpack.FInt, vpack.FInt)

func RegisterEmailVerificationMethods(app *vbeam.Application) {
	vbeam.RegisterProc(app, VerifyEmail)
	vbeam.RegisterProc(app, ResendVerificationEmail)
}

type VerifyEmailRequest struct {
	Token string `json:"token"`
}

type VerifyEmailResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

type ResendVerificationResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

func hashVerificationToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func createEmailVerificationTokenTx(tx *vbolt.Tx, userId int, now time.Time) (string, error) {
	tokenString, err := generateToken(32)
	if err != nil {
		return "", err
	}

	deleteUserVerificationTokensTx(tx, userId)

	token := EmailVerificationToken{
		Id:        vbolt.NextIntId(tx, EmailVerificationBkt),
		UserId:    userId,
		TokenHash: hashVerificationToken(tokenString),
		ExpiresAt: now.Add(emailVerificationLifetime),
		CreatedAt: now,
	}

	vbolt.Write(tx, EmailVerificationBkt, token.Id, &token)
	vbolt.Write(tx, EmailVerificationByHashBkt, token.TokenHash, &token.Id)
	vbolt.SetTargetSingleTerm(tx, EmailVerificationByUserIndex, token.Id, token.UserId)

	return tokenString, nil
}

func getVerificationTokenTx(tx *vbolt.Tx, tokenString string) (EmailVerificationToken, bool) {
	var tokenId int
	vbolt.Read(tx, EmailVerificationByHashBkt, hashVerificationToken(tokenString), &tokenId)
	if tokenId == 0 {
		return EmailVerificationToken{}, false
	}

	var token EmailVerificationToken
	vbolt.Read(tx, EmailVerificationBkt, tokenId, &token)
	return token, token.Id != 0
}

func deleteVerificationTokenTx(tx *vbolt.Tx, token EmailVerificationToken) {
	vbolt.Delete(tx, EmailVerificationBkt, token.Id)
	vbolt.Delete(tx, EmailVerificationByHashBkt, token.TokenHash)
	vbolt.SetTargetSingleTerm(tx, EmailVerificationByUserIndex, token.Id, -1)
}

func deleteUserVerificationTokensTx(tx *vbolt.Tx, userId int) {
	var tokenIds []int
	vbolt.ReadTermTargets(tx, EmailVerificationByUserIndex, userId, &tokenIds, vbolt.Window{})
	for _, tokenId := range tokenIds {
		var token EmailVerificationToken
		vbolt.Read(tx, EmailVerificationBkt, tokenId, &token)
		if token.Id == 0 {
			vbolt.SetTargetSingleTerm(tx, EmailVerificationByUserIndex, tokenId, -1)
			continue
		}
		deleteVerificationTokenTx(tx, token)
	}
}

func markEmailVerifiedTx(tx *vbolt.Tx, userId int) User {
	user := GetUser(tx, userId)
	if user.Id == 0 || user.EmailVerified {
		return user
	}
	user.EmailVerified = true
	vbolt.Write(tx, UsersBkt, user.Id, &user)
	deleteUserVerificationTokensTx(tx, user.Id)
	return user
}

func verificationLink(token string) string {
	siteRoot := os.Getenv("SITE_ROOT")
	if siteRoot == "" {
		siteRoot = cfg.SiteURL
	}
	return strings.TrimRight(siteRoot, "/") + "/verify-email?token=" + token
}

func verificationBody(name, link string) string {
	greeting := "Hello,"
	if name != "" {
		greeting = "Hello " + name + ","
	}
	return fmt.Sprintf(`%s

Confirm this address for your Family Record account by opening the link below:

%s

This link expires in seven days. If you did not create this account, you can ignore this email.
`, greeting, link)
}

var verificationSender = deliverVerificationEmail

func deliverVerificationEmail(user User, token string) error {
	return QueueMail(MailJob{
		To:      user.Email,
		Subject: "Confirm your email for Family Record",
		Body:    verificationBody(user.Name, verificationLink(token)),
		Kind:    "email_verification",
	})
}

// Failing to send must not fail the thing that triggered it: a new account is
// still a good account, and the user can ask for another link.
func sendVerificationEmailTx(tx *vbolt.Tx, user User, now time.Time) {
	if user.Id == 0 || user.Email == "" || user.EmailVerified {
		return
	}

	token, err := createEmailVerificationTokenTx(tx, user.Id, now)
	if err != nil {
		LogErrorSimple(LogCategoryAuth, "Could not create an email verification token", map[string]interface{}{
			"userId": user.Id,
			"error":  err.Error(),
		})
		return
	}

	if err := verificationSender(user, token); err != nil {
		LogErrorSimple(LogCategoryAuth, "Could not queue a verification email", map[string]interface{}{
			"userId": user.Id,
			"error":  err.Error(),
		})
	}
}

// Deliberately not authenticated: the token is the proof, and the link is often
// opened in a different browser from the one that signed up.
func VerifyEmail(ctx *vbeam.Context, req VerifyEmailRequest) (resp VerifyEmailResponse, err error) {
	tokenString := strings.TrimSpace(req.Token)
	if tokenString == "" {
		resp.Error = "This confirmation link is missing its code."
		return
	}

	vbeam.UseWriteTx(ctx)

	token, found := getVerificationTokenTx(ctx.Tx, tokenString)
	if !found {
		resp.Error = "This confirmation link is no longer valid. Sign in and ask for a new one."
		return
	}

	if time.Now().After(token.ExpiresAt) {
		deleteVerificationTokenTx(ctx.Tx, token)
		vbolt.TxCommit(ctx.Tx)
		resp.Error = "This confirmation link has expired. Sign in and ask for a new one."
		return
	}

	user := markEmailVerifiedTx(ctx.Tx, token.UserId)
	if user.Id == 0 {
		resp.Error = "This confirmation link is no longer valid. Sign in and ask for a new one."
		return
	}

	vbolt.TxCommit(ctx.Tx)

	LogInfo(LogCategoryAuth, "Email address confirmed", map[string]interface{}{
		"userId": user.Id,
	})

	resp.Success = true
	return
}

func ResendVerificationEmail(ctx *vbeam.Context, req Empty) (resp ResendVerificationResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}

	if user.EmailVerified {
		resp.Success = true
		return
	}

	if user.Email == "" {
		resp.Error = "This account has no email address to confirm."
		return
	}

	vbeam.UseWriteTx(ctx)
	sendVerificationEmailTx(ctx.Tx, user, time.Now())
	vbolt.TxCommit(ctx.Tx)

	resp.Success = true
	return
}

func CleanupExpiredVerificationTokens(tx *vbolt.Tx, now time.Time) int {
	var expired []EmailVerificationToken
	vbolt.IterateAll(tx, EmailVerificationBkt, func(key int, token EmailVerificationToken) bool {
		if !token.ExpiresAt.After(now) {
			expired = append(expired, token)
		}
		return true
	})
	for _, token := range expired {
		deleteVerificationTokenTx(tx, token)
	}
	return len(expired)
}
