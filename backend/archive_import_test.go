package backend

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"go.hasen.dev/vbeam"
)

func (fx *archiveFixture) exportRecords(t *testing.T, user User) string {
	t.Helper()
	var resp ExportDataResponse
	var err error
	fx.call(t, user, func(ctx *vbeam.Context) { resp, err = ExportData(ctx, ExportDataRequest{}) })
	if err != nil {
		t.Fatal(err)
	}
	return resp.JsonData
}

func (fx *archiveFixture) importRecords(t *testing.T, user User, req ImportDataRequest) (resp ImportDataResponse, err error) {
	t.Helper()
	fx.call(t, user, func(ctx *vbeam.Context) { resp, err = ImportData(ctx, req) })
	return
}

func bundleRecords(t *testing.T, bundle []byte) []byte {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(bundle), int64(len(bundle)))
	if err != nil {
		t.Fatal(err)
	}
	for _, zf := range zr.File {
		if zf.Name == "data.json" {
			rc, _ := zf.Open()
			defer rc.Close()
			data, _ := io.ReadAll(rc)
			return data
		}
	}
	t.Fatal("bundle has no data.json")
	return nil
}

// Each import stamps its own family name and creation times.
var perImportFields = []archiveDrop{
	{"families", "", nil, "each restorer has its own family"},
	{"*", "CreatedAt", nil, "stamped at import"},
	{"books", "UpdatedAt", nil, "stamped at import"},
	{"books", "ReviewedAt", nil, "stamped at import"},
}

func TestRecordsAndZipImportsGiveTheSameFamily(t *testing.T) {
	fx := setupArchiveFixture(t)
	records := fx.exportRecords(t, fx.owner)
	bundle := fx.exportBundle(t, fx.owner, "data_only")

	fromRecords := fx.newUser(t, "records")
	if _, err := fx.importRecords(t, fromRecords, ImportDataRequest{JsonData: records, ImportMilestones: true, ImportActivities: true}); err != nil {
		t.Fatal(err)
	}
	fromZip := fx.newUser(t, "zip")
	fx.importBundle(t, fromZip, bundle, importEverything)

	a := readArchiveContents(t, fx.db, fromRecords.FamilyId).without(perImportFields)
	b := readArchiveContents(t, fx.db, fromZip.FamilyId).without(perImportFields)
	if len(a["people"]) == 0 || len(a["milestones"]) == 0 {
		t.Fatal("the records import brought nothing across")
	}
	for _, d := range diffArchiveContents(a, b) {
		t.Errorf("zip and records imports differ: %s", d)
	}
}

func TestZipImportHonoursTheImportOptions(t *testing.T) {
	fx := setupArchiveFixture(t)
	bundle := fx.exportBundle(t, fx.owner, "with_photos")
	var records ImportDataStructure
	if err := json.Unmarshal(bundleRecords(t, bundle), &records); err != nil {
		t.Fatal(err)
	}
	chosen := records.People[0]

	restorer := fx.newUser(t, "restorer")
	resp := fx.importBundle(t, restorer, bundle, ImportDataRequest{FilterPersonIds: []int{chosen.Id}})
	if resp.ImportedPeople != 1 || resp.ImportedMilestones != 0 || resp.ImportedActivities.Activities != 0 {
		t.Errorf("imported %+v; want only %s, without moments or activities", resp, chosen.Name)
	}
	inScope := 0
	for _, photo := range records.Photos {
		if len(photo.PersonIds) == 0 || slices.Contains(photo.PersonIds, chosen.Id) {
			inScope++
		}
	}
	if resp.ImportedPhotos != inScope || inScope == len(records.Photos) {
		t.Errorf("imported %d of %d photos; want the %d of or without %s", resp.ImportedPhotos, len(records.Photos), inScope, chosen.Name)
	}
	restored := readArchiveContents(t, fx.db, restorer.FamilyId)
	if len(restored["people"]) != 1 || len(restored["milestones"]) != 0 || len(restored["activities"]) != 0 {
		t.Errorf("restored %d people, %d moments, %d activities; want 1, 0, 0",
			len(restored["people"]), len(restored["milestones"]), len(restored["activities"]))
	}
}

