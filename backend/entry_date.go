package backend

import (
	"errors"
	"time"
)

func validateEntryInputType(inputType string) error {
	if inputType != "today" && inputType != "date" && inputType != "age" {
		return errors.New("Input type must be 'today', 'date' or 'age'")
	}
	return nil
}

// noun names the date in the missing-date error, e.g. "Milestone".
func resolveEntryDate(noun string, inputType string, date *string, ageYears *int, ageMonths *int, birthday time.Time) (time.Time, error) {
	switch inputType {
	case "today":
		return dayStart(time.Now().UTC()), nil
	case "date":
		if date == nil || *date == "" {
			return time.Time{}, errors.New(noun + " date is required when input type is 'date'")
		}
		return time.Parse("2006-01-02", *date)
	case "age":
		if ageYears == nil || *ageYears < 0 {
			return time.Time{}, errors.New("Age years must be non-negative")
		}
		months := 0
		if ageMonths != nil {
			if *ageMonths < 0 || *ageMonths > 11 {
				return time.Time{}, errors.New("Age months must be between 0 and 11")
			}
			months = *ageMonths
		}
		return dayStart(birthday).AddDate(*ageYears, months, 0), nil
	}
	return time.Time{}, validateEntryInputType(inputType)
}
