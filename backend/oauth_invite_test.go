package backend

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"go.hasen.dev/vbolt"
)

func TestRememberOAuthInvite(t *testing.T) {
	t.Run("StoresTheCode", func(t *testing.T) {
		rec := httptest.NewRecorder()
		rememberOAuthInvite(rec, httptest.NewRequest(http.MethodGet, "/api/login/google?code=abcd1234", nil))

		cookie := findCookie(rec, oauthInviteCookieName)
		if cookie == nil || cookie.Value != "abcd1234" {
			t.Fatalf("cookie = %+v, want the invite code", cookie)
		}
		if cookie.SameSite != http.SameSiteNoneMode || !cookie.Secure || !cookie.HttpOnly {
			t.Errorf("cookie = %+v, want HttpOnly, Secure, SameSite=None to survive Apple's cross-site POST", cookie)
		}
	})

	t.Run("ClearsALeftoverCodeWhenThereIsNone", func(t *testing.T) {
		rec := httptest.NewRecorder()
		rememberOAuthInvite(rec, httptest.NewRequest(http.MethodGet, "/api/login/google", nil))

		cookie := findCookie(rec, oauthInviteCookieName)
		if cookie == nil || cookie.Value != "" || cookie.Expires.After(time.Unix(1, 0)) {
			t.Fatalf("cookie = %+v, want it cleared", cookie)
		}
	})
}

func TestTakeOAuthInviteClearsTheCookie(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/google/callback", nil)
	req.AddCookie(&http.Cookie{Name: oauthInviteCookieName, Value: "abcd1234"})
	rec := httptest.NewRecorder()

	if got := takeOAuthInvite(rec, req); got != "abcd1234" {
		t.Errorf("code = %q, want abcd1234", got)
	}
	if cookie := findCookie(rec, oauthInviteCookieName); cookie == nil || cookie.Value != "" {
		t.Errorf("cookie = %+v, want it cleared once spent", cookie)
	}
}

func TestAppleCallbackFollowsAnInvite(t *testing.T) {
	t.Run("NewAccountJoinsTheInvitingFamily", func(t *testing.T) {
		family := setupInviteTest(t)

		rec := runAppleCallbackWithInvite(t, "invitee@example.com", family.InviteCode)
		if rec.Code != http.StatusFound {
			t.Fatalf("status = %d (%s), want %d", rec.Code, rec.Body.String(), http.StatusFound)
		}

		var user User
		var familyCount int
		vbolt.WithReadTx(appDb, func(tx *vbolt.Tx) {
			user = GetUser(tx, GetUserId(tx, "invitee@example.com"))
			vbolt.IterateAll(tx, FamiliesBkt, func(id int, f Family) bool {
				familyCount++
				return true
			})
		})
		if user.FamilyId != family.Id {
			t.Errorf("family = %d, want the inviting family %d", user.FamilyId, family.Id)
		}
		if familyCount != 1 {
			t.Errorf("families = %d, want no second family made for the invitee", familyCount)
		}
	})

	t.Run("ExistingAccountIsAddedToTheInvitingFamily", func(t *testing.T) {
		family := setupInviteTest(t)

		var existing User
		vbolt.WithWriteTx(appDb, func(tx *vbolt.Tx) {
			existing = AddUserTx(tx, CreateAccountRequest{Name: "Existing", Email: "existing@example.com"}, []byte{})
			vbolt.TxCommit(tx)
		})

		rec := runAppleCallbackWithInvite(t, "existing@example.com", family.InviteCode)
		if rec.Code != http.StatusFound {
			t.Fatalf("status = %d (%s), want %d", rec.Code, rec.Body.String(), http.StatusFound)
		}

		var member bool
		var user User
		vbolt.WithReadTx(appDb, func(tx *vbolt.Tx) {
			_, member = FindMembership(tx, existing.Id, family.Id)
			user = GetUser(tx, existing.Id)
		})
		if !member {
			t.Error("the existing account was not added to the inviting family")
		}
		if user.FamilyId != existing.FamilyId {
			t.Errorf("primary family = %d, want it unchanged at %d", user.FamilyId, existing.FamilyId)
		}
	})

	t.Run("UnknownCodeStillCreatesAnAccount", func(t *testing.T) {
		setupInviteTest(t)

		rec := runAppleCallbackWithInvite(t, "stray@example.com", "nosuchcode")
		if rec.Code != http.StatusFound {
			t.Fatalf("status = %d (%s), want %d", rec.Code, rec.Body.String(), http.StatusFound)
		}

		var user User
		vbolt.WithReadTx(appDb, func(tx *vbolt.Tx) {
			user = GetUser(tx, GetUserId(tx, "stray@example.com"))
		})
		if user.Id == 0 || user.FamilyId == 0 {
			t.Errorf("user = %+v, want an account with a family of its own", user)
		}
	})
}

