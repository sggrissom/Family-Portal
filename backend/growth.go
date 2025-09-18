package backend

import (
	"errors"
	"family/cfg"
	"time"

	"go.hasen.dev/vbeam"
	"go.hasen.dev/vbolt"
	"go.hasen.dev/vpack"
)

func RegisterGrowthMethods(app *vbeam.Application) {
	vbeam.RegisterProc(app, AddGrowthData)
}

type MeasurementType int

const (
	Height MeasurementType = iota
	Weight
)

// Request/Response types
type AddGrowthDataRequest struct {
	PersonId        int     `json:"personId"`
	MeasurementType string  `json:"measurementType"` // "height" or "weight"
	Value           float64 `json:"value"`
	Unit            string  `json:"unit"` // cm, in, kg, lbs
	InputType       string  `json:"inputType"` // "date" or "age"
	MeasurementDate *string `json:"measurementDate,omitempty"` // YYYY-MM-DD format (if inputType is "date")
	AgeYears        *int    `json:"ageYears,omitempty"` // Age in years (if inputType is "age")
	AgeMonths       *int    `json:"ageMonths,omitempty"` // Additional months (if inputType is "age")
}

type AddGrowthDataResponse struct {
	GrowthData GrowthData `json:"growthData"`
}


// Database types
type GrowthData struct {
	Id              int               `json:"id"`
	PersonId        int               `json:"personId"`
	FamilyId        int               `json:"familyId"`
	MeasurementType MeasurementType   `json:"measurementType"`
	Value           float64           `json:"value"`
	Unit            string            `json:"unit"`
	MeasurementDate time.Time         `json:"measurementDate"`
	CreatedAt       time.Time         `json:"createdAt"`
}

// Packing function for vbolt serialization
func PackGrowthData(self *GrowthData, buf *vpack.Buffer) {
	vpack.Version(1, buf)
	vpack.Int(&self.Id, buf)
	vpack.Int(&self.PersonId, buf)
	vpack.Int(&self.FamilyId, buf)
	vpack.IntEnum(&self.MeasurementType, buf)
	vpack.Float64(&self.Value, buf)
	vpack.String(&self.Unit, buf)
	vpack.Time(&self.MeasurementDate, buf)
	vpack.Time(&self.CreatedAt, buf)
}

// Buckets for vbolt database storage
var GrowthDataBkt = vbolt.Bucket(&cfg.Info, "growth_data", vpack.FInt, PackGrowthData)

// GrowthDataByPersonIndex: term = person_id, target = growth_data_id
// This allows efficient lookup of growth data by person
var GrowthDataByPersonIndex = vbolt.Index(&cfg.Info, "growth_data_by_person", vpack.FInt, vpack.FInt)

// GrowthDataByFamilyIndex: term = family_id, target = growth_data_id
// This allows efficient lookup of growth data by family
var GrowthDataByFamilyIndex = vbolt.Index(&cfg.Info, "growth_data_by_family", vpack.FInt, vpack.FInt)

// Database helper functions
func GetGrowthDataById(tx *vbolt.Tx, growthDataId int) (growthData GrowthData) {
	vbolt.Read(tx, GrowthDataBkt, growthDataId, &growthData)
	return
}

func GetPersonGrowthDataTx(tx *vbolt.Tx, personId int) (growthData []GrowthData) {
	var growthDataIds []int
	vbolt.ReadTermTargets(tx, GrowthDataByPersonIndex, personId, &growthDataIds, vbolt.Window{})
	if len(growthDataIds) > 0 {
		vbolt.ReadSlice(tx, GrowthDataBkt, growthDataIds, &growthData)
	}
	return
}

func AddGrowthDataTx(tx *vbolt.Tx, req AddGrowthDataRequest, familyId int) (GrowthData, error) {
	var growthData GrowthData
	var err error

	// Validate person belongs to family
	person := GetPersonById(tx, req.PersonId)
	if person.Id == 0 || person.FamilyId != familyId {
		return growthData, errors.New("Person not found or not in your family")
	}

	// Parse measurement date
	growthData.MeasurementDate, err = parseMeasurementDate(req, person.Birthday)
	if err != nil {
		return growthData, err
	}

	// Convert string measurement type to enum
	var measurementType MeasurementType
	if req.MeasurementType == "height" {
		measurementType = Height
	} else if req.MeasurementType == "weight" {
		measurementType = Weight
	} else {
		return growthData, errors.New("Invalid measurement type")
	}

	// Create growth data record
	growthData.Id = vbolt.NextIntId(tx, GrowthDataBkt)
	growthData.PersonId = req.PersonId
	growthData.FamilyId = familyId
	growthData.MeasurementType = measurementType
	growthData.Value = req.Value
	growthData.Unit = req.Unit
	growthData.CreatedAt = time.Now()

	vbolt.Write(tx, GrowthDataBkt, growthData.Id, &growthData)

	updateGrowthDataIndices(tx, growthData)

	return growthData, nil
}

func updateGrowthDataIndices(tx *vbolt.Tx, growthData GrowthData) {
	vbolt.SetTargetSingleTerm(tx, GrowthDataByPersonIndex, growthData.Id, growthData.PersonId)
	vbolt.SetTargetSingleTerm(tx, GrowthDataByFamilyIndex, growthData.Id, growthData.FamilyId)
}

func parseMeasurementDate(req AddGrowthDataRequest, personBirthday time.Time) (time.Time, error) {
	if req.InputType == "date" {
		if req.MeasurementDate == nil || *req.MeasurementDate == "" {
			return time.Time{}, errors.New("Measurement date is required when input type is 'date'")
		}
		layout := "2006-01-02"
		return time.Parse(layout, *req.MeasurementDate)
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
		return time.Time{}, errors.New("Input type must be 'date' or 'age'")
	}
}

// vbeam procedures
func AddGrowthData(ctx *vbeam.Context, req AddGrowthDataRequest) (resp AddGrowthDataResponse, err error) {
	// Get authenticated user
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}

	// Validate request
	if err = validateAddGrowthDataRequest(req); err != nil {
		return
	}

	// Add growth data to database
	vbeam.UseWriteTx(ctx)
	growthData, err := AddGrowthDataTx(ctx.Tx, req, user.FamilyId)
	if err != nil {
		return
	}

	vbolt.TxCommit(ctx.Tx)

	resp.GrowthData = growthData
	return
}


func validateAddGrowthDataRequest(req AddGrowthDataRequest) error {
	if req.PersonId <= 0 {
		return errors.New("Person ID is required")
	}
	if req.MeasurementType != "height" && req.MeasurementType != "weight" {
		return errors.New("Measurement type must be 'height' or 'weight'")
	}
	if req.Value <= 0 {
		return errors.New("Measurement value must be positive")
	}
	if req.Unit == "" {
		return errors.New("Unit is required")
	}
	if req.InputType != "date" && req.InputType != "age" {
		return errors.New("Input type must be 'date' or 'age'")
	}

	// Validate units based on measurement type
	if req.MeasurementType == "height" {
		if req.Unit != "cm" && req.Unit != "in" {
			return errors.New("Height unit must be 'cm' or 'in'")
		}
	} else if req.MeasurementType == "weight" {
		if req.Unit != "kg" && req.Unit != "lbs" {
			return errors.New("Weight unit must be 'kg' or 'lbs'")
		}
	}

	return nil
}