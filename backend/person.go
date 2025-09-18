package backend

import (
	"errors"
	"family/cfg"
	"time"

	"go.hasen.dev/vbeam"
	"go.hasen.dev/vbolt"
	"go.hasen.dev/vpack"
)

func RegisterPersonMethods(app *vbeam.Application) {
	vbeam.RegisterProc(app, AddPerson)
	vbeam.RegisterProc(app, ListPeople)
	vbeam.RegisterProc(app, GetPerson)
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
	Name       string `json:"name"`
	PersonType int    `json:"personType"` // 0 = Parent, 1 = Child
	Gender     int    `json:"gender"`     // 0 = Male, 1 = Female, 2 = Unknown
	Birthdate  string `json:"birthdate"`  // YYYY-MM-DD format
}

type GetPersonRequest struct {
	Id     int    `json:"id"`     // 0 = Male, 1 = Female, 2 = Unknown
}

type ListPeopleResponse struct {
	People  []Person `json:"people"`
}

type GetPersonResponse struct {
	Person     Person      `json:"person,omitempty"`
	GrowthData []GrowthData `json:"growthData"`
}

// Database types
type Person struct {
	Id       int        `json:"id"`
	FamilyId int        `json:"familyId"`
	Name     string     `json:"name"`
	Type     PersonType `json:"type"`
	Gender   GenderType `json:"gender"`
	Birthday time.Time  `json:"birthday"`
	Age      int        `json:"age"`
}

// Packing function for vbolt serialization
func PackPerson(self *Person, buf *vpack.Buffer) {
	vpack.Version(1, buf)
	vpack.Int(&self.Id, buf)
	vpack.Int(&self.FamilyId, buf)
	vpack.String(&self.Name, buf)
	vpack.IntEnum(&self.Type, buf)
	vpack.IntEnum(&self.Gender, buf)
	vpack.Time(&self.Birthday, buf)
}

// Buckets for vbolt database storage
var PeopleBkt = vbolt.Bucket(&cfg.Info, "people", vpack.FInt, PackPerson)

// PersonIndex: term = person_id, target = family_id
// This allows efficient lookup of people by family
var PersonIndex = vbolt.Index(&cfg.Info, "person_by_family", vpack.FInt, vpack.FInt)

// Database helper functions
func GetPersonById(tx *vbolt.Tx, personId int) (person Person) {
	vbolt.Read(tx, PeopleBkt, personId, &person)
	return
}

func GetFamilyPeople(tx *vbolt.Tx, familyId int) (people []Person) {
	var personIds []int
	vbolt.ReadTermTargets(tx, PersonIndex, familyId, &personIds, vbolt.Window{})
	vbolt.ReadSlice(tx, PeopleBkt, personIds, &people)

	// Calculate age for each person
	for i := range people {
		people[i].Age = calculateAge(people[i].Birthday)
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
	person.Age = calculateAge(parsedTime)

	vbolt.Write(tx, PeopleBkt, person.Id, &person)

	updatePersonIndex(tx, person)

	return person, nil
}

func updatePersonIndex(tx *vbolt.Tx, person Person) {
	vbolt.SetTargetSingleTerm(tx, PersonIndex, person.Id, person.FamilyId)
}

func calculateAge(birthdate time.Time) int {
	now := time.Now()
	age := now.Year() - birthdate.Year()

	// Adjust if birthday hasn't occurred this year
	if now.Month() < birthdate.Month() ||
		(now.Month() == birthdate.Month() && now.Day() < birthdate.Day()) {
		age--
	}

	return age
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

	// Add person to user's family
	vbeam.UseWriteTx(ctx)
	person, err := AddPersonTx(ctx.Tx, req, user.FamilyId)
	if err != nil {
		return
	}

	vbolt.TxCommit(ctx.Tx)

	resp.Person = person
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
	resp.People = GetFamilyPeople(ctx.Tx, user.FamilyId)
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

	// Validate person belongs to user's family
	if resp.Person.Id == 0 || resp.Person.FamilyId != user.FamilyId {
		err = errors.New("Person not found or not in your family")
		return
	}

	// Calculate age
	resp.Person.Age = calculateAge(resp.Person.Birthday)

	// Get growth data for person
	resp.GrowthData = GetPersonGrowthDataTx(ctx.Tx, req.Id)

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
