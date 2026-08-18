// Exporting a family's activities.
//
// The bundle nests rather than listing five parallel arrays with cross
// references. The activities tree is five levels deep (activity → season →
// competition/routine → performance → result), and flattening it is what makes
// import.go's milestone handling as long as it is: every flat array needs its
// own id-remapping pass. Nested, the parent's new id is simply in scope when
// the child is written.
//
// The one place that cannot nest is a performance's routine: a performance
// belongs to both a competition and a routine, and only one of them can be its
// parent in a tree. Performances hang under the competition — that is how a
// results sheet arrives — and carry EntryId to rejoin the routine.
//
// See docs/activities-plan.md, phase 7.
package backend

import (
	"time"

	"go.hasen.dev/vbolt"
)

type ExportActivity struct {
	Id        int            `json:"id"`
	Name      string         `json:"name"`
	Kind      string         `json:"kind"`
	CreatedAt time.Time      `json:"createdAt"`
	Seasons   []ExportSeason `json:"seasons,omitempty"`
}

type ExportSeason struct {
	Id        int           `json:"id"`
	Name      string        `json:"name"`
	StartDate time.Time     `json:"startDate"`
	EndDate   time.Time     `json:"endDate"`
	Notes     string        `json:"notes,omitempty"`
	CreatedAt time.Time     `json:"createdAt"`
	Entries   []ExportEntry `json:"entries,omitempty"`
	Events    []ExportEvent `json:"events,omitempty"`
}

