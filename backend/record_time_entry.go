package backend

import (
	"errors"
	"time"

	"go.hasen.dev/vbolt"
)

var ErrDateRequired = errors.New("Choose when this happened")

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
