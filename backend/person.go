package backend

import (
	"encoding/binary"
	"errors"
	"family/cfg"
	"fmt"
	"math"
	"time"

	"go.hasen.dev/vbeam"
	"go.hasen.dev/vbolt"
	"go.hasen.dev/vpack"
)

func RegisterPersonMethods(app *vbeam.Application) {
	vbeam.RegisterProc(app, AddPerson)
	vbeam.RegisterProc(app, ListPeople)
	vbeam.RegisterProc(app, GetPerson)
	vbeam.RegisterProc(app, ComparePeople)
	vbeam.RegisterProc(app, UpdatePerson)
	vbeam.RegisterProc(app, SetProfilePhoto)
	vbeam.RegisterProc(app, MergePeople)
	vbeam.RegisterProc(app, GetFamilyTimeline)
}

type GenderType int

const (
	Male GenderType = iota
	Female
	Unknown
)

type PersonType int

const (
	Parent PersonType = iota
	Child
)

// Request/Response types
type AddPersonRequest struct {
	Name        string `json:"name"`
	PersonType  int    `json:"personType"`  // 0 = Parent, 1 = Child
	Gender      int    `json:"gender"`      // 0 = Male, 1 = Female, 2 = Unknown
	Birthdate   string `json:"birthdate"`   // YYYY-MM-DD format
	IsPregnancy bool   `json:"isPregnancy"` // true when birthdate is an expected due date
	FamilyId    int    `json:"familyId,omitempty"`
}

type UpdatePersonRequest struct {
	Id          int    `json:"id"`
	Name        string `json:"name"`
	PersonType  int    `json:"personType"`  // 0 = Parent, 1 = Child
	Gender      int    `json:"gender"`      // 0 = Male, 1 = Female, 2 = Unknown
	Birthdate   string `json:"birthdate"`   // YYYY-MM-DD format
	IsPregnancy bool   `json:"isPregnancy"` // true when birthdate is an expected due date
}

type GetPersonRequest struct {
	Id int `json:"id"` // 0 = Male, 1 = Female, 2 = Unknown
}

type SetProfilePhotoRequest struct {
	PersonId  int     `json:"personId"`
	PhotoId   int     `json:"photoId"`
	CropX     float64 `json:"cropX"`     // 0-100 (default 50 = center)
	CropY     float64 `json:"cropY"`     // 0-100 (default 50 = center)
	CropScale float64 `json:"cropScale"` // 1.0+ (default 1.0 = no zoom)
}

type SetProfilePhotoResponse struct {
	Person Person `json:"person"`
}

type MergePeopleRequest struct {
	SourcePersonId int `json:"sourcePersonId"` // Person to merge from (will be deleted)
	TargetPersonId int `json:"targetPersonId"` // Person to merge into (will keep)
}

type MergePeopleResponse struct {
	Success           bool   `json:"success"`
	TargetPerson      Person `json:"targetPerson"`
	MergedGrowthCount int    `json:"mergedGrowthCount"`
	MergedMilestones  int    `json:"mergedMilestones"`
	MergedPhotos      int    `json:"mergedPhotos"`
}

type ListPeopleResponse struct {
	People []Person `json:"people"`
}

type GetPersonResponse struct {
	Person     Person       `json:"person,omitempty"`
	GrowthData []GrowthData `json:"growthData"`
	Milestones []Milestone  `json:"milestones"`
	Photos     []Image      `json:"photos"`
}

type ComparePeopleRequest struct {
	PersonIds []int `json:"personIds"`
}

type PersonComparisonData struct {
	Person     Person       `json:"person"`
	GrowthData []GrowthData `json:"growthData"`
	Milestones []Milestone  `json:"milestones"`
	Photos     []Image      `json:"photos"`
}

type ComparePeopleResponse struct {
	People []PersonComparisonData `json:"people"`
}

type GetFamilyTimelineRequest struct {
	// No parameters needed - uses family from auth
}

