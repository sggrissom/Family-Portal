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
