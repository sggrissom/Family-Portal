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
	vbeam.RegisterProc(app, AddCheckup)
	vbeam.RegisterProc(app, GetGrowthData)
	vbeam.RegisterProc(app, UpdateGrowthData)
	vbeam.RegisterProc(app, DeleteGrowthData)
}

type MeasurementType int

const (
	Height MeasurementType = iota
	Weight
)

type AddGrowthDataRequest struct {
	PersonId        int     `json:"personId"`
	MeasurementType string  `json:"measurementType"`
	Value           float64 `json:"value"`
	Unit            string  `json:"unit"`
	InputType       string  `json:"inputType"`
	MeasurementDate *string `json:"measurementDate,omitempty"`
	AgeYears        *int    `json:"ageYears,omitempty"`
	AgeMonths       *int    `json:"ageMonths,omitempty"`
}

type AddGrowthDataResponse struct {
	GrowthData GrowthData `json:"growthData"`
}

type CheckupValue struct {
	Value float64 `json:"value"`
	Unit  string  `json:"unit"`
}

type AddCheckupRequest struct {
	PersonId        int           `json:"personId"`
	InputType       string        `json:"inputType"`
	MeasurementDate *string       `json:"measurementDate,omitempty"`
	AgeYears        *int          `json:"ageYears,omitempty"`
	AgeMonths       *int          `json:"ageMonths,omitempty"`
	Height          *CheckupValue `json:"height,omitempty"`
	Weight          *CheckupValue `json:"weight,omitempty"`
}

type AddCheckupResponse struct {
	GrowthData []GrowthData `json:"growthData"`
}

type UpdateGrowthDataRequest struct {
	Id              int     `json:"id"`
	MeasurementType string  `json:"measurementType"`
	Value           float64 `json:"value"`
	Unit            string  `json:"unit"`
	InputType       string  `json:"inputType"`
	MeasurementDate *string `json:"measurementDate,omitempty"`
	AgeYears        *int    `json:"ageYears,omitempty"`
	AgeMonths       *int    `json:"ageMonths,omitempty"`
}

type UpdateGrowthDataResponse struct {
	GrowthData GrowthData `json:"growthData"`
}

type DeleteGrowthDataRequest struct {
	Id int `json:"id"`
}

type DeleteGrowthDataResponse struct {
	Success bool `json:"success"`
}

type GetGrowthDataRequest struct {
	Id int `json:"id"`
}

type GetGrowthDataResponse struct {
	GrowthData GrowthData `json:"growthData"`
}

type GrowthData struct {
	Id              int             `json:"id"`
	PersonId        int             `json:"personId"`
	FamilyId        int             `json:"familyId"`
	MeasurementType MeasurementType `json:"measurementType"`
	Value           float64         `json:"value"`
	Unit            string          `json:"unit"`
	MeasurementDate time.Time       `json:"measurementDate"`
	CreatedAt       time.Time       `json:"createdAt"`
}

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

var GrowthDataBkt = vbolt.Bucket(&cfg.Info, "growth_data", vpack.FInt, PackGrowthData)

var GrowthDataByPersonIndex = vbolt.Index(&cfg.Info, "growth_data_by_person", vpack.FInt, vpack.FInt)

var GrowthDataByFamilyIndex = vbolt.Index(&cfg.Info, "growth_data_by_family", vpack.FInt, vpack.FInt)

func GetGrowthDataById(tx *vbolt.Tx, growthDataId int) (growthData GrowthData) {
	vbolt.Read(tx, GrowthDataBkt, growthDataId, &growthData)
	return
}

func GetPersonGrowthDataTx(tx *vbolt.Tx, personId int) []GrowthData {
	growthData := []GrowthData{}
	var growthDataIds []int
	vbolt.ReadTermTargets(tx, GrowthDataByPersonIndex, personId, &growthDataIds, vbolt.Window{})
	if len(growthDataIds) > 0 {
		vbolt.ReadSlice(tx, GrowthDataBkt, growthDataIds, &growthData)
	}
	return growthData
}

func GetGrowthDataByIdAndFamily(tx *vbolt.Tx, growthDataId int, familyId int) (GrowthData, error) {
	growthData := GetGrowthDataById(tx, growthDataId)
	if growthData.Id == 0 {
		return growthData, errors.New("Growth data not found")
	}
	if !CanFamilyAccess(tx, familyId, growthData.FamilyId, AccessView) {
		return growthData, errors.New("Access denied: growth data belongs to another family")
	}
	return growthData, nil
}

