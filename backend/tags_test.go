package backend

import (
	"family/cfg"
	"os"
	"testing"
	"time"

	"go.hasen.dev/vbolt"
)

func TestGetTagsByFamily(t *testing.T) {
	testDBPath := "test_tags.db"
	db := vbolt.Open(testDBPath)
	vbolt.InitBuckets(db, &cfg.Info)
	defer os.Remove(testDBPath)
	defer db.Close()

	t.Run("returns empty slice when no tags exist", func(t *testing.T) {
		vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
			tags := getTagsByFamily(tx, 12345)
			if len(tags) != 0 {
				t.Fatalf("expected empty tags slice, got %d tags", len(tags))
			}
		})
	})

	t.Run("returns only tags for the requested family", func(t *testing.T) {
		var familyOneTag1, familyOneTag2, familyTwoTag Tag

		vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
			familyOneTag1 = Tag{Id: vbolt.NextIntId(tx, TagBkt), FamilyId: 1, Name: "Doctor", Color: "#FF0000", CreatedAt: time.Now()}
			familyOneTag2 = Tag{Id: vbolt.NextIntId(tx, TagBkt), FamilyId: 1, Name: "School", Color: "#00FF00", CreatedAt: time.Now()}
			familyTwoTag = Tag{Id: vbolt.NextIntId(tx, TagBkt), FamilyId: 2, Name: "Sports", Color: "#0000FF", CreatedAt: time.Now()}

			vbolt.Write(tx, TagBkt, familyOneTag1.Id, &familyOneTag1)
			vbolt.Write(tx, TagBkt, familyOneTag2.Id, &familyOneTag2)
			vbolt.Write(tx, TagBkt, familyTwoTag.Id, &familyTwoTag)

			vbolt.SetTargetSingleTerm(tx, TagByFamilyIndex, familyOneTag1.Id, familyOneTag1.FamilyId)
			vbolt.SetTargetSingleTerm(tx, TagByFamilyIndex, familyOneTag2.Id, familyOneTag2.FamilyId)
			vbolt.SetTargetSingleTerm(tx, TagByFamilyIndex, familyTwoTag.Id, familyTwoTag.FamilyId)

			vbolt.TxCommit(tx)
		})

		vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
			tags := getTagsByFamily(tx, 1)
			if len(tags) != 2 {
				t.Fatalf("expected 2 tags, got %d", len(tags))
			}

			ids := map[int]bool{}
			for _, tag := range tags {
				if tag.FamilyId != 1 {
					t.Fatalf("expected all tags to be for family 1, found family %d", tag.FamilyId)
				}
				ids[tag.Id] = true
			}

			if !ids[familyOneTag1.Id] || !ids[familyOneTag2.Id] {
				t.Fatalf("expected tags %d and %d to be returned, got %#v", familyOneTag1.Id, familyOneTag2.Id, ids)
			}
			if ids[familyTwoTag.Id] {
				t.Fatalf("did not expect tag %d from family 2 to be returned", familyTwoTag.Id)
			}
		})
	})
}

func TestTagNameExistsInFamily(t *testing.T) {
	testDBPath := "test_tag_name_exists.db"
	db := vbolt.Open(testDBPath)
	vbolt.InitBuckets(db, &cfg.Info)
	defer os.Remove(testDBPath)
	defer db.Close()

	var existingTag Tag
	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		existingTag = Tag{Id: vbolt.NextIntId(tx, TagBkt), FamilyId: 7, Name: "School", Color: "#123456", CreatedAt: time.Now()}
		vbolt.Write(tx, TagBkt, existingTag.Id, &existingTag)
		vbolt.SetTargetSingleTerm(tx, TagByFamilyIndex, existingTag.Id, existingTag.FamilyId)
		vbolt.TxCommit(tx)
	})

	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		if !tagNameExistsInFamily(tx, "school", 7, -1) {
			t.Fatal("expected case-insensitive duplicate name detection")
		}

		if tagNameExistsInFamily(tx, "school", 7, existingTag.Id) {
			t.Fatal("expected excludeId to ignore the current tag")
		}

		if tagNameExistsInFamily(tx, "school", 99, -1) {
			t.Fatal("did not expect duplicate detection in different family")
		}
	})
}
