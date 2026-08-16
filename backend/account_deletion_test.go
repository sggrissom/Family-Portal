package backend

import (
	"encoding/json"
	"family/cfg"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.hasen.dev/vbolt"
	"golang.org/x/crypto/bcrypt"
)

// deletionFixture is one household with a full set of records in it, plus a
// second unrelated family. Every store the deletion is supposed to clear has
// something in it, so a store the code forgets shows up as a leftover.
type deletionFixture struct {
	db *vbolt.DB

	owner     User
	ownerAuth string
	familyId  int

	person    Person
	growth    GrowthData
	milestone Milestone
	photo     Image
	tag       Tag
	message   ChatMessage
	device    string

	outsider       User
	outsiderFamily int
	outsiderPerson Person
}

func setupDeletionFixture(t *testing.T) deletionFixture {
	t.Helper()

	db := vbolt.Open(t.TempDir() + "/deletion.db")
	vbolt.InitBuckets(db, &cfg.Info)
	t.Cleanup(func() { _ = db.Close() })
	appDb = db
	jwtKey = []byte("deletion-test-secret-key-at-least-32")

	fx := deletionFixture{db: db}
	hash, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.MinCost)

	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		var err error
		fx.owner = AddUserTx(tx, CreateAccountRequest{Name: "Owner", Email: "owner@example.com"}, hash)
		fx.familyId = fx.owner.FamilyId
		fx.outsider = AddUserTx(tx, CreateAccountRequest{Name: "Outsider", Email: "outsider@example.com"}, hash)
		fx.outsiderFamily = fx.outsider.FamilyId

		fx.person, err = AddPersonTx(tx, AddPersonRequest{
			Name: "Kid", PersonType: 1, Gender: 0, Birthdate: "2020-06-15",
		}, fx.familyId)
		if err != nil {
			t.Fatalf("AddPersonTx() error = %v", err)
		}
		fx.outsiderPerson, err = AddPersonTx(tx, AddPersonRequest{
			Name: "Their Kid", PersonType: 1, Gender: 1, Birthdate: "2021-01-05",
		}, fx.outsiderFamily)
		if err != nil {
			t.Fatalf("AddPersonTx(outsider) error = %v", err)
		}

		// A face descriptor, which is the derived data the plan calls out.
		person := GetPersonById(tx, fx.person.Id)
		person.FaceDescriptor = make([]float32, 128)
		vbolt.Write(tx, PeopleBkt, person.Id, &person)

		measurementDate := "2024-01-05"
		fx.growth, err = AddGrowthDataTx(tx, AddGrowthDataRequest{
			PersonId: fx.person.Id, MeasurementType: "height", Value: 90, Unit: "cm",
			InputType: "date", MeasurementDate: &measurementDate,
		}, fx.familyId)
		if err != nil {
			t.Fatalf("AddGrowthDataTx() error = %v", err)
		}

		milestoneDate := "2024-02-02"
		fx.milestone, err = AddMilestoneTx(tx, AddMilestoneRequest{
			PersonId: fx.person.Id, Description: "First words", Category: "development",
			InputType: "date", MilestoneDate: &milestoneDate,
		}, fx.familyId)
		if err != nil {
			t.Fatalf("AddMilestoneTx() error = %v", err)
		}

		fx.tag = Tag{
			Id: vbolt.NextIntId(tx, TagBkt), FamilyId: fx.familyId,
			Name: "Holiday", Color: "#112233", CreatedAt: time.Now(),
		}
		vbolt.Write(tx, TagBkt, fx.tag.Id, &fx.tag)
		vbolt.SetTargetSingleTerm(tx, TagByFamilyIndex, fx.tag.Id, fx.familyId)

		fx.photo = Image{
			Id:       vbolt.NextIntId(tx, ImagesBkt),
			FamilyId: fx.familyId,
			FilePath: "photos/deletion-test.jpg",
			Title:    "A photo",
		}
		vbolt.Write(tx, ImagesBkt, fx.photo.Id, &fx.photo)
		vbolt.SetTargetSingleTerm(tx, ImageByFamilyIndex, fx.photo.Id, fx.familyId)
		tagPersonInPhoto(tx, fx.photo.Id, fx.person.Id, fx.familyId)

		fx.message, err = AddChatMessageTx(tx, SendMessageRequest{
			Content: "hello", ClientMessageId: "m-1",
		}, fx.familyId, fx.owner.Id, fx.owner.Name)
		if err != nil {
			t.Fatalf("AddChatMessageTx() error = %v", err)
		}

		fx.device = strings.Repeat("c", apnsDeviceTokenHexLength)
		if _, err := upsertPushDeviceToken(tx, fx.owner.Id, RegisterPushDeviceRequest{
			Token: fx.device, Platform: "ios", Environment: "sandbox", BundleId: "com.example.family",
		}); err != nil {
			t.Fatalf("upsertPushDeviceToken() error = %v", err)
		}

		if _, err := createPasswordResetTokenTx(tx, fx.owner.Id, time.Now()); err != nil {
			t.Fatalf("createPasswordResetTokenTx() error = %v", err)
		}
		if _, _, err := CreateRefreshToken(tx, fx.owner.Id, refreshTokenLifetime); err != nil {
			t.Fatalf("CreateRefreshToken() error = %v", err)
		}

		vbolt.TxCommit(tx)
	})

	var err error
	fx.ownerAuth, err = generateJwtTokenString(fx.owner)
	if err != nil {
		t.Fatalf("generateJwtTokenString() error = %v", err)
	}
	return fx
}