func GetGrowthDataForUser(tx *vbolt.Tx, growthDataId int, user User, need AccessLevel) (GrowthData, error) {
	growthData := GetGrowthDataById(tx, growthDataId)
	if growthData.Id == 0 {
		return growthData, errors.New("Growth data not found")
	}
	if !CanAccessRecordOfPerson(tx, user, growthData.FamilyId, growthData.PersonId, ScopeGrowth, need) {
		return growthData, errors.New("Access denied: growth data belongs to another family")
	}
	return growthData, nil
}

func UpdateGrowthDataTx(tx *vbolt.Tx, req UpdateGrowthDataRequest, familyId int) (GrowthData, error) {
	var err error

	growthData, err := GetGrowthDataByIdAndFamily(tx, req.Id, familyId)
	if err != nil {
		return growthData, err
	}

	person := GetPersonById(tx, growthData.PersonId)
	if person.Id == 0 {
		return growthData, errors.New("Person not found")
	}

	growthData.MeasurementDate, err = resolveEntryDate("Measurement", req.InputType, req.MeasurementDate, req.AgeYears, req.AgeMonths, person.Birthday)
	if err != nil {
		return growthData, err
	}

	measurementType, ok := measurementTypes[req.MeasurementType]
	if !ok {
		return growthData, errors.New("Invalid measurement type")
	}

	growthData.MeasurementType = measurementType
	growthData.Value = req.Value
	growthData.Unit = req.Unit

	writeGrowthData(tx, growthData)

	return growthData, nil
}

func getFamilyGrowthData(tx *vbolt.Tx, familyId int) (growthData []GrowthData) {
	var growthDataIds []int
	vbolt.ReadTermTargets(tx, GrowthDataByFamilyIndex, familyId, &growthDataIds, vbolt.Window{})
	if len(growthDataIds) > 0 {
		vbolt.ReadSlice(tx, GrowthDataBkt, growthDataIds, &growthData)
	}
	return
}

func DeleteGrowthDataTx(tx *vbolt.Tx, growthDataId int, familyId int) error {
	growthData, err := GetGrowthDataByIdAndFamily(tx, growthDataId, familyId)
	if err != nil {
		return err
	}

	vbolt.DeleteTargetTerms(tx, GrowthDataByPersonIndex, growthData.Id)
	vbolt.DeleteTargetTerms(tx, GrowthDataByFamilyIndex, growthData.Id)
	vbolt.Delete(tx, GrowthDataBkt, growthData.Id)

	return nil
}

func AddGrowthDataTx(tx *vbolt.Tx, req AddGrowthDataRequest, familyId int) (GrowthData, error) {
	measurementType, ok := measurementTypes[req.MeasurementType]
	if !ok {
		return GrowthData{}, errors.New("Invalid measurement type")
	}
	date, err := resolveGrowthDate(tx, req.PersonId, familyId, req.InputType, req.MeasurementDate, req.AgeYears, req.AgeMonths)
	if err != nil {
		return GrowthData{}, err
	}
	return addGrowthRecord(tx, req.PersonId, familyId, measurementType, CheckupValue{req.Value, req.Unit}, date), nil
}

func AddCheckupTx(tx *vbolt.Tx, req AddCheckupRequest, familyId int) ([]GrowthData, error) {
	date, err := resolveGrowthDate(tx, req.PersonId, familyId, req.InputType, req.MeasurementDate, req.AgeYears, req.AgeMonths)
	if err != nil {
		return nil, err
	}
	var records []GrowthData
	if req.Height != nil {
		records = append(records, addGrowthRecord(tx, req.PersonId, familyId, Height, *req.Height, date))
	}
	if req.Weight != nil {
		records = append(records, addGrowthRecord(tx, req.PersonId, familyId, Weight, *req.Weight, date))
	}
	return records, nil
}

func resolveGrowthDate(tx *vbolt.Tx, personId int, familyId int, inputType string, date *string, ageYears *int, ageMonths *int) (time.Time, error) {
	person := GetPersonById(tx, personId)
	if person.Id == 0 || !CanFamilyAccess(tx, familyId, person.FamilyId, AccessContribute) {
		return time.Time{}, errors.New("Person not found or not in your family")
	}
	return resolveEntryDate("Measurement", inputType, date, ageYears, ageMonths, person.Birthday)
}

func addGrowthRecord(tx *vbolt.Tx, personId int, familyId int, measurementType MeasurementType, value CheckupValue, date time.Time) GrowthData {
	growthData := GrowthData{
		Id:              vbolt.NextIntId(tx, GrowthDataBkt),
		PersonId:        personId,
		FamilyId:        familyId,
		MeasurementType: measurementType,
		Value:           value.Value,
		Unit:            value.Unit,
		MeasurementDate: date,
		CreatedAt:       time.Now(),
	}
	writeGrowthData(tx, growthData)
	return growthData
}

