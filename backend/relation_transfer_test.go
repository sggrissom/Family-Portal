package backend

import (
	"encoding/json"
	"family/cfg"
	"os"
	"strings"
	"testing"
	"time"

	"go.hasen.dev/vbolt"
	"golang.org/x/crypto/bcrypt"
)

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

func findExportedRelation(relations []ExportRelation, fromId int, toId int) (ExportRelation, bool) {
	for _, relation := range relations {
		if relation.FromId == fromId && relation.ToId == toId {
			return relation, true
		}
	}
	return ExportRelation{}, false
}

func TestExportIncludesRelations(t *testing.T) {
	db := openTransferTestDb(t, "test_export_relations")

	var user User
	var parent, child, outsider Person

	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		user = addTransferTestUser(t, tx, "Parent", "parent@example.com")
		other := addTransferTestUser(t, tx, "Other", "other@example.com")

		parent = addTransferTestPerson(t, tx, user.FamilyId, "Ruth", "1985-03-02")
		child = addTransferTestPerson(t, tx, user.FamilyId, "Nora", "2015-07-11")
		outsider = addTransferTestPerson(t, tx, other.FamilyId, "Cousin", "2016-01-20")

		AddRelationTx(tx, Relation{FromId: parent.Id, ToId: child.Id, Kind: RelationParent})
		AddRelationTx(tx, Relation{FromId: child.Id, ToId: outsider.Id, Kind: RelationSibling})

		vbolt.TxCommit(tx)
	})

	var exportData ExportDataStructure
	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		var err error
		exportData, err = buildExportData(tx, user.FamilyId)
		if err != nil {
			t.Fatalf("buildExportData failed: %v", err)
		}
	})

	if len(exportData.Relations) != 1 {
		t.Fatalf("Expected 1 exported relation, got %d", len(exportData.Relations))
	}
	if exportData.TotalRelations != 1 {
		t.Errorf("Expected TotalRelations 1, got %d", exportData.TotalRelations)
	}

	relation, found := findExportedRelation(exportData.Relations, parent.Id, child.Id)
	if !found {
		t.Fatal("Parent edge missing from export")
	}
	if relation.Kind != "parent" {
		t.Errorf("Expected kind 'parent', got '%s'", relation.Kind)
	}
	if relation.FromName != "Ruth" || relation.ToName != "Nora" {
		t.Errorf("Expected names Ruth/Nora, got %s/%s", relation.FromName, relation.ToName)
	}

	if _, found := findExportedRelation(exportData.Relations, child.Id, outsider.Id); found {
		t.Error("Edge to a person outside the family should not be exported")
	}
}

func TestImportRestoresRelations(t *testing.T) {
	db := openTransferTestDb(t, "test_import_relations")

	var source User
	var parentA, parentB, kid, sibling Person

	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		source = addTransferTestUser(t, tx, "Source", "source@example.com")

		parentA = addTransferTestPerson(t, tx, source.FamilyId, "Ruth", "1985-03-02")
		parentB = addTransferTestPerson(t, tx, source.FamilyId, "Steven", "1984-11-30")
		kid = addTransferTestPerson(t, tx, source.FamilyId, "Nora", "2015-07-11")
		sibling = addTransferTestPerson(t, tx, source.FamilyId, "Sam", "2018-02-14")

		AddRelationTx(tx, Relation{FromId: parentA.Id, ToId: kid.Id, Kind: RelationParent})
		AddRelationTx(tx, Relation{FromId: parentB.Id, ToId: kid.Id, Kind: RelationParent})
		AddRelationTx(tx, Relation{FromId: parentA.Id, ToId: parentB.Id, Kind: RelationPartner})
		AddRelationTx(tx, Relation{FromId: kid.Id, ToId: sibling.Id, Kind: RelationSibling})

		vbolt.TxCommit(tx)
	})

	var exportData ExportDataStructure
	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		var err error
		exportData, err = buildExportData(tx, source.FamilyId)
		if err != nil {
			t.Fatalf("buildExportData failed: %v", err)
		}
	})

	jsonBytes, err := json.Marshal(exportData)
	if err != nil {
		t.Fatalf("Failed to marshal export: %v", err)
	}
	var importData ImportDataStructure
	if err := json.Unmarshal(jsonBytes, &importData); err != nil {
		t.Fatalf("Failed to unmarshal export as import data: %v", err)
	}
	if err := validateImportData(importData); err != nil {
		t.Fatalf("Exported data failed import validation: %v", err)
	}
	if len(importData.Relations) != 4 {
		t.Fatalf("Expected 4 relations in the round-tripped file, got %d", len(importData.Relations))
	}

	var target User
	var mapping map[int]int
	var imported, skipped int

	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		target = addTransferTestUser(t, tx, "Target", "target@example.com")

		var errs []string
		mapping, _, _, errs, _ = importPeople(tx, importData.People, target.FamilyId, "create_all")
		if len(errs) > 0 {
			t.Fatalf("Unexpected people import errors: %v", errs)
		}

		imported, skipped, errs = importRelations(tx, importData.Relations, mapping)
		if len(errs) > 0 {
			t.Fatalf("Unexpected relation import errors: %v", errs)
		}

		vbolt.TxCommit(tx)
	})

	if imported != 4 {
		t.Errorf("Expected 4 imported relations, got %d (skipped %d)", imported, skipped)
	}

	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		newKid := mapping[kid.Id]

		parents := parentsOf(tx, newKid)
		if len(parents) != 2 {
			t.Errorf("Expected 2 parents for the imported child, got %d", len(parents))
		}
		if !containsId(parents, mapping[parentA.Id]) || !containsId(parents, mapping[parentB.Id]) {
			t.Error("Imported child is not linked to both imported parents")
		}

		siblings := SiblingsOf(tx, newKid)
		if !containsId(siblings, mapping[sibling.Id]) {
			t.Error("Imported sibling edge is missing")
		}

		partners := partnersOf(tx, mapping[parentA.Id])
		if !containsId(partners, mapping[parentB.Id]) {
			t.Error("Imported partner edge is missing")
		}
	})
}

