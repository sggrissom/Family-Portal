package backend

import (
	"testing"
	"time"

	"go.hasen.dev/vbeam"
	"go.hasen.dev/vbolt"
)

func day(s string) time.Time {
	t, err := time.Parse(dateOnlyLayout, s)
	if err != nil {
		panic(err)
	}
	return t
}

func TestMeasureNudge(t *testing.T) {
	growth := func(date string) []GrowthData {
		return []GrowthData{{MeasurementDate: day(date)}}
	}
	child := Person{Id: 4, Name: "Clara Whitfield", Birthday: day("2023-05-02")}
	baby := Person{Id: 5, Name: "Rowan Whitfield", Birthday: day("2026-01-10")}
	adult := Person{Id: 6, Name: "Owen Whitfield", Birthday: day("1985-03-01")}

	tests := []struct {
		name   string
		person Person
		growth []GrowthData
		want   string
	}{
		{"a child measured within six months", child, growth("2026-04-01"), ""},
		{"a child not measured in six months", child, growth("2026-03-20"), "Clara hasn't been measured since March"},
		{"last measured in an earlier year", child, growth("2025-11-02"), "Clara hasn't been measured since November 2025"},
		{"a baby waits only three months", baby, growth("2026-06-20"), "Rowan hasn't been measured since June"},
		{"a baby measured recently", baby, growth("2026-08-01"), ""},
		{"never measured", child, nil, ""},
		{"an adult", adult, growth("2020-01-01"), ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nudge, ok := measureNudge(tt.person, tt.growth, day("2026-09-27"))
			if tt.want == "" {
				if ok {
					t.Fatalf("measureNudge() = %q, want none", nudge.Text)
				}
				return
			}
			if !ok || nudge.Text != tt.want {
				t.Fatalf("measureNudge() = %q, %v; want %q", nudge.Text, ok, tt.want)
			}
			if nudge.PersonId != tt.person.Id {
				t.Errorf("PersonId = %d, want %d", nudge.PersonId, tt.person.Id)
			}
		})
	}
}

func TestBirthdayNudge(t *testing.T) {
	jake := Person{Id: 3, Name: "Jake", Birthday: day("2019-10-03")}
	today := day("2026-09-27")

	tests := []struct {
		name  string
		today time.Time
		want  string
	}{
		{"within the week", today, "Jake turns 7 on Saturday"},
		{"tomorrow", day("2026-10-02"), "Jake turns 7 tomorrow"},
		{"today", day("2026-10-03"), "Jake turns 7 today"},
		{"too far off", day("2026-09-20"), ""},
		{"just passed", day("2026-10-04"), ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nudge, ok := birthdayNudge(jake, tt.today)
			if tt.want == "" {
				if ok {
					t.Fatalf("birthdayNudge() = %q, want none", nudge.Text)
				}
				return
			}
			if !ok || nudge.Text != tt.want {
				t.Fatalf("birthdayNudge() = %q, %v; want %q", nudge.Text, ok, tt.want)
			}
		})
	}

	due := Person{Id: 9, Name: "Baby", Birthday: day("2026-09-30"), IsPregnancy: true}
	if _, ok := birthdayNudge(due, today); ok {
		t.Error("a pregnancy should not get a birthday nudge")
	}
}

func TestBirthdayTodayOutranksOtherNudges(t *testing.T) {
	birthday, _ := birthdayNudge(Person{Name: "Jake", Birthday: day("2019-09-27")}, day("2026-09-27"))
	faces, _ := faceNudge(4)
	if birthday.priority >= faces.priority {
		t.Errorf("birthday today priority %d, faces %d", birthday.priority, faces.priority)
	}
	if faces.Text != "4 photos have faces to review" {
		t.Errorf("faces text = %q", faces.Text)
	}
	if one, _ := faceNudge(1); one.Text != "1 photo has faces to review" {
		t.Errorf("single face text = %q", one.Text)
	}
}

