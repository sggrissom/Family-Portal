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

type AddPersonRequest struct {
	Name        string `json:"name"`
	PersonType  int    `json:"personType"`
	Gender      int    `json:"gender"`
	Birthdate   string `json:"birthdate"`
	IsPregnancy bool   `json:"isPregnancy"`
	FamilyId    int    `json:"familyId,omitempty"`
}

type UpdatePersonRequest struct {
	Id          int    `json:"id"`
	Name        string `json:"name"`
	PersonType  int    `json:"personType"`
	Gender      int    `json:"gender"`
	Birthdate   string `json:"birthdate"`
	IsPregnancy bool   `json:"isPregnancy"`
}

type GetPersonRequest struct {
	Id int `json:"id"`
}

type SetProfilePhotoRequest struct {
	PersonId  int     `json:"personId"`
	PhotoId   int     `json:"photoId"`
	CropX     float64 `json:"cropX"`
	CropY     float64 `json:"cropY"`
	CropScale float64 `json:"cropScale"`
}

type SetProfilePhotoResponse struct {
	Person Person `json:"person"`
}

type MergePeopleRequest struct {
	SourcePersonId int `json:"sourcePersonId"`
	TargetPersonId int `json:"targetPersonId"`
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

type Person struct {
	Id               int        `json:"id"`
	FamilyId         int        `json:"familyId"`
	Name             string     `json:"name"`
	Type             PersonType `json:"type"`
	Gender           GenderType `json:"gender"`
	Birthday         time.Time  `json:"birthday"`
	Age              string     `json:"age"`
	ProfilePhotoId   int        `json:"profilePhotoId"`
	ProfileCropX     float64    `json:"profileCropX"`
	ProfileCropY     float64    `json:"profileCropY"`
	ProfileCropScale float64    `json:"profileCropScale"`
	FaceDescriptor   []float32  `json:"-"`
	IsPregnancy      bool       `json:"isPregnancy"`
}

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

var PeopleBkt = vbolt.Bucket(&cfg.Info, "people", vpack.FInt, PackPerson)

var PersonIndex = vbolt.Index(&cfg.Info, "person_by_family", vpack.FInt, vpack.FInt)

func GetPersonById(tx *vbolt.Tx, personId int) (person Person) {
	vbolt.Read(tx, PeopleBkt, personId, &person)
	return
}

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

func GetFamilyOwnPeople(tx *vbolt.Tx, familyId int) (people []Person) {
	var personIds []int
	vbolt.ReadTermTargets(tx, PersonIndex, familyId, &personIds, vbolt.Window{})
	vbolt.ReadSlice(tx, PeopleBkt, personIds, &people)
	for i := range people {
		people[i].Age = calculatePersonAge(people[i].Birthday, people[i].IsPregnancy)
	}
	return
}

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
	parsedTime, err := time.Parse("2006-01-02", req.Birthdate)
	if err != nil {
		return Person{}, errors.New("Invalid birthdate format. Use YYYY-MM-DD")
	}

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

func updatePersonIndex(tx *vbolt.Tx, person Person) {
	vbolt.SetTargetSingleTerm(tx, PersonIndex, person.Id, person.FamilyId)
	EnsurePersonFamilyTx(tx, person.Id, person.FamilyId, person.Type)
}

func calculateAgeAt(birthdate, referenceDate time.Time) string {
	if referenceDate.Before(birthdate) {
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

	if months < 0 || (months == 0 && days < 0) {
		years--
		months += 12
	}

	if days < 0 {
		months--
		if months < 0 {
			years--
			months += 12
		}
	}

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

func AddPerson(ctx *vbeam.Context, req AddPersonRequest) (resp GetPersonResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}

	if err = validateAddPersonRequest(req); err != nil {
		return
	}

	familyId, err := ResolveActingFamily(ctx.Tx, user, req.FamilyId, AccessContribute)
	if err != nil {
		return
	}

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
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}

	resp.People = GetVisiblePeople(ctx.Tx, user)
	return
}

func GetPerson(ctx *vbeam.Context, req GetPersonRequest) (resp GetPersonResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}

	resp.Person = GetPersonById(ctx.Tx, req.Id)

	if !CanAccessPerson(ctx.Tx, user, resp.Person, ScopePeople, AccessView) {
		resp.Person = Person{}
		err = errors.New("Person not found or not in your family")
		return
	}

	resp.Person.Age = calculatePersonAge(resp.Person.Birthday, resp.Person.IsPregnancy)

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

	if len(req.PersonIds) == 0 {
		err = errors.New("At least one person ID is required")
		return
	}

	if len(req.PersonIds) > 5 {
		err = errors.New("Cannot compare more than 5 people at once")
		return
	}

	resp.People = make([]PersonComparisonData, 0, len(req.PersonIds))

	for _, personId := range req.PersonIds {
		person := GetPersonById(ctx.Tx, personId)

		if !CanAccessPerson(ctx.Tx, user, person, ScopePeople, AccessView) {
			err = fmt.Errorf("Person ID %d not found or not in your family", personId)
			return
		}

		person.Age = calculateAge(person.Birthday)

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
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}

	if req.PersonId <= 0 {
		err = errors.New("Invalid person ID")
		return
	}
	if req.PhotoId <= 0 {
		err = errors.New("Invalid photo ID")
		return
	}

	vbeam.UseWriteTx(ctx)

	person := GetPersonById(ctx.Tx, req.PersonId)
	if person.Id == 0 || !CanAccessFamily(ctx.Tx, user, person.FamilyId, AccessContribute) {
		err = errors.New("Person not found or access denied")
		return
	}

	photo := GetImageById(ctx.Tx, req.PhotoId)
	if photo.Id == 0 || !CanAccessFamily(ctx.Tx, user, photo.FamilyId, AccessView) {
		err = errors.New("Photo not found or access denied")
		return
	}

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

	person.ProfilePhotoId = req.PhotoId
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

	go TriggerPersonFaceUpdate(req.PersonId)

	person.Age = calculateAge(person.Birthday)
	resp.Person = person
	return
}