func TestAppleCancelReturnsToTheInvite(t *testing.T) {
	configureAppleForTest(t)
	appleTestDB(t)

	req := appleCallbackRequest(url.Values{"error": {"user_cancelled_authorize"}}, "expected.nonce123")
	req.AddCookie(&http.Cookie{Name: oauthInviteCookieName, Value: "abcd1234"})
	rec := httptest.NewRecorder()
	appleCallbackHandler(rec, req)

	if got := rec.Header().Get("Location"); got != "/create-account?code=abcd1234" {
		t.Errorf("Location = %q, want the invite page so the code is not lost", got)
	}
}

func setupInviteTest(t *testing.T) Family {
	t.Helper()
	configureAppleForTest(t)
	appleTestDB(t)

	var family Family
	vbolt.WithWriteTx(appDb, func(tx *vbolt.Tx) {
		family = createFamilyTx(tx, "Inviting Family", 0)
		vbolt.TxCommit(tx)
	})
	return family
}

func runAppleCallbackWithInvite(t *testing.T, email, inviteCode string) *httptest.ResponseRecorder {
	t.Helper()
	key := startAppleKeyServer(t)

	claims := appleClaims("app.familyrecord.web")
	claims["email"] = email
	claims["nonce"] = "nonce123"
	idToken := signAppleIDToken(t, key, claims)

	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"id_token":%q}`, idToken)
	}))
	t.Cleanup(tokenServer.Close)

	originalEndpoint := appleTokenEndpoint
	appleTokenEndpoint = tokenServer.URL
	t.Cleanup(func() { appleTokenEndpoint = originalEndpoint })

	req := appleCallbackRequest(url.Values{"state": {"expected"}, "code": {"the-code"}}, "expected.nonce123")
	req.AddCookie(&http.Cookie{Name: oauthInviteCookieName, Value: inviteCode})
	rec := httptest.NewRecorder()
	appleCallbackHandler(rec, req)
	return rec
}

func findCookie(rec *httptest.ResponseRecorder, name string) *http.Cookie {
	for _, cookie := range rec.Result().Cookies() {
		if cookie.Name == name {
			return cookie
		}
	}
	return nil
}

func TestAppleTokenLoginFollowsAnInvite(t *testing.T) {
	family := setupInviteTest(t)
	key := startAppleKeyServer(t)

	claims := appleClaims("app.familyrecord.ios")
	claims["email"] = "app-invitee@example.com"
	body, _ := json.Marshal(AppleTokenLoginRequest{
		IDToken:    signAppleIDToken(t, key, claims),
		FamilyCode: family.InviteCode,
	})

	resp := decodeLoginResponse(t, appleTokenLoginRequest(string(body)))
	if !resp.Success {
		t.Fatalf("response = %+v, want success", resp)
	}

	var user User
	vbolt.WithReadTx(appDb, func(tx *vbolt.Tx) {
		user = GetUser(tx, GetUserId(tx, "app-invitee@example.com"))
	})
	if user.FamilyId != family.Id {
		t.Errorf("family = %d, want the inviting family %d", user.FamilyId, family.Id)
	}
}