type FamilyTimelineItem struct {
	Person     Person       `json:"person"`
	GrowthData []GrowthData `json:"growthData"`
	Milestones []Milestone  `json:"milestones"`
	Photos     []Image      `json:"photos"`
}

type GetFamilyTimelineResponse struct {
	People []FamilyTimelineItem `json:"people"`
}

// Database types
type Person struct {
	Id               int        `json:"id"`
	FamilyId         int        `json:"familyId"`
	Name             string     `json:"name"`
	Type             PersonType `json:"type"`
	Gender           GenderType `json:"gender"`
	Birthday         time.Time  `json:"birthday"`
	Age              string     `json:"age"`
	ProfilePhotoId   int        `json:"profilePhotoId"`
	ProfileCropX     float64    `json:"profileCropX"`     // X offset 0-100 (default 50 = center)
	ProfileCropY     float64    `json:"profileCropY"`     // Y offset 0-100 (default 50 = center)
	ProfileCropScale float64    `json:"profileCropScale"` // Zoom level 1.0+ (default 1.0 = no zoom)
	FaceDescriptor   []float32  `json:"-"`                // 128-dim face embedding, nil if not extracted
	IsPregnancy      bool       `json:"isPregnancy"`      // Birthday stores due date until the baby is born
}

// packFloat32Slice serializes a []float32 as a byte slice using little-endian IEEE 754
func packFloat32Slice(data *[]float32, buf *vpack.Buffer) {
	if buf.Writing {
		b := make([]byte, len(*data)*4)
		for i, f := range *data {
			binary.LittleEndian.PutUint32(b[i*4:], math.Float32bits(f))
		}
		vpack.ByteSlice(&b, buf)
	} else {
		var b []byte
		vpack.ByteSlice(&b, buf)
		if len(b) > 0 && len(b)%4 == 0 {
			*data = make([]float32, len(b)/4)
			for i := range *data {
				(*data)[i] = math.Float32frombits(binary.LittleEndian.Uint32(b[i*4:]))
			}
		}
	}
}

// Packing function for vbolt serialization
func PackPerson(self *Person, buf *vpack.Buffer) {
	version := vpack.Version(5, buf)
	vpack.Int(&self.Id, buf)
	vpack.Int(&self.FamilyId, buf)
	vpack.String(&self.Name, buf)
	vpack.IntEnum(&self.Type, buf)
	vpack.IntEnum(&self.Gender, buf)
	vpack.Time(&self.Birthday, buf)
	if version >= 2 {
		vpack.Int(&self.ProfilePhotoId, buf)
	}
	if version >= 3 {
		vpack.Float64(&self.ProfileCropX, buf)
		vpack.Float64(&self.ProfileCropY, buf)
		vpack.Float64(&self.ProfileCropScale, buf)
	}
	if version >= 4 {
		packFloat32Slice(&self.FaceDescriptor, buf)
	}
	if version >= 5 {
		vpack.Bool(&self.IsPregnancy, buf)
	}
}

// Buckets for vbolt database storage
var PeopleBkt = vbolt.Bucket(&cfg.Info, "people", vpack.FInt, PackPerson)

// PersonIndex: term = family_id, target = person_id
// This is the *home* family of each person, and remains the anchor for leaf
// record provenance. Rosters are read from PersonFamilyByFamilyIndex instead —
// a person can appear on families beyond the one that owns them.
var PersonIndex = vbolt.Index(&cfg.Info, "person_by_family", vpack.FInt, vpack.FInt)

// Database helper functions
func GetPersonById(tx *vbolt.Tx, personId int) (person Person) {
	vbolt.Read(tx, PeopleBkt, personId, &person)
	return
}

