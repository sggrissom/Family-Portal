//go:build !release

package backend

// Demo data for a development database.
//
// The tool that calls this is cmd/seed; `make seed-fresh` throws away
// .serve/db.bolt and rebuilds it from here, so a fresh checkout is one command
// away from a populated site with credentials you already know.
//
// This file is excluded from release builds. Every account below shares one
// hardcoded password, and that must never be compiled into a binary that could
// be deployed.
//
// The shape of the data is chosen to exercise both access mechanisms described
// in docs/permissions.md at once: memberships inside the Rivera household
// (including the sub-admin roles nothing in the UI currently issues), and
// family links outward to two sets of grandparents with deliberately different
// scopes, one of them still pending.

import (
	"fmt"
	"math"
	"math/rand"
	"time"

	"go.hasen.dev/vbolt"
	"golang.org/x/crypto/bcrypt"
)

// SeedPassword signs in every seeded account.
const SeedPassword = "family123"

type SeedOptions struct {
	// Password for every seeded account. Empty means SeedPassword.
	Password string
	// Scale divides the interval between measurements, so 2 produces twice as
	// many rows. Zero or less means 1.
	Scale int
	// Now anchors ages, activity seasons, and chat timestamps. Zero means
	// time.Now().
	Now time.Time
}

// SeedAccount is one row of the credentials table the seeder prints.
type SeedAccount struct {
	Email  string
	Name   string
	Family string
	Access string
}

type SeedSummary struct {
	Accounts     []SeedAccount
	Families     int
	People       int
	Relations    int
	Links        int
	Shares       int
	Tags         int
	Milestones   int
	Measurements int
	Activities   int
	Events       int
	Results      int
	ChatMessages int
}

// SeedDemoData writes the whole demo dataset into tx. The caller commits.
func SeedDemoData(tx *vbolt.Tx, opts SeedOptions) (SeedSummary, error) {
	password := opts.Password
	if password == "" {
		password = SeedPassword
	}
	scale := opts.Scale
	if scale < 1 {
		scale = 1
	}
	now := opts.Now
	if now.IsZero() {
		now = time.Now()
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return SeedSummary{}, err
	}

	s := &seeder{
		tx:   tx,
		now:  now,
		hash: hash,
		rng:  rand.New(rand.NewSource(20260829)),
	}
	s.build(scale)
	return s.sum, s.err
}

type seeder struct {
	tx   *vbolt.Tx
	now  time.Time
	hash []byte
	rng  *rand.Rand
	sum  SeedSummary
	err  error
}

func (s *seeder) fail(err error) {
	if s.err == nil {
		s.err = err
	}
}

// ── primitives ────────────────────────────────────────────────────────────────

// household creates a user who founds a family, named for the family rather
// than for the user, since AddUserTx would otherwise call it "X's Family".
func (s *seeder) household(name, email, familyName string) (User, Family) {
	user := AddUserTx(s.tx, CreateAccountRequest{Name: name, Email: email}, s.hash)
	user.EmailVerified = true
	vbolt.Write(s.tx, UsersBkt, user.Id, &user)

	family := GetFamily(s.tx, user.FamilyId)
	family.Name = familyName
	vbolt.Write(s.tx, FamiliesBkt, family.Id, &family)

	s.sum.Families++
	return user, family
}

// owner creates a household and records the credentials for it.
func (s *seeder) owner(name, email, familyName, access string) (User, Family) {
	user, family := s.household(name, email, familyName)
	s.account(user, family.Name, access)
	return user, family
}

// member creates a user who registers with an existing family's invite code,
// which is what the signup form does. AddUserTx always grants admin, and there
// is no point issuing anything less: CanAccessFamily falls back to
// User.FamilyId and grants admin on a user's own household regardless of what
// their membership row says. A limited role only bites on a family that is not
// the user's primary one — see guest.
func (s *seeder) member(name, email string, family Family, access string) User {
	user := AddUserTx(s.tx, CreateAccountRequest{
		Name:       name,
		Email:      email,
		FamilyCode: family.InviteCode,
	}, s.hash)
	user.EmailVerified = true
	vbolt.Write(s.tx, UsersBkt, user.Id, &user)

	s.account(user, family.Name, access)
	return user
}

// guest creates a user with a household of their own plus a secondary
// membership in someone else's at a role below admin. That combination is the
// only shape in which a reduced role has any effect, so it is how a caregiver
// or a helper who is not family gets a foothold.
func (s *seeder) guest(name, email, ownFamily string, into Family, role AccessLevel, access string) User {
	user, _ := s.household(name, email, ownFamily)
	EnsureMembershipTx(s.tx, user.Id, into.Id, role)
	s.account(user, into.Name+" + "+ownFamily, access)
	return user
}

