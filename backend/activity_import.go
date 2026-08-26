package backend

import (
	"fmt"
	"strings"
	"time"

	"go.hasen.dev/vbolt"
)

type ActivityImportCounts struct {
	Activities  int `json:"activities"`
	Seasons     int `json:"seasons"`
	Events      int `json:"events"`
	Entries     int `json:"entries"`
	Appearances int `json:"appearances"`
	Results     int `json:"results"`
	Reused      int `json:"reused"`
	Skipped     int `json:"skipped"`
}

func importActivities(
	tx *vbolt.Tx,
	activities []ExportActivity,
	familyId int,
	personIdMapping map[int]int,
	photoIdMapping map[int]int,
) (ActivityImportCounts, []string) {
	var counts ActivityImportCounts
	var warnings []string
	now := time.Now()

	for _, source := range activities {
		name := trimField(source.Name, maxNameLength)
		if name == "" {
			counts.Skipped++
			warnings = append(warnings, "Skipped an activity with no name")
			continue
		}

		activity, reused := findOrCreateActivity(tx, familyId, name, source.Kind, now)
		if reused {
			counts.Reused++
		} else {
			counts.Activities++
		}

		for _, sourceSeason := range source.Seasons {
			importSeason(tx, importSeasonArgs{
				season:          sourceSeason,
				activityId:      activity.Id,
				familyId:        familyId,
				personIdMapping: personIdMapping,
				photoIdMapping:  photoIdMapping,
				now:             now,
			}, &counts, &warnings)
		}
	}

	return counts, warnings
}

func findOrCreateActivity(
	tx *vbolt.Tx,
	familyId int,
	name string,
	kind string,
	now time.Time,
) (Activity, bool) {
	for _, existing := range GetFamilyActivities(tx, familyId) {
		if strings.EqualFold(existing.Name, name) {
			return existing, true
		}
	}
	activity := Activity{
		Id:        vbolt.NextIntId(tx, ActivityBkt),
		FamilyId:  familyId,
		Name:      name,
		Kind:      normalizeActivityKind(kind),
		CreatedAt: now,
	}
	writeActivityTx(tx, &activity)
	return activity, false
}

type importSeasonArgs struct {
	season          ExportSeason
	activityId      int
	familyId        int
	personIdMapping map[int]int
	photoIdMapping  map[int]int
	now             time.Time
}

func importSeason(tx *vbolt.Tx, args importSeasonArgs, counts *ActivityImportCounts, warnings *[]string) {
	name := trimField(args.season.Name, maxNameLength)
	if name == "" {
		counts.Skipped++
		*warnings = append(*warnings, "Skipped a season with no name")
		return
	}

	season, reused := findOrCreateSeason(tx, args, name)
	if reused {
		counts.Reused++
	} else {
		counts.Seasons++
	}

	entryIdMapping := make(map[int]int, len(args.season.Entries))
	entryRosters := make(map[int]map[int]bool, len(args.season.Entries))
	for _, sourceEntry := range args.season.Entries {
		entryName := trimField(sourceEntry.Name, maxNameLength)
		if entryName == "" {
			counts.Skipped++
			*warnings = append(*warnings, "Skipped an entry with no name")
			continue
		}

		entry, entryReused := findOrCreateEntry(tx, season.Id, args.familyId, entryName, sourceEntry, args.now)
		if entryReused {
			counts.Reused++
		} else {
			counts.Entries++
		}
		entryIdMapping[sourceEntry.Id] = entry.Id
		entryRosters[entry.Id] = importRoster(tx, entry, sourceEntry, args.personIdMapping, entryReused, args.now, warnings)
	}

	for _, sourceEvent := range args.season.Events {
		importEvent(tx, importEventArgs{
			event:           sourceEvent,
			seasonId:        season.Id,
			familyId:        args.familyId,
			entryIdMapping:  entryIdMapping,
			entryRosters:    entryRosters,
			personIdMapping: args.personIdMapping,
			photoIdMapping:  args.photoIdMapping,
			now:             args.now,
		}, counts, warnings)
	}
}