// GetFamilyPeople returns the family's roster. Each person carries the Type they
// hold *in this family*, taken from their PersonFamily row rather than from the
// person record, since the same person is a Parent at home and a Child on their
// parents' roster.
func GetFamilyPeople(tx *vbolt.Tx, familyId int) (people []Person) {
	for _, row := range GetFamilyRoster(tx, familyId) {
		var person Person
		if !vbolt.Read(tx, PeopleBkt, row.PersonId, &person) {
			continue
		}
		person.Type = row.Role
		person.Age = calculatePersonAge(person.Birthday, person.IsPregnancy)
		people = append(people, person)
	}
	return
}

// GetFamilyOwnPeople returns the people this family *owns* — the ones homed
// here — rather than the ones on its roster.
//
// The two differed only in theory until family links let a household host
// someone else's child. Anything that treats a person as this family's
// wants this list: export, which would otherwise put a shared person in a bundle
// that re-creates them as a separate record on the way back in; import matching,
// which would otherwise hang the importing family's records off a person another
// family owns; and face tagging, which would otherwise write a photo-person join
// across a family boundary that manual tagging refuses.
func GetFamilyOwnPeople(tx *vbolt.Tx, familyId int) (people []Person) {
	var personIds []int
	vbolt.ReadTermTargets(tx, PersonIndex, familyId, &personIds, vbolt.Window{})
	vbolt.ReadSlice(tx, PeopleBkt, personIds, &people)
	for i := range people {
		people[i].Age = calculatePersonAge(people[i].Birthday, people[i].IsPregnancy)
	}
	return
}

// GetVisiblePeople returns the roster of every family user can read, with each
// person appearing once. A shared person is on more than one of those rosters,
// so the first hit wins: familiesVisibleTo puts the primary family first, which
// makes the role in the user's own household the one they see.
func GetVisiblePeople(tx *vbolt.Tx, user User) (people []Person) {
	seen := make(map[int]bool)
	for _, familyId := range familiesVisibleTo(tx, user) {
		for _, person := range GetFamilyPeople(tx, familyId) {
			if seen[person.Id] {
				continue
			}
			seen[person.Id] = true
			people = append(people, person)
		}
	}
	return
}

func AddPersonTx(tx *vbolt.Tx, req AddPersonRequest, familyId int) (Person, error) {
	// Parse birthdate
	parsedTime, err := time.Parse("2006-01-02", req.Birthdate)
	if err != nil {
		return Person{}, errors.New("Invalid birthdate format. Use YYYY-MM-DD")
	}

	// Create person
	var person Person
	person.Id = vbolt.NextIntId(tx, PeopleBkt)
	person.FamilyId = familyId
	person.Name = req.Name
	person.Type = PersonType(req.PersonType)
	person.Gender = GenderType(req.Gender)
	person.Birthday = parsedTime
	person.IsPregnancy = req.IsPregnancy
	person.Age = calculatePersonAge(parsedTime, person.IsPregnancy)
	person.ProfilePhotoId = 0

	vbolt.Write(tx, PeopleBkt, person.Id, &person)

	updatePersonIndex(tx, person)

	return person, nil
}

// updatePersonIndex keeps both halves of a person's home family in sync: the
// home-family index, and the roster row that puts them on that family's roster.
func updatePersonIndex(tx *vbolt.Tx, person Person) {
	vbolt.SetTargetSingleTerm(tx, PersonIndex, person.Id, person.FamilyId)
	EnsurePersonFamilyTx(tx, person.Id, person.FamilyId, person.Type)
}

