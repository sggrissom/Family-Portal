package backend

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"go.hasen.dev/vpack"
)

const RecordTimeVersion = 1
const recordResolverVersion = 1

const (
	MeaningOccurrence = "occurrence"
	MeaningPeriod     = "period"
)

const (
	BasisCalendar = "calendar"
	BasisAge      = "age"
	BasisUnknown  = "unknown"
)

const (
	PrecisionDay     = "day"
	PrecisionMonth   = "month"
	PrecisionYear    = "year"
	PrecisionRange   = "range"
	PrecisionUnknown = "unknown"
)

const (
	SourceFamily      = "family"
	SourceInscription = "inscription"
	SourceMetadata    = "metadata"
	SourceLegacy      = "legacy"
	SourceAI          = "ai"
)

// RecordTime is what a family asserted about when a moment or piece of media
// happened, or which period of life it describes. Start and End are derived by
// the server as a half-open range of civil days; both are empty when the
// assertion cannot be placed on the calendar.
type RecordTime struct {
	Version   int    `json:"version"`
	Meaning   string `json:"meaning"`
	Basis     string `json:"basis"`
	Precision string `json:"precision"`
	Estimated bool   `json:"estimated"`

	From   string `json:"from,omitempty"`
	To     string `json:"to,omitempty"`
	Around string `json:"around,omitempty"`

	AnchorPersonId int `json:"anchorPersonId,omitempty"`
	AgeYears       int `json:"ageYears"`
	AgeMonths      int `json:"ageMonths"`
	AgeToYears     int `json:"ageToYears"`

	Source    string    `json:"source"`
	Note      string    `json:"note,omitempty"`
	ChangedBy int       `json:"changedBy,omitempty"`
	ChangedAt time.Time `json:"changedAt"`

	Start           string `json:"start"`
	End             string `json:"end"`
	Resolved        bool   `json:"resolved"`
	ResolverVersion int    `json:"resolverVersion"`
}

func PackRecordTime(self *RecordTime, buf *vpack.Buffer) {
	vpack.Version(1, buf)
	vpack.Int(&self.Version, buf)
	vpack.String(&self.Meaning, buf)
	vpack.String(&self.Basis, buf)
	vpack.String(&self.Precision, buf)
	vpack.Bool(&self.Estimated, buf)
	vpack.String(&self.From, buf)
	vpack.String(&self.To, buf)
	vpack.String(&self.Around, buf)
	vpack.Int(&self.AnchorPersonId, buf)
	vpack.Int(&self.AgeYears, buf)
	vpack.Int(&self.AgeMonths, buf)
	vpack.Int(&self.AgeToYears, buf)
	vpack.String(&self.Source, buf)
	vpack.String(&self.Note, buf)
	vpack.Int(&self.ChangedBy, buf)
	vpack.Time(&self.ChangedAt, buf)
	vpack.String(&self.Start, buf)
	vpack.String(&self.End, buf)
	vpack.Bool(&self.Resolved, buf)
	vpack.Int(&self.ResolverVersion, buf)
}

// civilDate is a calendar day with no time zone.
type civilDate struct {
	Year  int
	Month time.Month
	Day   int
}

func civilOf(t time.Time) civilDate {
	return civilDate{t.Year(), t.Month(), t.Day()}
}

func (d civilDate) String() string {
	return fmt.Sprintf("%04d-%02d-%02d", d.Year, d.Month, d.Day)
}

func (d civilDate) Time() time.Time {
	return time.Date(d.Year, d.Month, d.Day, 0, 0, 0, 0, time.UTC)
}

func daysIn(year int, month time.Month) int {
	return time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day()
}

// addMonths clamps to the last day of the target month, so Jan 31 plus a
// month is Feb 28/29 and Feb 29 plus a year is Feb 28.
func (d civilDate) addMonths(n int) civilDate {
	total := d.Year*12 + int(d.Month-1) + n
	year, month := total/12, time.Month(total%12+1)
	return civilDate{year, month, min(d.Day, daysIn(year, month))}
}

func (d civilDate) addDays(n int) civilDate {
	return civilOf(d.Time().AddDate(0, 0, n))
}

// parseCivil reads "YYYY", "YYYY-MM" or "YYYY-MM-DD" and returns the first day
// it names, the first day after it, and its precision.
func parseCivil(value string) (start, end civilDate, precision string, err error) {
	parts := strings.Split(value, "-")
	invalid := fmt.Errorf("%q is not a valid date", value)
	if len(parts) > 3 || len(parts[0]) != 4 {
		return start, end, "", invalid
	}
	nums := make([]int, len(parts))
	for i, part := range parts {
		if i > 0 && len(part) != 2 {
			return start, end, "", invalid
		}
		if nums[i], err = strconv.Atoi(part); err != nil || nums[i] < 0 {
			return start, end, "", invalid
		}
	}
	year := nums[0]
	if year < 1 {
		return start, end, "", invalid
	}
	switch len(nums) {
	case 1:
		start = civilDate{year, time.January, 1}
		return start, start.addMonths(12), PrecisionYear, nil
	case 2:
		if nums[1] < 1 || nums[1] > 12 {
			return start, end, "", invalid
		}
		start = civilDate{year, time.Month(nums[1]), 1}
		return start, start.addMonths(1), PrecisionMonth, nil
	}
	if nums[1] < 1 || nums[1] > 12 || nums[2] < 1 || nums[2] > daysIn(year, time.Month(nums[1])) {
		return start, end, "", invalid
	}
	start = civilDate{year, time.Month(nums[1]), nums[2]}
	return start, start.addDays(1), PrecisionDay, nil
}

