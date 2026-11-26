import { CircleCheck, CircleX, ClipboardList, Target, TriangleAlert } from "lucide-preact";
import { Icon } from "../../components/Icon";
import * as preact from "preact";
import * as vlens from "vlens";
import * as rpc from "vlens/rpc";
import * as auth from "../../lib/authCache";
import * as server from "../../server";
import { Header, Footer } from "../../layout";
import { requireAuthInView } from "../../lib/authHelpers";
import { FamilySelect } from "../../components/FamilySelect";
import "./import-styles";

type Data = {};

type ImportForm = {
  jsonData: string;
  file: File | null;
  isZip: boolean;
  error: string;
  loading: boolean;
  success: boolean;
  result: server.ImportDataResponse | null;
  previewData: server.ImportDataResponse | null;
  showFilters: boolean;
  selectedFamilyIds: number[];
  selectedPersonIds: number[];
  importMilestones: boolean;
  importActivities: boolean;
  dryRun: boolean;
  targetFamilyId: number;
  archiveImportId: number;
  archiveFamilyId: number;
  uploadPercent: number;
  job: server.ArchiveImportStatus | null;
};

const useImportForm = vlens.declareHook(
  (): ImportForm => ({
    jsonData: "",
    file: null,
    isZip: false,
    error: "",
    loading: false,
    success: false,
    result: null,
    previewData: null,
    showFilters: false,
    selectedFamilyIds: [],
    selectedPersonIds: [],
    importMilestones: true,
    importActivities: true,
    dryRun: false,
    targetFamilyId: 0,
    archiveImportId: 0,
    archiveFamilyId: 0,
    uploadPercent: 0,
    job: null,
  })
);

const jobStateLabels: Record<string, string> = {
  uploading: "Uploading",
  queued: "Waiting to start",
  staging: "Reading the archive",
  importing: "Importing",
  placing: "Putting photos and videos in place",
  done: "Finished",
  failed: "Failed",
};

const putArchive = (form: ImportForm, file: File, upload: server.UploadInstructions) =>
  new Promise<string>(resolve => {
    const xhr = new XMLHttpRequest();
    xhr.open(upload.method, upload.url);
    for (const [name, value] of Object.entries(upload.headers || {})) {
      xhr.setRequestHeader(name, value);
    }
    xhr.upload.onprogress = e => {
      if (e.lengthComputable) {
        form.uploadPercent = Math.round((e.loaded / e.total) * 100);
        vlens.scheduleRedraw();
      }
    };
    xhr.onload = () =>
      resolve(xhr.status >= 200 && xhr.status < 300 ? "" : "Upload failed: " + xhr.responseText);
    xhr.onerror = () => resolve("The upload did not finish. Check your connection and try again.");
    xhr.send(file);
  });

const sleep = (ms: number) => new Promise(resolve => setTimeout(resolve, ms));

// A chosen file, an Everything archive or a Records file, uploads once,
// straight to storage; each preview, dry run or import is then a background
// job on that upload.
const runArchiveJob = async (
  form: ImportForm,
  options: server.ImportDataRequest
): Promise<[server.ImportDataResponse | null, string]> => {
  const file = form.file!;
  if (!form.archiveImportId || form.archiveFamilyId !== form.targetFamilyId) {
    form.uploadPercent = 0;
    form.job = null;
    const [begun, err] = await server.BeginArchiveImport({
      familyId: form.targetFamilyId,
      filename: file.name,
      size: file.size,
    });
    if (!begun) {
      return [null, err];
    }
    form.job = { state: "uploading" } as server.ArchiveImportStatus;
    vlens.scheduleRedraw();
    const putErr = await putArchive(form, file, begun.upload);
    if (putErr) {
      return [null, putErr];
    }
    form.archiveImportId = begun.importId;
    form.archiveFamilyId = form.targetFamilyId;
  }

  form.job = { state: "queued" } as server.ArchiveImportStatus;
  vlens.scheduleRedraw();
  let [status, err] = await server.CompleteArchiveImport({
    importId: form.archiveImportId,
    options,
  });
  while (status && status.state !== "done" && status.state !== "failed") {
    form.job = status;
    vlens.scheduleRedraw();
    await sleep(1000);
    [status, err] = await server.GetArchiveImportStatus({ importId: form.archiveImportId });
  }
  form.job = status;
  if (!status) {
    return [null, err];
  }
  if (status.state === "failed") {
    return [null, status.error || "Import failed"];
  }
  return [status.result, ""];
};

