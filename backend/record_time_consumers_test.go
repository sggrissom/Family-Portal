package backend

import (
	"slices"
	"strings"
	"testing"
	"time"

	"go.hasen.dev/vbeam"
	"go.hasen.dev/vbolt"
)

type rtConsumerFixture struct {
	isolationFixture
	t *testing.T
}

func (fx rtConsumerFixture) addMoment(description string, rt RecordTime) Milestone {
	var m Milestone
	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		m = Milestone{Id: vbolt.NextIntId(tx, MilestoneBkt), PersonId: fx.person.Id, FamilyId: fx.ownerFamily,
			Description: description, Category: "other", RecordTime: rt, CreatedAt: time.Now()}
		writeMilestone(tx, &m)
		vbolt.TxCommit(tx)
	})
	return m
}

func (fx rtConsumerFixture) addPhoto(title string, rt RecordTime, at time.Time) Image {
	var image Image
	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		image = Image{Id: vbolt.NextIntId(tx, ImagesBkt), FamilyId: fx.ownerFamily, OwnerUserId: fx.owner.Id,
			Title: title, MimeType: "image/jpeg", FilePath: "photos/x.jpg", CreatedAt: time.Now(),
			PhotoDate: at, RecordTime: rt}
		writeImage(tx, &image)
		vbolt.SetTargetSingleTerm(tx, ImageByFamilyIndex, image.Id, fx.ownerFamily)
		AddPersonToPhoto(tx, image.Id, fx.person.Id, fx.ownerFamily)
		ReindexPhotoDates(tx, image.Id)
		vbolt.TxCommit(tx)
	})
	return image
}

func (fx rtConsumerFixture) ctx(tx *vbolt.Tx) *vbeam.Context {
	token, err := generateJwtTokenString(fx.owner)
	if err != nil {
		fx.t.Fatal(err)
	}
	return &vbeam.Context{Tx: tx, Token: token}
}

func setupRecordTimeConsumers(t *testing.T) rtConsumerFixture {
	fx, cleanup := setupIsolationFixture(t)
	t.Cleanup(cleanup)
	if !fx.person.Birthday.Equal(day("2020-06-15")) {
		t.Fatalf("fixture birthday = %v", fx.person.Birthday)
	}
	return rtConsumerFixture{fx, t}
}

func ageOccurrence(anchor, years int) RecordTime {
	rt := agePeriod(anchor, years)
	rt.Meaning = MeaningOccurrence
	return rt
}

func TestAgePeriodsNeverBecomeAnniversaries(t *testing.T) {
	fx := setupRecordTimeConsumers(t)
	for i := 0; i < 20; i++ {
		fx.addMoment("loved painting", agePeriod(fx.person.Id, 4))
	}
	fx.addMoment("rode a bike", ageOccurrence(fx.person.Id, 4))
	estimate := exactRecordTime(day("2024-06-15"), SourceFamily)
	estimate.Estimated = true
	fx.addMoment("around then", estimate)
	party := fx.addMoment("fourth birthday party", exactRecordTime(day("2024-06-15"), SourceFamily))

	for i := 0; i < onThisDayPhotos+2; i++ {
		fx.addPhoto("sometime at four", ageOccurrence(fx.person.Id, 4), time.Time{})
	}
	cake := fx.addPhoto("cake", exactRecordTime(day("2024-06-15"), SourceFamily), time.Date(2024, 6, 15, 16, 0, 0, 0, time.UTC))

	var resp GetDashboardResponse
	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		var err error
		resp, err = GetDashboard(fx.ctx(tx), GetDashboardRequest{Today: "2025-06-15"})
		if err != nil {
			t.Fatal(err)
		}
	})
	if len(resp.OnThisDay) != 1 || resp.OnThisDay[0].YearsAgo != 1 {
		t.Fatalf("OnThisDay = %+v", resp.OnThisDay)
	}
	year := resp.OnThisDay[0]
	if len(year.Milestones) != 1 || year.Milestones[0].Id != party.Id {
		t.Errorf("milestones = %v, want only the birthday party", descriptions(year.Milestones))
	}
	if len(year.Photos) != 1 || year.Photos[0].Id != cake.Id {
		t.Errorf("photos = %d, want only the exact birthday photo", len(year.Photos))
	}
}

func descriptions(ms []Milestone) []string {
	out := []string{}
	for _, m := range ms {
		out = append(out, m.Description)
	}
	return out
}