func calculateAgeAt(birthdate, referenceDate time.Time) string {
	if referenceDate.Before(birthdate) {
		// Treat future birthdates as due dates and show gestational age in weeks.
		birthdateUTC := time.Date(birthdate.Year(), birthdate.Month(), birthdate.Day(), 0, 0, 0, 0, time.UTC)
		referenceDateUTC := time.Date(referenceDate.Year(), referenceDate.Month(), referenceDate.Day(), 0, 0, 0, 0, time.UTC)
		daysUntilDue := int(birthdateUTC.Sub(referenceDateUTC).Hours() / 24)
		weeksPregnant := 40 - int(math.Ceil(float64(daysUntilDue)/7.0))
		if weeksPregnant < 0 {
			weeksPregnant = 0
		}
		if weeksPregnant == 1 {
			return "1 week"
		}
		return fmt.Sprintf("%d weeks", weeksPregnant)
	}

	years := referenceDate.Year() - birthdate.Year()
	months := int(referenceDate.Month()) - int(birthdate.Month())
	days := referenceDate.Day() - birthdate.Day()

	// Adjust for birthday not yet occurred this year
	if months < 0 || (months == 0 && days < 0) {
		years--
		months += 12
	}

	// Adjust months if day hasn't occurred this month
	if days < 0 {
		months--
		if months < 0 {
			years--
			months += 12
		}
	}

	// For under 1 year
	if years == 0 {
		totalMonths := months
		if totalMonths <= 0 {
			return "< 1 month"
		}
		if totalMonths == 1 {
			return "1 month"
		}
		return fmt.Sprintf("%d months", totalMonths)
	}

	if years == 1 {
		return "1 year"
	}
	return fmt.Sprintf("%d years", years)
}

// wrapper for current time
func calculateAge(birthdate time.Time) string {
	return calculateAgeAt(birthdate, time.Now())
}

func calculatePersonAgeAt(birthdate, referenceDate time.Time, isPregnancy bool) string {
	if isPregnancy {
		return calculateGestationalAgeAt(birthdate, referenceDate)
	}
	return calculateAgeAt(birthdate, referenceDate)
}

func calculatePersonAge(birthdate time.Time, isPregnancy bool) string {
	return calculatePersonAgeAt(birthdate, time.Now(), isPregnancy)
}

func calculateGestationalAgeAt(dueDate, referenceDate time.Time) string {
	dueDateUTC := time.Date(dueDate.Year(), dueDate.Month(), dueDate.Day(), 0, 0, 0, 0, time.UTC)
	referenceDateUTC := time.Date(referenceDate.Year(), referenceDate.Month(), referenceDate.Day(), 0, 0, 0, 0, time.UTC)
	daysUntilDue := int(dueDateUTC.Sub(referenceDateUTC).Hours() / 24)
	weeksPregnant := 40 - int(math.Ceil(float64(daysUntilDue)/7.0))
	if weeksPregnant < 0 {
		weeksPregnant = 0
	}
	if weeksPregnant == 1 {
		return "1 week"
	}
	return fmt.Sprintf("%d weeks", weeksPregnant)
}

// vbeam procedures
func AddPerson(ctx *vbeam.Context, req AddPersonRequest) (resp GetPersonResponse, err error) {
	// Get authenticated user
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}

	// Validate request
	if err = validateAddPersonRequest(req); err != nil {
		return
	}

	familyId, err := ResolveActingFamily(ctx.Tx, user, req.FamilyId, AccessContribute)
	if err != nil {
		return
	}

	// Add person to the named family
	vbeam.UseWriteTx(ctx)
	person, err := AddPersonTx(ctx.Tx, req, familyId)
	if err != nil {
		return
	}

	vbolt.TxCommit(ctx.Tx)

	resp.Person = person
	resp.GrowthData = []GrowthData{}
	resp.Milestones = []Milestone{}
	resp.Photos = []Image{}
	return
}

