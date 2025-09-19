package backend

import (
	"errors"
	"family/cfg"
	"strings"
	"time"

	"go.hasen.dev/vbeam"
	"go.hasen.dev/vbolt"
	"go.hasen.dev/vpack"
)

func RegisterMilestoneMethods(app *vbeam.Application) {
	vbeam.RegisterProc(app, AddMilestone)
	vbeam.RegisterProc(app, GetPersonMilestones)
}

// Request/Response types
type AddMilestoneRequest struct {
	PersonId        int     `json:"personId"`
	Description     string  `json:"description"`
	Category        string  `json:"category"`        // "development", "behavior", "health", "achievement", "first", "other"
	InputType       string  `json:"inputType"`       // "today", "date" or "age"
	MilestoneDate   *string `json:"milestoneDate,omitempty"` // YYYY-MM-DD format (if inputType is "date")
	AgeYears        *int    `json:"ageYears,omitempty"`      // Age in years (if inputType is "age")
	AgeMonths       *int    `json:"ageMonths,omitempty"`     // Additional months (if inputType is "age")
}

type AddMilestoneResponse struct {
	Milestone Milestone `json:"milestone"`
}

type GetPersonMilestonesRequest struct {
	PersonId int `json:"personId"`
}

type GetPersonMilestonesResponse struct {
	Milestones []Milestone `json:"milestones"`
}

// Database types
type Milestone struct {
	Id            int       `json:"id"`
	PersonId      int       `json:"personId"`
	FamilyId      int       `json:"familyId"`
	Description   string    `json:"description"`
	Category      string    `json:"category"`
	MilestoneDate time.Time `json:"milestoneDate"`
	CreatedAt     time.Time `json:"createdAt"`
}

// Packing function for vbolt serialization
func PackMilestone(self *Milestone, buf *vpack.Buffer) {
	vpack.Version(1, buf)
	vpack.Int(&self.Id, buf)
	vpack.Int(&self.PersonId, buf)
	vpack.Int(&self.FamilyId, buf)
	vpack.String(&self.Description, buf)
	vpack.String(&self.Category, buf)
	vpack.Time(&self.MilestoneDate, buf)
	vpack.Time(&self.CreatedAt, buf)
}

// Buckets for vbolt database storage
var MilestoneBkt = vbolt.Bucket(&cfg.Info, "milestones", vpack.FInt, PackMilestone)

// MilestoneByPersonIndex: term = person_id, target = milestone_id
// This allows efficient lookup of milestones by person
var MilestoneByPersonIndex = vbolt.Index(&cfg.Info, "milestones_by_person", vpack.FInt, vpack.FInt)

// MilestoneByFamilyIndex: term = family_id, target = milestone_id
// This allows efficient lookup of milestones by family
var MilestoneByFamilyIndex = vbolt.Index(&cfg.Info, "milestones_by_family", vpack.FInt, vpack.FInt)

// Database helper functions
func GetMilestoneById(tx *vbolt.Tx, milestoneId int) (milestone Milestone) {
	vbolt.Read(tx, MilestoneBkt, milestoneId, &milestone)
	return
}

func GetPersonMilestonesTx(tx *vbolt.Tx, personId int) (milestones []Milestone) {
	var milestoneIds []int
	vbolt.ReadTermTargets(tx, MilestoneByPersonIndex, personId, &milestoneIds, vbolt.Window{})
	if len(milestoneIds) > 0 {
		vbolt.ReadSlice(tx, MilestoneBkt, milestoneIds, &milestones)
	}
	return
}

