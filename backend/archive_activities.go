package backend

import (
	"fmt"
	"strings"
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
	// MembersAddedAt is when each person, by PersonIds id, joined the roster.
	MembersAddedAt map[int]time.Time `json:"membersAddedAt,omitempty"`
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
	// PhotosAddedAt is when each photo, by PhotoIds id, was attached.
	PhotosAddedAt map[int]time.Time `json:"photosAddedAt,omitempty"`
}

type ExportAppearance struct {
	Id            int               `json:"id"`
	EntryId       int               `json:"entryId"`
	EntryName     string            `json:"entryName,omitempty"`
	OccurredAt    time.Time         `json:"occurredAt"`
	Notes         string            `json:"notes,omitempty"`
	CreatedAt     time.Time         `json:"createdAt"`
	Round         string            `json:"round,omitempty"`
	Opponent      string            `json:"opponent,omitempty"`
	Outcome       string            `json:"outcome,omitempty"`
	ScoreFor      *int              `json:"scoreFor,omitempty"`
	ScoreAgainst  *int              `json:"scoreAgainst,omitempty"`
	PhotoIds      []int             `json:"photoIds,omitempty"`
	PhotosAddedAt map[int]time.Time `json:"photosAddedAt,omitempty"`
	Results       []ExportResult    `json:"results,omitempty"`
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
			members := GetEntryMembers(tx, entry.Id)
			personIds := make([]int, 0, len(members))
			addedAt := map[int]time.Time{}
			for _, member := range members {
				personIds = append(personIds, member.PersonId)
				addedAt[member.PersonId] = member.CreatedAt
			}
			exportedEntries = append(exportedEntries, ExportEntry{
				Id:             entry.Id,
				Name:           entry.Name,
				Format:         entry.Format,
				Style:          entry.Style,
				Division:       entry.Division,
				Level:          entry.Level,
				Notes:          entry.Notes,
				CreatedAt:      entry.CreatedAt,
				PersonIds:      personIds,
				PersonNames:    namesFor(personIds, personNames),
				MembersAddedAt: addedAt,
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
		photoIds, addedAt := eventPhotoLinks(GetEventPhotoJoins(tx, event.Id))
		exported = append(exported, ExportEvent{
			Id:            event.Id,
			Name:          event.Name,
			Host:          event.Host,
			Location:      event.Location,
			StartDate:     event.StartDate,
			EndDate:       event.EndDate,
			Notes:         event.Notes,
			CreatedAt:     event.CreatedAt,
			PhotoIds:      photoIds,
			PhotosAddedAt: addedAt,
			Appearances:   exportAppearances(tx, event.Id, entryNames, personNames),
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
		photoIds, addedAt := appearancePhotoLinks(GetAppearancePhotoJoins(tx, appearance.Id))
		exported = append(exported, ExportAppearance{
			Id:            appearance.Id,
			EntryId:       appearance.EntryId,
			EntryName:     entryNames[appearance.EntryId],
			OccurredAt:    appearance.OccurredAt,
			Notes:         appearance.Notes,
			CreatedAt:     appearance.CreatedAt,
			Round:         appearance.Round,
			Opponent:      appearance.Opponent,
			Outcome:       appearance.Outcome,
			ScoreFor:      appearance.ScoreFor,
			ScoreAgainst:  appearance.ScoreAgainst,
			PhotoIds:      photoIds,
			PhotosAddedAt: addedAt,
			Results:       exportResults(tx, appearance.Id, personNames),
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

func eventPhotoLinks(joins []EventPhoto) (photoIds []int, addedAt map[int]time.Time) {
	addedAt = map[int]time.Time{}
	for _, join := range joins {
		photoIds = append(photoIds, join.PhotoId)
		addedAt[join.PhotoId] = join.CreatedAt
	}
	return
}

func appearancePhotoLinks(joins []AppearancePhoto) (photoIds []int, addedAt map[int]time.Time) {
	addedAt = map[int]time.Time{}
	for _, join := range joins {
		photoIds = append(photoIds, join.PhotoId)
		addedAt[join.PhotoId] = join.CreatedAt
	}
	return
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
	l *archiveLedger,
) (ActivityImportCounts, []string) {
	var counts ActivityImportCounts
	var warnings []string

	for _, source := range activities {
		name := trimField(source.Name, maxNameLength)
		if name == "" {
			counts.Skipped++
			warnings = append(warnings, "Skipped an activity with no name")
			continue
		}

		activity, reused := findOrCreateActivity(tx, familyId, name, source.Kind, archiveCreatedAt(source.CreatedAt))
		if reused {
			counts.Reused++
		} else {
			counts.Activities++
		}
		l.activities[source.Id] = activity.Id

		for _, sourceSeason := range source.Seasons {
			importSeason(tx, importSeasonArgs{
				season:     sourceSeason,
				activityId: activity.Id,
				familyId:   familyId,
				ledger:     l,
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
	createdAt time.Time,
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
		CreatedAt: createdAt,
	}
	writeActivityTx(tx, &activity)
	return activity, false
}

type importSeasonArgs struct {
	season     ExportSeason
	activityId int
	familyId   int
	ledger     *archiveLedger
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
	args.ledger.seasons[args.season.Id] = season.Id

	entryRosters := make(map[int]map[int]bool, len(args.season.Entries))
	for _, sourceEntry := range args.season.Entries {
		entryName := trimField(sourceEntry.Name, maxNameLength)
		if entryName == "" {
			counts.Skipped++
			*warnings = append(*warnings, "Skipped an entry with no name")
			continue
		}

		entry, entryReused := findOrCreateEntry(tx, season.Id, args.familyId, entryName, sourceEntry)
		if entryReused {
			counts.Reused++
		} else {
			counts.Entries++
		}
		args.ledger.entries[sourceEntry.Id] = entry.Id
		entryRosters[entry.Id] = importRoster(tx, entry, sourceEntry, args.ledger.people, entryReused, warnings)
	}

	for _, sourceEvent := range args.season.Events {
		importEvent(tx, importEventArgs{
			event:        sourceEvent,
			seasonId:     season.Id,
			familyId:     args.familyId,
			entryRosters: entryRosters,
			ledger:       args.ledger,
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
		CreatedAt:  archiveCreatedAt(args.season.CreatedAt),
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
		CreatedAt: archiveCreatedAt(source.CreatedAt),
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
			CreatedAt: archiveCreatedAt(source.MembersAddedAt[oldPersonId]),
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
	event        ExportEvent
	seasonId     int
	familyId     int
	entryRosters map[int]map[int]bool
	ledger       *archiveLedger
}

func importEvent(tx *vbolt.Tx, args importEventArgs, counts *ActivityImportCounts, warnings *[]string) {
	name := trimField(args.event.Name, maxNameLength)
	if name == "" {
		counts.Skipped++
		*warnings = append(*warnings, "Skipped an event with no name")
		return
	}

	event, reused := findOrCreateEvent(tx, args, name)
	args.ledger.events[args.event.Id] = event.Id
	if reused {
		counts.Reused++
	} else {
		counts.Events++
		attachPhotos(args.ledger.photos, args.event.PhotoIds, func(oldPhotoId, photoId int) {
			join := EventPhoto{
				Id:        vbolt.NextIntId(tx, EventPhotoBkt),
				EventId:   event.Id,
				PhotoId:   photoId,
				FamilyId:  args.familyId,
				CreatedAt: archiveCreatedAt(args.event.PhotosAddedAt[oldPhotoId]),
			}
			writeEventPhotoTx(tx, &join)
		})
	}

	for _, sourceAppearance := range args.event.Appearances {
		entryId := args.ledger.entries[sourceAppearance.EntryId]
		if _, inSeason := args.entryRosters[entryId]; !inSeason {
			counts.Skipped++
			*warnings = append(*warnings, fmt.Sprintf(
				"%q: a performance named an entry that is not in this bundle's season", name))
			continue
		}

		appearance := Appearance{
			EventId:    event.Id,
			EntryId:    entryId,
			FamilyId:   args.familyId,
			OccurredAt: sourceAppearance.OccurredAt,
			Notes:      trimField(sourceAppearance.Notes, maxNotesLength),
			CreatedAt:  archiveCreatedAt(sourceAppearance.CreatedAt),
		}
		matchup := MatchupInput{
			Round:        sourceAppearance.Round,
			Opponent:     sourceAppearance.Opponent,
			Outcome:      sourceAppearance.Outcome,
			ScoreFor:     sourceAppearance.ScoreFor,
			ScoreAgainst: sourceAppearance.ScoreAgainst,
		}
		if err := applyMatchup(&appearance, &matchup); err != nil {
			matchup.Outcome, matchup.ScoreFor, matchup.ScoreAgainst = "", nil, nil
			_ = applyMatchup(&appearance, &matchup)
			*warnings = append(*warnings, fmt.Sprintf(
				"%q: dropped a performance's outcome or score: %v", name, err))
		}

		if appearanceExists(tx, appearance) {
			counts.Reused++
			continue
		}

		appearance.Id = vbolt.NextIntId(tx, AppearanceBkt)
		writeAppearanceTx(tx, &appearance)
		counts.Appearances++

		attachPhotos(args.ledger.photos, sourceAppearance.PhotoIds, func(oldPhotoId, photoId int) {
			join := AppearancePhoto{
				Id:           vbolt.NextIntId(tx, AppearancePhotoBkt),
				AppearanceId: appearance.Id,
				PhotoId:      photoId,
				FamilyId:     args.familyId,
				CreatedAt:    archiveCreatedAt(sourceAppearance.PhotosAddedAt[oldPhotoId]),
			}
			writeAppearancePhotoTx(tx, &join)
		})

		importResults(tx, appearance, sourceAppearance.Results,
			args.ledger.people, args.entryRosters[entryId], counts, warnings)
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
		CreatedAt: archiveCreatedAt(args.event.CreatedAt),
	}
	writeEventTx(tx, &event)
	return event, false
}

func appearanceExists(tx *vbolt.Tx, candidate Appearance) bool {
	for _, existing := range GetEventAppearances(tx, candidate.EventId) {
		if existing.EntryId == candidate.EntryId &&
			existing.OccurredAt.Equal(candidate.OccurredAt) &&
			strings.EqualFold(existing.Round, candidate.Round) &&
			strings.EqualFold(existing.Opponent, candidate.Opponent) {
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
			CreatedAt:    archiveCreatedAt(source.CreatedAt),
		}
		writeResultTx(tx, &result)
		counts.Results++
	}
}

func attachPhotos(photoIdMapping map[int]int, photoIds []int, write func(oldPhotoId, photoId int)) {
	if len(photoIdMapping) == 0 {
		return
	}
	for _, oldPhotoId := range photoIds {
		if newPhotoId, ok := photoIdMapping[oldPhotoId]; ok {
			write(oldPhotoId, newPhotoId)
		}
	}
}