// RecordAnchor is what resolving an age assertion needs to know about the
// person it is anchored to.
type RecordAnchor struct {
	Birthday      time.Time
	BirthdayKnown bool
}

func anchorOf(person Person) RecordAnchor {
	return RecordAnchor{
		Birthday:      person.Birthday,
		BirthdayKnown: person.Id != 0 && !person.IsPregnancy && !person.Birthday.IsZero(),
	}
}

func validateRecordTime(rt RecordTime, allowPeriod bool) error {
	if rt.Version != RecordTimeVersion {
		return fmt.Errorf("Unsupported date format version %d", rt.Version)
	}
	switch rt.Meaning {
	case MeaningOccurrence:
	case MeaningPeriod:
		if !allowPeriod {
			return errors.New("Only moments can describe a period of life")
		}
	default:
		return errors.New("Choose whether this happened then or describes that period")
	}
	switch rt.Source {
	case SourceFamily, SourceInscription, SourceMetadata, SourceLegacy, SourceAI:
	default:
		return errors.New("Unknown date source")
	}
	if rt.Around != "" && !rt.Estimated {
		return errors.New("Only an estimate can have a central value")
	}

	switch rt.Basis {
	case BasisUnknown:
		if rt.Precision != PrecisionUnknown {
			return errors.New("An unknown date has no precision")
		}
		if rt.Estimated || rt.Meaning != MeaningOccurrence {
			return errors.New("An unknown date can't be an estimate or a period")
		}
		if rt.From != "" || rt.To != "" || rt.AnchorPersonId != 0 || rt.AgeYears != 0 || rt.AgeMonths != 0 || rt.AgeToYears != 0 {
			return errors.New("An unknown date can't also have a date or age")
		}
		return nil
	case BasisCalendar:
		return validateCalendarAssertion(rt)
	case BasisAge:
		return validateAgeAssertion(rt)
	}
	return errors.New("Choose a date, an age, or unknown")
}

func validateCalendarAssertion(rt RecordTime) error {
	if rt.AnchorPersonId != 0 || rt.AgeYears != 0 || rt.AgeMonths != 0 || rt.AgeToYears != 0 {
		return errors.New("A calendar date can't also have an age")
	}
	_, fromEnd, fromPrecision, err := parseCivil(rt.From)
	if err != nil {
		return err
	}
	switch rt.Precision {
	case PrecisionDay, PrecisionMonth, PrecisionYear:
		if rt.To != "" {
			return errors.New("Only a range has an end date")
		}
		if fromPrecision != rt.Precision {
			return fmt.Errorf("%q is not a %s", rt.From, rt.Precision)
		}
	case PrecisionRange:
		_, toEnd, _, err := parseCivil(rt.To)
		if err != nil {
			return err
		}
		if toEnd.String() < fromEnd.String() {
			return errors.New("The range ends before it starts")
		}
	default:
		return errors.New("Unsupported date precision")
	}
	if rt.Around != "" {
		if _, _, _, err := parseCivil(rt.Around); err != nil {
			return err
		}
	}
	return nil
}

func validateAgeAssertion(rt RecordTime) error {
	if rt.From != "" || rt.To != "" {
		return errors.New("An age can't also have a calendar date")
	}
	if rt.AnchorPersonId <= 0 {
		return errors.New("Choose whose age this is")
	}
	if rt.AgeYears < 0 || rt.AgeMonths < 0 || rt.AgeToYears < 0 {
		return errors.New("Ages can't be negative")
	}
	switch rt.Precision {
	case PrecisionYear:
		if rt.AgeMonths != 0 || rt.AgeToYears != 0 {
			return errors.New("An age in years has no months or end age")
		}
	case PrecisionMonth:
		if rt.AgeMonths > 11 {
			return errors.New("Months should be 0 to 11")
		}
		if rt.AgeToYears != 0 {
			return errors.New("An age in months has no end age")
		}
	case PrecisionRange:
		if rt.AgeMonths != 0 {
			return errors.New("An age range is in whole years")
		}
		if rt.AgeToYears < rt.AgeYears {
			return errors.New("The age range ends before it starts")
		}
	default:
		return errors.New("Unsupported age precision")
	}
	if rt.Around != "" {
		if n, err := strconv.Atoi(rt.Around); err != nil || n < 0 {
			return errors.New("An estimated age is a whole number of years")
		}
	}
	return nil
}