func UpdatePerson(ctx *vbeam.Context, req UpdatePersonRequest) (resp GetPersonResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}

	if err = validateAddPersonRequest(AddPersonRequest{
		Name:        req.Name,
		PersonType:  req.PersonType,
		Gender:      req.Gender,
		Birthdate:   req.Birthdate,
		IsPregnancy: req.IsPregnancy,
	}); err != nil {
		return
	}

	vbeam.UseWriteTx(ctx)

	person := GetPersonById(ctx.Tx, req.Id)
	if person.Id == 0 || !CanAccessFamily(ctx.Tx, user, person.FamilyId, AccessContribute) {
		err = errors.New("Person not found or not in your family")
		return
	}

	parsedTime, parseErr := time.Parse("2006-01-02", req.Birthdate)
	if parseErr != nil {
		err = errors.New("Invalid birthdate format. Use YYYY-MM-DD")
		return
	}

	person.Name = req.Name
	person.Type = PersonType(req.PersonType)
	person.Gender = GenderType(req.Gender)
	person.Birthday = parsedTime
	person.IsPregnancy = req.IsPregnancy
	person.Age = calculatePersonAge(parsedTime, person.IsPregnancy)

	vbolt.Write(ctx.Tx, PeopleBkt, person.Id, &person)
	// The edit form is scoped to the person's own household, so it sets the role
	// on the home roster only. Roles on extended rosters are untouched.
	SetPersonFamilyRoleTx(ctx.Tx, person.Id, person.FamilyId, person.Type)

	resp.Person = person
	resp.GrowthData = GetPersonGrowthDataTx(ctx.Tx, req.Id)
	resp.Milestones = GetPersonMilestonesTx(ctx.Tx, req.Id)
	for i := range resp.Milestones {
		resp.Milestones[i].PhotoIds = GetMilestonePhotoIds(ctx.Tx, resp.Milestones[i].Id)
	}
	resp.Photos = GetPersonImages(ctx.Tx, req.Id)

	vbolt.TxCommit(ctx.Tx)
	return
}

func ListPeople(ctx *vbeam.Context, req Empty) (resp ListPeopleResponse, err error) {
	// Get authenticated user
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}

	// Get family members
	resp.People = GetVisiblePeople(ctx.Tx, user)
	return
}

func GetPerson(ctx *vbeam.Context, req GetPersonRequest) (resp GetPersonResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}

	// Get person data
	resp.Person = GetPersonById(ctx.Tx, req.Id)

	// Validate person belongs to user's family, or has been shared into one of
	// their families by a link.
	if !CanAccessPerson(ctx.Tx, user, resp.Person, ScopePeople, AccessView) {
		resp.Person = Person{}
		err = errors.New("Person not found or not in your family")
		return
	}

	// Calculate age
	resp.Person.Age = calculatePersonAge(resp.Person.Birthday, resp.Person.IsPregnancy)

	// Each section is gated on its own scope. Someone in the person's family
	// clears all of them; someone seeing a shared person gets only the kinds of
	// record their link carries, which is what makes "milestones yes,
	// measurements no" expressible.
	if CanAccessPerson(ctx.Tx, user, resp.Person, ScopeGrowth, AccessView) {
		resp.GrowthData = GetPersonGrowthDataTx(ctx.Tx, req.Id)
	}

	if CanAccessPerson(ctx.Tx, user, resp.Person, ScopeMilestones, AccessView) {
		resp.Milestones = GetPersonMilestonesTx(ctx.Tx, req.Id)
		for i := range resp.Milestones {
			resp.Milestones[i].PhotoIds = GetMilestonePhotoIds(ctx.Tx, resp.Milestones[i].Id)
			resp.Milestones[i].TagIds = GetMilestoneTagIds(ctx.Tx, resp.Milestones[i].Id)
		}
	}

	if CanAccessPerson(ctx.Tx, user, resp.Person, ScopePhotos, AccessView) {
		resp.Photos = GetPersonImages(ctx.Tx, req.Id)
		for i := range resp.Photos {
			resp.Photos[i].TagIds = GetPhotoTagIds(ctx.Tx, resp.Photos[i].Id)
		}
	}

	return
}