func TestZipImportPreviewAndDryRunWriteNothing(t *testing.T) {
	fx := setupArchiveFixture(t)
	bundle := fx.exportBundle(t, fx.owner, "with_photos")
	restorer := fx.newUser(t, "restorer")
	before := readArchiveContents(t, fx.db, restorer.FamilyId)
	photoDir := filepath.Dir(photoOriginalPath("photos/x.jpg"))
	filesBefore, _ := os.ReadDir(photoDir)

	preview := fx.importBundle(t, restorer, bundle, ImportDataRequest{PreviewOnly: true})
	if len(preview.MatchedPeople) == 0 || len(preview.AvailablePeople) != len(preview.MatchedPeople) {
		t.Errorf("preview listed %d people with %d matches", len(preview.AvailablePeople), len(preview.MatchedPeople))
	}

	dry := importEverything
	dry.DryRun = true
	dryResp := fx.importBundle(t, restorer, bundle, dry)
	for _, d := range diffArchiveContents(before, readArchiveContents(t, fx.db, restorer.FamilyId)) {
		t.Errorf("a preview or dry run changed the family: %s", d)
	}
	if filesAfter, _ := os.ReadDir(photoDir); len(filesAfter) != len(filesBefore) {
		t.Errorf("a dry run wrote %d photo files", len(filesAfter)-len(filesBefore))
	}

	real := fx.importBundle(t, restorer, bundle, importEverything)
	dryResp.PersonIdMapping, real.PersonIdMapping = nil, nil
	dryJSON, _ := json.Marshal(dryResp)
	realJSON, _ := json.Marshal(real)
	if real.ImportedPhotos == 0 || string(dryJSON) != string(realJSON) {
		t.Errorf("dry run reported\n%s\nbut the import reported\n%s", dryJSON, realJSON)
	}
}

func TestRecordsImportReportsTheMediaItLeftOut(t *testing.T) {
	fx := setupArchiveFixture(t)
	data := bundleRecords(t, fx.exportBundle(t, fx.owner, "with_photos"))
	var records ImportDataStructure
	if err := json.Unmarshal(data, &records); err != nil {
		t.Fatal(err)
	}
	if len(records.Photos) == 0 || len(records.Videos) == 0 {
		t.Fatalf("the seed exported %d photos and %d videos", len(records.Photos), len(records.Videos))
	}

	restorer := fx.newUser(t, "restorer")
	resp, err := fx.importRecords(t, restorer, ImportDataRequest{JsonData: string(data), ImportMilestones: true})
	if err != nil {
		t.Fatal(err)
	}
	if resp.ImportedPhotos != 0 || resp.SkippedPhotos != len(records.Photos) || resp.SkippedVideos != len(records.Videos) {
		t.Errorf("imported %d photos, skipped %d photos and %d videos; want 0, %d, %d",
			resp.ImportedPhotos, resp.SkippedPhotos, resp.SkippedVideos, len(records.Photos), len(records.Videos))
	}
	if !strings.Contains(strings.Join(resp.Warnings, "\n"), "files aren't in this file") {
		t.Errorf("no warning about the missing media in %v", resp.Warnings)
	}
	if n := len(readArchiveContents(t, fx.db, restorer.FamilyId)["images"]); n != 0 {
		t.Errorf("a records import created %d media records without files", n)
	}
}

func TestImportingRecordsTwiceChangesNothing(t *testing.T) {
	fx := setupArchiveFixture(t)
	records := fx.exportRecords(t, fx.owner)
	restorer := fx.newUser(t, "restorer")
	req := ImportDataRequest{JsonData: records, ImportMilestones: true, ImportActivities: true}

	first, err := fx.importRecords(t, restorer, req)
	if err != nil {
		t.Fatal(err)
	}
	once := readArchiveContents(t, fx.db, restorer.FamilyId)
	again, err := fx.importRecords(t, restorer, req)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range diffArchiveContents(once, readArchiveContents(t, fx.db, restorer.FamilyId)) {
		t.Errorf("the second import changed the family: %s", d)
	}
	if again.ImportedPeople != 0 || again.MergedPeople != first.ImportedPeople ||
		again.ImportedMeasurements != 0 || again.SkippedMeasurements != first.ImportedMeasurements ||
		again.ImportedMilestones != 0 || again.SkippedMilestones != first.ImportedMilestones {
		t.Errorf("second import reported %+v after a first of %+v", again, first)
	}
}