func (s *seeder) account(user User, family, access string) {
	s.sum.Accounts = append(s.sum.Accounts, SeedAccount{
		Email:  user.Email,
		Name:   user.Name,
		Family: family,
		Access: access,
	})
}

// represent points a user at the person record standing for them, which is
// what relationship labels are phrased against.
func (s *seeder) represent(user User, person Person) {
	user.PersonId = person.Id
	vbolt.Write(s.tx, UsersBkt, user.Id, &user)
}

func (s *seeder) person(familyId int, name string, gender GenderType, birthdate string) Person {
	person, err := AddPersonTx(s.tx, AddPersonRequest{
		Name:      name,
		Gender:    int(gender),
		Birthdate: birthdate,
	}, familyId)
	if err != nil {
		s.fail(fmt.Errorf("person %q: %w", name, err))
		return Person{}
	}
	s.sum.People++
	return person
}

func (s *seeder) pregnancy(familyId int, name string, dueDate string) Person {
	person, err := AddPersonTx(s.tx, AddPersonRequest{
		Name:        name,
		Gender:      int(Unknown),
		Birthdate:   dueDate,
		IsPregnancy: true,
	}, familyId)
	if err != nil {
		s.fail(fmt.Errorf("pregnancy %q: %w", name, err))
		return Person{}
	}
	s.sum.People++
	return person
}

func (s *seeder) relate(from Person, to Person, kind RelationKind) {
	if _, err := AddRelationTx(s.tx, Relation{FromId: from.Id, ToId: to.Id, Kind: kind}); err != nil {
		s.fail(fmt.Errorf("relation %d->%d: %w", from.Id, to.Id, err))
		return
	}
	s.sum.Relations++
}

func (s *seeder) parents(father, mother Person, children ...Person) {
	for _, child := range children {
		s.relate(father, child, RelationParent)
		s.relate(mother, child, RelationParent)
	}
}

func (s *seeder) link(from, to Family, kind string, scopes LinkScopes, status LinkStatus) FamilyLink {
	link := createFamilyLinkTx(s.tx, from.Id, to.Id, kind, AccessView, normalizeLinkScopes(scopes).ToMask())
	link.Status = status
	writeFamilyLinkTx(s.tx, link)
	s.sum.Links++
	return link
}

func (s *seeder) share(person Person, into Family, relationship string) {
	EnsurePersonFamilyTx(s.tx, person.Id, into.Id)
	SetPersonFamilyRelationshipTx(s.tx, person.Id, into.Id, relationship)
	s.sum.Shares++
}

func (s *seeder) tag(familyId int, name, color string) Tag {
	tag := Tag{
		Id:        vbolt.NextIntId(s.tx, TagBkt),
		FamilyId:  familyId,
		Name:      name,
		Color:     color,
		CreatedAt: s.now,
	}
	vbolt.Write(s.tx, TagBkt, tag.Id, &tag)
	vbolt.SetTargetSingleTerm(s.tx, TagByFamilyIndex, tag.Id, familyId)
	s.sum.Tags++
	return tag
}

func (s *seeder) milestone(person Person, on time.Time, description, category string, tags ...Tag) {
	date := on.Format("2006-01-02")
	milestone, err := AddMilestoneTx(s.tx, AddMilestoneRequest{
		PersonId:      person.Id,
		Description:   description,
		Category:      category,
		InputType:     "date",
		MilestoneDate: &date,
	}, person.FamilyId)
	if err != nil {
		s.fail(fmt.Errorf("milestone %q: %w", description, err))
		return
	}
	for _, tag := range tags {
		addTagToMilestone(s.tx, milestone.Id, tag.Id, person.FamilyId)
	}
	s.sum.Milestones++
}

func (s *seeder) measurement(person Person, on time.Time, kind string, value float64, unit string) {
	date := on.Format("2006-01-02")
	_, err := AddGrowthDataTx(s.tx, AddGrowthDataRequest{
		PersonId:        person.Id,
		MeasurementType: kind,
		Value:           math.Round(value*10) / 10,
		Unit:            unit,
		InputType:       "date",
		MeasurementDate: &date,
	}, person.FamilyId)
	if err != nil {
		s.fail(fmt.Errorf("measurement for %q: %w", person.Name, err))
		return
	}
	s.sum.Measurements++
}

// chat writes the message row directly rather than through AddChatMessageTx,
// which stamps time.Now() and would pile the whole history onto one instant.
func (s *seeder) chat(familyId int, user User, at time.Time, content string) {
	message := ChatMessage{
		Id:        vbolt.NextIntId(s.tx, ChatMessagesBkt),
		FamilyId:  familyId,
		UserId:    user.Id,
		UserName:  user.Name,
		Content:   content,
		CreatedAt: at,
	}
	vbolt.Write(s.tx, ChatMessagesBkt, message.Id, &message)
	updateChatMessageIndices(s.tx, message)
	s.sum.ChatMessages++
}