func ComparePeople(ctx *vbeam.Context, req ComparePeopleRequest) (resp ComparePeopleResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}

	// Validate request
	if len(req.PersonIds) == 0 {
		err = errors.New("At least one person ID is required")
		return
	}

	if len(req.PersonIds) > 5 {
		err = errors.New("Cannot compare more than 5 people at once")
		return
	}

	// Fetch data for each person
	resp.People = make([]PersonComparisonData, 0, len(req.PersonIds))

	for _, personId := range req.PersonIds {
		// Get person data
		person := GetPersonById(ctx.Tx, personId)

		// Validate person exists and belongs to user's family, or was shared
		// into one of their families by a link.
		if !CanAccessPerson(ctx.Tx, user, person, ScopePeople, AccessView) {
			err = fmt.Errorf("Person ID %d not found or not in your family", personId)
			return
		}

		// Calculate age
		person.Age = calculateAge(person.Birthday)

		// Build comparison data, each section gated on its own scope so a
		// shared person contributes only what their link carries.
		comparisonData := PersonComparisonData{Person: person}
		if CanAccessPerson(ctx.Tx, user, person, ScopeGrowth, AccessView) {
			comparisonData.GrowthData = GetPersonGrowthDataTx(ctx.Tx, personId)
		}
		if CanAccessPerson(ctx.Tx, user, person, ScopeMilestones, AccessView) {
			milestones := GetPersonMilestonesTx(ctx.Tx, personId)
			for i := range milestones {
				milestones[i].PhotoIds = GetMilestonePhotoIds(ctx.Tx, milestones[i].Id)
			}
			comparisonData.Milestones = milestones
		}
		if CanAccessPerson(ctx.Tx, user, person, ScopePhotos, AccessView) {
			comparisonData.Photos = GetPersonImages(ctx.Tx, personId)
		}

		resp.People = append(resp.People, comparisonData)
	}

	return
}

func validateAddPersonRequest(req AddPersonRequest) error {
	if req.Name == "" {
		return errors.New("Name is required")
	}
	if req.PersonType < 0 || req.PersonType > 1 {
		return errors.New("Invalid person type")
	}
	if req.Gender < 0 || req.Gender > 2 {
		return errors.New("Invalid gender")
	}
	if req.Birthdate == "" {
		return errors.New("Birthdate is required")
	}
	return nil
}

func SetProfilePhoto(ctx *vbeam.Context, req SetProfilePhotoRequest) (resp SetProfilePhotoResponse, err error) {
	// Get authenticated user
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}

	// Validate request
	if req.PersonId <= 0 {
		err = errors.New("Invalid person ID")
		return
	}
	if req.PhotoId <= 0 {
		err = errors.New("Invalid photo ID")
		return
	}

	vbeam.UseWriteTx(ctx)

	// Get and validate person
	person := GetPersonById(ctx.Tx, req.PersonId)
	if person.Id == 0 || !CanAccessFamily(ctx.Tx, user, person.FamilyId, AccessContribute) {
		err = errors.New("Person not found or access denied")
		return
	}

	// Get and validate photo
	photo := GetImageById(ctx.Tx, req.PhotoId)
	if photo.Id == 0 || !CanAccessFamily(ctx.Tx, user, photo.FamilyId, AccessView) {
		err = errors.New("Photo not found or access denied")
		return
	}

	// Check if person is associated with this photo
	photoPeople := GetPhotoPeople(ctx.Tx, req.PhotoId)
	personInPhoto := false
	for _, photoPerson := range photoPeople {
		if photoPerson.Id == req.PersonId {
			personInPhoto = true
			break
		}
	}

	if !personInPhoto {
		err = errors.New("Person is not associated with this photo")
		return
	}

	// Update person's profile photo and crop settings
	person.ProfilePhotoId = req.PhotoId
	// Set default crop values if not provided (0 values mean center crop, no zoom)
	if req.CropX == 0 && req.CropY == 0 {
		person.ProfileCropX = 50
		person.ProfileCropY = 50
	} else {
		person.ProfileCropX = req.CropX
		person.ProfileCropY = req.CropY
	}
	if req.CropScale == 0 {
		person.ProfileCropScale = 1.0
	} else {
		person.ProfileCropScale = req.CropScale
	}
	vbolt.Write(ctx.Tx, PeopleBkt, person.Id, &person)

	vbolt.TxCommit(ctx.Tx)

	// Trigger background face embedding extraction for the new profile photo
	go TriggerPersonFaceUpdate(req.PersonId)

	// Calculate age for response
	person.Age = calculateAge(person.Birthday)
	resp.Person = person
	return
}