const JobProgress = ({ form }: { form: ImportForm }) => {
  const job = form.job;
  if (!form.loading || !form.file || !job) {
    return null;
  }
  return (
    <div className="zip-bundle-card" role="status">
      <p>
        <strong>{jobStateLabels[job.state] || job.state}</strong>
        {job.state === "uploading" && ` ${form.uploadPercent}%`}
      </p>
      {job.records > 0 && (
        <p>
          {job.media > 0
            ? `${job.records} records and ${job.media} photo and video files in this archive.`
            : `${job.records} records in this file.`}
        </p>
      )}
      {job.state === "staging" && job.stagedMedia > 0 && (
        <p>{job.stagedMedia} files unpacked so far.</p>
      )}
      <p>You can leave this page; the import keeps running.</p>
    </div>
  );
};

export async function fetch(route: string, prefix: string) {
  return rpc.ok<Data>({});
}

export function view(route: string, prefix: string, data: Data): preact.ComponentChild {
  const currentAuth = requireAuthInView();
  if (!currentAuth) {
    return null;
  }

  const form = useImportForm();

  return (
    <div>
      <Header isHome={false} />
      <main id="app" className="import-container">
        <ImportPage form={form} />
      </main>
      <Footer />
    </div>
  );
}

interface ImportPageProps {
  form: ImportForm;
}

const clearPreview = (form: ImportForm) => {
  form.previewData = null;
  form.showFilters = false;
  form.selectedFamilyIds = [];
  form.selectedPersonIds = [];
};