// ── growth curves ─────────────────────────────────────────────────────────────

// Control points are (age in years, value), linearly interpolated between and
// held flat outside. They are eyeballed from CDC medians — close enough that a
// chart looks right, and not close enough to be worth citing.
type curvePoint struct {
	age   float64
	value float64
}

var heightInchesMale = []curvePoint{
	{0, 19.7}, {0.5, 26.6}, {1, 29.9}, {2, 34.2}, {3, 37.5}, {4, 40.3},
	{5, 43.0}, {6, 45.5}, {7, 47.7}, {8, 50.0}, {9, 52.0}, {10, 54.3},
	{11, 56.4}, {12, 58.9}, {13, 61.4}, {14, 64.5}, {15, 66.9}, {16, 68.3},
	{17, 69.1}, {18, 69.4}, {25, 69.8}, {70, 69.0},
}

var heightInchesFemale = []curvePoint{
	{0, 19.4}, {0.5, 25.9}, {1, 29.2}, {2, 33.7}, {3, 37.1}, {4, 39.8},
	{5, 42.5}, {6, 45.0}, {7, 47.3}, {8, 49.6}, {9, 51.9}, {10, 54.3},
	{11, 56.7}, {12, 59.4}, {13, 61.4}, {14, 62.5}, {15, 63.4}, {16, 63.8},
	{17, 64.0}, {18, 64.1}, {25, 64.5}, {70, 63.6},
}

var weightPoundsMale = []curvePoint{
	{0, 7.5}, {0.5, 17.0}, {1, 21.0}, {2, 27.5}, {3, 31.0}, {4, 36.0},
	{5, 40.5}, {6, 45.5}, {7, 51.0}, {8, 57.0}, {9, 64.0}, {10, 71.0},
	{11, 79.0}, {12, 89.0}, {13, 100.0}, {14, 112.0}, {15, 124.0}, {16, 134.0},
	{17, 142.0}, {18, 147.0}, {30, 176.0}, {70, 184.0},
}

var weightPoundsFemale = []curvePoint{
	{0, 7.2}, {0.5, 16.0}, {1, 19.8}, {2, 26.5}, {3, 30.0}, {4, 35.0},
	{5, 39.5}, {6, 44.0}, {7, 49.5}, {8, 56.0}, {9, 63.0}, {10, 71.5},
	{11, 81.0}, {12, 91.5}, {13, 101.0}, {14, 109.0}, {15, 115.0}, {16, 118.0},
	{17, 120.0}, {18, 121.0}, {30, 143.0}, {70, 152.0},
}

func curveAt(points []curvePoint, age float64) float64 {
	if age <= points[0].age {
		return points[0].value
	}
	for i := 1; i < len(points); i++ {
		if age > points[i].age {
			continue
		}
		lo, hi := points[i-1], points[i]
		span := hi.age - lo.age
		if span <= 0 {
			return hi.value
		}
		return lo.value + (hi.value-lo.value)*(age-lo.age)/span
	}
	return points[len(points)-1].value
}

func curvesFor(gender GenderType) (height, weight []curvePoint) {
	if gender == Female {
		return heightInchesFemale, weightPoundsFemale
	}
	return heightInchesMale, weightPoundsMale
}

// measurementStepDays thins the series out as a person ages, the way a real
// record does: a newborn gets weighed at every checkup, a teenager once or
// twice a year.
func measurementStepDays(age float64) int {
	switch {
	case age < 1:
		return 60
	case age < 2:
		return 90
	case age < 5:
		return 122
	case age < 13:
		return 183
	default:
		return 244
	}
}

// growthSeries records height and weight from birth to now, following the
// median curve for the person's sex, offset by a per-person percentile so no
// two children trace the same line, and jittered so the chart is not a
// suspiciously smooth arc.
func (s *seeder) growthSeries(person Person, scale int) {
	if person.Id == 0 || person.IsPregnancy {
		return
	}
	heightPoints, weightPoints := curvesFor(person.Gender)
	heightPct := 0.94 + s.rng.Float64()*0.12
	weightPct := 0.90 + s.rng.Float64()*0.20

	start := person.Birthday
	// Adults have no plausible childhood record in a family portal, so their
	// history starts a few years back rather than at birth.
	if s.now.Sub(start) > 25*365*24*time.Hour {
		start = s.now.AddDate(-6, 0, 0)
	}

	for date := start; !date.After(s.now); {
		age := date.Sub(person.Birthday).Hours() / 24 / 365.2425
		s.measurement(person, date, "height",
			curveAt(heightPoints, age)*heightPct+s.rng.NormFloat64()*0.25, "in")
		s.measurement(person, date, "weight",
			curveAt(weightPoints, age)*weightPct+s.rng.NormFloat64()*0.8, "lbs")

		step := measurementStepDays(age) / scale
		if step < 7 {
			step = 7
		}
		date = date.AddDate(0, 0, step)
	}
}