func writeGrowthData(tx *vbolt.Tx, growthData GrowthData) {
	vbolt.Write(tx, GrowthDataBkt, growthData.Id, &growthData)
	vbolt.SetTargetSingleTerm(tx, GrowthDataByPersonIndex, growthData.Id, growthData.PersonId)
	vbolt.SetTargetSingleTerm(tx, GrowthDataByFamilyIndex, growthData.Id, growthData.FamilyId)
}

func AddGrowthData(ctx *vbeam.Context, req AddGrowthDataRequest) (resp AddGrowthDataResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}

	if req.PersonId <= 0 {
		err = errors.New("Person ID is required")
		return
	}
	if err = validateMeasurementFields(req.MeasurementType, req.Value, req.Unit, req.InputType); err != nil {
		return
	}

	familyId, err := ActingFamilyForPerson(ctx.Tx, user, req.PersonId, AccessContribute)
	if err != nil {
		return
	}

	vbeam.UseWriteTx(ctx)
	growthData, err := AddGrowthDataTx(ctx.Tx, req, familyId)
	if err != nil {
		return
	}

	vbolt.TxCommit(ctx.Tx)

	resp.GrowthData = growthData
	return
}

func AddCheckup(ctx *vbeam.Context, req AddCheckupRequest) (resp AddCheckupResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}

	if req.PersonId <= 0 {
		err = errors.New("Person ID is required")
		return
	}
	if req.Height == nil && req.Weight == nil {
		err = errors.New("Enter a height or a weight")
		return
	}
	if req.Height != nil {
		if err = validateMeasurementFields("height", req.Height.Value, req.Height.Unit, req.InputType); err != nil {
			return
		}
	}
	if req.Weight != nil {
		if err = validateMeasurementFields("weight", req.Weight.Value, req.Weight.Unit, req.InputType); err != nil {
			return
		}
	}

	familyId, err := ActingFamilyForPerson(ctx.Tx, user, req.PersonId, AccessContribute)
	if err != nil {
		return
	}

	vbeam.UseWriteTx(ctx)
	resp.GrowthData, err = AddCheckupTx(ctx.Tx, req, familyId)
	if err != nil {
		return
	}

	vbolt.TxCommit(ctx.Tx)
	return
}

func GetGrowthData(ctx *vbeam.Context, req GetGrowthDataRequest) (resp GetGrowthDataResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}

	if req.Id <= 0 {
		err = errors.New("Growth data ID is required")
		return
	}

	growthData, err := GetGrowthDataForUser(ctx.Tx, req.Id, user, AccessView)
	if err != nil {
		return
	}

	resp.GrowthData = growthData
	return
}

func UpdateGrowthData(ctx *vbeam.Context, req UpdateGrowthDataRequest) (resp UpdateGrowthDataResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}

	if req.Id <= 0 {
		err = errors.New("Growth data ID is required")
		return
	}
	if err = validateMeasurementFields(req.MeasurementType, req.Value, req.Unit, req.InputType); err != nil {
		return
	}

	existing, err := GetGrowthDataForUser(ctx.Tx, req.Id, user, AccessContribute)
	if err != nil {
		return
	}

	vbeam.UseWriteTx(ctx)
	growthData, err := UpdateGrowthDataTx(ctx.Tx, req, existing.FamilyId)
	if err != nil {
		return
	}

	vbolt.TxCommit(ctx.Tx)

	resp.GrowthData = growthData
	return
}

func DeleteGrowthData(ctx *vbeam.Context, req DeleteGrowthDataRequest) (resp DeleteGrowthDataResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}

	if req.Id <= 0 {
		err = errors.New("Growth data ID is required")
		return
	}

	existing, err := GetGrowthDataForUser(ctx.Tx, req.Id, user, AccessContribute)
	if err != nil {
		return
	}

	vbeam.UseWriteTx(ctx)
	err = DeleteGrowthDataTx(ctx.Tx, req.Id, existing.FamilyId)
	if err != nil {
		return
	}

	vbolt.TxCommit(ctx.Tx)

	resp.Success = true
	return
}

var measurementTypes = map[string]MeasurementType{"height": Height, "weight": Weight}

func validateMeasurementFields(measurementType string, value float64, unit string, inputType string) error {
	if _, ok := measurementTypes[measurementType]; !ok {
		return errors.New("Measurement type must be 'height' or 'weight'")
	}
	if value <= 0 {
		return errors.New("Measurement value must be positive")
	}
	if unit == "" {
		return errors.New("Unit is required")
	}
	if err := validateEntryInputType(inputType); err != nil {
		return err
	}
	if measurementType == "height" && unit != "cm" && unit != "in" {
		return errors.New("Height unit must be 'cm' or 'in'")
	}
	if measurementType == "weight" && unit != "kg" && unit != "lbs" {
		return errors.New("Weight unit must be 'kg' or 'lbs'")
	}
	return nil
}
