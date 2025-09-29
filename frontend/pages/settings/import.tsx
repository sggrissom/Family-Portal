import * as preact from "preact";
import * as vlens from "vlens";
import * as rpc from "vlens/rpc";
import * as auth from "../../lib/authCache";
import * as server from "../../server";
import { Header, Footer } from "../../layout";
import { requireAuthInView } from "../../lib/authHelpers";
import "./import-styles";

type Data = {};

type ImportForm = {
  jsonData: string;
  file: File | null;
  error: string;
  loading: boolean;
  success: boolean;
  result: server.ImportDataResponse | null;
  previewData: server.ImportDataResponse | null;
  showFilters: boolean;
  selectedFamilyIds: number[];
  selectedPersonIds: number[];
  mergeStrategy: string;
  importMilestones: boolean;
  dryRun: boolean;
  // AI Import fields
  activeTab: "json" | "ai";
  selectedPersonId: number | null;
  peopleList: server.Person[];
  unstructuredText: string;
  aiProcessing: boolean;
};

const useImportForm = vlens.declareHook(
  (): ImportForm => ({
    jsonData: "",
    file: null,
    error: "",
    loading: false,
    success: false,
    result: null,
    previewData: null,
    showFilters: false,
    selectedFamilyIds: [],
    selectedPersonIds: [],
    mergeStrategy: "create_all",
    importMilestones: true,
    dryRun: false,
    // AI Import fields
    activeTab: "json",
    selectedPersonId: null,
    peopleList: [],
    unstructuredText: "",
    aiProcessing: false,
  })
);

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

