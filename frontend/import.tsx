import * as preact from "preact";
import * as vlens from "vlens";
import * as rpc from "vlens/rpc";
import * as auth from "./authCache";
import * as core from "vlens/core";
import * as server from "./server";
import { Header, Footer } from "./layout";
import { requireAuthInView } from "./authHelpers";

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
}

const useImportForm = vlens.declareHook((): ImportForm => ({
  jsonData: "",
  file: null,
  error: "",
  loading: false,
  success: false,
  result: null,
  previewData: null,
  showFilters: false,
  selectedFamilyIds: [],
  selectedPersonIds: []
}))

export async function fetch(route: string, prefix: string) {
  return rpc.ok<Data>({});
}

export function view(
  route: string,
  prefix: string,
  data: Data,
): preact.ComponentChild {
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
      reader.onload = (e) => {
        const result = e.target?.result;
        if (typeof result === 'string') {
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
        previewOnly: true
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
        previewOnly: false
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
    form.error = "";
    form.success = false;
    form.result = null;
    form.previewData = null;
    form.showFilters = false;
    form.selectedFamilyIds = [];
    form.selectedPersonIds = [];
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
              <div className="stat">
                <span className="stat-number">{form.result.importedMeasurements}</span>
                <span className="stat-label">Measurements Imported</span>
              </div>
            </div>

            {form.result.errors && form.result.errors.length > 0 && (
              <div className="import-warnings">
                <h3>⚠️ Warnings:</h3>
                <ul>
                  {form.result.errors.map((error, index) => (
                    <li key={index}>{error}</li>
                  ))}
                </ul>
              </div>
            )}
          </div>

          <div className="success-actions">
            <a href="/dashboard" className="btn btn-primary">Go to Dashboard</a>
            <button onClick={clearForm} className="btn btn-secondary">Import More Data</button>
          </div>
        </div>
      ) : (
        <div className="import-form-container">
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
                onChange={handleTextareaChange}
                placeholder="Paste your JSON export data here..."
                className="json-textarea"
                rows={10}
              />
            </div>

            {form.error && (
              <div className="error-message">
                {form.error}
              </div>
            )}

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
                disabled={form.loading || !form.jsonData.trim() || (!form.showFilters && !form.previewData)}
                className="btn btn-primary"
              >
                {form.loading ? "Importing..." : "Import Data"}
              </button>
              <button
                type="button"
                onClick={clearForm}
                className="btn btn-secondary"
              >
                Clear
              </button>
            </div>
          </form>

          {form.showFilters && form.previewData && (
            <FilteringInterface form={form} />
          )}

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
    const peopleInFamily = form.previewData?.availablePeople?.filter(p => p.FamilyId === familyId) || [];
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
        const familyPeople = form.previewData?.availablePeople?.filter(p => p.FamilyId === familyId) || [];

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
                          {person.Gender === 0 ? "Male" : person.Gender === 1 ? "Female" : "Unknown"}
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
          <strong>Selected:</strong> {form.selectedPersonIds.length} people from {form.selectedFamilyIds.length} families
        </p>
        {form.selectedPersonIds.length === 0 && (
          <p className="warning">⚠️ No people selected. Please select at least one person to import.</p>
        )}
      </div>
    </div>
  );
};