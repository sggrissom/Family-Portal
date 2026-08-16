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

// Account deletion.
//
// What goes and what stays
// ------------------------
// The account goes: credentials, sessions, refresh tokens, reset links, push
// device registrations, and the user record itself. So does the account's own
// speech — chat messages are deleted, because chat is the one store that holds
// a person's words under their name, and a request to delete an account is
// partly a request to stop being present.
//
// Family records stay with the family, exactly as they do when somebody merely
// leaves: people, growth measurements, milestones, photos and tags are about
// the children, not about whichever adult typed them in, and a household should
// not lose half its history because one member closed their account.
//
// The exception is a family nobody is left in. Once the last membership row is
// gone there is no route back to that household's content — it would sit on
// disk forever, unreachable and undeletable. So a family the deletion empties is
// destroyed with everything in it, including the photo files and the face
// descriptors derived from them. That is also what makes "delete my account"
// honest for the single-user case, which is most of them.
type DeleteAccountRequest struct {
	// Password is required when the account has one. A Google-only account has
	// no password to prove; ConfirmEmail carries the whole weight there.
	Password string `json:"password"`
	// ConfirmEmail must match the account's address. Typing it is the "are you
	// sure" gate that a checkbox is not.
	ConfirmEmail string `json:"confirmEmail"`
}

type DeleteAccountResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

const confirmEmailMismatchMessage = "Type your account's email address exactly to confirm"

func respondDeleteAccount(w http.ResponseWriter, status int, resp DeleteAccountResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(resp)
}

// deleteAccountHandler removes the caller's account. Like the password change
// it is a plain handler rather than a proc, because it has to clear the
// session cookies it is invalidating.
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

	// An account with a password must prove it. One without signs in through
	// Google, where the live session is the only proof available.
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

	// Files come after the commit. The database is the record of what exists;
	// once it says the photo is gone, a slow or failing filesystem must not be
	// able to roll that back or hold the write lock while it works.
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

// clearAuthCookies expires both session cookies. Logout does the same thing
// inline; deletion needs it without the rest of the logout path, which assumes
// the account still exists.
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

// deleteAccountTx removes the user and every family the removal empties.
//
// It returns the photos whose files still need deleting, and the families that
// were destroyed, because both are things the caller reports on after the
// transaction has committed.
func deleteAccountTx(tx *vbolt.Tx, user User) (orphanedPhotos []Image, destroyedFamilies []int) {
	familyIds := userFamilyIds(tx, user)

	// Drop every membership first, so "does this family still have members?"
	// below is asking about the state after the deletion rather than before it.
	for _, familyId := range familyIds {
		if membership, found := FindMembership(tx, user.Id, familyId); found {
			deleteMembershipTx(tx, membership)
		}
	}

	for _, familyId := range familyIds {
		if familyHasOtherMembers(tx, familyId, user.Id) {
			// Somebody is still here. The household survives with its content,
			// but must not be left pointing at a departed owner.
			reassignFamilyOwnerTx(tx, familyId, user.Id)
			continue
		}
		orphanedPhotos = append(orphanedPhotos, deleteFamilyContentTx(tx, familyId)...)
		destroyedFamilies = append(destroyedFamilies, familyId)
	}

	deleteUserChatMessagesTx(tx, user.Id)
	deleteUserPushDeviceTokensTx(tx, user.Id)
	DeleteUserRefreshTokens(tx, user.Id)
	deleteUserPasswordResetTokensTx(tx, user.Id)

	vbolt.Delete(tx, PasswdBkt, user.Id)
	vbolt.Delete(tx, EmailBkt, user.Email)
	vbolt.SetTargetSingleTerm(tx, UsersByFamilyIndex, user.Id, -1)
	vbolt.Delete(tx, UsersBkt, user.Id)

	return
}

// familyHasOtherMembers reports whether anybody besides excludeUserId is still
// in the family.
//
// It asks GetFamilyUserIds rather than counting membership rows, because that
// function unions the membership table with the primary-family index. The two
// should agree; if they ever disagree, this errs toward keeping a family that
// might still have somebody in it, and the cost of being wrong the other way is
// destroying a household's photos.
func familyHasOtherMembers(tx *vbolt.Tx, familyId int, excludeUserId int) bool {
	for _, userId := range GetFamilyUserIds(tx, familyId) {
		if userId != excludeUserId && userId != 0 {
			return true
		}
	}
	return false
}

// userFamilyIds is every family the user belongs to, including their primary
// one if a membership row for it somehow went missing. Deletion is the wrong
// place to trust an invariant: a family skipped here is a family that can never
// be reached again.
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

// deleteFamilyContentTx destroys a family and everything it owns, returning the
// photos whose files the caller must remove.
//
// Order matters only where one store's cleanup reads another: photos go before
// the people and milestones they join to, so the join rows are removed while
// both ends still resolve.
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

	// The family's own people, and with them the face descriptors derived from
	// their photos.
	for _, person := range GetFamilyOwnPeople(tx, familyId) {
		deletePersonRecordTx(tx, person)
	}

	// Whatever is left on this family's roster belongs to another household —
	// people shared in by a link. The roster row goes; the person does not.
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

// deletePersonRecordTx removes a person, their roster rows, and any photo tag
// still pointing at them. The photo joins are normally gone already — the
// family's photos were deleted first — but a person can be tagged in a photo
// owned by a family that is not being deleted, and that row must not outlive
// the person it names.
func deletePersonRecordTx(tx *vbolt.Tx, person Person) {
	for _, photoPerson := range GetPhotoPersonsByPerson(tx, person.Id) {
		vbolt.Delete(tx, PhotoPersonBkt, photoPerson.Id)
		vbolt.SetTargetSingleTerm(tx, PhotoPersonByPhotoIndex, photoPerson.Id, -1)
		vbolt.SetTargetSingleTerm(tx, PhotoPersonByPersonIndex, photoPerson.Id, -1)
		vbolt.SetTargetSingleTerm(tx, PhotoPersonByFamilyIndex, photoPerson.Id, -1)
	}

	deletePersonRostersTx(tx, person.Id)
	vbolt.Delete(tx, PeopleBkt, person.Id)
	vbolt.SetTargetSingleTerm(tx, PersonIndex, person.Id, -1)
}

// familyLinksTouching returns every link with this family on either end, each
// one once.
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

// deleteUserChatMessagesTx removes everything the user said, in every family,
// including families that survive the deletion.
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

// deleteUserPushDeviceTokensTx removes every device registered to the account,
// active or not. Deactivating would be enough to stop notifications, but the
// row holds an APNs token that identifies a physical device, and deletion is
// supposed to mean deletion.
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