func TestRecentLeavesOutBroadWindows(t *testing.T) {
	fx := setupRecordTimeConsumers(t)
	month := RecordTime{Version: RecordTimeVersion, Meaning: MeaningOccurrence, Basis: BasisCalendar,
		Precision: PrecisionMonth, From: "2026-11", Source: SourceFamily}
	fx.addMoment("sometime this month", month)
	today := fx.addMoment("today", exactRecordTime(day("2026-11-02"), SourceFamily))

	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		resp, err := GetDashboard(fx.ctx(tx), GetDashboardRequest{Today: "2026-11-02"})
		if err != nil {
			t.Fatal(err)
		}
		if got := descriptions(resp.Recent.Milestones); len(got) != 1 || got[0] != today.Description {
			t.Errorf("recent = %v", got)
		}
	})
}

func TestSearchMatchesAgePeriodsByOverlap(t *testing.T) {
	fx := setupRecordTimeConsumers(t)
	fx.addMoment("loved painting", agePeriod(fx.person.Id, 4))
	fx.addMoment("first day of school", exactRecordTime(day("2025-09-02"), SourceFamily))

	search := func(query string) []string {
		var resp SearchResponse
		vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
			var err error
			resp, err = Search(fx.ctx(tx), SearchRequest{Query: query})
			if err != nil {
				t.Fatal(err)
			}
		})
		return descriptions(resp.Moments)
	}

	for _, query := range []string{"2024", "2025", "painting 2025"} {
		if got := search(query); !slices.Contains(got, "loved painting") {
			t.Errorf("%q = %v, want the age-4 period, which spans 2024-06-15 to 2025-06-15", query, got)
		}
	}
	if got := search("2026"); slices.Contains(got, "loved painting") {
		t.Errorf("2026 = %v, the period ended in 2025", got)
	}
	if got := search("june"); slices.Contains(got, "loved painting") {
		t.Errorf("a month without a year must not match a year-long span: %v", got)
	}
	if got := search("september"); !slices.Contains(got, "first day of school") {
		t.Errorf("september = %v", got)
	}
}

func TestSearchAgeUsesTheAssertionWithoutABirthday(t *testing.T) {
	rt := agePeriod(7, 4)
	from, to, ok := rt.AgeMonthsFor(Person{Id: 7})
	if !ok || from != 48 || to != 60 {
		t.Errorf("anchored age span = [%d, %d) %v", from, to, ok)
	}
	if _, _, ok := rt.AgeMonthsFor(Person{Id: 8}); ok {
		t.Error("someone else's age needs a calendar placement and a birthday")
	}

	exact := exactRecordTime(day("2024-09-14"), SourceFamily)
	from, to, ok = exact.AgeMonthsFor(Person{Id: 8, Birthday: day("2020-06-15")})
	if !ok || from != 50 || to != 51 {
		t.Errorf("exact day age span = [%d, %d) %v", from, to, ok)
	}
}

func TestSameAgeKeepsApproximateMomentsAsContext(t *testing.T) {
	fx := setupRecordTimeConsumers(t)
	fx.addMoment("loved painting", agePeriod(fx.person.Id, 4))
	fx.addMoment("ages four to six", RecordTime{Version: RecordTimeVersion, Meaning: MeaningPeriod, Basis: BasisAge,
		Precision: PrecisionRange, AnchorPersonId: fx.person.Id, AgeYears: 4, AgeToYears: 6, Source: SourceFamily})
	fx.addMoment("lost a tooth", exactRecordTime(day("2024-12-15"), SourceFamily))

	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		age := 54
		resp, err := GetSameAge(fx.ctx(tx), GetSameAgeRequest{AgeMonths: &age, FromPersonId: fx.person.Id, Today: "2026-11-02"})
		if err != nil {
			t.Fatal(err)
		}
		var row *SameAgeRow
		for i := range resp.Rows {
			if resp.Rows[i].Person.Id == fx.person.Id {
				row = &resp.Rows[i]
			}
		}
		if row == nil {
			t.Fatal("no row for the fixture child")
		}
		if got := descriptions(row.Milestones); len(got) != 1 || got[0] != "lost a tooth" {
			t.Errorf("precise = %v", got)
		}
		if got := descriptions(row.ContextMilestones); strings.Join(got, ",") != "loved painting,ages four to six" {
			t.Errorf("context = %v, want the narrower period first", got)
		}
	})
}