const ImportPage = ({ form }: ImportPageProps) => {
  const handleFileSelect = (event: Event) => {
    const target = event.target as HTMLInputElement;
    const file = target.files?.[0];

    if (file) {
      form.file = file;
      form.error = "";

      // Read file content
      const reader = new FileReader();
      reader.onload = e => {
        const result = e.target?.result;
        if (typeof result === "string") {
          form.jsonData = result;
          vlens.scheduleRedraw();
        }
      };
      reader.readAsText(file);
      vlens.scheduleRedraw();
    }
  };

  const handleTextareaChange = (event: Event) => {
    const target = event.target as HTMLTextAreaElement;
    form.jsonData = target.value;
    form.file = null;
    form.error = "";
    vlens.scheduleRedraw();
  };

  const handleJSONPaste = (event: ClipboardEvent) => {
    // Explicitly handle paste events for JSON textarea
    const clipboardData = event.clipboardData;
    if (clipboardData) {
      const pastedText = clipboardData.getData("text");
      if (pastedText) {
        setTimeout(() => {
          const target = event.target as HTMLTextAreaElement;
          form.jsonData = target.value;
          form.file = null;
          form.error = "";
          vlens.scheduleRedraw();
        }, 0);
      }
    }
  };

  const loadPeopleList = async () => {
    if (form.peopleList.length > 0) {
      return; // Already loaded
    }

    try {
      const [resp, err] = await server.ListPeople({});
      if (resp && resp.people) {
        form.peopleList = resp.people;
        vlens.scheduleRedraw();
      }
    } catch (error) {
      console.error("Failed to load people list:", error);
    }
  };

  const handlePreview = async () => {
    if (!form.jsonData.trim()) {
      form.error = "Please provide JSON data to preview.";
      vlens.scheduleRedraw();
      return;
    }

    // Basic JSON validation
    try {
      JSON.parse(form.jsonData);
    } catch (e) {
      form.error = "Invalid JSON format. Please check your data.";
      vlens.scheduleRedraw();
      return;
    }

    form.loading = true;
    form.error = "";
    vlens.scheduleRedraw();

    try {
      let [resp, err] = await server.ImportData({
        jsonData: form.jsonData,
        filterFamilyIds: [],
        filterPersonIds: [],
        previewOnly: true,
        mergeStrategy: form.mergeStrategy,
        importMilestones: form.importMilestones,
        dryRun: false,
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

    if (!form.jsonData.trim()) {
      form.error = "Please provide JSON data either by uploading a file or pasting it directly.";
      vlens.scheduleRedraw();
      return;
    }

    form.loading = true;
    form.error = "";
    form.success = false;
    vlens.scheduleRedraw();

    try {
      let [resp, err] = await server.ImportData({
        jsonData: form.jsonData,
        filterFamilyIds: form.selectedFamilyIds,
        filterPersonIds: form.selectedPersonIds,
        previewOnly: false,
        mergeStrategy: form.mergeStrategy,
        importMilestones: form.importMilestones,
        dryRun: form.dryRun,
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

  const handleAITextChange = (event: Event) => {
    const target = event.target as HTMLTextAreaElement;
    form.unstructuredText = target.value;
    form.error = "";
    vlens.scheduleRedraw();
  };

  const handleAIPaste = (event: ClipboardEvent) => {
    // Explicitly handle paste events
    const clipboardData = event.clipboardData;
    if (clipboardData) {
      const pastedText = clipboardData.getData("text");
      if (pastedText) {
        // Let the default paste happen, then update via onInput
        setTimeout(() => {
          const target = event.target as HTMLTextAreaElement;
          form.unstructuredText = target.value;
          form.error = "";
          vlens.scheduleRedraw();
        }, 0);
      }
    }
  };

  const handleAIProcess = async () => {
    if (!form.selectedPersonId) {
      form.error = "Please select a person for AI import";
      vlens.scheduleRedraw();
      return;
    }

    if (!form.unstructuredText.trim()) {
      form.error = "Please provide text for AI processing";
      vlens.scheduleRedraw();
      return;
    }

    form.aiProcessing = true;
    form.error = "";
    vlens.scheduleRedraw();

    try {
      const [resp, err] = await server.ProcessAIImport({
        personId: form.selectedPersonId,
        unstructuredText: form.unstructuredText,
        generateFile: false,
      });

      form.aiProcessing = false;

      if (resp && resp.success) {
        // Set the generated JSON in the main form
        form.jsonData = resp.generatedJSON;

        // Switch to JSON tab to show result
        form.activeTab = "json";

        // Show any validation warnings
        if (resp.validationWarnings && resp.validationWarnings.length > 0) {
          form.error =
            "AI processing succeeded with warnings: " + resp.validationWarnings.join(", ");
        }

        vlens.scheduleRedraw();
      } else {
        form.error = resp?.error || err || "AI processing failed";
        vlens.scheduleRedraw();
      }
    } catch (error) {
      form.error =
        error instanceof Error ? error.message : "An error occurred during AI processing";
      form.aiProcessing = false;
      vlens.scheduleRedraw();
    }
  };

  const clearForm = () => {
    form.jsonData = "";
    form.file = null;
    form.error = "";
    form.success = false;
    form.result = null;
    form.previewData = null;
    form.showFilters = false;
    form.selectedFamilyIds = [];
    form.selectedPersonIds = [];
    form.mergeStrategy = "create_all";
    form.importMilestones = true;
    form.dryRun = false;
    // AI fields
    form.selectedPersonId = null;
    form.unstructuredText = "";
    form.aiProcessing = false;
    // Clear file input by resetting the form
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
          <div className="success-message">
            <h2>✅ Import Successful!</h2>
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
              {form.result.importedMilestones > 0 && (
                <div className="stat">
                  <span className="stat-number">{form.result.importedMilestones}</span>
                  <span className="stat-label">Milestones Imported</span>
                </div>
              )}
              {form.result.skippedMilestones > 0 && (
                <div className="stat">
                  <span className="stat-number">{form.result.skippedMilestones}</span>
                  <span className="stat-label">Milestones Skipped</span>
                </div>
              )}
            </div>

            {form.result.warnings && form.result.warnings.length > 0 && (
              <div className="import-warnings">
                <h3>⚠️ Warnings:</h3>
                <ul>
                  {form.result.warnings.map((warning, index) => (
                    <li key={index}>{warning}</li>
                  ))}
                </ul>
              </div>
            )}

            {form.result.errors && form.result.errors.length > 0 && (
              <div className="import-errors">
                <h3>❌ Errors:</h3>
                <ul>
                  {form.result.errors.map((error, index) => (
                    <li key={index}>{error}</li>
                  ))}
                </ul>
              </div>
            )}
          </div>

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
          <div className="import-tabs">
            <button
              className={`tab-button ${form.activeTab === "json" ? "active" : ""}`}
              onClick={() => {
                form.activeTab = "json";
                vlens.scheduleRedraw();
              }}
            >
              JSON Import
            </button>
            <button
              className={`tab-button ${form.activeTab === "ai" ? "active" : ""}`}
              onClick={() => {
                form.activeTab = "ai";
                vlens.scheduleRedraw();
              }}
            >
              AI Import
            </button>
          </div>

          {form.activeTab === "json" ? (
            <form onSubmit={handleSubmit} className="import-form">
              <div className="form-section">
                <h3>Upload JSON File</h3>
                <div className="file-input-container">
                  <input
                    type="file"
                    accept=".json"
                    onChange={handleFileSelect}
                    className="file-input"
                    id="json-file"
                  />
                  <label htmlFor="json-file" className="file-input-label">
                    {form.file ? form.file.name : "Choose JSON file..."}
                  </label>
                </div>
              </div>

              <div className="form-divider">
                <span>OR</span>
              </div>

              <div className="form-section">
                <h3>Paste JSON Data</h3>
                <textarea
                  value={form.jsonData}
                  onInput={handleTextareaChange}
                  onPaste={handleJSONPaste}
                  placeholder="Paste your JSON export data here..."
                  className="json-textarea"
                  rows={10}
                />
              </div>

              {form.error && <div className="error-message">{form.error}</div>}

              <div className="form-section">
                <h3>Import Options</h3>

                <div className="import-options">
                  <div className="option-group">
                    <label className="option-label">
                      <strong>Merge Strategy</strong>
                    </label>
                    <div className="radio-group">
                      <label className="radio-option">
                        <input
                          type="radio"
                          name="mergeStrategy"
                          value="create_all"
                          checked={form.mergeStrategy === "create_all"}
                          onChange={e => {
                            form.mergeStrategy = (e.target as HTMLInputElement).value;
                            vlens.scheduleRedraw();
                          }}
                        />
                        <span>Create All</span>
                        <small>Create new records for everyone (may create duplicates)</small>
                      </label>
                      <label className="radio-option">
                        <input
                          type="radio"
                          name="mergeStrategy"
                          value="merge_people"
                          checked={form.mergeStrategy === "merge_people"}
                          onChange={e => {
                            form.mergeStrategy = (e.target as HTMLInputElement).value;
                            vlens.scheduleRedraw();
                          }}
                        />
                        <span>Merge People</span>
                        <small>Merge with existing people when name & birthday match</small>
                      </label>
                      <label className="radio-option">
                        <input
                          type="radio"
                          name="mergeStrategy"
                          value="skip_duplicates"
                          checked={form.mergeStrategy === "skip_duplicates"}
                          onChange={e => {
                            form.mergeStrategy = (e.target as HTMLInputElement).value;
                            vlens.scheduleRedraw();
                          }}
                        />
                        <span>Skip Duplicates</span>
                        <small>Skip people who already exist (name & birthday match)</small>
                      </label>
                    </div>
                  </div>

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
                      <span>Import Milestones</span>
                      <small>Include milestone data in the import</small>
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
                  disabled={form.loading || !form.jsonData.trim()}
                  className="btn btn-secondary"
                >
                  {form.loading ? "Loading..." : "Preview Data"}
                </button>
                <button
                  type="submit"
                  disabled={
                    form.loading ||
                    !form.jsonData.trim() ||
                    (!form.showFilters && !form.previewData)
                  }
                  className="btn btn-primary"
                >
                  {form.loading ? "Importing..." : "Import Data"}
                </button>
                <button type="button" onClick={clearForm} className="btn btn-secondary">
                  Clear
                </button>
              </div>
            </form>
          ) : (
            <div className="ai-import-form">
              <div className="form-section">
                <h3>Select Person</h3>
                <p className="section-description">
                  Choose which person this data is for. The AI will extract heights, weights, and
                  milestones for this person only.
                </p>
                <select
                  value={form.selectedPersonId || ""}
                  onChange={e => {
                    const value = (e.target as HTMLSelectElement).value;
                    form.selectedPersonId = value ? parseInt(value, 10) : null;
                    vlens.scheduleRedraw();
                  }}
                  onFocus={loadPeopleList}
                  className="person-selector"
                >
                  <option value="">-- Select a person --</option>
                  {form.peopleList.map(person => (
                    <option key={person.id} value={person.id}>
                      {person.name} ({person.age})
                    </option>
                  ))}
                </select>
              </div>

              <div className="form-section">
                <h3>Paste Unstructured Text</h3>
                <p className="section-description">
                  Paste information about heights, weights, or milestones for the selected person.
                </p>
                <textarea
                  value={form.unstructuredText}
                  onInput={handleAITextChange}
                  onPaste={handleAIPaste}
                  placeholder="Paste information about the selected person. For example:&#10;&#10;Last checkup showed he was 4 feet 2 inches tall and weighed 52 pounds. Started kindergarten in September 2020. Had his 8th birthday party last month..."
                  className="unstructured-textarea"
                  rows={12}
                />
              </div>

              {form.error && <div className="error-message">{form.error}</div>}

              <div className="form-actions">
                <button
                  type="button"
                  onClick={handleAIProcess}
                  disabled={form.aiProcessing || !form.unstructuredText.trim()}
                  className="btn btn-primary"
                >
                  {form.aiProcessing ? "Processing with AI..." : "Process with AI"}
                </button>
                <button type="button" onClick={clearForm} className="btn btn-secondary">
                  Clear
                </button>
              </div>

              <div className="ai-help">
                <h3>🤖 AI Import Instructions</h3>
                <ul>
                  <li>Paste any unstructured text containing family information</li>
                  <li>The AI will extract people, measurements, and milestones</li>
                  <li>Review the generated JSON before importing</li>
                  <li>Supported formats: stories, lists, medical records, etc.</li>
                  <li>Make sure your chosen AI provider is configured with API keys</li>
                </ul>
              </div>
            </div>
          )}

          {form.showFilters && form.previewData && <FilteringInterface form={form} />}

          {form.activeTab === "json" && (
            <div className="import-help">
              <h3>📋 Import Instructions</h3>
              <ul>
                <li>Upload a JSON file exported from a previous version</li>
                <li>Or paste the JSON data directly into the text area</li>
                <li>The import will create new entries in your family portal</li>
                <li>Existing data will not be affected</li>
                <li>People and their measurements will be imported together</li>
              </ul>
            </div>
          )}
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
    // This would require parsing the JSON data to count measurements
    // For now, return placeholder counts
    return { heights: "?", weights: "?" };
  };

  return (
    <div className="filtering-interface">
      <h3>🎯 Select Data to Import</h3>
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
                          {new Date(person.Birthday).getFullYear()} •
                          {person.Type === 0 ? "Parent" : "Child"} •
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
            ⚠️ No people selected. Please select at least one person to import.
          </p>
        )}
      </div>
    </div>
  );
};
