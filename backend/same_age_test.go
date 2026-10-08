package backend

import (
	"testing"
	"time"

	"go.hasen.dev/vbeam"
	"go.hasen.dev/vbolt"
)

func TestSameAgeStep(t *testing.T) {
	for age, want := range map[int]int{0: 1, 23: 1, 24: 3, 71: 3, 72: 6, 200: 6} {
		if got := sameAgeStep(age); got != want {
			t.Errorf("sameAgeStep(%d) = %d, want %d", age, got, want)
		}
	}
	if got, want := sameAgeRecordWindow(40), monthsDuration(1.5); got != want {
		t.Errorf("sameAgeRecordWindow(40) = %v, want %v", got, want)
	}
	if got, want := sameAgeGrowthWindow(10), monthsDuration(1); got != want {
		t.Errorf("sameAgeGrowthWindow(10) = %v, want %v", got, want)
	}
}

func TestSortSameAgeRows(t *testing.T) {
	older := SameAgeRow{Person: Person{Id: 1, Birthday: day("2015-01-01")}, PhotoIds: []int{7}}
	younger := SameAgeRow{Person: Person{Id: 2, Birthday: day("2018-01-01")}, PhotoIds: []int{8}}
	empty := SameAgeRow{Person: Person{Id: 3, Birthday: day("2019-01-01")}}
	from := SameAgeRow{Person: Person{Id: 4, Birthday: day("2021-01-01")}}

	rows := []SameAgeRow{empty, older, from, younger}
	sortSameAgeRows(rows, 4)

	var ids []int
	for _, row := range rows {
		ids = append(ids, row.Person.Id)
	}
	want := []int{4, 2, 1, 3}
	for i := range want {
		if ids[i] != want[i] {
			t.Fatalf("order = %v, want %v", ids, want)
		}
	}
}

func months(n int) *int { return &n }

func TestGetSameAge(t *testing.T) {
	fx, cleanup := setupIsolationFixture(t)
	defer cleanup()

	call := func(user User, req GetSameAgeRequest) (GetSameAgeResponse, error) {
		t.Helper()
		token, err := generateJwtTokenString(user)
		if err != nil {
			t.Fatalf("generateJwtTokenString() error = %v", err)
		}
		var resp GetSameAgeResponse
		vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
			resp, err = GetSameAge(&vbeam.Context{Tx: tx, Token: token}, req)
		})
		return resp, err
	}

	// The fixture child, born 2020-06-15, was 43 months old on 2024-01-15:
	// measured 2024-01-05 and a milestone on 2024-02-10.
	resp, err := call(fx.owner, GetSameAgeRequest{AgeMonths: months(43), Today: "2026-09-27"})
	if err != nil {
		t.Fatalf("GetSameAge() error = %v", err)
	}
	if resp.FromPersonId != fx.person.Id {
		t.Errorf("FromPersonId = %d, want the fixture child %d", resp.FromPersonId, fx.person.Id)
	}
	if len(resp.Rows) == 0 || resp.Rows[0].Person.Id != fx.person.Id {
		t.Fatalf("Rows = %+v, want the fixture child first", resp.Rows)
	}
	row := resp.Rows[0]
	if row.Height == nil || row.Height.Id != fx.growth.Id {
		t.Errorf("Height = %+v, want the 2024-01-05 measurement", row.Height)
	}
	if len(row.Milestones) != 1 || row.Milestones[0].Id != fx.milestone.Id {
		t.Errorf("Milestones = %+v, want the 2024-02-10 milestone", row.Milestones)
	}
	if !row.Date.Equal(time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("Date = %v, want 2024-01-15", row.Date)
	}

	defaulted, err := call(fx.owner, GetSameAgeRequest{Today: "2026-09-27"})
	if err != nil {
		t.Fatalf("GetSameAge(default) error = %v", err)
	}
	if defaulted.AgeMonths != 75 || defaulted.FromPersonId != fx.person.Id {
		t.Errorf("default = %d months from %d, want 75 months from the fixture child",
			defaulted.AgeMonths, defaulted.FromPersonId)
	}

	future, err := call(fx.owner, GetSameAgeRequest{AgeMonths: months(120), Today: "2026-09-27"})
	if err != nil {
		t.Fatalf("GetSameAge(120) error = %v", err)
	}
	for _, r := range future.Rows {
		if r.Person.Id == fx.person.Id {
			t.Error("a child who has not reached the age should not get a row")
		}
	}

	outsider, err := call(fx.outsider, GetSameAgeRequest{AgeMonths: months(43), Today: "2026-09-27"})
	if err != nil {
		t.Fatalf("GetSameAge(outsider) error = %v", err)
	}
	for _, r := range outsider.Rows {
		if r.Person.Id == fx.person.Id {
			t.Error("an outsider got a row for another family's child")
		}
	}

	newborn, err := call(fx.owner, GetSameAgeRequest{AgeMonths: months(0), Today: "2026-09-27"})
	if err != nil || newborn.AgeMonths != 0 {
		t.Errorf("an explicit age of zero = %d months (%v), want 0", newborn.AgeMonths, err)
	}

	if _, err := call(fx.owner, GetSameAgeRequest{AgeMonths: months(-1)}); err != ErrInvalidAge {
		t.Errorf("negative age error = %v, want ErrInvalidAge", err)
	}
}

