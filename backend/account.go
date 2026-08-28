package backend

import (
	"encoding/json"
	"errors"
	"net/http"

	"go.hasen.dev/vbeam"
	"go.hasen.dev/vbolt"
	"golang.org/x/crypto/bcrypt"
)

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

const incorrectPasswordMessage = "Current password is incorrect"

const noPasswordAccountMessage = "This account signs in with Google or Apple. Use the forgot-password link to set a password."

func respondChangePassword(w http.ResponseWriter, status int, resp ChangePasswordResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(resp)
}

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
		respondChangePassword(w, http.StatusBadRequest, ChangePasswordResponse{Error: noPasswordAccountMessage})
		return
	}

	if err := bcrypt.CompareHashAndPassword(passHash, []byte(req.CurrentPassword)); err != nil {
		LogWarnWithRequest(r, LogCategoryAuth, "Password change with incorrect current password", map[string]interface{}{
			"userId": user.Id,
		})
		respondChangePassword(w, http.StatusBadRequest, ChangePasswordResponse{Error: incorrectPasswordMessage})
		return
	}

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
		deleteUserPasswordResetTokensTx(tx, user.Id)
		DeleteUserRefreshTokens(tx, user.Id)
		vbolt.TxCommit(tx)
	})

	clearRefreshTokenCookie(w)

	token, tokenErr := generateAuthJwt(user, w)
	if tokenErr != nil {
		LogErrorWithRequest(r, LogCategoryAuth, "Failed to issue session after password change", map[string]interface{}{
			"userId": user.Id,
			"error":  tokenErr.Error(),
		})
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

	if sendErr := passwordChangedSender(user); sendErr != nil {
		LogErrorSimple(LogCategoryAuth, "Failed to queue password changed notice", map[string]interface{}{
			"userId": user.Id,
			"error":  sendErr.Error(),
		})
	}

	respondChangePassword(w, http.StatusOK, ChangePasswordResponse{Success: true, Token: token})
}
