package backend

import (
	"context"
	"family/cfg"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
	"time"

	"go.hasen.dev/vbeam"
	"go.hasen.dev/vbolt"
)

type matrixFixture struct {
	db     *vbolt.DB
	owner  User
	famA   int
	kid    Person
	photo  Image
	joined map[string]User
}

func setupMatrixFixture(t *testing.T) *matrixFixture {
	t.Helper()
	db := vbolt.Open(t.TempDir() + "/matrix.db")
	vbolt.InitBuckets(db, &cfg.Info)
	t.Cleanup(func() { _ = db.Close() })
	previous := appDb
	appDb = db
	t.Cleanup(func() { appDb = previous })
	jwtKey = []byte("membership-matrix-test-secret-key-32b")

	fx := &matrixFixture{db: db, joined: map[string]User{}}
	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		fx.owner = AddUserTx(tx, CreateAccountRequest{Name: "Owner", Email: "owner@matrix.test"}, nil)
		fx.famA = fx.owner.FamilyId
		var err error
		fx.kid, err = AddPersonTx(tx, AddPersonRequest{Name: "Kid", Birthdate: "2020-01-01"}, fx.famA)
		if err != nil {
			t.Fatal(err)
		}
		fx.photo = writeTestImage(tx, fx.famA, fx.owner.Id, "kid.jpg")
		fx.photo.Status = 1
		vbolt.Write(tx, ImagesBkt, fx.photo.Id, &fx.photo)

		invite := GetFamily(tx, fx.famA).InviteCode
		for _, name := range []string{"member", "leaver", "removed"} {
			fx.joined[name] = AddUserTx(tx, CreateAccountRequest{Name: name, Email: name + "@matrix.test", FamilyCode: invite}, nil)
		}
		for name, role := range map[string]AccessLevel{"viewer": AccessView, "contributor": AccessContribute} {
			user := AddUserTx(tx, CreateAccountRequest{Name: name, Email: name + "@matrix.test"}, nil)
			EnsureMembershipTx(tx, user.Id, fx.famA, role)
			fx.joined[name] = user
		}

		low := AddUserTx(tx, CreateAccountRequest{Name: "low", Email: "low@matrix.test", FamilyCode: invite}, nil)
		row, _ := FindMembership(tx, low.Id, fx.famA)
		row.Role = AccessView
		vbolt.Write(tx, FamilyMembershipBkt, row.Id, &row)
		fx.joined["low primary"] = low

		drifted := AddUserTx(tx, CreateAccountRequest{Name: "drifted", Email: "drifted@matrix.test", FamilyCode: invite}, nil)
		row, _ = FindMembership(tx, drifted.Id, fx.famA)
		deleteMembershipTx(tx, row)
		fx.joined["no row"] = drifted

		fx.joined["outsider"] = AddUserTx(tx, CreateAccountRequest{Name: "outsider", Email: "outsider@matrix.test"}, nil)
		vbolt.TxCommit(tx)
	})
	return fx
}

func (fx *matrixFixture) user(name string) User {
	if name == "owner" {
		return fx.owner
	}
	var user User
	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) { user = GetUser(tx, fx.joined[name].Id) })
	return user
}

func (fx *matrixFixture) call(t *testing.T, user User, fn func(ctx *vbeam.Context) error) error {
	t.Helper()
	token, err := generateJwtTokenString(user)
	if err != nil {
		t.Fatal(err)
	}
	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		err = fn(&vbeam.Context{Tx: tx, Token: token})
	})
	return err
}

func (fx *matrixFixture) level(t *testing.T, name string) AccessLevel {
	t.Helper()
	user := fx.user(name)
	var got AccessLevel
	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		for _, need := range []AccessLevel{AccessView, AccessContribute, AccessAdmin} {
			if CanAccessFamily(tx, user, fx.famA, need) {
				got = need
			}
		}
	})

	reached := AccessNone
	if fx.call(t, user, func(ctx *vbeam.Context) error {
		_, err := GetPersonMilestones(ctx, GetPersonMilestonesRequest{PersonId: fx.kid.Id})
		return err
	}) == nil {
		reached = AccessView
	}
	if fx.call(t, user, func(ctx *vbeam.Context) error {
		_, err := AddGrowthData(ctx, AddGrowthDataRequest{PersonId: fx.kid.Id, MeasurementType: "height", Value: 40, Unit: "in", InputType: "today"})
		return err
	}) == nil {
		reached = AccessContribute
	}
	if fx.call(t, user, func(ctx *vbeam.Context) error {
		_, err := ListFamilyMembers(ctx, ListFamilyMembersRequest{FamilyId: fx.famA})
		if err == nil {
			_, err = RotateInviteCode(ctx, FamilyIdRequest{FamilyId: fx.famA})
		}
		return err
	}) == nil {
		reached = AccessAdmin
	}
	if reached != got {
		t.Errorf("%s: CanAccessFamily says %d but the procedures reach %d", name, got, reached)
	}

	token, _ := generateJwtTokenString(user)
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/photo/%d/thumb", fx.photo.Id), nil)
	req = req.WithContext(context.WithValue(req.Context(), UserContextKey, user))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	servePhotoHandler(rec, req)
	if served := rec.Code == http.StatusOK; served != (got >= AccessView) {
		t.Errorf("%s: photo handler answered %d at level %d", name, rec.Code, got)
	}

	var listed bool
	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		listed = slices.Contains(GetFamilyUserIds(tx, fx.famA), user.Id)
	})
	if listed != (got >= AccessView) {
		t.Errorf("%s: family user list membership = %v at level %d", name, listed, got)
	}
	return got
}