func TestSameAgePortraits(t *testing.T) {
	fx := setupResultsFixture(t)
	// Everyone in the fixture was born 2014-03-02, so six months is 2014-09-02.
	sixMonths := fx.alice.Birthday.AddDate(0, 6, 0)

	onTheDay := fx.addPhotoAt(t, "on-the-day.jpg", sixMonths)
	closeUp := fx.addPhotoAt(t, "close-up.jpg", sixMonths.AddDate(0, 0, 8))
	tooLate := fx.addPhotoAt(t, "too-late.jpg", sixMonths.AddDate(0, 0, 40))
	for _, p := range []Image{onTheDay, closeUp, tooLate} {
		fx.tagPerson(t, p, fx.alice)
	}
	fx.addFace(t, closeUp, fx.alice, box(0.3, 0.2, 0.7, 0.6), FaceConfirmed)

	noFace := fx.addPhotoAt(t, "no-face.jpg", sixMonths.AddDate(0, 0, 3))
	fx.tagPerson(t, noFace, fx.bob)

	resp, err := callAs(t, fx, GetSameAge, GetSameAgeRequest{AgeMonths: months(6), Today: "2026-09-27"})
	if err != nil {
		t.Fatal(err)
	}
	portraits := map[int][]PortraitPhoto{}
	for _, row := range resp.Rows {
		if row.Portraits == nil {
			t.Errorf("%s has nil portraits, want an empty list", row.Person.Name)
		}
		portraits[row.Person.Id] = row.Portraits
	}

	alice := portraits[fx.alice.Id]
	if len(alice) != 2 || alice[0].PhotoId != closeUp.Id || alice[1].PhotoId != onTheDay.Id {
		t.Fatalf("alice = %+v, want the confirmed close-up ahead of the closer photo, and nothing outside the window", alice)
	}
	if alice[0].Box.Right != 0.7 || !alice[0].Date.Equal(closeUp.PhotoDate) {
		t.Errorf("close-up should carry its face box and actual date: %+v", alice[0])
	}

	bob := portraits[fx.bob.Id]
	if len(bob) != 1 || bob[0].PhotoId != noFace.Id || bob[0].Box != (FaceBox{}) {
		t.Errorf("bob = %+v, want the whole photo without a face box", bob)
	}
	if carol := portraits[fx.carol.Id]; len(carol) != 0 {
		t.Errorf("carol = %+v, want a gap", carol)
	}
}

func TestSameAgePortraitsRespectLinkScopes(t *testing.T) {
	fx, cleanup := setupFamilyLinkFixture(t)
	defer cleanup()
	jwtKey = []byte("same-age-portraits-test-secret-key-at-least-32")

	firstBirthday := fx.alice.Birthday.AddDate(1, 0, 0)
	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		photo := writeTestImage(tx, fx.famA, fx.userA.Id, "first-birthday.jpg")
		photo.PhotoDate = firstBirthday
		vbolt.Write(tx, ImagesBkt, photo.Id, &photo)
		AddPersonToPhoto(tx, photo.Id, fx.alice.Id, fx.famA)
		ReindexPhotoDates(tx, photo.Id)
		vbolt.TxCommit(tx)
	})

	photosAllowed := true
	aliceAtOne := func() []PortraitPhoto {
		t.Helper()
		token, err := generateJwtTokenString(fx.userB)
		if err != nil {
			t.Fatal(err)
		}
		var resp GetSameAgeResponse
		vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
			resp, err = GetSameAge(&vbeam.Context{Tx: tx, Token: token}, GetSameAgeRequest{IncludeAvailableAges: true, AgeMonths: months(12), Today: "2026-09-27"})
		})
		if err != nil {
			t.Fatal(err)
		}
		if photosAllowed && len(resp.AvailableAges) == 0 {
			t.Error("the photo scope should make saved ages available")
		}
		if !photosAllowed && len(resp.AvailableAges) != 0 {
			t.Errorf("ages leaked without the photo scope: %+v", resp.AvailableAges)
		}
		for _, row := range resp.Rows {
			if row.Person.Id == fx.alice.Id {
				return row.Portraits
			}
		}
		t.Fatal("the linked household got no row for alice")
		return nil
	}

	if got := aliceAtOne(); len(got) != 1 {
		t.Errorf("with the photos scope, portraits = %+v, want the birthday photo", got)
	}
	setLinkScopes(t, fx, fx.linkAB, LinkScopes{People: true})
	photosAllowed = false
	if got := aliceAtOne(); len(got) != 0 {
		t.Errorf("without the photos scope, portraits = %+v, want none", got)
	}
}