func MergePeople(ctx *vbeam.Context, req MergePeopleRequest) (resp MergePeopleResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}

	if req.SourcePersonId <= 0 || req.TargetPersonId <= 0 {
		err = errors.New("Both source and target person IDs are required")
		return
	}

	if req.SourcePersonId == req.TargetPersonId {
		err = errors.New("Cannot merge a person with themselves")
		return
	}

	vbeam.UseWriteTx(ctx)

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

	if sourcePerson.FamilyId != targetPerson.FamilyId {
		err = errors.New("Cannot merge people from different families")
		return
	}

	growthData := GetPersonGrowthDataTx(ctx.Tx, req.SourcePersonId)
	for _, gd := range growthData {
		gd.PersonId = req.TargetPersonId
		vbolt.Write(ctx.Tx, GrowthDataBkt, gd.Id, &gd)
		vbolt.SetTargetSingleTerm(ctx.Tx, GrowthDataByPersonIndex, gd.Id, req.TargetPersonId)
	}
	resp.MergedGrowthCount = len(growthData)

	milestones := GetPersonMilestonesTx(ctx.Tx, req.SourcePersonId)
	for _, milestone := range milestones {
		milestone.PersonId = req.TargetPersonId
		vbolt.Write(ctx.Tx, MilestoneBkt, milestone.Id, &milestone)
		vbolt.SetTargetSingleTerm(ctx.Tx, MilestoneByPersonIndex, milestone.Id, req.TargetPersonId)
	}
	resp.MergedMilestones = len(milestones)

	photoPersons := GetPhotoPersonsByPerson(ctx.Tx, req.SourcePersonId)
	mergedPhotoCount := 0
	for _, photoPerson := range photoPersons {
		existingPhotoPersons := GetPhotoPersonsByPhoto(ctx.Tx, photoPerson.PhotoId)
		alreadyAssociated := false
		for _, existing := range existingPhotoPersons {
			if existing.PersonId == req.TargetPersonId {
				alreadyAssociated = true
				break
			}
		}

		if alreadyAssociated {
			vbolt.Delete(ctx.Tx, PhotoPersonBkt, photoPerson.Id)
			vbolt.SetTargetSingleTerm(ctx.Tx, PhotoPersonByPersonIndex, photoPerson.Id, -1)
			vbolt.SetTargetSingleTerm(ctx.Tx, PhotoPersonByPhotoIndex, photoPerson.Id, -1)
			vbolt.SetTargetSingleTerm(ctx.Tx, PhotoPersonByFamilyIndex, photoPerson.Id, -1)
		} else {
			photoPerson.PersonId = req.TargetPersonId
			vbolt.Write(ctx.Tx, PhotoPersonBkt, photoPerson.Id, &photoPerson)
			vbolt.SetTargetSingleTerm(ctx.Tx, PhotoPersonByPersonIndex, photoPerson.Id, req.TargetPersonId)
			mergedPhotoCount++
		}
	}
	resp.MergedPhotos = mergedPhotoCount

	for _, row := range GetPersonFamilies(ctx.Tx, req.SourcePersonId) {
		EnsurePersonFamilyTx(ctx.Tx, req.TargetPersonId, row.FamilyId, row.Role)
	}

	deletePersonRostersTx(ctx.Tx, req.SourcePersonId)
	vbolt.Delete(ctx.Tx, PeopleBkt, req.SourcePersonId)
	vbolt.SetTargetSingleTerm(ctx.Tx, PersonIndex, req.SourcePersonId, -1)

	vbolt.TxCommit(ctx.Tx)

	resp.Success = true
	targetPerson.Age = calculateAge(targetPerson.Birthday)
	resp.TargetPerson = targetPerson

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

	people := GetVisiblePeople(ctx.Tx, user)

	resp.People = make([]FamilyTimelineItem, 0, len(people))

	for _, person := range people {
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