func AddMilestoneTx(tx *vbolt.Tx, req AddMilestoneRequest, familyId int) (Milestone, error) {
	var milestone Milestone
	var err error

	// Validate person belongs to family
	person := GetPersonById(tx, req.PersonId)
	if person.Id == 0 || person.FamilyId != familyId {
		return milestone, errors.New("Person not found or not in your family")
	}

	// Parse milestone date
	milestone.MilestoneDate, err = parseMilestoneDate(req, person.Birthday)
	if err != nil {
		return milestone, err
	}

	// Create milestone record
	milestone.Id = vbolt.NextIntId(tx, MilestoneBkt)
	milestone.PersonId = req.PersonId
	milestone.FamilyId = familyId
	milestone.Description = strings.TrimSpace(req.Description)
	milestone.Category = req.Category
	milestone.CreatedAt = time.Now()

	vbolt.Write(tx, MilestoneBkt, milestone.Id, &milestone)

	updateMilestoneIndices(tx, milestone)

	return milestone, nil
}

func updateMilestoneIndices(tx *vbolt.Tx, milestone Milestone) {
	vbolt.SetTargetSingleTerm(tx, MilestoneByPersonIndex, milestone.Id, milestone.PersonId)
	vbolt.SetTargetSingleTerm(tx, MilestoneByFamilyIndex, milestone.Id, milestone.FamilyId)
}

func parseMilestoneDate(req AddMilestoneRequest, personBirthday time.Time) (time.Time, error) {
	if req.InputType == "today" {
		// Use current date for "today" input type
		return time.Now(), nil
	} else if req.InputType == "date" {
		if req.MilestoneDate == nil || *req.MilestoneDate == "" {
			return time.Time{}, errors.New("Milestone date is required when input type is 'date'")
		}
		return time.Parse("2006-01-02", *req.MilestoneDate)
	} else if req.InputType == "age" {
		if req.AgeYears == nil || *req.AgeYears < 0 {
			return time.Time{}, errors.New("Age years must be non-negative")
		}
		ageMonths := 0
		if req.AgeMonths != nil {
			if *req.AgeMonths < 0 || *req.AgeMonths > 11 {
				return time.Time{}, errors.New("Age months must be between 0 and 11")
			}
			ageMonths = *req.AgeMonths
		}

		// Calculate date based on person's birthday + age
		targetDate := personBirthday.AddDate(*req.AgeYears, ageMonths, 0)
		return targetDate, nil
	} else {
		return time.Time{}, errors.New("Input type must be 'today', 'date' or 'age'")
	}
}

func validateAddMilestoneRequest(req AddMilestoneRequest) error {
	if req.PersonId <= 0 {
		return errors.New("Person ID is required")
	}
	if strings.TrimSpace(req.Description) == "" {
		return errors.New("Description is required")
	}
	validCategories := []string{"development", "behavior", "health", "achievement", "first", "other"}
	isValidCategory := false
	for _, category := range validCategories {
		if req.Category == category {
			isValidCategory = true
			break
		}
	}
	if !isValidCategory {
		return errors.New("Category must be one of: development, behavior, health, achievement, first, other")
	}
	if req.InputType != "today" && req.InputType != "date" && req.InputType != "age" {
		return errors.New("Input type must be 'today', 'date' or 'age'")
	}
	return nil
}

// vbeam procedures
func AddMilestone(ctx *vbeam.Context, req AddMilestoneRequest) (resp AddMilestoneResponse, err error) {
	// Get authenticated user
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}

	// Validate request
	if err = validateAddMilestoneRequest(req); err != nil {
		return
	}

	// Add milestone to database
	vbeam.UseWriteTx(ctx)
	milestone, err := AddMilestoneTx(ctx.Tx, req, user.FamilyId)
	if err != nil {
		return
	}

	vbolt.TxCommit(ctx.Tx)

	resp.Milestone = milestone
	return
}

func GetPersonMilestones(ctx *vbeam.Context, req GetPersonMilestonesRequest) (resp GetPersonMilestonesResponse, err error) {
	// Get authenticated user
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}

	// Validate that the person belongs to the user's family
	person := GetPersonById(ctx.Tx, req.PersonId)
	if person.Id == 0 || person.FamilyId != user.FamilyId {
		err = errors.New("Person not found or not in your family")
		return
	}

	// Get milestones for this person
	resp.Milestones = GetPersonMilestonesTx(ctx.Tx, req.PersonId)
	return
}