func deleteAccountRequest(t *testing.T, authToken, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/api/delete-account", strings.NewReader(body))
	if authToken != "" {
		req.AddCookie(&http.Cookie{Name: "authToken", Value: authToken})
	}
	recorder := httptest.NewRecorder()
	deleteAccountHandler(recorder, req)
	return recorder
}

func decodeDeleteAccount(t *testing.T, recorder *httptest.ResponseRecorder) DeleteAccountResponse {
	t.Helper()

	var resp DeleteAccountResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response %q: %v", recorder.Body.String(), err)
	}
	return resp
}

// countRows reports how many rows a bucket holds, which is how the "every store
// is clear" assertion avoids depending on the ids it happened to write.
func countRows[T any](t *testing.T, db *vbolt.DB, bkt *vbolt.BucketInfo[int, T]) int {
	t.Helper()

	count := 0
	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		vbolt.IterateAll(tx, bkt, func(int, T) bool {
			count++
			return true
		})
	})
	return count
}

func TestDeleteAccountClearsEveryStore(t *testing.T) {
	fx := setupDeletionFixture(t)

	recorder := deleteAccountRequest(t, fx.ownerAuth,
		`{"password":"password123","confirmEmail":"owner@example.com"}`)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	if resp := decodeDeleteAccount(t, recorder); !resp.Success {
		t.Fatalf("success = false, error = %q", resp.Error)
	}

	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		if GetUser(tx, fx.owner.Id).Id != 0 {
			t.Error("user record survived")
		}
		if GetUserId(tx, fx.owner.Email) != 0 {
			t.Error("email index still resolves to the deleted user")
		}
		if len(GetPassHash(tx, fx.owner.Id)) != 0 {
			t.Error("password hash survived")
		}
		if len(GetUserMemberships(tx, fx.owner.Id)) != 0 {
			t.Error("membership rows survived")
		}
		if GetFamily(tx, fx.familyId).Id != 0 {
			t.Error("the emptied family survived")
		}

		// Everything the family owned.
		if GetPersonById(tx, fx.person.Id).Id != 0 {
			t.Error("person survived, and with it their face descriptor")
		}
		if GetImageById(tx, fx.photo.Id).Id != 0 {
			t.Error("photo survived")
		}
		if len(getFamilyGrowthData(tx, fx.familyId)) != 0 {
			t.Error("growth data survived")
		}
		if len(getFamilyMilestones(tx, fx.familyId)) != 0 {
			t.Error("milestones survived")
		}
		if len(getTagsByFamily(tx, fx.familyId)) != 0 {
			t.Error("tags survived")
		}
		if len(GetFamilyChatMessages(tx, fx.familyId, 0, 0)) != 0 {
			t.Error("chat messages survived")
		}
		if len(GetFamilyRoster(tx, fx.familyId)) != 0 {
			t.Error("roster rows survived")
		}
		if GetPushDeviceTokenByToken(tx, fx.device).Id != 0 {
			t.Error("push device token survived")
		}
	})

	// Nothing may be left pointing at the deleted account or its family. The
	// outsider's own records are the control: they must still be there.
	if got := countRows(t, fx.db, UsersBkt); got != 1 {
		t.Errorf("users remaining = %d, want 1 (the outsider)", got)
	}
	if got := countRows(t, fx.db, PeopleBkt); got != 1 {
		t.Errorf("people remaining = %d, want 1 (the outsider's)", got)
	}
	for name, got := range map[string]int{
		"images":         countRows(t, fx.db, ImagesBkt),
		"photo_person":   countRows(t, fx.db, PhotoPersonBkt),
		"growth":         countRows(t, fx.db, GrowthDataBkt),
		"milestones":     countRows(t, fx.db, MilestoneBkt),
		"tags":           countRows(t, fx.db, TagBkt),
		"chat":           countRows(t, fx.db, ChatMessagesBkt),
		"refresh tokens": countRows(t, fx.db, RefreshTokenBkt),
		"reset tokens":   countRows(t, fx.db, PasswordResetBkt),
		"device tokens":  countRows(t, fx.db, PushDeviceTokenBkt),
	} {
		if got != 0 {
			t.Errorf("%s remaining = %d, want 0", name, got)
		}
	}

	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		if GetPersonById(tx, fx.outsiderPerson.Id).Id == 0 {
			t.Error("another family's person was deleted")
		}
		if GetUser(tx, fx.outsider.Id).Id == 0 {
			t.Error("another family's user was deleted")
		}
	})
}