func MergePeople(ctx *vbeam.Context, req MergePeopleRequest) (resp MergePeopleResponse, err error) {
	// Get authenticated user
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}

	// Validate request
	if req.SourcePersonId <= 0 || req.TargetPersonId <= 0 {
		err = errors.New("Both source and target person IDs are required")
		return
	}

	if req.SourcePersonId == req.TargetPersonId {
		err = errors.New("Cannot merge a person with themselves")
		return
	}

	vbeam.UseWriteTx(ctx)

	// Get and validate both people
	sourcePerson := GetPersonById(ctx.Tx, req.SourcePersonId)
	if sourcePerson.Id == 0 || !CanAccessFamily(ctx.Tx, user, sourcePerson.FamilyId, AccessAdmin) {
		err = errors.New("Source person not found or not in your family")
		return
	}

	targetPerson := GetPersonById(ctx.Tx, req.TargetPersonId)
	if targetPerson.Id == 0 || !CanAccessFamily(ctx.Tx, user, targetPerson.FamilyId, AccessAdmin) {
		err = errors.New("Target person not found or not in your family")
		return
	}

	// A cross-home-family merge would have to decide what happens to the
	// FamilyId on every leaf record it rewrites, which is the provenance of a
	// family that is losing the person. Stage 5 revisited this and the answer
	// is still no, now for a concrete reason rather than a missing model: a
	// link tops out at AccessView, so it cannot confer the admin rights a merge
	// needs, and the duplicate a link actually produces — the same child on two
	// rosters — is one Person record already, which is exactly what a merge
	// would be for. There is nothing left to merge across a link.
	if sourcePerson.FamilyId != targetPerson.FamilyId {
		err = errors.New("Cannot merge people from different families")
		return
	}

	// Merge growth data
	growthData := GetPersonGrowthDataTx(ctx.Tx, req.SourcePersonId)
	for _, gd := range growthData {
		gd.PersonId = req.TargetPersonId
		vbolt.Write(ctx.Tx, GrowthDataBkt, gd.Id, &gd)
		vbolt.SetTargetSingleTerm(ctx.Tx, GrowthDataByPersonIndex, gd.Id, req.TargetPersonId)
	}
	resp.MergedGrowthCount = len(growthData)

	// Merge milestones
	milestones := GetPersonMilestonesTx(ctx.Tx, req.SourcePersonId)
	for _, milestone := range milestones {
		milestone.PersonId = req.TargetPersonId
		vbolt.Write(ctx.Tx, MilestoneBkt, milestone.Id, &milestone)
		vbolt.SetTargetSingleTerm(ctx.Tx, MilestoneByPersonIndex, milestone.Id, req.TargetPersonId)
	}
	resp.MergedMilestones = len(milestones)

	// Merge photo associations
	photoPersons := GetPhotoPersonsByPerson(ctx.Tx, req.SourcePersonId)
	mergedPhotoCount := 0
	for _, photoPerson := range photoPersons {
		// Check if target person is already associated with this photo
		existingPhotoPersons := GetPhotoPersonsByPhoto(ctx.Tx, photoPerson.PhotoId)
		alreadyAssociated := false
		for _, existing := range existingPhotoPersons {
			if existing.PersonId == req.TargetPersonId {
				alreadyAssociated = true
				break
			}
		}

		if alreadyAssociated {
			// Delete the duplicate association
			vbolt.Delete(ctx.Tx, PhotoPersonBkt, photoPerson.Id)
			vbolt.SetTargetSingleTerm(ctx.Tx, PhotoPersonByPersonIndex, photoPerson.Id, -1)
			vbolt.SetTargetSingleTerm(ctx.Tx, PhotoPersonByPhotoIndex, photoPerson.Id, -1)
			vbolt.SetTargetSingleTerm(ctx.Tx, PhotoPersonByFamilyIndex, photoPerson.Id, -1)
		} else {
			// Update to target person
			photoPerson.PersonId = req.TargetPersonId
			vbolt.Write(ctx.Tx, PhotoPersonBkt, photoPerson.Id, &photoPerson)
			vbolt.SetTargetSingleTerm(ctx.Tx, PhotoPersonByPersonIndex, photoPerson.Id, req.TargetPersonId)
			mergedPhotoCount++
		}
	}
	resp.MergedPhotos = mergedPhotoCount

	// Union the rosters before the source disappears: any extended family that
	// could see the source must still see the surviving record. The target's
	// existing role wins where both are on the same roster, since
	// EnsurePersonFamilyTx never overwrites one.
	for _, row := range GetPersonFamilies(ctx.Tx, req.SourcePersonId) {
		EnsurePersonFamilyTx(ctx.Tx, req.TargetPersonId, row.FamilyId, row.Role)
	}

	// Delete source person
	deletePersonRostersTx(ctx.Tx, req.SourcePersonId)
	vbolt.Delete(ctx.Tx, PeopleBkt, req.SourcePersonId)
	vbolt.SetTargetSingleTerm(ctx.Tx, PersonIndex, req.SourcePersonId, -1)

	vbolt.TxCommit(ctx.Tx)

	// Prepare response
	resp.Success = true
	targetPerson.Age = calculateAge(targetPerson.Birthday)
	resp.TargetPerson = targetPerson

	// Log the merge operation
	LogInfo("DATA", "People merged", map[string]any{
		"userId":           user.Id,
		"familyId":         user.FamilyId,
		"sourcePersonId":   req.SourcePersonId,
		"targetPersonId":   req.TargetPersonId,
		"mergedGrowth":     resp.MergedGrowthCount,
		"mergedMilestones": resp.MergedMilestones,
		"mergedPhotos":     resp.MergedPhotos,
	})

	return
}