// resolveRecordTime fills the derived range. It assumes rt is valid.
func resolveRecordTime(rt *RecordTime, anchor RecordAnchor) {
	rt.ResolverVersion = recordResolverVersion
	rt.Start, rt.End, rt.Resolved = "", "", false

	var start, end civilDate
	switch rt.Basis {
	case BasisCalendar:
		start, end, _, _ = parseCivil(rt.From)
		if rt.Precision == PrecisionRange {
			_, end, _, _ = parseCivil(rt.To)
		}
	case BasisAge:
		if !anchor.BirthdayKnown {
			return
		}
		born := civilOf(anchor.Birthday)
		switch rt.Precision {
		case PrecisionYear:
			start, end = born.addMonths(rt.AgeYears*12), born.addMonths((rt.AgeYears+1)*12)
		case PrecisionMonth:
			months := rt.AgeYears*12 + rt.AgeMonths
			start, end = born.addMonths(months), born.addMonths(months+1)
		case PrecisionRange:
			start, end = born.addMonths(rt.AgeYears*12), born.addMonths((rt.AgeToYears+1)*12)
		}
	default:
		return
	}
	rt.Start, rt.End, rt.Resolved = start.String(), end.String(), true
}

// checkRecordTimeNotFuture rejects a resolved assertion that starts after today.
func checkRecordTimeNotFuture(rt RecordTime, today time.Time) error {
	if rt.Resolved && rt.Start > civilOf(today).String() {
		if rt.Basis == BasisAge {
			return errors.New("That age hasn't happened yet")
		}
		return errors.New("That date hasn't happened yet")
	}
	return nil
}

// ExactDay reports whether rt is a known occurrence on one calendar day, the
// only kind of record an anniversary can be built from.
func (rt RecordTime) ExactDay() bool {
	return rt.Meaning == MeaningOccurrence && rt.Basis == BasisCalendar &&
		rt.Precision == PrecisionDay && !rt.Estimated
}

// Overlaps reports whether rt's derived range overlaps [start, end), both
// given as civil days.
func (rt RecordTime) Overlaps(start, end string) bool {
	return rt.Resolved && rt.Start < end && rt.End > start
}

// SortKey is a deterministic ordering position, the start of the derived
// range; unresolved records have none and belong in an undated group.
func (rt RecordTime) SortKey() (time.Time, bool) {
	if !rt.Resolved {
		return time.Time{}, false
	}
	start, _, _, _ := parseCivil(rt.Start)
	return start.Time(), true
}

func exactRecordTime(day time.Time, source string) RecordTime {
	rt := RecordTime{
		Version:   RecordTimeVersion,
		Meaning:   MeaningOccurrence,
		Basis:     BasisCalendar,
		Precision: PrecisionDay,
		From:      civilOf(day).String(),
		Source:    source,
	}
	resolveRecordTime(&rt, RecordAnchor{})
	return rt
}

var monthNames = [...]string{"January", "February", "March", "April", "May", "June", "July",
	"August", "September", "October", "November", "December"}

func civilLabel(value string) string {
	start, _, precision, err := parseCivil(value)
	if err != nil {
		return value
	}
	switch precision {
	case PrecisionYear:
		return strconv.Itoa(start.Year)
	case PrecisionMonth:
		return monthNames[start.Month-1] + " " + strconv.Itoa(start.Year)
	}
	return start.Time().Format("Jan 2, 2006")
}

func joinRange(from, to string) string {
	if strings.Contains(from, " ") || strings.Contains(to, " ") {
		return from + " – " + to
	}
	return from + "–" + to
}

func plural(n int, unit string) string {
	if n == 1 {
		return "1 " + unit
	}
	return strconv.Itoa(n) + " " + unit + "s"
}

// Label is the plain-language form of what the family asserted. Shared
// fixtures keep it identical to the web and iOS labels.
func (rt RecordTime) Label() string {
	switch rt.Basis {
	case BasisCalendar:
		base := civilLabel(rt.From)
		if rt.Precision == PrecisionRange {
			base = joinRange(base, civilLabel(rt.To))
		}
		if !rt.Estimated {
			return base
		}
		if rt.Around != "" {
			return "Around " + civilLabel(rt.Around) + " (" + base + ")"
		}
		return "Around " + base
	case BasisAge:
		var age string
		switch rt.Precision {
		case PrecisionYear:
			age = "age " + strconv.Itoa(rt.AgeYears)
		case PrecisionMonth:
			age = "age " + plural(rt.AgeYears, "year") + ", " + plural(rt.AgeMonths, "month")
		case PrecisionRange:
			age = "ages " + strconv.Itoa(rt.AgeYears) + "–" + strconv.Itoa(rt.AgeToYears)
		}
		if rt.Estimated {
			if rt.Around != "" {
				return "Around age " + rt.Around + " (" + age + ")"
			}
			return "Around " + age
		}
		if rt.Meaning == MeaningPeriod {
			return "At " + age
		}
		return "Sometime at " + age
	}
	return "Date unknown"
}