func TestDeleteAccountDeletesOrphanedPhotoFiles(t *testing.T) {
	fx := setupDeletionFixture(t)

	// Stand in for the variants a processed photo leaves on disk.
	photosDir := filepath.Join(cfg.StaticDir, "photos")
	if err := os.MkdirAll(photosDir, 0755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	written := []string{
		filepath.Join(photosDir, "deletion-test.jpg"),
		filepath.Join(photosDir, "deletion-test_thumb.webp"),
		filepath.Join(photosDir, "deletion-test_original.jpg"),
	}
	for _, path := range written {
		if err := os.WriteFile(path, []byte("x"), 0644); err != nil {
			t.Fatalf("WriteFile(%s) error = %v", path, err)
		}
		t.Cleanup(func() { _ = os.Remove(path) })
	}

	recorder := deleteAccountRequest(t, fx.ownerAuth,
		`{"password":"password123","confirmEmail":"owner@example.com"}`)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}

	for _, path := range written {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("%s still on disk after account deletion", path)
		}
	}
}

func TestDeleteAccountRevokesSessions(t *testing.T) {
	fx := setupDeletionFixture(t)

	var live string
	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		_, tokenString, err := CreateRefreshToken(tx, fx.owner.Id, refreshTokenLifetime)
		if err != nil {
			t.Fatalf("CreateRefreshToken() error = %v", err)
		}
		live = tokenString
		vbolt.TxCommit(tx)
	})

	recorder := deleteAccountRequest(t, fx.ownerAuth,
		`{"password":"password123","confirmEmail":"owner@example.com"}`)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}

	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		if _, valid := ValidateRefreshToken(tx, live); valid {
			t.Error("a refresh token outlived the account it belonged to")
		}
	})

	// Both session cookies are cleared, so the browser is not left holding a
	// JWT that still parses for another day.
	cleared := map[string]bool{}
	for _, cookie := range recorder.Result().Cookies() {
		if cookie.Value == "" {
			cleared[cookie.Name] = true
		}
	}
	for _, name := range []string{"authToken", "refreshToken"} {
		if !cleared[name] {
			t.Errorf("%s cookie was not cleared", name)
		}
	}

	// The JWT no longer authenticates anything, because the user is gone.
	if _, err := AuthenticateRequest(authedRequest(fx.ownerAuth)); err == nil {
		t.Error("the deleted account's token still authenticates")
	}
}

func authedRequest(token string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: "authToken", Value: token})
	return req
}

func TestDeleteAccountLeavesASharedFamilyIntact(t *testing.T) {
	fx := setupDeletionFixture(t)

	// A second member of the same household, so the family is not emptied.
	hash, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.MinCost)
	var partner User
	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		partner = AddUserTx(tx, CreateAccountRequest{
			Name: "Partner", Email: "partner@example.com",
			FamilyCode: GetFamily(tx, fx.familyId).InviteCode,
		}, hash)
		vbolt.TxCommit(tx)
	})

	recorder := deleteAccountRequest(t, fx.ownerAuth,
		`{"password":"password123","confirmEmail":"owner@example.com"}`)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}

	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		if GetFamily(tx, fx.familyId).Id == 0 {
			t.Fatal("a family with a remaining member was destroyed")
		}
		// Family records stay with the family.
		if GetPersonById(tx, fx.person.Id).Id == 0 {
			t.Error("the family's person was deleted along with one member's account")
		}
		if GetImageById(tx, fx.photo.Id).Id == 0 {
			t.Error("the family's photo was deleted along with one member's account")
		}
		if len(getFamilyMilestones(tx, fx.familyId)) == 0 {
			t.Error("the family's milestones were deleted")
		}
		if len(getFamilyGrowthData(tx, fx.familyId)) == 0 {
			t.Error("the family's growth data was deleted")
		}
		// The account's own speech does not.
		if len(GetFamilyChatMessages(tx, fx.familyId, 0, 0)) != 0 {
			t.Error("the deleted account's chat messages stayed behind")
		}
		// Ownership moved rather than pointing at a user that no longer exists.
		if got := familyOwnerId(tx, fx.familyId); got != partner.Id {
			t.Errorf("owner = %d, want the remaining member %d", got, partner.Id)
		}
		if _, member := FindMembership(tx, partner.Id, fx.familyId); !member {
			t.Error("the remaining member lost their membership")
		}
	})
}