// ── milestones ────────────────────────────────────────────────────────────────

type milestoneTemplate struct {
	years       int
	months      int
	description string
	category    string
	tags        []string
}

// Every child gets the templates that fall at or before their current age, so
// the teenager has a long history and the toddler has a short one, without
// either list being written out twice.
var childMilestones = []milestoneTemplate{
	{0, 2, "First real smile", "development", []string{"Firsts"}},
	{0, 4, "Rolled over unassisted", "development", []string{"Firsts"}},
	{0, 6, "First solid food — sweet potato, mostly on the wall", "first", []string{"Firsts", "Funny"}},
	{0, 9, "Crawled the length of the living room", "development", []string{"Firsts"}},
	{0, 11, "Said \"mama\" and meant it", "first", []string{"Firsts"}},
	{1, 0, "First birthday, first cake, first sugar crash", "achievement", []string{"Firsts", "Funny"}},
	{1, 2, "Took five steps without holding on", "development", []string{"Firsts"}},
	{1, 6, "Slept through the night a whole week running", "behavior", nil},
	{1, 9, "Learned to climb out of the crib, which nobody asked for", "behavior", []string{"Funny"}},
	{2, 0, "Two-word sentences, mostly demands", "development", nil},
	{2, 4, "Out of diapers", "achievement", nil},
	{2, 9, "Memorized every dinosaur name, corrects adults", "behavior", []string{"Funny"}},
	{3, 0, "First day of preschool", "first", []string{"School", "Firsts"}},
	{3, 4, "First stitches — coffee table, one to nothing", "health", nil},
	{3, 8, "Rode a balance bike the length of the driveway", "achievement", []string{"Sports"}},
	{4, 0, "Wrote own name legibly", "development", []string{"School"}},
	{4, 6, "First time seeing the ocean", "first", []string{"Travel", "Firsts"}},
	{5, 0, "Started kindergarten", "first", []string{"School", "Firsts"}},
	{5, 4, "Lost first tooth", "health", []string{"Firsts"}},
	{5, 10, "Swam a full length without floaties", "achievement", []string{"Sports"}},
	{6, 0, "Read a chapter book alone, start to finish", "achievement", []string{"School"}},
	{6, 8, "First soccer goal, celebrated for a week", "achievement", []string{"Sports"}},
	{7, 0, "Broke an arm on the monkey bars", "health", nil},
	{7, 6, "Joined the school choir", "achievement", []string{"School"}},
	{8, 0, "Flew on a plane, own seat, own snacks", "first", []string{"Travel", "Firsts"}},
	{8, 8, "Won the class spelling bee", "achievement", []string{"School"}},
	{9, 0, "First sleepover away from home", "first", []string{"Firsts"}},
	{9, 6, "Started piano lessons", "development", []string{"School"}},
	{10, 0, "Double digits — birthday at the trampoline park", "achievement", nil},
	{10, 6, "Cooked dinner for the family, unsupervised, edible", "achievement", []string{"Funny"}},
	{11, 0, "Made the travel team", "achievement", []string{"Sports"}},
	{11, 6, "Braces on", "health", nil},
	{12, 0, "First school dance", "first", []string{"School", "Firsts"}},
	{12, 6, "Grew two inches over one summer", "development", nil},
	{13, 0, "Became a teenager, immediately started sighing", "behavior", []string{"Funny"}},
	{13, 6, "First solo trip across town on the bus", "first", []string{"Firsts"}},
	{14, 0, "Started high school", "first", []string{"School", "Firsts"}},
	{14, 8, "Braces off", "health", nil},
	{15, 0, "First job — scooping ice cream on weekends", "first", []string{"Firsts"}},
	{15, 6, "Ran a 5K under 25 minutes", "achievement", []string{"Sports"}},
	{16, 0, "Learner's permit", "achievement", []string{"Firsts"}},
	{16, 6, "Drove the family to dinner, nobody gripped the door", "behavior", []string{"Funny"}},
	{17, 0, "First college visit", "first", []string{"School", "Travel"}},
}

func (s *seeder) childMilestones(person Person, tags map[string]Tag) {
	if person.Id == 0 || person.IsPregnancy {
		return
	}
	for _, template := range childMilestones {
		date := person.Birthday.AddDate(template.years, template.months, 0)
		if date.After(s.now) {
			break
		}
		s.milestone(person, date, template.description, template.category, lookupTags(tags, template.tags)...)
	}
}