const ImportPage = ({ form }: ImportPageProps) => {
  const handleFileSelect = (event: Event) => {
    const target = event.target as HTMLInputElement;
    const file = target.files?.[0];

    if (file) {
      form.file = file;
      form.archiveImportId = 0;
      form.error = "";
      clearPreview(form);
      form.isZip = file.name.toLowerCase().endsWith(".zip");
      form.jsonData = "";
      vlens.scheduleRedraw();
    }
  };

  const runImport = async (
    options: Omit<server.ImportDataRequest, "jsonData">
  ): Promise<[server.ImportDataResponse | null, string]> => {
    if (!form.file) {
      const [resp, err] = await server.ImportData({ ...options, jsonData: form.jsonData });
      return [resp, err];
    }
    return runArchiveJob(form, { ...options, jsonData: "" });
  };

  const hasInput = () => form.file !== null || form.jsonData.trim() !== "";

  const handleTextareaChange = (event: Event) => {
    const target = event.target as HTMLTextAreaElement;
    form.jsonData = target.value;
    form.file = null;
    form.error = "";
    clearPreview(form);
    vlens.scheduleRedraw();
  };

  const handleJSONPaste = (event: ClipboardEvent) => {
    const clipboardData = event.clipboardData;
    if (clipboardData) {
      const pastedText = clipboardData.getData("text");
      if (pastedText) {
        setTimeout(() => {
          const target = event.target as HTMLTextAreaElement;
          form.jsonData = target.value;
          form.file = null;
          form.error = "";
          clearPreview(form);
          vlens.scheduleRedraw();
        }, 0);
      }
    }
  };

  const handlePreview = async () => {
    if (!hasInput()) {
      form.error = "Please choose a file or paste JSON data to preview.";
      vlens.scheduleRedraw();
      return;
    }

    if (!form.file) {
      try {
        JSON.parse(form.jsonData);
      } catch (e) {
        form.error = "Invalid JSON format. Please check your data.";
        vlens.scheduleRedraw();
        return;
      }
    }

    form.loading = true;
    form.error = "";
    vlens.scheduleRedraw();

    try {
      let [resp, err] = await runImport({
        filterFamilyIds: [],
        filterPersonIds: [],
        previewOnly: true,
        importMilestones: form.importMilestones,
        importActivities: form.importActivities,
        dryRun: false,
        familyId: form.targetFamilyId,
      });

      form.loading = false;

      if (resp) {
        form.previewData = resp;
        form.showFilters = true;
        vlens.scheduleRedraw();
      } else {
        form.error = err || "Preview failed";
        vlens.scheduleRedraw();
      }
    } catch (error) {
      form.error = error instanceof Error ? error.message : "An error occurred during preview";
      form.loading = false;
      vlens.scheduleRedraw();
    }
  };

  const handleSubmit = async (event: Event) => {
    event.preventDefault();

    if (!hasInput()) {
      form.error = "Please provide JSON data either by uploading a file or pasting it directly.";
      vlens.scheduleRedraw();
      return;
    }

    form.loading = true;
    form.error = "";
    form.success = false;
    vlens.scheduleRedraw();

    try {
      let [resp, err] = await runImport({
        filterFamilyIds: form.selectedFamilyIds,
        filterPersonIds: form.selectedPersonIds,
        previewOnly: false,
        importMilestones: form.importMilestones,
        importActivities: form.importActivities,
        dryRun: form.dryRun,
        familyId: form.targetFamilyId,
      });

      form.loading = false;

      if (resp) {
        form.result = resp;
        form.success = true;
        vlens.scheduleRedraw();
      } else {
        form.error = err || "Import failed";
        form.success = false;
        vlens.scheduleRedraw();
      }
    } catch (error) {
      form.error = error instanceof Error ? error.message : "An error occurred during import";
      form.loading = false;
      form.success = false;
      vlens.scheduleRedraw();
    }
  };

  const clearForm = () => {
    form.jsonData = "";
    form.file = null;
    form.isZip = false;
    form.archiveImportId = 0;
    form.job = null;
    form.error = "";
    form.success = false;
    form.result = null;
    form.previewData = null;
    form.showFilters = false;
    form.selectedFamilyIds = [];
    form.selectedPersonIds = [];
    form.importMilestones = true;
    form.importActivities = true;
    form.dryRun = false;
    const fileInput = document.getElementById("json-file") as HTMLInputElement;
    if (fileInput) {
      fileInput.value = "";
    }
    vlens.scheduleRedraw();
  };

  return (
    <div className="import-page">
      <div className="page-header">
        <h1>Import Family Data</h1>
        <p>Import people and measurements from a backup file</p>
      </div>

      {form.success && form.result ? (
        <div className="import-success">
          <h2>
            <Icon symbol={CircleCheck} />{" "}
            {form.dryRun ? "Dry Run Finished: Nothing Was Saved" : "Import Successful!"}
          </h2>
          <div className="import-stats">
            <div className="stat">
              <span className="stat-number">{form.result.importedPeople}</span>
              <span className="stat-label">People Imported</span>
            </div>
            {form.result.mergedPeople > 0 && (
              <div className="stat">
                <span className="stat-number">{form.result.mergedPeople}</span>
                <span className="stat-label">People Merged</span>
              </div>
            )}
            {form.result.skippedPeople > 0 && (
              <div className="stat">
                <span className="stat-number">{form.result.skippedPeople}</span>
                <span className="stat-label">People Skipped</span>
              </div>
            )}
            <div className="stat">
              <span className="stat-number">{form.result.importedMeasurements}</span>
              <span className="stat-label">Measurements Imported</span>
            </div>
            {form.result.skippedMeasurements > 0 && (
              <div className="stat">
                <span className="stat-number">{form.result.skippedMeasurements}</span>
                <span className="stat-label">Measurements Skipped</span>
              </div>
            )}
            {form.result.importedRelations > 0 && (
              <div className="stat">
                <span className="stat-number">{form.result.importedRelations}</span>
                <span className="stat-label">Relationships Imported</span>
              </div>
            )}
            {form.result.skippedRelations > 0 && (
              <div className="stat">
                <span className="stat-number">{form.result.skippedRelations}</span>
                <span className="stat-label">Relationships Skipped</span>
              </div>
            )}
            {form.result.importedMilestones > 0 && (
              <div className="stat">
                <span className="stat-number">{form.result.importedMilestones}</span>
                <span className="stat-label">Moments Imported</span>
              </div>
            )}
            {form.result.skippedMilestones > 0 && (
              <div className="stat">
                <span className="stat-number">{form.result.skippedMilestones}</span>
                <span className="stat-label">Moments Skipped</span>
              </div>
            )}
            {form.result.importedTags > 0 && (
              <div className="stat">
                <span className="stat-number">{form.result.importedTags}</span>
                <span className="stat-label">Tags Imported</span>
              </div>
            )}
            {form.result.skippedTags > 0 && (
              <div className="stat">
                <span className="stat-number">{form.result.skippedTags}</span>
                <span className="stat-label">Tags Skipped</span>
              </div>
            )}
            {form.result.importedPhotos > 0 && (
              <div className="stat">
                <span className="stat-number">{form.result.importedPhotos}</span>
                <span className="stat-label">Photos Imported</span>
              </div>
            )}
            {form.result.skippedPhotos > 0 && (
              <div className="stat">
                <span className="stat-number">{form.result.skippedPhotos}</span>
                <span className="stat-label">Photos Skipped</span>
              </div>
            )}
            {form.result.importedVideos > 0 && (
              <div className="stat">
                <span className="stat-number">{form.result.importedVideos}</span>
                <span className="stat-label">Videos Imported</span>
              </div>
            )}
            {form.result.skippedVideos > 0 && (
              <div className="stat">
                <span className="stat-number">{form.result.skippedVideos}</span>
                <span className="stat-label">Videos Skipped</span>
              </div>
            )}
          </div>

          {form.result.warnings && form.result.warnings.length > 0 && (
            <div className="import-warnings">
              <h3>
                <Icon symbol={TriangleAlert} /> Warnings:
              </h3>
              <ul>
                {form.result.warnings.map((warning, index) => (
                  <li key={index}>{warning}</li>
                ))}
              </ul>
            </div>
          )}

          {form.result.errors && form.result.errors.length > 0 && (
            <div className="import-errors">
              <h3>
                <Icon symbol={CircleX} /> Errors:
              </h3>
              <ul>
                {form.result.errors.map((error, index) => (
                  <li key={index}>{error}</li>
                ))}
              </ul>
            </div>
          )}

          <div className="success-actions">
            <a href="/dashboard" className="btn btn-primary">
              Go to Dashboard
            </a>
            <button onClick={clearForm} className="btn btn-secondary">
              Import More Data
            </button>
          </div>
        </div>
      ) : (
        <div className="import-form-container">
          <form onSubmit={handleSubmit} className="import-form">
            <div className="form-section">
              <h3>Upload File</h3>
              <div className="file-input-container">
                <input
                  type="file"
                  accept=".json,.zip"
                  onChange={handleFileSelect}
                  className="file-input"
                  id="json-file"
                />
                <label htmlFor="json-file" className="file-input-label">
                  {form.file ? form.file.name : "Choose JSON or ZIP file..."}
                </label>
              </div>
            </div>

            {form.file ? (
              <div className="zip-bundle-card">
                <p>
                  <strong>{form.isZip ? "Everything archive" : "Records file"} selected:</strong>{" "}
                  {form.file.name}
                </p>
                <p>
                  {form.isZip
                    ? "Your records plus their photo and video files."
                    : "Your records, without photo or video files."}
                </p>
              </div>
            ) : (
              <>
                <div className="form-divider">
                  <span>OR</span>
                </div>

                <div className="form-section">
                  <h3 id="jsonPasteHeading">Paste JSON Data</h3>
                  <textarea
                    aria-labelledby="jsonPasteHeading"
                    value={form.jsonData}
                    onInput={handleTextareaChange}
                    onPaste={handleJSONPaste}
                    placeholder="Paste your JSON export data here..."
                    className="json-textarea"
                    rows={10}
                  />
                </div>
              </>
            )}

            <JobProgress form={form} />

            {form.error && (
              <div className="error-message" role="alert">
                {form.error}
              </div>
            )}

            <div className="form-section">
              <h3>Import Options</h3>

              <div className="import-options">
                <FamilySelect
                  id="importFamilyId"
                  label="Import into family"
                  value={form.targetFamilyId}
                  onChange={familyId => {
                    form.targetFamilyId = familyId;
                  }}
                  disabled={form.loading}
                />

                <div className="option-group">
                  <label className="checkbox-option">
                    <input
                      type="checkbox"
                      checked={form.importMilestones}
                      onChange={e => {
                        form.importMilestones = (e.target as HTMLInputElement).checked;
                        vlens.scheduleRedraw();
                      }}
                    />
                    <span>Import Moments</span>
                    <small>Include moments in the import</small>
                  </label>
                </div>

                <div className="option-group">
                  <label className="checkbox-option">
                    <input
                      type="checkbox"
                      checked={form.importActivities}
                      onChange={e => {
                        form.importActivities = (e.target as HTMLInputElement).checked;
                        vlens.scheduleRedraw();
                      }}
                    />
                    <span>Import Activities</span>
                    <small>Include seasons, competitions, routines, and results</small>
                  </label>
                </div>

                <div className="option-group">
                  <label className="checkbox-option">
                    <input
                      type="checkbox"
                      checked={form.dryRun}
                      onChange={e => {
                        form.dryRun = (e.target as HTMLInputElement).checked;
                        vlens.scheduleRedraw();
                      }}
                    />
                    <span>Dry Run</span>
                    <small>Preview changes without saving to database</small>
                  </label>
                </div>
              </div>
            </div>

            <div className="form-actions">
              <button
                type="button"
                onClick={handlePreview}
                disabled={form.loading || !hasInput()}
                className="btn btn-secondary"
              >
                {form.loading ? "Loading..." : "Preview Data"}
              </button>
              <button
                type="submit"
                disabled={form.loading || !hasInput() || (!form.showFilters && !form.previewData)}
                className="btn btn-primary"
              >
                {form.loading ? "Importing..." : "Import Data"}
              </button>
              <button type="button" onClick={clearForm} className="btn btn-secondary">
                Clear
              </button>
            </div>
          </form>

          {form.showFilters && form.previewData && <FilteringInterface form={form} />}

          <div className="import-help">
            <h3>
              <Icon symbol={ClipboardList} /> Import Instructions
            </h3>
            <ul>
              <li>
                Upload a Records file (JSON) or an Everything archive (ZIP) exported from Family
                Record
              </li>
              <li>Or paste the JSON data directly into the text area</li>
              <li>
                People already in the family are matched and have missing details filled in rather
                than added again
              </li>
              <li>People and their measurements will be imported together</li>
              <li>
                Relationships are restored between imported people; a relationship to someone left
                out of the import is skipped
              </li>
            </ul>
          </div>
        </div>
      )}
    </div>
  );
};