func TestImportRelationsIsIdempotent(t *testing.T) {
	db := openTransferTestDb(t, "test_import_relations_idempotent")

	var user User
	var parent, child Person

	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		user = addTransferTestUser(t, tx, "Owner", "owner@example.com")
		parent = addTransferTestPerson(t, tx, user.FamilyId, "Ruth", "1985-03-02")
		child = addTransferTestPerson(t, tx, user.FamilyId, "Nora", "2015-07-11")
		AddRelationTx(tx, Relation{FromId: parent.Id, ToId: child.Id, Kind: RelationParent})
		vbolt.TxCommit(tx)
	})

	var exportData ExportDataStructure
	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		var err error
		exportData, err = buildExportData(tx, user.FamilyId)
		if err != nil {
			t.Fatalf("buildExportData failed: %v", err)
		}
	})

	var imported, skipped int
	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		mapping, _, merged, _, _ := importPeople(tx, exportData.People, user.FamilyId, "merge_people")
		if merged != 2 {
			t.Fatalf("Expected both people to merge onto themselves, got %d", merged)
		}
		imported, skipped, _ = importRelations(tx, exportData.Relations, mapping)
		vbolt.TxCommit(tx)
	})

	if imported != 0 {
		t.Errorf("Re-importing an export into its own family should add no relations, added %d", imported)
	}
	if skipped != 1 {
		t.Errorf("Expected the existing relation to be skipped, skipped %d", skipped)
	}

	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		if rows := GetPersonRelationsTx(tx, child.Id); len(rows) != 1 {
			t.Errorf("Expected 1 stored relation after re-import, got %d", len(rows))
		}
	})
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

	relations := []ExportRelation{
		{FromId: 101, ToId: 102, Kind: "parent"},
		{FromId: 101, ToId: 103, Kind: "sibling"},
	}
	// 102 is absent from the mapping; 101 and 103 both land on the same person,
	// which is what happens when two import rows merge onto one existing record.
	mapping := map[int]int{101: kept.Id, 103: kept.Id}

	var imported, skipped int
	var errs []string
	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		imported, skipped, errs = importRelations(tx, relations, mapping)
		vbolt.TxCommit(tx)
	})

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

func TestValidateImportRelation(t *testing.T) {
	people := []ImportPerson{
		{Id: 1, Name: "Ruth", Birthday: time.Date(1985, 3, 2, 0, 0, 0, 0, time.UTC)},
		{Id: 2, Name: "Nora", Birthday: time.Date(2015, 7, 11, 0, 0, 0, 0, time.UTC)},
	}

	tests := []struct {
		name          string
		relation      ExportRelation
		errorContains string
	}{
		{
			name:     "valid parent edge",
			relation: ExportRelation{FromId: 1, ToId: 2, Kind: "parent"},
		},
		{
			name:          "unknown person",
			relation:      ExportRelation{FromId: 1, ToId: 99, Kind: "parent"},
			errorContains: "unknown person",
		},
		{
			name:          "self relation",
			relation:      ExportRelation{FromId: 1, ToId: 1, Kind: "sibling"},
			errorContains: "themselves",
		},
		{
			name:          "unknown kind",
			relation:      ExportRelation{FromId: 1, ToId: 2, Kind: "godparent"},
			errorContains: "unknown kind",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateImportData(ImportDataStructure{
				People:    people,
				Relations: []ExportRelation{test.relation},
			})

			if test.errorContains == "" {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("Expected an error containing %q, got none", test.errorContains)
			}
			if !strings.Contains(err.Error(), test.errorContains) {
				t.Errorf("Expected error containing %q, got %q", test.errorContains, err.Error())
			}
		})
	}
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