func lookupTags(tags map[string]Tag, names []string) []Tag {
	var found []Tag
	for _, name := range names {
		if tag, ok := tags[name]; ok {
			found = append(found, tag)
		}
	}
	return found
}

// ── activities ────────────────────────────────────────────────────────────────

func (s *seeder) activity(familyId int, name, kind string) Activity {
	activity := Activity{
		Id:        vbolt.NextIntId(s.tx, ActivityBkt),
		FamilyId:  familyId,
		Name:      name,
		Kind:      kind,
		CreatedAt: s.now,
	}
	writeActivityTx(s.tx, &activity)
	s.sum.Activities++
	return activity
}

func (s *seeder) season(activity Activity, name string, start, end time.Time, notes string) Season {
	season := Season{
		Id:         vbolt.NextIntId(s.tx, SeasonBkt),
		ActivityId: activity.Id,
		FamilyId:   activity.FamilyId,
		Name:       name,
		StartDate:  start,
		EndDate:    end,
		Notes:      notes,
		CreatedAt:  s.now,
	}
	writeSeasonTx(s.tx, &season)
	return season
}

func (s *seeder) event(season Season, name, host, location string, start, end time.Time) Event {
	event := Event{
		Id:        vbolt.NextIntId(s.tx, EventBkt),
		SeasonId:  season.Id,
		FamilyId:  season.FamilyId,
		Name:      name,
		Host:      host,
		Location:  location,
		StartDate: start,
		EndDate:   end,
		CreatedAt: s.now,
	}
	writeEventTx(s.tx, &event)
	s.sum.Events++
	return event
}

func (s *seeder) entry(season Season, name, format, style, division, level string, members ...Person) Entry {
	entry := Entry{
		Id:        vbolt.NextIntId(s.tx, EntryBkt),
		SeasonId:  season.Id,
		FamilyId:  season.FamilyId,
		Name:      name,
		Format:    format,
		Style:     style,
		Division:  division,
		Level:     level,
		CreatedAt: s.now,
	}
	writeEntryTx(s.tx, &entry)
	for _, person := range members {
		member := EntryMember{
			Id:        vbolt.NextIntId(s.tx, EntryMemberBkt),
			EntryId:   entry.Id,
			PersonId:  person.Id,
			FamilyId:  entry.FamilyId,
			CreatedAt: s.now,
		}
		writeEntryMemberTx(s.tx, &member)
	}
	return entry
}

func (s *seeder) appearance(event Event, entry Entry, at time.Time, notes string) Appearance {
	appearance := Appearance{
		Id:         vbolt.NextIntId(s.tx, AppearanceBkt),
		EventId:    event.Id,
		EntryId:    entry.Id,
		FamilyId:   event.FamilyId,
		OccurredAt: at,
		Notes:      notes,
		CreatedAt:  s.now,
	}
	writeAppearanceTx(s.tx, &appearance)
	return appearance
}

// result fills in the identity and ownership fields so callers only state what
// actually varies between one result and the next.
func (s *seeder) result(appearance Appearance, result Result) {
	result.Id = vbolt.NextIntId(s.tx, ResultBkt)
	result.AppearanceId = appearance.Id
	result.FamilyId = appearance.FamilyId
	result.CreatedAt = s.now
	writeResultTx(s.tx, &result)
	s.sum.Results++
}

func seedInt(n int) *int { return &n }

func seedFloat(f float64) *float64 { return &f }