func TestSameAgeBrowse(t *testing.T) {
	fx := setupResultsFixture(t)
	date := fx.alice.Birthday.AddDate(0, 6, 0)
	photo := fx.addPhotoAt(t, "shared-six-months.jpg", date)
	fx.tagPerson(t, photo, fx.alice)
	fx.tagPerson(t, photo, fx.bob)
	// Many photos of one child must not inflate the number of people.
	extra := fx.addPhotoAt(t, "another-six-months.jpg", date)
	fx.tagPerson(t, extra, fx.alice)
	later := fx.addPhotoAt(t, "one-person-later.jpg", date.AddDate(1, 0, 0))
	fx.tagPerson(t, later, fx.alice)

	resp, err := callAs(t, fx, GetSameAge, GetSameAgeRequest{IncludeAvailableAges: true, Today: "2026-09-27"})
	if err != nil {
		t.Fatal(err)
	}
	if resp.AgeMonths != 6 {
		t.Fatalf("default age = %d, want the two-person comparison at six months", resp.AgeMonths)
	}
	found := false
	for _, option := range resp.AvailableAges {
		if option.AgeMonths == 6 {
			found = true
			if option.PeopleCount != 2 {
				t.Errorf("six months count = %d, want 2 people", option.PeopleCount)
			}
		}
	}
	if !found {
		t.Fatal("photo-only age was not offered")
	}

	explicit, err := callAs(t, fx, GetSameAge, GetSameAgeRequest{IncludeAvailableAges: true, AgeMonths: months(0), Today: "2026-09-27"})
	if err != nil || explicit.AgeMonths != 0 {
		t.Fatalf("explicit newborn age changed: %+v, %v", explicit, err)
	}
	contextual, err := callAs(t, fx, GetSameAge, GetSameAgeRequest{IncludeAvailableAges: true, FromPersonId: fx.alice.Id, Today: "2026-09-27"})
	if err != nil || contextual.AgeMonths != monthsBetween(fx.alice.Birthday, day("2026-09-27")) {
		t.Fatalf("contextual age changed: %+v, %v", contextual, err)
	}
}

func TestBestSameAge(t *testing.T) {
	options := []SameAgeOption{{6, 2}, {12, 2}, {18, 1}}
	if got := bestSameAge(options, 16); got != 12 {
		t.Errorf("best = %d, want closest two-person age 12", got)
	}
	if got := bestSameAge(nil, 16); got != 16 {
		t.Errorf("empty fallback = %d, want 16", got)
	}
}

func TestSameAgeOptionsIgnoreHiddenAndInvalidRecords(t *testing.T) {
	fx, cleanup := setupIsolationFixture(t)
	defer cleanup()
	token, err := generateJwtTokenString(fx.outsider)
	if err != nil {
		t.Fatal(err)
	}
	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		resp, err := GetSameAge(&vbeam.Context{Tx: tx, Token: token}, GetSameAgeRequest{IncludeAvailableAges: true, Today: "2026-09-27"})
		if err != nil {
			t.Fatal(err)
		}
		if len(resp.AvailableAges) != 0 {
			t.Errorf("outsider sees ages: %+v", resp.AvailableAges)
		}
	})
	token, err = generateJwtTokenString(fx.owner)
	if err != nil {
		t.Fatal(err)
	}
	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		person := fx.person
		people := []FamilyTimelineItem{{Person: person, Milestones: []Milestone{
			{MilestoneDate: person.Birthday.AddDate(0, 0, -1)},
			{MilestoneDate: day("2030-01-01")},
		}}}
		if got, _ := sameAgeOptions(&vbeam.Context{Tx: tx}, fx.owner, people, day("2026-09-27")); len(got) != 0 {
			t.Errorf("invalid record ages = %+v", got)
		}
	})
}

