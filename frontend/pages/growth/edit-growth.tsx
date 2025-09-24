import * as preact from "preact";
import * as vlens from "vlens";
import * as rpc from "vlens/rpc";
import * as auth from "../../lib/authCache";
import * as core from "vlens/core";
import * as server from "../../server";
import { Header, Footer } from "../../layout";
import { requireAuthInView } from "../../lib/authHelpers";
import "./growth-styles";

type EditGrowthForm = {
  selectedPersonId: string;
  measurementType: string; // 'height' | 'weight'
  value: string;
  unit: string; // cm, in, kg, lbs
  inputType: string; // 'today' | 'date' | 'age'
  measurementDate: string;
  ageYears: string;
  ageMonths: string;
  error: string;
  loading: boolean;
}

const useEditGrowthForm = vlens.declareHook((growthData?: server.GrowthData): EditGrowthForm => ({
  selectedPersonId: growthData?.personId?.toString() || "",
  measurementType: growthData?.measurementType === server.Height ? "height" : "weight",
  value: growthData?.value?.toString() || "",
  unit: growthData?.unit || "in",
  inputType: "date", // Default to date since we have the original date
  measurementDate: growthData?.measurementDate ? growthData.measurementDate.split('T')[0] : "",
  ageYears: "",
  ageMonths: "",
  error: "",
  loading: false
}));

export async function fetch(route: string, prefix: string) {
  // Extract growth record ID from URL (e.g., /edit-growth/123)
  const urlParts = route.split('/');
  const growthId = urlParts.length > 2 ? parseInt(urlParts[2]) : null;

  if (!growthId) {
    throw new Error("Growth record ID is required");
  }

  // For now, just fetch the growth data - we'll get people list separately
  return server.GetGrowthData({ id: growthId });
}

export function view(
  route: string,
  prefix: string,
  data: server.GetGrowthDataResponse,
): preact.ComponentChild {
  const currentAuth = requireAuthInView();
  if (!currentAuth) {
    return;
  }

  if (!data.growthData) {
    return (
      <div>
        <Header isHome={false} />
        <main id="app" className="add-growth-container">
          <div className="error-page">
            <h1>Growth Record Not Found</h1>
            <p>The growth record you're trying to edit could not be found</p>
            <a href="/dashboard" className="btn btn-primary">Back to Dashboard</a>
          </div>
        </main>
        <Footer />
      </div>
    );
  }

  const form = useEditGrowthForm(data.growthData);

  return (
    <div>
      <Header isHome={false} />
      <main id="app" className="add-growth-container">
        <EditGrowthPage form={form} growthData={data.growthData} />
      </main>
      <Footer />
    </div>
  );
}

async function onSubmitGrowth(form: EditGrowthForm, growthData: server.GrowthData, event: Event) {
  event.preventDefault();
  form.loading = true;
  form.error = "";

  // Validation
  if (!form.value || parseFloat(form.value) <= 0) {
    form.error = "Please enter a valid measurement value";
    form.loading = false;
    vlens.scheduleRedraw();
    return;
  }

  if (form.inputType === 'date' && !form.measurementDate) {
    form.error = "Please select a date";
    form.loading = false;
    vlens.scheduleRedraw();
    return;
  }

  if (form.inputType === 'age' && (form.ageYears === "" || parseInt(form.ageYears) < 0)) {
    form.error = "Please enter a valid age";
    form.loading = false;
    vlens.scheduleRedraw();
    return;
  }

  // Prepare API request
  const request: server.UpdateGrowthDataRequest = {
    id: growthData.id,
    measurementType: form.measurementType,
    value: parseFloat(form.value),
    unit: form.unit,
    inputType: form.inputType,
    measurementDate: form.inputType === 'date' ? form.measurementDate : null,
    ageYears: form.inputType === 'age' ? parseInt(form.ageYears) : null,
    ageMonths: form.inputType === 'age' && form.ageMonths ? parseInt(form.ageMonths) : null
  };

  try {
    let [resp, err] = await server.UpdateGrowthData(request);

    if (resp) {
      // Redirect immediately to profile page
      core.setRoute(`/profile/${form.selectedPersonId}`);
    } else {
      form.loading = false;
      form.error = err || "Failed to update growth measurement";
      vlens.scheduleRedraw();
    }
  } catch (error) {
    form.loading = false;
    form.error = "Network error. Please try again.";
    vlens.scheduleRedraw();
  }
}

function onMeasurementTypeChange(form: EditGrowthForm, newType: string) {
  form.measurementType = newType;
  form.unit = newType === 'height' ? 'in' : 'lbs';
  vlens.scheduleRedraw();
}

function onInputTypeChange(form: EditGrowthForm, newType: string) {
  form.inputType = newType;
  vlens.scheduleRedraw();
}

interface EditGrowthPageProps {
  form: EditGrowthForm;
  growthData: server.GrowthData;
}