func TestMembershipRowsAreTheOnlyAuthority(t *testing.T) {
	fx := setupMatrixFixture(t)

	for _, want := range []struct {
		name  string
		level AccessLevel
	}{
		{"owner", AccessAdmin},
		{"member", AccessAdmin},
		{"viewer", AccessView},
		{"contributor", AccessContribute},
		{"low primary", AccessView},
		{"no row", AccessNone},
		{"outsider", AccessNone},
	} {
		if got := fx.level(t, want.name); got != want.level {
			t.Errorf("%s reaches level %d, want %d", want.name, got, want.level)
		}
	}

	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		viewer := fx.user("viewer")
		if families := familiesVisibleTo(tx, viewer); !slices.Equal(families, []int{viewer.FamilyId, fx.famA}) {
			t.Errorf("viewer's families = %v, want own household first then %d", families, fx.famA)
		}
		if families := familiesVisibleTo(tx, fx.user("no row")); len(families) != 0 {
			t.Errorf("a user with no rows sees families %v", families)
		}
	})
}

func TestLeavingAndRemovalCannotBeUndoneByThePrimaryFamily(t *testing.T) {
	fx := setupMatrixFixture(t)

	if err := fx.call(t, fx.user("leaver"), func(ctx *vbeam.Context) error {
		resp, err := LeaveFamily(ctx, FamilyIdRequest{FamilyId: fx.famA})
		if err == nil && !resp.Success {
			err = fmt.Errorf("%s", resp.Error)
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if err := fx.call(t, fx.owner, func(ctx *vbeam.Context) error {
		resp, err := RemoveFamilyMember(ctx, RemoveFamilyMemberRequest{FamilyId: fx.famA, UserId: fx.joined["removed"].Id})
		if err == nil && !resp.Success {
			err = fmt.Errorf("%s", resp.Error)
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}

	for _, name := range []string{"leaver", "removed"} {
		user := fx.user(name)
		if user.FamilyId == fx.famA {
			t.Errorf("%s still has the family as primary", name)
		}
		if got := fx.level(t, name); got != AccessNone {
			t.Errorf("%s reaches level %d after leaving", name, got)
		}
		vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
			if !CanAccessFamily(tx, user, user.FamilyId, AccessAdmin) {
				t.Errorf("%s was not given a household of their own", name)
			}
		})
	}

	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		BackfillFamilyMemberships(tx)
		vbolt.TxCommit(tx)
	})
	for _, name := range []string{"leaver", "removed"} {
		if got := fx.level(t, name); got != AccessNone {
			t.Errorf("the backfill gave %s level %d back", name, got)
		}
	}
	if got := fx.level(t, "no row"); got != AccessAdmin {
		t.Errorf("the backfill left a primary family without a row: level %d", got)
	}
}

func TestOwnerLeavingHandsTheFamilyOnAndAccountDeletionDropsRows(t *testing.T) {
	fx := setupMatrixFixture(t)

	if err := fx.call(t, fx.owner, func(ctx *vbeam.Context) error {
		_, err := LeaveFamily(ctx, FamilyIdRequest{FamilyId: fx.famA})
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if got := fx.level(t, "owner"); got != AccessNone {
		t.Errorf("departed owner reaches level %d", got)
	}
	member := fx.user("member")
	if err := fx.call(t, member, func(ctx *vbeam.Context) error {
		resp, err := RemoveFamilyMember(ctx, RemoveFamilyMemberRequest{FamilyId: fx.famA, UserId: fx.joined["contributor"].Id})
		if err == nil && !resp.Success {
			err = fmt.Errorf("%s", resp.Error)
		}
		return err
	}); err != nil {
		t.Errorf("the next member did not inherit ownership: %v", err)
	}

	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		deleteAccountTx(tx, fx.user("viewer"))
		vbolt.TxCommit(tx)
	})
	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		if slices.Contains(GetFamilyUserIds(tx, fx.famA), fx.joined["viewer"].Id) {
			t.Error("a deleted account is still listed in the family")
		}
		if rows := GetUserMemberships(tx, fx.joined["viewer"].Id); len(rows) != 0 {
			t.Errorf("a deleted account kept rows %+v", rows)
		}
	})
}

func TestChatSocketRefusesAPrimaryFamilyWithoutARow(t *testing.T) {
	fx := setupMatrixFixture(t)
	if GetChatHub() == nil {
		InitializeChatHub()
	}
	handler := HandleWebSocketChat(nil)
	for name, wantRefused := range map[string]bool{"no row": true, "member": false} {
		token, _ := generateJwtTokenString(fx.user(name))
		req := httptest.NewRequest(http.MethodGet, "/ws/chat", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		done := make(chan struct{})
		go func() { handler(rec, req); close(done) }()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatalf("%s: handler did not return", name)
		}
		if refused := rec.Code == http.StatusForbidden; refused != wantRefused {
			t.Errorf("%s: socket answered %d", name, rec.Code)
		}
	}
}