// ExportEntry carries PersonNames alongside PersonIds for the same reason
// ExportMilestone carries PersonName: the ids mean nothing to someone reading
// the bundle, and nothing on import depends on the names.
type ExportEntry struct {
	Id          int       `json:"id"`
	Name        string    `json:"name"`
	Format      string    `json:"format,omitempty"`
	Style       string    `json:"style,omitempty"`
	Division    string    `json:"division,omitempty"`
	Level       string    `json:"level,omitempty"`
	Notes       string    `json:"notes,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
	PersonIds   []int     `json:"personIds,omitempty"`
	PersonNames []string  `json:"personNames,omitempty"`
}

type ExportEvent struct {
	Id          int                `json:"id"`
	Name        string             `json:"name"`
	Host        string             `json:"host,omitempty"`
	Location    string             `json:"location,omitempty"`
	StartDate   time.Time          `json:"startDate"`
	EndDate     time.Time          `json:"endDate"`
	Notes       string             `json:"notes,omitempty"`
	CreatedAt   time.Time          `json:"createdAt"`
	PhotoIds    []int              `json:"photoIds,omitempty"`
	Appearances []ExportAppearance `json:"appearances,omitempty"`
}

type ExportAppearance struct {
	Id         int            `json:"id"`
	EntryId    int            `json:"entryId"`
	EntryName  string         `json:"entryName,omitempty"`
	OccurredAt time.Time      `json:"occurredAt"`
	Notes      string         `json:"notes,omitempty"`
	CreatedAt  time.Time      `json:"createdAt"`
	PhotoIds   []int          `json:"photoIds,omitempty"`
	Results    []ExportResult `json:"results,omitempty"`
}

// ExportResult keeps Rank, OutOf, Score and PersonId as pointers so the bundle
// distinguishes "no placement" from "first" the same way the record does.
// Marshalling them as zero would quietly turn every award into a 0th-place
// finish on the way back in.
type ExportResult struct {
	Id         int       `json:"id"`
	Kind       string    `json:"kind"`
	Label      string    `json:"label,omitempty"`
	Rank       *int      `json:"rank,omitempty"`
	OutOf      *int      `json:"outOf,omitempty"`
	Category   string    `json:"category,omitempty"`
	Score      *float64  `json:"score,omitempty"`
	PersonId   *int      `json:"personId,omitempty"`
	PersonName string    `json:"personName,omitempty"`
	Notes      string    `json:"notes,omitempty"`
	SortOrder  int       `json:"sortOrder"`
	CreatedAt  time.Time `json:"createdAt"`
}

// buildActivityExport walks a family's activities top-down. Every read is an
// index walk from a parent id, so the cost is proportional to what the family
// actually recorded rather than to the size of the buckets.
func buildActivityExport(tx *vbolt.Tx, familyId int, personNames map[int]string) []ExportActivity {
	activities := GetFamilyActivities(tx, familyId)
	exported := make([]ExportActivity, 0, len(activities))
	for _, activity := range activities {
		exported = append(exported, ExportActivity{
			Id:        activity.Id,
			Name:      activity.Name,
			Kind:      activity.Kind,
			CreatedAt: activity.CreatedAt,
			Seasons:   exportSeasons(tx, activity.Id, personNames),
		})
	}
	return exported
}

func exportSeasons(tx *vbolt.Tx, activityId int, personNames map[int]string) []ExportSeason {
	seasons := GetActivitySeasons(tx, activityId)
	exported := make([]ExportSeason, 0, len(seasons))
	for _, season := range seasons {
		// Routine names are needed by the performances below, and reading the
		// routines once here is cheaper than a lookup per performance.
		entries := GetSeasonEntries(tx, season.Id)
		entryNames := make(map[int]string, len(entries))
		exportedEntries := make([]ExportEntry, 0, len(entries))
		for _, entry := range entries {
			entryNames[entry.Id] = entry.Name
			personIds := GetEntryPersonIds(tx, entry.Id)
			exportedEntries = append(exportedEntries, ExportEntry{
				Id:          entry.Id,
				Name:        entry.Name,
				Format:      entry.Format,
				Style:       entry.Style,
				Division:    entry.Division,
				Level:       entry.Level,
				Notes:       entry.Notes,
				CreatedAt:   entry.CreatedAt,
				PersonIds:   personIds,
				PersonNames: namesFor(personIds, personNames),
			})
		}

		exported = append(exported, ExportSeason{
			Id:        season.Id,
			Name:      season.Name,
			StartDate: season.StartDate,
			EndDate:   season.EndDate,
			Notes:     season.Notes,
			CreatedAt: season.CreatedAt,
			Entries:   exportedEntries,
			Events:    exportEvents(tx, season.Id, entryNames, personNames),
		})
	}
	return exported
}

func exportEvents(
	tx *vbolt.Tx,
	seasonId int,
	entryNames map[int]string,
	personNames map[int]string,
) []ExportEvent {
	events := GetSeasonEvents(tx, seasonId)
	exported := make([]ExportEvent, 0, len(events))
	for _, event := range events {
		exported = append(exported, ExportEvent{
			Id:          event.Id,
			Name:        event.Name,
			Host:        event.Host,
			Location:    event.Location,
			StartDate:   event.StartDate,
			EndDate:     event.EndDate,
			Notes:       event.Notes,
			CreatedAt:   event.CreatedAt,
			PhotoIds:    GetEventPhotoIds(tx, event.Id),
			Appearances: exportAppearances(tx, event.Id, entryNames, personNames),
		})
	}
	return exported
}

func exportAppearances(
	tx *vbolt.Tx,
	eventId int,
	entryNames map[int]string,
	personNames map[int]string,
) []ExportAppearance {
	appearances := GetEventAppearances(tx, eventId)
	exported := make([]ExportAppearance, 0, len(appearances))
	for _, appearance := range appearances {
		exported = append(exported, ExportAppearance{
			Id:         appearance.Id,
			EntryId:    appearance.EntryId,
			EntryName:  entryNames[appearance.EntryId],
			OccurredAt: appearance.OccurredAt,
			Notes:      appearance.Notes,
			CreatedAt:  appearance.CreatedAt,
			PhotoIds:   GetAppearancePhotoIds(tx, appearance.Id),
			Results:    exportResults(tx, appearance.Id, personNames),
		})
	}
	return exported
}

func exportResults(tx *vbolt.Tx, appearanceId int, personNames map[int]string) []ExportResult {
	results := sortResults(GetAppearanceResults(tx, appearanceId))
	exported := make([]ExportResult, 0, len(results))
	for _, result := range results {
		var personName string
		if result.PersonId != nil {
			personName = personNames[*result.PersonId]
		}
		exported = append(exported, ExportResult{
			Id:         result.Id,
			Kind:       result.Kind,
			Label:      result.Label,
			Rank:       result.Rank,
			OutOf:      result.OutOf,
			Category:   result.Category,
			Score:      result.Score,
			PersonId:   result.PersonId,
			PersonName: personName,
			Notes:      result.Notes,
			SortOrder:  result.SortOrder,
			CreatedAt:  result.CreatedAt,
		})
	}
	return exported
}

// namesFor drops ids it cannot resolve rather than emitting blanks. The names
// are a reading aid; a roster naming somebody outside this family's own people
// still round-trips on its ids.
func namesFor(personIds []int, personNames map[int]string) []string {
	if len(personIds) == 0 {
		return nil
	}
	names := make([]string, 0, len(personIds))
	for _, personId := range personIds {
		if name, ok := personNames[personId]; ok {
			names = append(names, name)
		}
	}
	if len(names) == 0 {
		return nil
	}
	return names
}

// countExportedActivities counts every record the activities tree holds, so
// the bundle's totals say whether a season came through rather than only how
// many programs did.
func countExportedActivities(activities []ExportActivity) (seasons, events, entries, appearances, results int) {
	for _, activity := range activities {
		seasons += len(activity.Seasons)
		for _, season := range activity.Seasons {
			entries += len(season.Entries)
			events += len(season.Events)
			for _, event := range season.Events {
				appearances += len(event.Appearances)
				for _, appearance := range event.Appearances {
					results += len(appearance.Results)
				}
			}
		}
	}
	return
}
