package backend

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"go.hasen.dev/vbeam"
	"go.hasen.dev/vbolt"
	"golang.org/x/crypto/bcrypt"
)

type DeleteAccountRequest struct {
	Password     string `json:"password"`
	ConfirmEmail string `json:"confirmEmail"`
}

type DeleteAccountResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

const confirmEmailMismatchMessage = "Type your account's email address exactly to confirm"

const adminUndeletableMessage = "The administrator account cannot be deleted."

func respondDeleteAccount(w http.ResponseWriter, status int, resp DeleteAccountResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(resp)
}

func deleteAccountHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		vbeam.RespondError(w, errors.New("delete account call must be POST"))
		return
	}

	user, authErr := AuthenticateRequest(r)
	if authErr != nil {
		RespondAuthError(w, r, "Authentication required")
		return
	}

	// Deleting user 1 locks everyone out of /admin and silently stops the health
	// alerts, which address this account.
	if user.Id == AdminUserId {
		respondDeleteAccount(w, http.StatusForbidden, DeleteAccountResponse{Error: adminUndeletableMessage})
		return
	}

	var req DeleteAccountRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondDeleteAccount(w, http.StatusBadRequest, DeleteAccountResponse{Error: "Invalid request"})
		return
	}

	if !strings.EqualFold(strings.TrimSpace(req.ConfirmEmail), strings.TrimSpace(user.Email)) {
		respondDeleteAccount(w, http.StatusBadRequest, DeleteAccountResponse{Error: confirmEmailMismatchMessage})
		return
	}

	var passHash []byte
	vbolt.WithReadTx(appDb, func(tx *vbolt.Tx) {
		passHash = GetPassHash(tx, user.Id)
	})

	if len(passHash) > 0 {
		if err := bcrypt.CompareHashAndPassword(passHash, []byte(req.Password)); err != nil {
			LogWarnWithRequest(r, LogCategoryAuth, "Account deletion with incorrect password", map[string]interface{}{
				"userId": user.Id,
			})
			respondDeleteAccount(w, http.StatusBadRequest, DeleteAccountResponse{Error: incorrectPasswordMessage})
			return
		}
	}

	var orphanedPhotos []Image
	var destroyedFamilies []int
	vbolt.WithWriteTx(appDb, func(tx *vbolt.Tx) {
		orphanedPhotos, destroyedFamilies = deleteAccountTx(tx, user)
		vbolt.TxCommit(tx)
	})

	for _, photo := range orphanedPhotos {
		if err := deletePhotoFiles(photo); err != nil {
			LogErrorSimple(LogCategoryPhoto, "Failed to delete photo files during account deletion", map[string]interface{}{
				"photoId": photo.Id,
				"error":   err.Error(),
			})
		}
	}

	clearAuthCookies(w)

	LogInfoWithRequest(r, LogCategoryAuth, "Account deleted", map[string]interface{}{
		"userId":            user.Id,
		"destroyedFamilies": len(destroyedFamilies),
		"deletedPhotos":     len(orphanedPhotos),
	})

	respondDeleteAccount(w, http.StatusOK, DeleteAccountResponse{Success: true})
}

func clearAuthCookies(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     "authToken",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
		Expires:  time.Unix(0, 0),
	})
	clearRefreshTokenCookie(w)
}

func deleteAccountTx(tx *vbolt.Tx, user User) (orphanedPhotos []Image, destroyedFamilies []int) {
	familyIds := userFamilyIds(tx, user)

	for _, familyId := range familyIds {
		if membership, found := FindMembership(tx, user.Id, familyId); found {
			deleteMembershipTx(tx, membership)
		}
	}

	for _, familyId := range familyIds {
		if familyHasOtherMembers(tx, familyId, user.Id) {
			reassignFamilyOwnerTx(tx, familyId, user.Id)
			continue
		}
		orphanedPhotos = append(orphanedPhotos, deleteFamilyContentTx(tx, familyId)...)
		destroyedFamilies = append(destroyedFamilies, familyId)
	}

	deleteUserChatMessagesTx(tx, user.Id)
	deleteUserPushDeviceTokensTx(tx, user.Id)
	deleteNotificationPreferencesTx(tx, user.Id)
	DeleteUserRefreshTokens(tx, user.Id)
	deleteUserPasswordResetTokensTx(tx, user.Id)
	deleteUserVerificationTokensTx(tx, user.Id)

	vbolt.Delete(tx, PasswdBkt, user.Id)
	vbolt.Delete(tx, EmailBkt, user.Email)
	vbolt.SetTargetSingleTerm(tx, UsersByFamilyIndex, user.Id, -1)
	vbolt.Delete(tx, UsersBkt, user.Id)

	return
}

func familyHasOtherMembers(tx *vbolt.Tx, familyId int, excludeUserId int) bool {
	for _, userId := range GetFamilyUserIds(tx, familyId) {
		if userId != excludeUserId && userId != 0 {
			return true
		}
	}
	return false
}

