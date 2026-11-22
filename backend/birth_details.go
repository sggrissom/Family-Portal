package backend

import (
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"go.hasen.dev/vbolt"
)

const maxBirthPlace = 200

// BirthDetails are optional on AddPerson and UpdatePerson. A nil field
// leaves what is stored alone. An empty time or place clears it, and a
// measurement with value 0 removes it.
type BirthDetails struct {
	BirthTime   *string       `json:"birthTime,omitempty"`
	BirthPlace  *string       `json:"birthPlace,omitempty"`
	BirthWeight *CheckupValue `json:"birthWeight,omitempty"`
	BirthLength *CheckupValue `json:"birthLength,omitempty"`
}

func validateBirthDetails(d BirthDetails) error {
	if d.BirthTime != nil && *d.BirthTime != "" {
		if _, err := time.Parse("15:04", *d.BirthTime); err != nil {
			return errors.New("Birth time should look like 14:30")
		}
	}
	if d.BirthPlace != nil && utf8.RuneCountInString(strings.TrimSpace(*d.BirthPlace)) > maxBirthPlace {
		return errors.New("Birth place is too long")
	}
	if d.BirthWeight != nil && d.BirthWeight.Value != 0 {
		if err := validateMeasurementFields("weight", d.BirthWeight.Value, d.BirthWeight.Unit, "date"); err != nil {
			return err
		}
	}
	if d.BirthLength != nil && d.BirthLength.Value != 0 {
		if err := validateMeasurementFields("height", d.BirthLength.Value, d.BirthLength.Unit, "date"); err != nil {
			return err
		}
	}
	return nil
}

// applyBirthDetails stores the time and place on the person. Weight and length
// are growth records on the birthday, so the growth chart starts at day zero.
// Someone not yet born has no birth details.
func applyBirthDetails(tx *vbolt.Tx, person *Person, userId int, d BirthDetails) {
	if person.IsPregnancy {
		return
	}
	if d.BirthTime != nil {
		person.BirthTime = *d.BirthTime
	}
	if d.BirthPlace != nil {
		person.BirthPlace = strings.TrimSpace(*d.BirthPlace)
	}
	if d.BirthWeight != nil {
		setBirthMeasurement(tx, *person, userId, Weight, *d.BirthWeight)
	}
	if d.BirthLength != nil {
		setBirthMeasurement(tx, *person, userId, Height, *d.BirthLength)
	}
}

func birthRecords(tx *vbolt.Tx, person Person, kind MeasurementType) (records []GrowthData) {
	for _, g := range GetPersonGrowthDataTx(tx, person.Id) {
		if g.MeasurementType == kind && g.MeasurementDate.Equal(person.Birthday) {
			records = append(records, g)
		}
	}
	return
}

func setBirthMeasurement(tx *vbolt.Tx, person Person, userId int, kind MeasurementType, value CheckupValue) {
	existing := birthRecords(tx, person, kind)
	if value.Value == 0 {
		for _, g := range existing {
			deleteGrowthRecord(tx, g)
		}
		return
	}
	if len(existing) == 0 {
		addGrowthRecord(tx, person.Id, person.FamilyId, userId, kind, value, person.Birthday)
		return
	}
	record := existing[0]
	if record.Value == value.Value && record.Unit == value.Unit {
		return
	}
	record.Value = value.Value
	record.Unit = value.Unit
	writeGrowthData(tx, record)
}

// moveBirthRecords keeps measurements taken on the birthday at birth when the
// birthday is corrected.
func moveBirthRecords(tx *vbolt.Tx, personId int, from time.Time, to time.Time) {
	if from.Equal(to) {
		return
	}
	for _, g := range GetPersonGrowthDataTx(tx, personId) {
		if g.MeasurementDate.Equal(from) {
			g.MeasurementDate = to
			writeGrowthData(tx, g)
		}
	}
}

func importedBirthDetails(in ImportPerson) (birthTime string, birthPlace string) {
	if validateBirthDetails(BirthDetails{BirthTime: &in.BirthTime}) == nil {
		birthTime = in.BirthTime
	}
	birthPlace = strings.TrimSpace(in.BirthPlace)
	if utf8.RuneCountInString(birthPlace) > maxBirthPlace {
		birthPlace = string([]rune(birthPlace)[:maxBirthPlace])
	}
	return
}

// fillBirthDetails adds an imported time or place to a person who has none,
// and never overwrites one already recorded.
func fillBirthDetails(tx *vbolt.Tx, person Person, in ImportPerson) {
	birthTime, birthPlace := importedBirthDetails(in)
	changed := false
	if person.BirthTime == "" && birthTime != "" {
		person.BirthTime = birthTime
		changed = true
	}
	if person.BirthPlace == "" && birthPlace != "" {
		person.BirthPlace = birthPlace
		changed = true
	}
	if changed {
		vbolt.Write(tx, PeopleBkt, person.Id, &person)
	}
}