interface FilteringInterfaceProps {
  form: ImportForm;
}

const FilteringInterface = ({ form }: FilteringInterfaceProps) => {
  const toggleFamilyId = (familyId: number) => {
    const index = form.selectedFamilyIds.indexOf(familyId);
    if (index === -1) {
      form.selectedFamilyIds.push(familyId);
    } else {
      form.selectedFamilyIds.splice(index, 1);
    }
    vlens.scheduleRedraw();
  };

  const togglePersonId = (personId: number) => {
    const index = form.selectedPersonIds.indexOf(personId);
    if (index === -1) {
      form.selectedPersonIds.push(personId);
    } else {
      form.selectedPersonIds.splice(index, 1);
    }
    vlens.scheduleRedraw();
  };

  const selectAllFromFamily = (familyId: number) => {
    const peopleInFamily =
      form.previewData?.availablePeople?.filter(p => p.FamilyId === familyId) || [];
    for (const person of peopleInFamily) {
      if (!form.selectedPersonIds.includes(person.Id)) {
        form.selectedPersonIds.push(person.Id);
      }
    }
    vlens.scheduleRedraw();
  };

  const getPersonCountsInFamily = (familyId: number) => {
    const people = form.previewData?.availablePeople?.filter(p => p.FamilyId === familyId) || [];
    const selected = people.filter(p => form.selectedPersonIds.includes(p.Id)).length;
    return { total: people.length, selected };
  };

  const getMeasurementCounts = (personId: number) => {
    return { heights: "?", weights: "?" };
  };

  return (
    <div className="filtering-interface">
      <h3>
        <Icon symbol={Target} /> Select Data to Import
      </h3>
      <p>Choose which families and people you want to import:</p>

      {form.previewData?.availableFamilyIds?.map(familyId => {
        const counts = getPersonCountsInFamily(familyId);
        const familyPeople =
          form.previewData?.availablePeople?.filter(p => p.FamilyId === familyId) || [];

        return (
          <div key={familyId} className="family-group">
            <div className="family-header">
              <h4>
                <label className="family-checkbox">
                  <input
                    type="checkbox"
                    checked={form.selectedFamilyIds.includes(familyId)}
                    onChange={() => toggleFamilyId(familyId)}
                  />
                  Family ID {familyId} ({counts.selected}/{counts.total} people selected)
                </label>
              </h4>
              <button
                type="button"
                onClick={() => selectAllFromFamily(familyId)}
                className="btn btn-small btn-secondary"
                disabled={counts.selected === counts.total}
              >
                Select All
              </button>
            </div>

            <div className="people-list">
              {familyPeople.map(person => {
                const measurements = getMeasurementCounts(person.Id);
                return (
                  <div key={person.Id} className="person-item">
                    <label className="person-checkbox">
                      <input
                        type="checkbox"
                        checked={form.selectedPersonIds.includes(person.Id)}
                        onChange={() => togglePersonId(person.Id)}
                      />
                      <div className="person-details">
                        <span className="person-name">{person.Name}</span>
                        <span className="person-meta">
                          {new Date(person.Birthday).getUTCFullYear()} •{" "}
                          {person.Gender === 0
                            ? "Male"
                            : person.Gender === 1
                              ? "Female"
                              : "Unknown"}
                        </span>
                        <span className="person-measurements">
                          Heights: {measurements.heights}, Weights: {measurements.weights}
                        </span>
                      </div>
                    </label>
                  </div>
                );
              })}
            </div>
          </div>
        );
      })}

      <div className="filter-summary">
        <p>
          <strong>Selected:</strong> {form.selectedPersonIds.length} people from{" "}
          {form.selectedFamilyIds.length} families
        </p>
        {form.selectedPersonIds.length === 0 && (
          <p className="warning">
            <Icon symbol={TriangleAlert} /> No people selected. Please select at least one person to
            import.
          </p>
        )}
      </div>
    </div>
  );
};
