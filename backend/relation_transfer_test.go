package backend

import (
	"encoding/json"
	"family/cfg"
	"os"
	"testing"

	"go.hasen.dev/vbeam"
	"go.hasen.dev/vbolt"
	"golang.org/x/crypto/bcrypt"
)

func importRecordsInto(t *testing.T, db *vbolt.DB, user User, req ImportDataRequest) (resp ImportDataResponse) {
	t.Helper()
	jwtKey = []byte("relation-transfer-test-secret-key-32b")
	token := testAuthToken(t, user)
	var err error
	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		resp, err = ImportData(&vbeam.Context{Tx: tx, Token: token}, req)
		vbolt.TxCommit(tx)
	})
	if err != nil {
		t.Fatalf("import failed: %v", err)
	}
	return resp
}

func openTransferTestDb(t *testing.T, name string) *vbolt.DB {
	t.Helper()
	path := name + ".db"
	db := vbolt.Open(path)
	vbolt.InitBuckets(db, &cfg.Info)
	t.Cleanup(func() {
		db.Close()
		os.Remove(path)
	})
	return db
}

func addTransferTestUser(t *testing.T, tx *vbolt.Tx, name string, email string) User {
	t.Helper()
	req := CreateAccountRequest{
		Name:            name,
		Email:           email,
		Password:        "password123",
		ConfirmPassword: "password123",
	}
	hash, _ := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	return AddUserTx(tx, req, hash)
}

func addTransferTestPerson(t *testing.T, tx *vbolt.Tx, familyId int, name string, birthdate string) Person {
	t.Helper()
	person, err := AddPersonTx(tx, AddPersonRequest{Name: name, Birthdate: birthdate}, familyId)
	if err != nil {
		t.Fatalf("Failed to add person %s: %v", name, err)
	}
	return person
}

func TestImportRelationsSkipsUnmappedAndCollapsedPeople(t *testing.T) {
	db := openTransferTestDb(t, "test_import_relations_skips")

	var user User
	var kept, dropped Person

	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		user = addTransferTestUser(t, tx, "Owner", "owner2@example.com")
		kept = addTransferTestPerson(t, tx, user.FamilyId, "Ruth", "1985-03-02")
		dropped = addTransferTestPerson(t, tx, user.FamilyId, "Nora", "2015-07-11")
		vbolt.TxCommit(tx)
	})

	// 102 is filtered out; 101 and 103 both match the same person, as when two
	// rows of one file merge onto one existing record.
	records, err := json.Marshal(ImportDataStructure{
		People: []ImportPerson{
			{Id: 101, Name: kept.Name, Birthday: kept.Birthday, Gender: int(kept.Gender)},
			{Id: 102, Name: "Elsewhere", Birthday: kept.Birthday},
			{Id: 103, Name: kept.Name, Birthday: kept.Birthday, Gender: int(kept.Gender)},
		},
		Relations: []ExportRelation{
			{FromId: 101, ToId: 102, Kind: "parent"},
			{FromId: 101, ToId: 103, Kind: "sibling"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	resp := importRecordsInto(t, db, user, ImportDataRequest{JsonData: string(records), FilterPersonIds: []int{101, 103}})
	imported, skipped, errs := resp.ImportedRelations, resp.SkippedRelations, resp.Errors

	if imported != 0 {
		t.Errorf("Expected no relations imported, got %d", imported)
	}
	if skipped != 2 {
		t.Errorf("Expected 2 skipped relations, got %d", skipped)
	}
	if len(errs) > 0 {
		t.Errorf("Skipping should not report errors, got %v", errs)
	}

	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		if rows := GetPersonRelationsTx(tx, kept.Id); len(rows) != 0 {
			t.Errorf("Expected no stored relations, got %d", len(rows))
		}
		if rows := GetPersonRelationsTx(tx, dropped.Id); len(rows) != 0 {
			t.Errorf("Expected no stored relations for the unmapped person, got %d", len(rows))
		}
	})
}

func TestParseRelationKindRoundTrip(t *testing.T) {
	kinds := []RelationKind{RelationParent, RelationSibling, RelationPartner}
	for _, kind := range kinds {
		name := kind.exportName()
		if name == "" {
			t.Errorf("Relation kind %d has no export name", kind)
			continue
		}
		parsed, ok := parseRelationKind(name)
		if !ok || parsed != kind {
			t.Errorf("Round trip of %q gave (%d, %v), want (%d, true)", name, parsed, ok, kind)
		}
	}

	if _, ok := parseRelationKind("  PARENT  "); !ok {
		t.Error("parseRelationKind should tolerate case and surrounding space")
	}
	if _, ok := parseRelationKind("godparent"); ok {
		t.Error("parseRelationKind accepted an unknown kind")
	}
}