func TestTimelineWindowIncludesOverlappingPeriods(t *testing.T) {
	fx := setupRecordTimeConsumers(t)
	fx.addMoment("loved painting", agePeriod(fx.person.Id, 4))
	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		resp, err := GetFamilyTimeline(fx.ctx(tx), GetFamilyTimelineRequest{From: "2025-01-01", To: "2025-01-31", SkipPhotos: true})
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, item := range resp.People {
			found = found || slices.Contains(descriptions(item.Milestones), "loved painting")
		}
		if !found {
			t.Error("January 2025 is inside age 4, so the period belongs in that window")
		}
		if !slices.Contains(resp.Years, 2024) || !slices.Contains(resp.Years, 2025) {
			t.Errorf("years = %v, want both years the period spans", resp.Years)
		}
	})
}

func TestRecordFileDate(t *testing.T) {
	if got := recordFileDate(exactRecordTime(day("2022-10-31"), SourceFamily)); got != "2022-10-31" {
		t.Errorf("exact = %q", got)
	}
	rt := RecordTime{Version: RecordTimeVersion, Meaning: MeaningOccurrence, Basis: BasisCalendar,
		Precision: PrecisionRange, Estimated: true, From: "1968", To: "1972", Around: "1970", Source: SourceFamily}
	if got := recordFileDate(rt); got != "around-1970-1968-1972" {
		t.Errorf("estimate = %q", got)
	}
}

func yearRange(from, to string) RecordTime {
	return RecordTime{Version: RecordTimeVersion, Meaning: MeaningOccurrence, Basis: BasisCalendar,
		Precision: PrecisionRange, From: from, To: to, Source: SourceFamily}
}

func (fx rtConsumerFixture) listPhotos(req ListFamilyPhotosRequest) (titles []string, cursor string) {
	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		resp, err := ListFamilyPhotos(fx.ctx(tx), req)
		if err != nil {
			fx.t.Fatal(err)
		}
		for _, p := range resp.Photos {
			titles = append(titles, p.Image.Title)
		}
		cursor = resp.NextCursor
	})
	return
}

func TestPhotoDateFilterFindsOverlappingApproximatePhotos(t *testing.T) {
	fx := setupRecordTimeConsumers(t)
	fx.addPhoto("school years", yearRange("2022", "2025"), time.Time{})
	fx.addPhoto("sometime at four", ageOccurrence(fx.person.Id, 4), time.Time{})
	fx.addPhoto("shoebox", unknownRecordTime(SourceFamily), time.Time{})
	for _, d := range []string{"2024-03-01", "2024-08-01", "2024-11-01"} {
		fx.addPhoto(d, exactRecordTime(day(d), SourceFamily), day(d))
	}

	all, _ := fx.listPhotos(ListFamilyPhotosRequest{})
	if slices.Index(all, "shoebox") < slices.Index(all, "school years") {
		t.Errorf("unfiltered = %v, want the undated photo after every dated one", all)
	}

	var paged []string
	cursor := ""
	for {
		titles, next := fx.listPhotos(ListFamilyPhotosRequest{DateFrom: "2024-06-01", DateTo: "2024-12-31", Limit: 2, Cursor: cursor})
		paged = append(paged, titles...)
		if next == "" {
			break
		}
		cursor = next
	}
	want := "2024-11-01,2024-08-01,sometime at four,school years"
	if strings.Join(paged, ",") != want {
		t.Errorf("second half of 2024, two per page = %v, want %s", paged, want)
	}

	first, _ := fx.listPhotos(ListFamilyPhotosRequest{DateFrom: "2024-01-01", DateTo: "2024-05-31"})
	if strings.Join(first, ",") != "2024-03-01,school years" {
		t.Errorf("first half of 2024 = %v; age 4 starts on 2024-06-15", first)
	}

	exact, _ := fx.listPhotos(ListFamilyPhotosRequest{DateFrom: "2024-06-01", DateTo: "2024-12-31", ExactDatesOnly: true})
	if strings.Join(exact, ",") != "2024-11-01,2024-08-01" {
		t.Errorf("exact only = %v", exact)
	}

	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		resp, err := GetFamilyTimeline(fx.ctx(tx), GetFamilyTimelineRequest{From: "2023-01-01", To: "2023-12-31"})
		if err != nil {
			t.Fatal(err)
		}
		var titles []string
		for _, item := range resp.People {
			for _, p := range item.Photos {
				titles = append(titles, p.Title)
			}
		}
		if !slices.Contains(titles, "school years") || slices.Contains(titles, "shoebox") {
			t.Errorf("2023 timeline photos = %v", titles)
		}
	})
}