func TestImportRejectsFilesItCannotRead(t *testing.T) {
	fx := setupArchiveFixture(t)
	var records map[string]any
	if err := json.Unmarshal([]byte(fx.exportRecords(t, fx.owner)), &records); err != nil {
		t.Fatal(err)
	}
	records["formatVersion"] = archiveFormatVersion + 1
	newer, _ := json.Marshal(records)

	restorer := fx.newUser(t, "restorer")
	for name, data := range map[string]string{"newer format": string(newer), "not json": "not a backup"} {
		if _, err := fx.importRecords(t, restorer, ImportDataRequest{JsonData: data}); err == nil {
			t.Errorf("%s: imported without an error", name)
		}
	}
	if n := len(readArchiveContents(t, fx.db, restorer.FamilyId)["people"]); n != 0 {
		t.Errorf("a rejected file left %d people", n)
	}
}

func TestZipImportCannotReachAnotherFamily(t *testing.T) {
	fx := setupArchiveFixture(t)
	bundle := fx.exportBundle(t, fx.owner, "data_only")
	outsider := fx.newUser(t, "outsider")
	before := readArchiveContents(t, fx.db, fx.familyId)

	rec := postImportBundle(t, outsider, bundle, ImportDataRequest{FamilyId: fx.familyId, ImportMilestones: true})
	if rec.Code == http.StatusOK {
		t.Fatalf("an outsider imported into another family: %s", rec.Body.String())
	}
	for _, d := range diffArchiveContents(before, readArchiveContents(t, fx.db, fx.familyId)) {
		t.Errorf("the refused import changed the family: %s", d)
	}
}

func TestBooksFindMomentsAlreadyInTheFamily(t *testing.T) {
	fx := setupArchiveFixture(t)
	records := fx.exportRecords(t, fx.owner)
	var withoutBooks map[string]any
	if err := json.Unmarshal([]byte(records), &withoutBooks); err != nil {
		t.Fatal(err)
	}
	delete(withoutBooks, "books")
	momentsOnly, _ := json.Marshal(withoutBooks)

	restorer := fx.newUser(t, "restorer")
	if _, err := fx.importRecords(t, restorer, ImportDataRequest{JsonData: string(momentsOnly), ImportMilestones: true}); err != nil {
		t.Fatal(err)
	}
	resp, err := fx.importRecords(t, restorer, ImportDataRequest{JsonData: records})
	if err != nil {
		t.Fatal(err)
	}
	if resp.ImportedBooks == 0 {
		t.Fatal("no books imported")
	}
	source := readArchiveContents(t, fx.db, fx.familyId)["books"]
	restored := readArchiveContents(t, fx.db, restorer.FamilyId)["books"]
	want := source[0]["Items.0.SourceId"]
	if len(restored) != 1 || want == "" || restored[0]["Items.0.SourceId"] != want {
		t.Errorf("restored books %v; want their first item to be the moment %s", restored, want)
	}
}

func TestFilteredOutMediaIsNotReportedAsSkipped(t *testing.T) {
	fx := setupArchiveFixture(t)
	bundle := fx.exportBundle(t, fx.owner, "with_photos")
	data := bundleRecords(t, bundle)
	var records ImportDataStructure
	if err := json.Unmarshal(data, &records); err != nil {
		t.Fatal(err)
	}
	chosen := records.People[0]
	wantPhotos, wantVideos := 0, 0
	for _, photo := range records.Photos {
		if len(photo.PersonIds) == 0 || slices.Contains(photo.PersonIds, chosen.Id) {
			wantPhotos++
		}
	}
	for _, video := range records.Videos {
		if len(video.PersonIds) == 0 || slices.Contains(video.PersonIds, chosen.Id) {
			wantVideos++
		}
	}
	if wantPhotos == len(records.Photos) && wantVideos == len(records.Videos) {
		t.Fatalf("the filter on %s leaves out no media", chosen.Name)
	}
	req := ImportDataRequest{FilterPersonIds: []int{chosen.Id}}

	fromZip := fx.importBundle(t, fx.newUser(t, "zip"), bundle, req)
	if fromZip.SkippedPhotos != 0 || fromZip.SkippedVideos != wantVideos {
		t.Errorf("zip skipped %d photos and %d videos; want 0 and %d", fromZip.SkippedPhotos, fromZip.SkippedVideos, wantVideos)
	}

	req.JsonData = string(data)
	fromRecords, err := fx.importRecords(t, fx.newUser(t, "records"), req)
	if err != nil {
		t.Fatal(err)
	}
	if fromRecords.SkippedPhotos != wantPhotos || fromRecords.SkippedVideos != wantVideos {
		t.Errorf("records skipped %d photos and %d videos; want %d and %d",
			fromRecords.SkippedPhotos, fromRecords.SkippedVideos, wantPhotos, wantVideos)
	}
}