const EditGrowthPage = ({ form, growthData }: EditGrowthPageProps) => {
  const getUnitOptions = () => {
    if (form.measurementType === 'height') {
      return [
        { value: 'in', label: 'inches' },
        { value: 'cm', label: 'cm' }
      ];
    } else {
      return [
        { value: 'lbs', label: 'lbs' }
      ];
    }
  };

  return (
    <div className="add-growth-page">
      <div className="auth-card">
        <div className="auth-header">
          <h1>Edit Growth Measurement</h1>
          <p>Update this growth measurement record</p>
        </div>

        {form.error && (
          <div className="error-message">{form.error}</div>
        )}

        <form className="auth-form" onSubmit={vlens.cachePartial(onSubmitGrowth, form, growthData)}>
          {/* Measurement Type */}
          <div className="form-group">
            <label>Measurement Type</label>
            <div className="radio-group">
              <label className="radio-option">
                <input
                  type="radio"
                  name="measurementType"
                  value="height"
                  checked={form.measurementType === 'height'}
                  onChange={() => onMeasurementTypeChange(form, 'height')}
                  disabled={form.loading}
                />
                <span>Height</span>
              </label>
              <label className="radio-option">
                <input
                  type="radio"
                  name="measurementType"
                  value="weight"
                  checked={form.measurementType === 'weight'}
                  onChange={() => onMeasurementTypeChange(form, 'weight')}
                  disabled={form.loading}
                />
                <span>Weight</span>
              </label>
            </div>
          </div>

          {/* Value and Unit */}
          <div className="form-row">
            <div className="form-group flex-2">
              <label htmlFor="value">
                {form.measurementType === 'height' ? 'Height' : 'Weight'}
              </label>
              <input
                id="value"
                type="number"
                step="0.1"
                {...vlens.attrsBindInput(vlens.ref(form, "value"))}
                placeholder={form.measurementType === 'height' ? '150.5' : '45.2'}
                required
                disabled={form.loading}
              />
            </div>
            <div className="form-group flex-1">
              <label htmlFor="unit">Unit</label>
              <select
                id="unit"
                {...vlens.attrsBindInput(vlens.ref(form, "unit"))}
                disabled={form.loading}
              >
                {getUnitOptions().map(option => (
                  <option key={option.value} value={option.value}>
                    {option.label}
                  </option>
                ))}
              </select>
            </div>
          </div>

          {/* Date or Age Toggle */}
          <div className="form-group">
            <label>When was this measured?</label>
            <div className="radio-group">
              <label className="radio-option">
                <input
                  type="radio"
                  name="inputType"
                  value="today"
                  checked={form.inputType === 'today'}
                  onChange={() => onInputTypeChange(form, 'today')}
                  disabled={form.loading}
                />
                <span>Today</span>
              </label>
              <label className="radio-option">
                <input
                  type="radio"
                  name="inputType"
                  value="date"
                  checked={form.inputType === 'date'}
                  onChange={() => onInputTypeChange(form, 'date')}
                  disabled={form.loading}
                />
                <span>Specific Date</span>
              </label>
              <label className="radio-option">
                <input
                  type="radio"
                  name="inputType"
                  value="age"
                  checked={form.inputType === 'age'}
                  onChange={() => onInputTypeChange(form, 'age')}
                  disabled={form.loading}
                />
                <span>At Age</span>
              </label>
            </div>
          </div>

          {/* Date Input */}
          {form.inputType === 'date' && (
            <div className="form-group">
              <label htmlFor="date">Measurement Date</label>
              <input
                id="date"
                type="date"
                {...vlens.attrsBindInput(vlens.ref(form, "measurementDate"))}
                max={new Date().toISOString().split('T')[0]}
                required
                disabled={form.loading}
              />
            </div>
          )}

          {/* Age Input */}
          {form.inputType === 'age' && (
            <div className="form-row">
              <div className="form-group flex-2">
                <label htmlFor="ageYears">Age (Years)</label>
                <input
                  id="ageYears"
                  type="number"
                  min="0"
                  max="100"
                  {...vlens.attrsBindInput(vlens.ref(form, "ageYears"))}
                  placeholder="5"
                  required
                  disabled={form.loading}
                />
              </div>
              <div className="form-group flex-1">
                <label htmlFor="ageMonths">Months</label>
                <input
                  id="ageMonths"
                  type="number"
                  min="0"
                  max="11"
                  {...vlens.attrsBindInput(vlens.ref(form, "ageMonths"))}
                  placeholder="0"
                  disabled={form.loading}
                />
              </div>
            </div>
          )}

          {/* Submit Button */}
          <div className="form-actions">
            <a href={`/profile/${form.selectedPersonId}`} className="btn btn-secondary">Cancel</a>
            <button
              type="submit"
              className="btn btn-primary auth-submit"
              disabled={form.loading}
            >
              {form.loading ? 'Saving...' : 'Update Measurement'}
            </button>
          </div>
        </form>

        {form.value && (
          <div className="measurement-preview">
            <h3>Preview</h3>
            <p>
              Updated {form.measurementType}: {form.value} {form.unit}
              {form.inputType === 'today' && (
                <span> today</span>
              )}
              {form.inputType === 'date' && form.measurementDate && (
                <span> on {new Date(form.measurementDate).toLocaleDateString()}</span>
              )}
              {form.inputType === 'age' && form.ageYears && (
                <span> at age {form.ageYears}{form.ageMonths ? `.${form.ageMonths}` : ''} years</span>
              )}
            </p>
          </div>
        )}
      </div>
    </div>
  );
};