func TestPickSeasonEvent(t *testing.T) {
	events := []Event{
		{Id: 1, StartDate: day("2026-09-05"), EndDate: day("2026-09-06")},
		{Id: 2, StartDate: day("2026-10-10")},
		{Id: 3, StartDate: day("2026-11-14")},
	}

	if event, timing := pickSeasonEvent(events, day("2026-09-06")); event.Id != 1 || timing != "now" {
		t.Errorf("during an event: got %d %q", event.Id, timing)
	}
	if event, timing := pickSeasonEvent(events, day("2026-09-27")); event.Id != 2 || timing != "next" {
		t.Errorf("between events: got %d %q", event.Id, timing)
	}
	if event, timing := pickSeasonEvent(events, day("2026-12-01")); event.Id != 3 || timing != "last" {
		t.Errorf("after the last event: got %d %q", event.Id, timing)
	}
	if event, _ := pickSeasonEvent(nil, day("2026-12-01")); event != nil {
		t.Errorf("no events: got %d", event.Id)
	}
}

func TestInResultWindow(t *testing.T) {
	event := Event{StartDate: day("2026-09-19"), EndDate: day("2026-09-20")}
	for date, want := range map[string]bool{
		"2026-09-18": false,
		"2026-09-19": true,
		"2026-09-23": true,
		"2026-09-24": false,
	} {
		if got := inResultWindow(event, day(date)); got != want {
			t.Errorf("inResultWindow(%s) = %v, want %v", date, got, want)
		}
	}
}

func TestGetDashboard(t *testing.T) {
	fx, cleanup := setupIsolationFixture(t)
	defer cleanup()

	call := func(user User, today string) GetDashboardResponse {
		t.Helper()
		token, err := generateJwtTokenString(user)
		if err != nil {
			t.Fatalf("generateJwtTokenString() error = %v", err)
		}
		var resp GetDashboardResponse
		vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
			resp, err = GetDashboard(&vbeam.Context{Tx: tx, Token: token}, GetDashboardRequest{Today: today})
		})
		if err != nil {
			t.Fatalf("GetDashboard() error = %v", err)
		}
		return resp
	}

	today := dayOf(time.Now())
	resp := call(fx.owner, today)

	if resp.Today != today {
		t.Errorf("Today = %q, want %q", resp.Today, today)
	}
	if len(resp.Seasons) != 1 || resp.Seasons[0].Event == nil || resp.Seasons[0].EventTiming != "now" {
		t.Fatalf("Seasons = %+v, want the fixture season with its event running", resp.Seasons)
	}
	if !resp.Seasons[0].CanAddResults {
		t.Error("the owner should be able to add results for an event running today")
	}

	var measure *DashboardNudge
	for i := range resp.Nudges {
		if resp.Nudges[i].Kind == "measure" {
			measure = &resp.Nudges[i]
		}
	}
	if measure == nil || measure.PersonId != fx.person.Id {
		t.Errorf("Nudges = %+v, want a measure nudge for the fixture child", resp.Nudges)
	}

	anniversary := call(fx.owner, "2026-02-11")
	if len(anniversary.OnThisDay) != 1 || anniversary.OnThisDay[0].YearsAgo != 2 ||
		len(anniversary.OnThisDay[0].Milestones) != 1 {
		t.Errorf("OnThisDay = %+v, want the 2024-02-10 milestone two years ago", anniversary.OnThisDay)
	}

	outsider := call(fx.outsider, today)
	if len(outsider.Seasons) != 0 {
		t.Errorf("an outsider saw %d seasons from another family", len(outsider.Seasons))
	}
	for _, nudge := range outsider.Nudges {
		if nudge.PersonId == fx.person.Id {
			t.Errorf("an outsider got a nudge about another family's child: %q", nudge.Text)
		}
	}
}

func TestListOpenEvents(t *testing.T) {
	fx, cleanup := setupIsolationFixture(t)
	defer cleanup()

	token, err := generateJwtTokenString(fx.owner)
	if err != nil {
		t.Fatalf("generateJwtTokenString() error = %v", err)
	}
	var resp ListOpenEventsResponse
	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		resp, err = ListOpenEvents(&vbeam.Context{Tx: tx, Token: token}, ListOpenEventsRequest{Today: dayOf(time.Now())})
	})
	if err != nil {
		t.Fatalf("ListOpenEvents() error = %v", err)
	}
	if len(resp.Events) != 1 || resp.Events[0].Event.Id != fx.event.Id || resp.Events[0].ActivityName != "Dance" {
		t.Errorf("Events = %+v, want the fixture event", resp.Events)
	}
}