func (s *seeder) danceSeason(familyId int, dancer Person) {
	ballet := s.activity(familyId, "Ballet — Meridian Dance Academy", ActivityKindDance)
	season := s.season(ballet, "Competition Season",
		s.now.AddDate(0, -11, 0), s.now.AddDate(0, 1, 0),
		"Two solos and one group piece. Nationals is the last weekend.")

	solo := s.entry(season, "Solo — Variation from Paquita", "solo", "Ballet", "Junior", "Intermediate", dancer)
	group := s.entry(season, "Group — Waltz of the Flowers", "group", "Ballet", "Junior", "Intermediate", dancer)

	autumn := s.event(season, "Autumn Classic", "Regional Dance Alliance",
		"Grand Theater, Springfield", s.now.AddDate(0, -9, 0), s.now.AddDate(0, -9, 1))
	winter := s.event(season, "Winter Showcase", "Meridian Dance Academy",
		"Meridian Studio B", s.now.AddDate(0, -6, 0), s.now.AddDate(0, -6, 0))
	spring := s.event(season, "Spring Regionals", "Regional Dance Alliance",
		"Civic Auditorium, Fairview", s.now.AddDate(0, -2, 0), s.now.AddDate(0, -2, 2))

	first := s.appearance(autumn, solo, autumn.StartDate, "Nerves in the first sixteen counts, clean after that.")
	s.result(first, Result{Kind: ResultKindPlacement, Label: "3rd", Rank: seedInt(3), OutOf: seedInt(18), Category: "Junior Solo Ballet", SortOrder: 0})
	s.result(first, Result{Kind: ResultKindAdjudication, Label: "High Gold", Category: "Junior Solo Ballet", SortOrder: 1})

	second := s.appearance(winter, group, winter.StartDate, "No adjudication at the studio showcase.")
	s.result(second, Result{Kind: ResultKindAward, Label: "Studio Choice", Category: "Group", SortOrder: 0})

	third := s.appearance(spring, solo, spring.StartDate, "Best run of the season.")
	s.result(third, Result{Kind: ResultKindPlacement, Label: "1st", Rank: seedInt(1), OutOf: seedInt(24), Category: "Junior Solo Ballet", SortOrder: 0})
	s.result(third, Result{Kind: ResultKindAdjudication, Label: "Platinum", Category: "Junior Solo Ballet", SortOrder: 1})
	s.result(third, Result{Kind: ResultKindScore, Label: "285.4", Score: seedFloat(285.4), Category: "Technique", SortOrder: 2})

	fourth := s.appearance(spring, group, spring.StartDate.AddDate(0, 0, 1), "")
	s.result(fourth, Result{Kind: ResultKindPlacement, Label: "2nd", Rank: seedInt(2), OutOf: seedInt(11), Category: "Junior Group Ballet", SortOrder: 0})
}

func (s *seeder) soccerSeason(familyId int, player Person) {
	soccer := s.activity(familyId, "Soccer — Riverside United", ActivityKindSport)
	season := s.season(soccer, "U15 Select", s.now.AddDate(0, -5, 0), s.now.AddDate(0, 1, 0),
		"Left back, moved to centre mid in April.")
	team := s.entry(season, "Riverside United U15", "team", "", "U15", "Select", player)

	matches := []struct {
		opponent string
		venue    string
		label    string
		goals    float64
	}{
		{"Northgate FC", "Riverside Complex, Field 3", "3–1 W", 0},
		{"Fairview Athletic", "Fairview High", "0–2 L", 0},
		{"Lakeshore Rovers", "Riverside Complex, Field 1", "2–2 D", 1},
		{"Springfield City", "Springfield Sports Park", "4–0 W", 2},
		{"Northgate FC", "Northgate Turf", "1–0 W", 0},
		{"Lakeshore Rovers", "Lakeshore Park", "2–3 L", 1},
	}

	for i, match := range matches {
		date := s.now.AddDate(0, -5, i*21)
		event := s.event(season, "vs "+match.opponent, "Metro Youth League", match.venue, date, date)
		appearance := s.appearance(event, team, date, "")
		s.result(appearance, Result{Kind: ResultKindScore, Label: match.label, Category: "Final", SortOrder: 0})
		if match.goals > 0 {
			s.result(appearance, Result{
				Kind:      ResultKindScore,
				Label:     fmt.Sprintf("%.0f goal(s)", match.goals),
				Score:     seedFloat(match.goals),
				Category:  "Goals",
				PersonId:  seedInt(player.Id),
				SortOrder: 1,
			})
		}
	}
}

func (s *seeder) crossCountrySeason(familyId int, runner Person) {
	xc := s.activity(familyId, "Cross Country — Springfield High", ActivityKindSport)
	season := s.season(xc, "Varsity Season", s.now.AddDate(0, -12, 0), s.now.AddDate(0, -8, 0),
		"Varsity all four meets. PR of 21:34 at the invitational.")
	entry := s.entry(season, "Varsity Girls 5K", "individual", "", "Varsity", "5K", runner)

	meets := []struct {
		name     string
		venue    string
		place    int
		field    int
		minutes  float64
		labelStr string
	}{
		{"Season Opener", "Riverbend Park", 14, 96, 23.18, "23:11"},
		{"Conference Invitational", "Hollow Creek", 6, 141, 21.57, "21:34"},
		{"District Championship", "Fairview Golf Course", 9, 118, 22.10, "22:06"},
		{"State Qualifier", "Capitol Cross Course", 21, 173, 22.75, "22:45"},
	}

	for i, meet := range meets {
		date := s.now.AddDate(0, -12, i*24)
		event := s.event(season, meet.name, "State High School Athletic Association", meet.venue, date, date)
		appearance := s.appearance(event, entry, date, "")
		s.result(appearance, Result{
			Kind:      ResultKindPlacement,
			Label:     fmt.Sprintf("%d of %d", meet.place, meet.field),
			Rank:      seedInt(meet.place),
			OutOf:     seedInt(meet.field),
			Category:  "Varsity Girls 5K",
			PersonId:  seedInt(runner.Id),
			SortOrder: 0,
		})
		s.result(appearance, Result{
			Kind:      ResultKindScore,
			Label:     meet.labelStr,
			Score:     seedFloat(meet.minutes),
			Category:  "Finish time",
			PersonId:  seedInt(runner.Id),
			SortOrder: 1,
		})
	}
}