func TestDeleteAccountRequiresConfirmation(t *testing.T) {
	cases := map[string]struct {
		body   string
		reason string
	}{
		"wrong password": {
			body:   `{"password":"nottherightone","confirmEmail":"owner@example.com"}`,
			reason: incorrectPasswordMessage,
		},
		"wrong email": {
			body:   `{"password":"password123","confirmEmail":"someone@example.com"}`,
			reason: confirmEmailMismatchMessage,
		},
		"no confirmation at all": {
			body:   `{}`,
			reason: confirmEmailMismatchMessage,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			fx := setupDeletionFixture(t)

			recorder := deleteAccountRequest(t, fx.ownerAuth, tc.body)
			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusBadRequest, recorder.Body.String())
			}
			if resp := decodeDeleteAccount(t, recorder); resp.Success || resp.Error != tc.reason {
				t.Errorf("response = %+v, want failure with %q", resp, tc.reason)
			}
			vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
				if GetUser(tx, fx.owner.Id).Id == 0 {
					t.Error("the account was deleted despite a failed confirmation")
				}
			})
		})
	}
}

func TestDeleteAccountRequiresAuthentication(t *testing.T) {
	fx := setupDeletionFixture(t)

	recorder := deleteAccountRequest(t, "",
		`{"password":"password123","confirmEmail":"owner@example.com"}`)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusUnauthorized, recorder.Body.String())
	}
	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		if GetUser(tx, fx.owner.Id).Id == 0 {
			t.Error("an unauthenticated request deleted an account")
		}
	})
}

// A Google-only account has no password to prove, so the typed address is the
// whole confirmation. It must still work — otherwise those users cannot delete
// their accounts at all.
func TestDeleteAccountWithoutAPasswordOnFile(t *testing.T) {
	db := vbolt.Open(t.TempDir() + "/deletion_google.db")
	vbolt.InitBuckets(db, &cfg.Info)
	t.Cleanup(func() { _ = db.Close() })
	appDb = db
	jwtKey = []byte("deletion-test-secret-key-at-least-32")

	var user User
	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		user = AddUserTx(tx, CreateAccountRequest{Name: "Googler", Email: "googler@example.com"}, nil)
		vbolt.TxCommit(tx)
	})
	token, err := generateJwtTokenString(user)
	if err != nil {
		t.Fatalf("generateJwtTokenString() error = %v", err)
	}

	recorder := deleteAccountRequest(t, token, `{"confirmEmail":"googler@example.com"}`)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		if GetUser(tx, user.Id).Id != 0 {
			t.Error("a password-less account could not be deleted")
		}
	})
}

// Deleting one account must not reach into a family the caller merely has a
// link to. A link shares content; it never makes the other household's records
// the caller's to destroy.
func TestDeleteAccountDoesNotTouchALinkedFamily(t *testing.T) {
	fx := setupDeletionFixture(t)

	var linkId int
	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		link := createFamilyLinkTx(tx, fx.outsiderFamily, fx.familyId, "Grandparents",
			AccessView, ScopePeople.bit()|ScopePhotos.bit())
		linkId = link.Id
		// The outsider's child is shared onto the deleted family's roster.
		EnsurePersonFamilyTx(tx, fx.outsiderPerson.Id, fx.familyId, fx.outsiderPerson.Type)
		vbolt.TxCommit(tx)
	})

	recorder := deleteAccountRequest(t, fx.ownerAuth,
		`{"password":"password123","confirmEmail":"owner@example.com"}`)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}

	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		// The shared person belongs to the other household and stays there.
		if GetPersonById(tx, fx.outsiderPerson.Id).Id == 0 {
			t.Error("a person shared in by a link was deleted with the account")
		}
		if GetFamily(tx, fx.outsiderFamily).Id == 0 {
			t.Error("the linked family was destroyed")
		}
		// The link itself has nothing left to point at.
		var link FamilyLink
		vbolt.Read(tx, FamilyLinkBkt, linkId, &link)
		if link.Id != 0 {
			t.Error("a link to the destroyed family survived it")
		}
	})
}
