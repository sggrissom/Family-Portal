package backend

import (
	"errors"
	"os"
	"time"

	"go.hasen.dev/vbeam"
	"go.hasen.dev/vbolt"
)

var ErrApproximateDatesOff = errors.New("Approximate dates aren't turned on yet")
var ErrApproximateDateNeedsUpdate = errors.New("This has an approximate date. Update the app to change it.")

func RegisterRecordTimeMethods(app *vbeam.Application) {
	vbeam.RegisterProc(app, GetRecordTimeSettings)
}

// ApproximateDatesEnabled turns on writing anything other than an exact day.
// Readers handle every assertion regardless.
func ApproximateDatesEnabled() bool {
	return os.Getenv("APPROXIMATE_DATES") == "on"
}

type RecordTimeSettings struct {
	ApproximateDates bool `json:"approximateDates"`
	Version          int  `json:"version"`
}

func GetRecordTimeSettings(ctx *vbeam.Context, req Empty) (resp RecordTimeSettings, err error) {
	if _, authErr := GetAuthUser(ctx); authErr != nil {
		err = ErrAuthFailure
		return
	}
	return RecordTimeSettings{ApproximateDates: ApproximateDatesEnabled(), Version: RecordTimeVersion}, nil
}

// applyRecordTime puts a client's assertion on a record after checking it.
// The derived range is always recomputed here; clients never set it.
func applyRecordTime(tx *vbolt.Tx, familyId int, userId int, requested RecordTime, allowPeriod bool, rt *RecordTime, at *time.Time) error {
	switch requested.Source {
	case "":
		requested.Source = SourceFamily
	case SourceFamily, SourceInscription:
	default:
		return errors.New("Unknown date source")
	}
	if err := validateRecordTime(requested, allowPeriod); err != nil {
		return err
	}
	if !requested.ExactDay() && !ApproximateDatesEnabled() {
		return ErrApproximateDatesOff
	}

	anchor := RecordAnchor{}
	if requested.Basis == BasisAge {
		person := GetPersonById(tx, requested.AnchorPersonId)
		if person.Id == 0 || !CanFamilyAccess(tx, familyId, person.FamilyId, AccessView) {
			return errors.New("Choose whose age this is")
		}
		anchor = anchorOf(person)
	}
	resolveRecordTime(&requested, anchor)
	if err := checkRecordTimeNotFuture(requested, time.Now().UTC()); err != nil {
		return err
	}
	if requested.ExactDay() && rt.ExactDay() && rt.From == requested.From && rt.Source == requested.Source && rt.Note == requested.Note {
		return nil
	}

	requested.ChangedBy = userId
	requested.ChangedAt = time.Now().UTC()
	if requested.ExactDay() && civilOf(*at).String() != requested.From {
		start, _, _, _ := parseCivil(requested.From)
		*at = start.Time()
	}
	*rt = requested
	return nil
}

// keepsApproximateDate decides what a legacy date field means for a record
// whose date is approximate. Older clients send back the sort key they were
// shown; that leaves the assertion alone. Any other date would flatten it.
func keepsApproximateDate(rt RecordTime, at time.Time, legacyDate *string) (bool, error) {
	if rt.ExactDay() {
		return false, nil
	}
	if legacyDate != nil && *legacyDate == dayOf(at) {
		return true, nil
	}
	return false, ErrApproximateDateNeedsUpdate
}
