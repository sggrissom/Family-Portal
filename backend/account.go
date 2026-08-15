package backend

import (
	"encoding/json"
	"errors"
	"net/http"

	"go.hasen.dev/vbeam"
	"go.hasen.dev/vbolt"
	"golang.org/x/crypto/bcrypt"
)

// Account self-service lives on plain HTTP handlers rather than vbeam procs
// because it has to rewrite the caller's cookies. A proc receives only a token
// and a transaction (vbeam.Context), so it cannot clear the refresh cookie or
// hand back the replacement one — and a password change that cannot reissue the
// current session can only sign the user out of the browser they are using.
func RegisterAccountHandlers(app *vbeam.Application) {
	app.HandleFunc("/api/change-password", changePasswordHandler)
	app.HandleFunc("/api/delete-account", deleteAccountHandler)
}

type ChangePasswordRequest struct {
	CurrentPassword string `json:"currentPassword"`
	NewPassword     string `json:"newPassword"`
	ConfirmPassword string `json:"confirmPassword"`
}

type ChangePasswordResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
	Token   string `json:"token,omitempty"`
}

// incorrectPasswordMessage answers a failed current-password check. The caller
// is already authenticated, so this leaks nothing about who has an account; it
// is kept as one constant so the wording cannot drift between call sites.
const incorrectPasswordMessage = "Current password is incorrect"

// googleOnlyAccountMessage is what an account with no password on file gets.
// Such accounts sign in through Google, so there is no current password to
// verify against and no safe way to set a first one from an authenticated
// session alone — proving control of the mailbox is the path that exists.
const googleOnlyAccountMessage = "This account signs in with Google. Use the forgot-password link to set a password."

func respondChangePassword(w http.ResponseWriter, status int, resp ChangePasswordResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(resp)
}

// changePasswordHandler replaces the caller's password after verifying the one
// they already have.
//
// Every other session is revoked. A password change is the action a user takes
// when they think somebody else is in their account, so leaving other sessions
// alive would defeat the point: the intruder's refresh token would outlive the
// credential it was obtained with. The browser making the request is issued a
// fresh session in the same response, so the user is not signed out of the tab
// they are looking at.
func changePasswordHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		vbeam.RespondError(w, errors.New("change password call must be POST"))
		return
	}

	user, authErr := AuthenticateRequest(r)
	if authErr != nil {
		RespondAuthError(w, r, "Authentication required")
		return
	}

	var req ChangePasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondChangePassword(w, http.StatusBadRequest, ChangePasswordResponse{Error: "Invalid request"})
		return
	}

	var passHash []byte
	vbolt.WithReadTx(appDb, func(tx *vbolt.Tx) {
		passHash = GetPassHash(tx, user.Id)
	})

	if len(passHash) == 0 {
		respondChangePassword(w, http.StatusBadRequest, ChangePasswordResponse{Error: googleOnlyAccountMessage})
		return
	}

	if err := bcrypt.CompareHashAndPassword(passHash, []byte(req.CurrentPassword)); err != nil {
		LogWarnWithRequest(r, LogCategoryAuth, "Password change with incorrect current password", map[string]interface{}{
			"userId": user.Id,
		})
		respondChangePassword(w, http.StatusBadRequest, ChangePasswordResponse{Error: incorrectPasswordMessage})
		return
	}

	// Validated after the current password is confirmed, so the rules on the new
	// password are never a way to probe the old one.
	if err := validateNewPassword(req.NewPassword, req.ConfirmPassword); err != nil {
		respondChangePassword(w, http.StatusBadRequest, ChangePasswordResponse{Error: err.Error()})
		return
	}

	newHash, hashErr := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if hashErr != nil {
		LogErrorWithRequest(r, LogCategoryAuth, "Failed to hash new password", map[string]interface{}{
			"userId": user.Id,
			"error":  hashErr.Error(),
		})
		respondChangePassword(w, http.StatusInternalServerError, ChangePasswordResponse{Error: "Failed to process password"})
		return
	}

	vbolt.WithWriteTx(appDb, func(tx *vbolt.Tx) {
		vbolt.Write(tx, PasswdBkt, user.Id, &newHash)
		// An outstanding reset link was issued against the old password and must
		// not survive it.
		deleteUserPasswordResetTokensTx(tx, user.Id)
		DeleteUserRefreshTokens(tx, user.Id)
		vbolt.TxCommit(tx)
	})

	// Clear first: if the new session cannot be issued the browser is left
	// holding nothing rather than a cookie naming a revoked token.
	clearRefreshTokenCookie(w)

	token, tokenErr := generateAuthJwt(user, w)
	if tokenErr != nil {
		LogErrorWithRequest(r, LogCategoryAuth, "Failed to issue session after password change", map[string]interface{}{
			"userId": user.Id,
			"error":  tokenErr.Error(),
		})
		// The password did change, so saying otherwise would send the user back
		// to try again with a password that no longer works. Signing in again is
		// the recovery, and it is what the message asks for.
		respondChangePassword(w, http.StatusOK, ChangePasswordResponse{
			Success: true,
			Error:   "Password changed. Please sign in again.",
		})
		return
	}

	LogInfoWithRequest(r, LogCategoryAuth, "Password changed", map[string]interface{}{
		"userId": user.Id,
		"email":  redactEmail(user.Email),
	})

	// The change is already durable, so a failed notice is logged rather than
	// reported — the same reasoning as the reset flow.
	if sendErr := passwordChangedSender(user); sendErr != nil {
		LogErrorSimple(LogCategoryAuth, "Failed to queue password changed notice", map[string]interface{}{
			"userId": user.Id,
			"error":  sendErr.Error(),
		})
	}

	respondChangePassword(w, http.StatusOK, ChangePasswordResponse{Success: true, Token: token})
}