func TestSameAgePhotoDays(t *testing.T) {
	born := day("2020-02-15")
	if from, to := sameAgePhotoDays(born, 0); from != "2020-02-15" || to != "2020-03-13" {
		t.Errorf("newborn = %s..%s, want the day of birth through day 27", from, to)
	}
	if from, _ := sameAgePhotoDays(born, 1); from < "2020-02-15" {
		t.Errorf("one month starts %s, before birth", from)
	}
}

func TestSameAgeNewbornPortraits(t *testing.T) {
	fx := setupResultsFixture(t)
	born := fx.alice.Birthday
	// Late evening on the day of birth stays on that calendar day.
	birthDay := fx.addPhotoAt(t, "birth-day.jpg", born.Add(23*time.Hour+30*time.Minute))
	dayTwentySeven := fx.addPhotoAt(t, "day-27.jpg", born.AddDate(0, 0, 27))
	dayTwentyEight := fx.addPhotoAt(t, "day-28.jpg", born.AddDate(0, 0, 28))
	beforeBirth := fx.addPhotoAt(t, "before-birth.jpg", born.AddDate(0, 0, -1))
	threeMonths := fx.addPhotoAt(t, "three-months.jpg", born.AddDate(0, 3, 0))
	for _, p := range []Image{birthDay, dayTwentySeven, dayTwentyEight, beforeBirth, threeMonths} {
		fx.tagPerson(t, p, fx.alice)
	}
	// Bob has only a three-month photo and a measurement at birth.
	bobLater := fx.addPhotoAt(t, "bob-three-months.jpg", fx.bob.Birthday.AddDate(0, 3, 0))
	fx.tagPerson(t, bobLater, fx.bob)

	resp, err := callAs(t, fx, GetSameAge, GetSameAgeRequest{IncludeAvailableAges: true, AgeMonths: months(0), Today: "2026-09-27"})
	if err != nil {
		t.Fatal(err)
	}
	if resp.AgeMonths != 0 {
		t.Fatalf("age = %d, want the requested newborn age", resp.AgeMonths)
	}
	got := map[int]bool{}
	for _, row := range resp.Rows {
		if row.Person.Id == fx.bob.Id && len(row.Portraits) != 0 {
			t.Errorf("bob's newborn portraits = %+v, want none rather than a later photo", row.Portraits)
		}
		if row.Person.Id == fx.alice.Id {
			for _, p := range row.Portraits {
				got[p.PhotoId] = true
			}
		}
	}
	if len(got) != 2 || !got[birthDay.Id] || !got[dayTwentySeven.Id] {
		t.Errorf("alice's newborn portraits = %v, want only the birth-day and day-27 photos", got)
	}

	portraitCount := func(ages []SameAgeOption, m int) int {
		for _, a := range ages {
			if a.AgeMonths == m {
				return a.PeopleCount
			}
		}
		return 0
	}
	if n := portraitCount(resp.PortraitAges, 0); n != 1 {
		t.Errorf("newborn portrait coverage = %d, want only alice", n)
	}
	if n := portraitCount(resp.PortraitAges, 3); n != 2 {
		t.Errorf("three-month portrait coverage = %d, want 2", n)
	}
}

func TestSameAgeDefaultFollowsTheView(t *testing.T) {
	fx := setupResultsFixture(t)
	photo := fx.addPhotoAt(t, "alice-six-months.jpg", fx.alice.Birthday.AddDate(0, 6, 0))
	fx.tagPerson(t, photo, fx.alice)
	nine := dayOf(fx.alice.Birthday.AddDate(0, 9, 0))
	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		for _, p := range []Person{fx.alice, fx.bob} {
			if _, err := AddMilestoneTx(tx, AddMilestoneRequest{
				PersonId: p.Id, Description: "Crawled", Category: "development",
				InputType: "date", MilestoneDate: &nine,
			}, fx.familyId); err != nil {
				t.Fatal(err)
			}
		}
		vbolt.TxCommit(tx)
	})

	portraits, err := callAs(t, fx, GetSameAge, GetSameAgeRequest{IncludeAvailableAges: true, Today: "2026-09-27"})
	if err != nil {
		t.Fatal(err)
	}
	if portraits.AgeMonths != 6 {
		t.Errorf("portrait default = %d, want six months where there is a photo", portraits.AgeMonths)
	}
	details, err := callAs(t, fx, GetSameAge, GetSameAgeRequest{IncludeAvailableAges: true, Details: true, Today: "2026-09-27"})
	if err != nil {
		t.Fatal(err)
	}
	if details.AgeMonths != 9 {
		t.Errorf("details default = %d, want nine months with two people's milestones", details.AgeMonths)
	}
}