func findOrCreateSeason(tx *vbolt.Tx, args importSeasonArgs, name string) (Season, bool) {
	for _, existing := range GetActivitySeasons(tx, args.activityId) {
		if strings.EqualFold(existing.Name, name) {
			return existing, true
		}
	}
	season := Season{
		Id:         vbolt.NextIntId(tx, SeasonBkt),
		ActivityId: args.activityId,
		FamilyId:   args.familyId,
		Name:       name,
		StartDate:  args.season.StartDate,
		EndDate:    args.season.EndDate,
		Notes:      trimField(args.season.Notes, maxNotesLength),
		CreatedAt:  args.now,
	}
	writeSeasonTx(tx, &season)
	return season, false
}

func findOrCreateEntry(
	tx *vbolt.Tx,
	seasonId int,
	familyId int,
	name string,
	source ExportEntry,
	now time.Time,
) (Entry, bool) {
	for _, existing := range GetSeasonEntries(tx, seasonId) {
		if strings.EqualFold(existing.Name, name) {
			return existing, true
		}
	}
	entry := Entry{
		Id:        vbolt.NextIntId(tx, EntryBkt),
		SeasonId:  seasonId,
		FamilyId:  familyId,
		Name:      name,
		Format:    trimField(source.Format, maxLabelLength),
		Style:     trimField(source.Style, maxLabelLength),
		Division:  trimField(source.Division, maxLabelLength),
		Level:     trimField(source.Level, maxLabelLength),
		Notes:     trimField(source.Notes, maxNotesLength),
		CreatedAt: now,
	}
	writeEntryTx(tx, &entry)
	return entry, false
}

func importRoster(
	tx *vbolt.Tx,
	entry Entry,
	source ExportEntry,
	personIdMapping map[int]int,
	reused bool,
	now time.Time,
	warnings *[]string,
) map[int]bool {
	if reused {
		roster := make(map[int]bool)
		for _, personId := range GetEntryPersonIds(tx, entry.Id) {
			roster[personId] = true
		}
		return roster
	}

	roster := make(map[int]bool, len(source.PersonIds))
	var dropped int
	for _, oldPersonId := range source.PersonIds {
		newPersonId, ok := personIdMapping[oldPersonId]
		if !ok {
			dropped++
			continue
		}
		if roster[newPersonId] {
			continue
		}
		roster[newPersonId] = true
		member := EntryMember{
			Id:        vbolt.NextIntId(tx, EntryMemberBkt),
			EntryId:   entry.Id,
			PersonId:  newPersonId,
			FamilyId:  entry.FamilyId,
			CreatedAt: now,
		}
		writeEntryMemberTx(tx, &member)
	}

	if dropped > 0 {
		*warnings = append(*warnings, fmt.Sprintf(
			"%q: %d person(s) on the roster were not imported and were left off",
			entry.Name, dropped))
	}
	return roster
}

type importEventArgs struct {
	event           ExportEvent
	seasonId        int
	familyId        int
	entryIdMapping  map[int]int
	entryRosters    map[int]map[int]bool
	personIdMapping map[int]int
	photoIdMapping  map[int]int
	now             time.Time
}

func importEvent(tx *vbolt.Tx, args importEventArgs, counts *ActivityImportCounts, warnings *[]string) {
	name := trimField(args.event.Name, maxNameLength)
	if name == "" {
		counts.Skipped++
		*warnings = append(*warnings, "Skipped an event with no name")
		return
	}

	event, reused := findOrCreateEvent(tx, args, name)
	if reused {
		counts.Reused++
	} else {
		counts.Events++
		attachPhotos(args.photoIdMapping, args.event.PhotoIds, func(photoId int) {
			join := EventPhoto{
				Id:        vbolt.NextIntId(tx, EventPhotoBkt),
				EventId:   event.Id,
				PhotoId:   photoId,
				FamilyId:  args.familyId,
				CreatedAt: args.now,
			}
			writeEventPhotoTx(tx, &join)
		})
	}

	for _, sourceAppearance := range args.event.Appearances {
		entryId, ok := args.entryIdMapping[sourceAppearance.EntryId]
		if !ok {
			counts.Skipped++
			*warnings = append(*warnings, fmt.Sprintf(
				"%q: a performance named an entry that is not in this bundle's season", name))
			continue
		}

		if appearanceExists(tx, event.Id, entryId) {
			counts.Reused++
			continue
		}

		appearance := Appearance{
			Id:         vbolt.NextIntId(tx, AppearanceBkt),
			EventId:    event.Id,
			EntryId:    entryId,
			FamilyId:   args.familyId,
			OccurredAt: sourceAppearance.OccurredAt,
			Notes:      trimField(sourceAppearance.Notes, maxNotesLength),
			CreatedAt:  args.now,
		}
		writeAppearanceTx(tx, &appearance)
		counts.Appearances++

		attachPhotos(args.photoIdMapping, sourceAppearance.PhotoIds, func(photoId int) {
			join := AppearancePhoto{
				Id:           vbolt.NextIntId(tx, AppearancePhotoBkt),
				AppearanceId: appearance.Id,
				PhotoId:      photoId,
				FamilyId:     args.familyId,
				CreatedAt:    args.now,
			}
			writeAppearancePhotoTx(tx, &join)
		})

		importResults(tx, appearance, sourceAppearance.Results,
			args.personIdMapping, args.entryRosters[entryId], args.now, counts, warnings)
	}
}