func userFamilyIds(tx *vbolt.Tx, user User) []int {
	seen := make(map[int]bool)
	var familyIds []int
	for _, membership := range GetUserMemberships(tx, user.Id) {
		if membership.FamilyId == 0 || seen[membership.FamilyId] {
			continue
		}
		seen[membership.FamilyId] = true
		familyIds = append(familyIds, membership.FamilyId)
	}
	if user.FamilyId != 0 && !seen[user.FamilyId] {
		familyIds = append(familyIds, user.FamilyId)
	}
	return familyIds
}

// Photos go before the people and milestones they join to, so the join rows are
// removed while both ends still resolve.
func deleteFamilyContentTx(tx *vbolt.Tx, familyId int) (photos []Image) {
	photos = GetFamilyImages(tx, familyId)
	for _, photo := range photos {
		deletePhotoRecordTx(tx, photo)
	}

	for _, milestone := range getFamilyMilestones(tx, familyId) {
		_ = DeleteMilestoneTx(tx, milestone.Id, familyId)
	}

	for _, growth := range getFamilyGrowthData(tx, familyId) {
		_ = DeleteGrowthDataTx(tx, growth.Id, familyId)
	}

	for _, message := range GetFamilyChatMessages(tx, familyId, 0, 0) {
		_ = DeleteChatMessageTx(tx, message.Id, familyId)
	}

	for _, tag := range getTagsByFamily(tx, familyId) {
		deleteTagTx(tx, tag)
	}

	deleteFamilyActivitiesTx(tx, familyId)

	for _, person := range GetFamilyOwnPeople(tx, familyId) {
		deletePersonRecordTx(tx, person)
	}

	for _, row := range GetFamilyRoster(tx, familyId) {
		deletePersonFamilyTx(tx, row)
	}

	for _, link := range familyLinksTouching(tx, familyId) {
		deleteFamilyLinkTx(tx, link)
	}

	family := GetFamily(tx, familyId)
	if family.InviteCode != "" {
		vbolt.Delete(tx, InviteCodeBkt, family.InviteCode)
	}
	vbolt.Delete(tx, FamiliesBkt, familyId)

	return
}

func deletePersonRecordTx(tx *vbolt.Tx, person Person) {
	for _, photoPerson := range GetPhotoPersonsByPerson(tx, person.Id) {
		vbolt.Delete(tx, PhotoPersonBkt, photoPerson.Id)
		vbolt.SetTargetSingleTerm(tx, PhotoPersonByPhotoIndex, photoPerson.Id, -1)
		vbolt.SetTargetSingleTerm(tx, PhotoPersonByPersonIndex, photoPerson.Id, -1)
		vbolt.SetTargetSingleTerm(tx, PhotoPersonByFamilyIndex, photoPerson.Id, -1)
	}

	removePersonFromActivitiesTx(tx, person.Id)
	deletePersonRostersTx(tx, person.Id)
	vbolt.Delete(tx, PeopleBkt, person.Id)
	vbolt.SetTargetSingleTerm(tx, PersonIndex, person.Id, -1)
}

func familyLinksTouching(tx *vbolt.Tx, familyId int) []FamilyLink {
	seen := make(map[int]bool)
	var links []FamilyLink
	for _, link := range append(GetLinksFromFamily(tx, familyId), GetLinksToFamily(tx, familyId)...) {
		if link.Id == 0 || seen[link.Id] {
			continue
		}
		seen[link.Id] = true
		links = append(links, link)
	}
	return links
}

func deleteFamilyLinkTx(tx *vbolt.Tx, link FamilyLink) {
	vbolt.Delete(tx, FamilyLinkBkt, link.Id)
	vbolt.SetTargetSingleTerm(tx, FamilyLinkByFromIndex, link.Id, -1)
	vbolt.SetTargetSingleTerm(tx, FamilyLinkByToIndex, link.Id, -1)
}

func deleteUserChatMessagesTx(tx *vbolt.Tx, userId int) {
	var messageIds []int
	vbolt.ReadTermTargets(tx, ChatMessagesByUserIndex, userId, &messageIds, vbolt.Window{})

	for _, messageId := range messageIds {
		message := GetChatMessageById(tx, messageId)
		if message.Id == 0 {
			continue
		}
		_ = DeleteChatMessageTx(tx, message.Id, message.FamilyId)
	}
}

func deleteUserPushDeviceTokensTx(tx *vbolt.Tx, userId int) {
	var tokenIds []int
	vbolt.ReadTermTargets(tx, PushDeviceTokenByUserIndex, userId, &tokenIds, vbolt.Window{})

	for _, tokenId := range tokenIds {
		device := GetPushDeviceTokenById(tx, tokenId)
		if device.Id == 0 {
			continue
		}
		vbolt.Delete(tx, PushDeviceTokenBkt, device.Id)
		vbolt.Delete(tx, PushDeviceTokenByTokenBkt, device.Token)
		vbolt.SetTargetSingleTerm(tx, PushDeviceTokenByUserIndex, device.Id, -1)
	}
}
