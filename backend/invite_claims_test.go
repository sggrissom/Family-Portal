package backend

import (
	"family/cfg"
	"os"
	"testing"

	"go.hasen.dev/vbeam"
	"go.hasen.dev/vbolt"
)

func TestJoiningByInviteCanClaimAnExistingAdult(t *testing.T) {
	testDBPath := "test_invite_claims.db"
	db := vbolt.Open(testDBPath)
	vbolt.InitBuckets(db, &cfg.Info)
	defer os.Remove(testDBPath)
	defer db.Close()

	var family Family
	var dana, marco, kid Person
	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		owner, _, rejection := createAccountTx(tx, CreateAccountRequest{
			Name: "Dana", Email: "dana@example.com", Password: "password123", ConfirmPassword: "password123",
			InitialPersonBirthdate: "1989-01-01",
		})
		if rejection != "" {
			t.Fatal(rejection)
		}
		family = GetFamily(tx, owner.FamilyId)
		dana = GetPersonById(tx, owner.PersonId)
		var err error
		if marco, err = AddPersonTx(tx, AddPersonRequest{Name: "Marco", Birthdate: "1987-05-05"}, family.Id); err != nil {
			t.Fatal(err)
		}
		if kid, err = AddPersonTx(tx, AddPersonRequest{Name: "Ada", Birthdate: "2019-05-05"}, family.Id); err != nil {
			t.Fatal(err)
		}
		vbolt.TxCommit(tx)
	})

	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		resp, _ := GetInviteClaims(&vbeam.Context{Tx: tx}, GetInviteClaimsRequest{Code: family.InviteCode})
		if len(resp.Claims) != 1 || resp.Claims[0].PersonId != marco.Id {
			t.Errorf("claims = %+v, want only Marco (Dana has an account, Ada is a child)", resp.Claims)
		}
		if empty, _ := GetInviteClaims(&vbeam.Context{Tx: tx}, GetInviteClaimsRequest{Code: "nope"}); len(empty.Claims) != 0 {
			t.Errorf("an unknown code listed %+v", empty.Claims)
		}
	})

	join := func(email string, claim int) (User, string) {
		var user User
		var rejection string
		vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
			user, _, rejection = createAccountTx(tx, CreateAccountRequest{
				Name: "Joiner", Email: email, Password: "password123", ConfirmPassword: "password123",
				FamilyCode: family.InviteCode, ClaimPersonId: claim,
			})
			if rejection == "" {
				vbolt.TxCommit(tx)
			}
		})
		return user, rejection
	}

	if _, rejection := join("kid@example.com", kid.Id); rejection == "" {
		t.Error("claiming a child should be refused")
	}
	if _, rejection := join("dana2@example.com", dana.Id); rejection == "" {
		t.Error("claiming someone who already has an account should be refused")
	}
	user, rejection := join("marco@example.com", marco.Id)
	if rejection != "" {
		t.Fatalf("claiming Marco: %s", rejection)
	}
	if user.PersonId != marco.Id || user.FamilyId != family.Id {
		t.Errorf("user person/family = %d/%d, want %d/%d", user.PersonId, user.FamilyId, marco.Id, family.Id)
	}
	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		if n := len(GetFamilyOwnPeople(tx, family.Id)); n != 3 {
			t.Errorf("family has %d people, want 3 (no duplicate Marco)", n)
		}
	})
}
