package backend

import (
	"testing"
	"time"
)

func TestImportDataValidation(t *testing.T) {
	tests := []struct {
		name          string
		importData    ImportDataStructure
		shouldError   bool
		errorContains string
	}{
		{
			name: "valid data",
			importData: ImportDataStructure{
				People: []ImportPerson{
					{
						Id:       1,
						Name:     "Test Person",
						Birthday: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
					},
				},
			},
			shouldError: false,
		},
		{
			name: "no people",
			importData: ImportDataStructure{
				People: []ImportPerson{},
			},
			shouldError:   true,
			errorContains: "No people found",
		},
		{
			name: "person with no name",
			importData: ImportDataStructure{
				People: []ImportPerson{
					{
						Id:       1,
						Name:     "",
						Birthday: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
					},
				},
			},
			shouldError:   true,
			errorContains: "has no name",
		},
		{
			name: "person with invalid birthday",
			importData: ImportDataStructure{
				People: []ImportPerson{
					{
						Id:       1,
						Name:     "Test Person",
						Birthday: time.Time{},
					},
				},
			},
			shouldError:   true,
			errorContains: "invalid birthday",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateImportData(test.importData)

			if test.shouldError {
				if err == nil {
					t.Errorf("Expected error for %s, but got none", test.name)
				} else if test.errorContains != "" && err.Error() != test.errorContains {
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error for %s: %v", test.name, err)
				}
			}
		})
	}
}