func GetFamilyTimeline(ctx *vbeam.Context, req GetFamilyTimelineRequest) (resp GetFamilyTimelineResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}

	// Get all family members
	people := GetVisiblePeople(ctx.Tx, user)

	// Build timeline data for each person
	resp.People = make([]FamilyTimelineItem, 0, len(people))

	for _, person := range people {
		// A person on the roster is not automatically a person whose records
		// the viewer may read: someone shared in by a link contributes only the
		// kinds of record that link carries.
		timelineItem := FamilyTimelineItem{Person: person}

		if CanAccessPerson(ctx.Tx, user, person, ScopeMilestones, AccessView) {
			timelineMilestones := GetPersonMilestonesTx(ctx.Tx, person.Id)
			for i := range timelineMilestones {
				timelineMilestones[i].PhotoIds = GetMilestonePhotoIds(ctx.Tx, timelineMilestones[i].Id)
				timelineMilestones[i].TagIds = GetMilestoneTagIds(ctx.Tx, timelineMilestones[i].Id)
			}
			timelineItem.Milestones = timelineMilestones
		}

		if CanAccessPerson(ctx.Tx, user, person, ScopePhotos, AccessView) {
			timelinePhotos := GetPersonImages(ctx.Tx, person.Id)
			for i := range timelinePhotos {
				timelinePhotos[i].TagIds = GetPhotoTagIds(ctx.Tx, timelinePhotos[i].Id)
			}
			timelineItem.Photos = timelinePhotos
		}

		if CanAccessPerson(ctx.Tx, user, person, ScopeGrowth, AccessView) {
			timelineItem.GrowthData = GetPersonGrowthDataTx(ctx.Tx, person.Id)
		}

		resp.People = append(resp.People, timelineItem)
	}

	return
}