// ── the dataset ───────────────────────────────────────────────────────────────

func (s *seeder) build(scale int) {
	// The first account created is user 1, which backend/admin.go treats as the
	// site administrator. Marcus has to come first for the admin pages to be
	// reachable at all.
	dad, riveras := s.owner("Marcus Rivera", "dad@example.test", "Rivera Family", "admin (site admin, user 1)")
	mom := s.member("Priya Rivera", "mom@example.test", riveras, "admin")
	teen := s.member("Sofia Rivera", "teen@example.test", riveras, "admin (the eldest child's own login)")
	nanny := s.guest("Dana Brooks", "nanny@example.test", "Brooks Household", riveras, AccessContribute,
		"contribute in the Riveras — adds records, cannot manage the family")
	s.guest("Theo Nakamura", "sitter@example.test", "Nakamura Household", riveras, AccessView,
		"view in the Riveras — read-only")

	grandpa, elders := s.owner("Robert Rivera", "grandpa@example.test", "Rivera Grandparents", "admin")
	grandma := s.member("Eleanor Rivera", "grandma@example.test", elders, "admin")

	nana, chandras := s.owner("Asha Chandra", "nana@example.test", "Chandra Grandparents", "admin")
	aunt, fords := s.owner("Camila Rivera-Ford", "aunt@example.test", "Ford Family", "admin (link to the Riveras is still pending)")
	s.owner("Jordan Vale", "outsider@example.test", "Vale Family", "admin (no links at all — the isolation case)")

	// People ------------------------------------------------------------------
	marcus := s.person(riveras.Id, "Marcus Rivera", Male, "1985-03-14")
	priya := s.person(riveras.Id, "Priya Rivera", Female, "1987-07-02")
	sofia := s.person(riveras.Id, "Sofia Rivera", Female, "2009-05-21")
	mateo := s.person(riveras.Id, "Mateo Rivera", Male, "2012-01-09")
	ines := s.person(riveras.Id, "Ines Rivera", Female, "2015-08-30")
	luca := s.person(riveras.Id, "Luca Rivera", Male, "2019-11-12")
	nora := s.person(riveras.Id, "Nora Rivera", Female, "2023-04-05")
	baby := s.pregnancy(riveras.Id, "Baby Rivera", s.now.AddDate(0, 4, 0).Format("2006-01-02"))
	kids := []Person{sofia, mateo, ines, luca, nora}

	robert := s.person(elders.Id, "Robert Rivera", Male, "1957-02-11")
	eleanor := s.person(elders.Id, "Eleanor Rivera", Female, "1959-09-27")

	asha := s.person(chandras.Id, "Asha Chandra", Female, "1961-06-18")
	vikram := s.person(chandras.Id, "Vikram Chandra", Male, "1958-12-03")

	camila := s.person(fords.Id, "Camila Rivera-Ford", Female, "1990-10-08")
	jesse := s.person(fords.Id, "Jesse Ford", Male, "1989-04-22")
	theoFord := s.person(fords.Id, "Theo Ford", Male, "2018-06-14")

	s.represent(dad, marcus)
	s.represent(mom, priya)
	s.represent(teen, sofia)
	s.represent(grandpa, robert)
	s.represent(grandma, eleanor)
	s.represent(nana, asha)
	s.represent(aunt, camila)

	// Relations. The grandparent edges cross family boundaries, which is what
	// makes RelationLabel produce "grandmother" rather than nothing.
	s.relate(marcus, priya, RelationPartner)
	s.parents(marcus, priya, sofia, mateo, ines, luca, nora, baby)

	s.relate(robert, eleanor, RelationPartner)
	s.parents(robert, eleanor, marcus, camila)

	s.relate(asha, vikram, RelationPartner)
	s.parents(vikram, asha, priya)

	s.relate(camila, jesse, RelationPartner)
	s.parents(jesse, camila, theoFord)

	// Links and sharing -------------------------------------------------------
	// The paternal grandparents see everything, in both directions.
	s.link(riveras, elders, "grandparents", LinkScopes{
		People: true, Milestones: true, Photos: true, Growth: true, Activities: true,
	}, LinkAccepted)
	s.share(marcus, elders, "Son")
	s.share(priya, elders, "Daughter-in-law")
	s.share(sofia, elders, "Granddaughter")
	s.share(mateo, elders, "Grandson")
	s.share(ines, elders, "Granddaughter")
	s.share(luca, elders, "Grandson")
	s.share(nora, elders, "Granddaughter")
	s.share(baby, elders, "Grandchild on the way")

	s.link(elders, riveras, "parents", LinkScopes{People: true, Photos: true}, LinkAccepted)
	s.share(robert, riveras, "Grandpa Rivera")
	s.share(eleanor, riveras, "Grandma Rivera")

	// The maternal grandparents get a narrower link: no growth, no activities,
	// and only three of the five children on the roster.
	s.link(riveras, chandras, "grandparents", LinkScopes{
		People: true, Milestones: true, Photos: true,
	}, LinkAccepted)
	s.share(priya, chandras, "Daughter")
	s.share(sofia, chandras, "Granddaughter")
	s.share(mateo, chandras, "Grandson")
	s.share(ines, chandras, "Granddaughter")

	s.link(chandras, riveras, "parents", LinkScopes{People: true}, LinkAccepted)
	s.share(asha, riveras, "Nana")
	s.share(vikram, riveras, "Grandpa Chandra")

	// Camila's link was offered but never accepted, so it grants nothing.
	s.link(riveras, fords, "aunt and uncle", LinkScopes{
		People: true, Milestones: true, Photos: true,
	}, LinkPending)

	// Tags, milestones, measurements -----------------------------------------
	tags := map[string]Tag{}
	for _, spec := range []struct{ name, color string }{
		{"School", "#3b82f6"},
		{"Sports", "#22c55e"},
		{"Travel", "#f59e0b"},
		{"Health", "#ef4444"},
		{"Funny", "#a855f7"},
		{"Firsts", "#ec4899"},
	} {
		tags[spec.name] = s.tag(riveras.Id, spec.name, spec.color)
	}
	elderTags := map[string]Tag{
		"Visits":    s.tag(elders.Id, "Visits", "#0ea5e9"),
		"Keepsakes": s.tag(elders.Id, "Keepsakes", "#8b5cf6"),
	}

	for _, kid := range kids {
		s.childMilestones(kid, tags)
		s.growthSeries(kid, scale)
	}
	s.growthSeries(marcus, scale)
	s.growthSeries(priya, scale)
	s.growthSeries(robert, scale)
	s.growthSeries(eleanor, scale)
	s.growthSeries(theoFord, scale)
	s.childMilestones(theoFord, nil)

	s.milestone(marcus, s.now.AddDate(-2, -3, 0), "Ran the Springfield half marathon", "achievement", tags["Sports"])
	s.milestone(marcus, s.now.AddDate(-1, -1, 0), "Started the new job downtown", "achievement")
	s.milestone(priya, s.now.AddDate(-3, 0, 0), "Finished the master's degree, finally", "achievement", tags["School"])
	s.milestone(priya, s.now.AddDate(0, -5, 0), "Announced the pregnancy at Sunday dinner", "first", tags["Firsts"])

	s.milestone(robert, s.now.AddDate(-1, -6, 0), "Retired after 38 years", "achievement", elderTags["Keepsakes"])
	s.milestone(eleanor, s.now.AddDate(0, -4, 0), "Drove out to see all five grandchildren in one weekend", "first", elderTags["Visits"])

	// Activities --------------------------------------------------------------
	s.danceSeason(riveras.Id, ines)
	s.soccerSeason(riveras.Id, mateo)
	s.crossCountrySeason(riveras.Id, sofia)

	// Chat --------------------------------------------------------------------
	transcript := []struct {
		user     User
		hoursAgo int
		content  string
	}{
		{mom, 96, "Reminder that Ines has dress rehearsal Thursday at 5, not 6."},
		{dad, 95, "Noted. I can take her if you get Mateo to practice."},
		{mom, 94, "Deal."},
		{teen, 80, "can someone sign my permission slip before friday"},
		{dad, 79, "Leave it on the counter."},
		{nanny, 52, "Nora skipped her nap but was cheerful about it. Snack at 3, no dinner yet."},
		{mom, 51, "Thank you! We'll be back by 6."},
		{teen, 40, "21:34 at the invitational 🎉"},
		{dad, 39, "That's a PR by 40 seconds. Very proud of you."},
		{mom, 39, "!!!!!"},
		{dad, 20, "Grandma and Grandpa are coming the weekend after next. Two nights."},
		{teen, 19, "am i giving up my room again"},
		{dad, 19, "You are."},
		{mom, 6, "Luca lost a tooth at breakfast and has told four separate people about it."},
	}
	for _, line := range transcript {
		s.chat(riveras.Id, line.user, s.now.Add(-time.Duration(line.hoursAgo)*time.Hour), line.content)
	}
}