func TestReimportMatchesPeopleWhoseBirthdayIsNotAtMidnight(t *testing.T) {
	fx := setupArchiveFixture(t)
	var records ImportDataStructure
	if err := json.Unmarshal([]byte(fx.exportRecords(t, fx.owner)), &records); err != nil {
		t.Fatal(err)
	}
	for i := range records.People {
		records.People[i].Birthday = records.People[i].Birthday.Add(15 * time.Hour)
	}
	data, err := json.Marshal(records)
	if err != nil {
		t.Fatal(err)
	}
	restorer := fx.newUser(t, "restorer")
	req := ImportDataRequest{JsonData: string(data)}
	first, err := fx.importRecords(t, restorer, req)
	if err != nil {
		t.Fatal(err)
	}
	again, err := fx.importRecords(t, restorer, req)
	if err != nil {
		t.Fatal(err)
	}
	if again.ImportedPeople != 0 || again.MergedPeople != first.ImportedPeople {
		t.Errorf("second import added %d and merged %d people; want 0 and %d", again.ImportedPeople, again.MergedPeople, first.ImportedPeople)
	}
}

func (fx *archiveFixture) listPeople(t *testing.T, user User) map[string]Person {
	t.Helper()
	var resp ListPeopleResponse
	var err error
	fx.call(t, user, func(ctx *vbeam.Context) { resp, err = ListPeople(ctx, Empty{}) })
	if err != nil {
		t.Fatal(err)
	}
	people := map[string]Person{}
	for _, p := range resp.People {
		people[p.Name] = p
	}
	return people
}

// withFutureBirthday moves one person's birthday a few months ahead and
// returns the file with formatVersion set to version (0 leaves it out).
func withFutureBirthday(t *testing.T, data []byte, name string, version int) string {
	t.Helper()
	var file map[string]any
	if err := json.Unmarshal(data, &file); err != nil {
		t.Fatal(err)
	}
	due := time.Now().AddDate(0, 4, 0).UTC().Truncate(24 * time.Hour)
	found := false
	for _, p := range file["people"].([]any) {
		person := p.(map[string]any)
		if person["Name"] == name {
			person["Birthday"] = due.Format(time.RFC3339)
			delete(person, "IsPregnancy")
			found = true
		}
	}
	if !found {
		t.Fatalf("no person %q in the file", name)
	}
	delete(file, "formatVersion")
	if version > 0 {
		file["formatVersion"] = version
	}
	out, err := json.Marshal(file)
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

func TestLegacyFileWithAFutureBirthdayImportsItAsAPregnancy(t *testing.T) {
	fx := setupArchiveFixture(t)
	data, err := os.ReadFile("testdata/archive_v1.json")
	if err != nil {
		t.Fatal(err)
	}
	restorer := fx.newUser(t, "restorer")
	resp, err := fx.importRecords(t, restorer, ImportDataRequest{
		JsonData: withFutureBirthday(t, data, "Bump", 0), ImportMilestones: true, ImportActivities: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.ImportedPeople != 3 || resp.ImportedMeasurements != 2 {
		t.Errorf("imported %d people and %d measurements; want 3 and 2", resp.ImportedPeople, resp.ImportedMeasurements)
	}
	if bump, ok := fx.listPeople(t, restorer)["Bump"]; !ok || !bump.IsPregnancy {
		t.Errorf("Bump = %+v; want a pregnancy", bump)
	}
	if !slices.ContainsFunc(resp.Warnings, func(w string) bool { return strings.Contains(w, "Bump") && strings.Contains(w, "pregnancy") }) {
		t.Errorf("warnings %q don't say Bump was imported as a pregnancy", resp.Warnings)
	}
}

func TestFutureBirthdayOnSomeoneNotExpectedIsRejected(t *testing.T) {
	fx := setupArchiveFixture(t)
	restorer := fx.newUser(t, "restorer")
	_, err := fx.importRecords(t, restorer, ImportDataRequest{
		JsonData: withFutureBirthday(t, []byte(fx.exportRecords(t, fx.owner)), "Ben", archiveFormatVersion),
	})
	if err == nil || !strings.Contains(err.Error(), "Ben") {
		t.Errorf("import err = %v; want Ben's future birthday rejected", err)
	}
	if people := fx.listPeople(t, restorer); len(people) != 0 {
		t.Errorf("rejected import still added %d people", len(people))
	}
}