func findOrCreateEvent(tx *vbolt.Tx, args importEventArgs, name string) (Event, bool) {
	for _, existing := range GetSeasonEvents(tx, args.seasonId) {
		if strings.EqualFold(existing.Name, name) {
			return existing, true
		}
	}
	event := Event{
		Id:        vbolt.NextIntId(tx, EventBkt),
		SeasonId:  args.seasonId,
		FamilyId:  args.familyId,
		Name:      name,
		Host:      trimField(args.event.Host, maxLabelLength),
		Location:  trimField(args.event.Location, maxLabelLength),
		StartDate: args.event.StartDate,
		EndDate:   args.event.EndDate,
		Notes:     trimField(args.event.Notes, maxNotesLength),
		CreatedAt: args.now,
	}
	writeEventTx(tx, &event)
	return event, false
}

func appearanceExists(tx *vbolt.Tx, eventId, entryId int) bool {
	for _, existing := range GetEventAppearances(tx, eventId) {
		if existing.EntryId == entryId {
			return true
		}
	}
	return false
}

func importResults(
	tx *vbolt.Tx,
	appearance Appearance,
	sources []ExportResult,
	personIdMapping map[int]int,
	roster map[int]bool,
	now time.Time,
	counts *ActivityImportCounts,
	warnings *[]string,
) {
	if len(sources) > maxResultsPerAppearance {
		sources = sources[:maxResultsPerAppearance]
		*warnings = append(*warnings, fmt.Sprintf(
			"A performance carried more than %d results; the rest were dropped",
			maxResultsPerAppearance))
	}

	for i, source := range sources {
		kind, ok := normalizeResultKind(source.Kind)
		if !ok {
			counts.Skipped++
			*warnings = append(*warnings, fmt.Sprintf(
				"Dropped a result of unknown kind %q", source.Kind))
			continue
		}

		var personId *int
		if source.PersonId != nil {
			newPersonId, mapped := personIdMapping[*source.PersonId]
			if mapped && roster[newPersonId] {
				personId = &newPersonId
			} else {
				*warnings = append(*warnings,
					"A result named someone not on the imported roster; kept the result without them")
			}
		}

		result := Result{
			Id:           vbolt.NextIntId(tx, ResultBkt),
			AppearanceId: appearance.Id,
			FamilyId:     appearance.FamilyId,
			Kind:         kind,
			Label:        trimField(source.Label, maxLabelLength),
			Rank:         source.Rank,
			OutOf:        source.OutOf,
			Category:     trimField(source.Category, maxLabelLength),
			Score:        source.Score,
			PersonId:     personId,
			Notes:        trimField(source.Notes, maxNotesLength),
			SortOrder:    i,
			CreatedAt:    now,
		}
		writeResultTx(tx, &result)
		counts.Results++
	}
}

func attachPhotos(photoIdMapping map[int]int, photoIds []int, write func(int)) {
	if len(photoIdMapping) == 0 {
		return
	}
	for _, oldPhotoId := range photoIds {
		if newPhotoId, ok := photoIdMapping[oldPhotoId]; ok {
			write(newPhotoId)
		}
	}
}
