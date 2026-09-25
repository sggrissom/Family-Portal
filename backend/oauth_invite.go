package backend

import (
	"net/http"
	"time"

	"go.hasen.dev/vbolt"
)

// An invite link sends a newcomer through Google or Apple, and the provider's
// callback knows nothing of the link. The code rides along in this cookie.
const (
	oauthInviteCookieName = "oauthInviteCode"
	oauthInviteLifetime   = 10 * time.Minute
	maxInviteCodeLength   = 64
)

// rememberOAuthInvite runs at the start of every browser sign-in. A sign-in
// without a code clears any code left over from an abandoned one, so it cannot
// land a later, unrelated sign-in in someone else's family.
func rememberOAuthInvite(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	if code == "" || len(code) > maxInviteCodeLength {
		clearOAuthInvite(w)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     oauthInviteCookieName,
		Value:    code,
		Path:     "/api",
		HttpOnly: true,
		Secure:   true,
		// Apple's callback is a cross-site POST, which a Lax cookie misses.
		SameSite: http.SameSiteNoneMode,
		MaxAge:   int(oauthInviteLifetime.Seconds()),
	})
}

func clearOAuthInvite(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     oauthInviteCookieName,
		Value:    "",
		Path:     "/api",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteNoneMode,
		Expires:  time.Unix(0, 0),
	})
}

func takeOAuthInvite(w http.ResponseWriter, r *http.Request) string {
	cookie, err := r.Cookie(oauthInviteCookieName)
	if err != nil || cookie.Value == "" {
		return ""
	}
	clearOAuthInvite(w)
	return cookie.Value
}

// joinFamilyByInviteTx adds an existing account to the family an invite code
// names. It reports whether anything changed; an unknown code or an existing
// membership is not an error.
func joinFamilyByInviteTx(tx *vbolt.Tx, user User, inviteCode string) (User, bool) {
	if inviteCode == "" {
		return user, false
	}
	family := GetFamilyByInviteCode(tx, inviteCode)
	if family.Id == 0 {
		return user, false
	}
	if _, alreadyMember := FindMembership(tx, user.Id, family.Id); alreadyMember || user.FamilyId == family.Id {
		return user, false
	}

	EnsureMembershipTx(tx, user.Id, family.Id, AccessAdmin)
	if user.FamilyId == 0 {
		user.FamilyId = family.Id
		vbolt.Write(tx, UsersBkt, user.Id, &user)
		vbolt.SetTargetSingleTerm(tx, UsersByFamilyIndex, user.Id, user.FamilyId)
	}
	return user, true
}
