package backend

import (
	"family/cfg"
	"fmt"
	"testing"
	"time"

	"go.hasen.dev/vbeam"
	"go.hasen.dev/vbolt"
)

var benchWords = []string{
	"walked", "park", "beach", "laughed", "first", "steps", "birthday", "cake", "grandma", "dog",
	"swing", "slide", "bath", "story", "sang", "danced", "drew", "school", "soccer", "snow",
}

// BenchmarkSearch runs global search over a family of 3,000 moments and
// 20,000 photos without the daemon. The budget is 50 ms a search.
func BenchmarkSearch(b *testing.B) {
	db := vbolt.Open(b.TempDir() + "/search_bench.db")
	vbolt.InitBuckets(db, &cfg.Info)
	b.Cleanup(func() { db.Close() })
	previous := globalVisionClient
	globalVisionClient = nil
	b.Cleanup(func() { globalVisionClient = previous })

	var user User
	var people []Person
	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		user = AddUserTx(tx, CreateAccountRequest{Name: "Owner", Email: "bench@example.com"}, nil)
		for i, name := range []string{"Alice", "Bob", "Carol", "Dan"} {
			p, err := AddPersonTx(tx, AddPersonRequest{Name: name, Birthdate: fmt.Sprintf("201%d-03-02", i*2)}, user.FamilyId)
			if err != nil {
				b.Fatal(err)
			}
			people = append(people, p)
		}
		tag := Tag{Id: vbolt.NextIntId(tx, TagBkt), FamilyId: user.FamilyId, Name: "Beach", CreatedAt: time.Now()}
		vbolt.Write(tx, TagBkt, tag.Id, &tag)
		vbolt.SetTargetSingleTerm(tx, TagByFamilyIndex, tag.Id, user.FamilyId)
		vbolt.TxCommit(tx)
	})

	start := time.Date(2012, 1, 1, 0, 0, 0, 0, time.UTC)
	for batch := 0; batch < 3; batch++ {
		vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
			for i := batch * 1000; i < (batch+1)*1000; i++ {
				date := start.AddDate(0, 0, i*4).Format(dateOnlyLayout)
				text := fmt.Sprintf("%s %s %s", benchWords[i%20], benchWords[(i*7)%20], benchWords[(i*13)%20])
				if _, err := AddMilestoneTx(tx, AddMilestoneRequest{
					PersonId: people[i%4].Id, Description: text, Category: "development", InputType: "date", MilestoneDate: &date,
				}, user.FamilyId); err != nil {
					b.Fatal(err)
				}
			}
			vbolt.TxCommit(tx)
		})
	}
	for batch := 0; batch < 20; batch++ {
		vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
			for i := batch * 1000; i < (batch+1)*1000; i++ {
				image := Image{
					Id: vbolt.NextIntId(tx, ImagesBkt), FamilyId: user.FamilyId, OwnerUserId: user.Id,
					Title: benchWords[i%20], PhotoDate: start.Add(time.Duration(i) * 13 * time.Hour), CreatedAt: time.Now(),
				}
				vbolt.Write(tx, ImagesBkt, image.Id, &image)
				vbolt.SetTargetSingleTerm(tx, ImageByFamilyIndex, image.Id, user.FamilyId)
				if i%2 == 0 {
					AddPersonToPhoto(tx, image.Id, people[i%4].Id, user.FamilyId)
				}
				ReindexPhotoDates(tx, image.Id)
				if i%3 == 0 {
					lat, lng := 36.16+float64(i%7), -86.78+float64(i%5)
					f := PhotoFeatures{PhotoId: image.Id, FamilyId: user.FamilyId, HasLocation: true, Latitude: lat, Longitude: lng}
					if city, ok := NearestCity(lat, lng); ok {
						f.CityId = city.Id
					}
					writeFeaturesTx(tx, &f)
				}
			}
			vbolt.TxCommit(tx)
		})
	}

	token, err := generateJwtTokenString(user)
	if err != nil {
		b.Fatal(err)
	}
	for _, query := range []string{"walk", "alice at 2", "beach 2016", "alice park swing"} {
		b.Run(query, func(b *testing.B) {
			for b.Loop() {
				vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
					if _, err := Search(&vbeam.Context{Tx: tx, Token: token}, SearchRequest{Query: query}); err != nil {
						b.Fatal(err)
					}
				})
			}
		})
	}
	b.Run("photos page walk", func(b *testing.B) {
		for b.Loop() {
			vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
				if _, err := ListFamilyPhotos(&vbeam.Context{Tx: tx, Token: token}, ListFamilyPhotosRequest{Query: "walk", Limit: 60}); err != nil {
					b.Fatal(err)
				}
			})
		}
	})
}
