import * as preact from "preact";
import * as vlens from "vlens";
import * as rpc from "vlens/rpc";
import * as auth from "./authCache";
import * as core from "vlens/core";
import * as server from "./server";
import { Header, Footer } from "./layout";
import { requireAuthInView } from "./authHelpers";

type AddGrowthForm = {
  selectedPersonId: string;
  measurementType: string; // 'height' | 'weight'
  value: string;
  unit: string; // cm, in, kg, lbs
  inputType: string; // 'date' | 'age'
  measurementDate: string;
  ageYears: string;
  ageMonths: string;
  error: string;
  loading: boolean;
  success: boolean;
}

const useAddGrowthForm = vlens.declareHook((personId?: string): AddGrowthForm => ({
  selectedPersonId: personId || "",
  measurementType: "height",
  value: "",
  unit: "in",
  inputType: "date",
  measurementDate: "",
  ageYears: "",
  ageMonths: "",
  error: "",
  loading: false,
  success: false
}));

export async function fetch(route: string, prefix: string) {
  // Fetch people list to populate the person selector
  return server.ListPeople({});
}

export function view(
  route: string,
  prefix: string,
  data: server.ListPeopleResponse,
): preact.ComponentChild {
  const currentAuth = requireAuthInView();
  if (!currentAuth) {
    return;
  }

  if (!data.people || data.people.length === 0) {
    return (
      <div>
        <Header isHome={false} />
        <main id="app" className="add-growth-container">
          <div className="error-page">
            <h1>No Family Members</h1>
            <p>Please add family members before tracking growth data</p>
            <a href="/add-person" className="btn btn-primary">Add Family Member</a>
            <a href="/dashboard" className="btn btn-secondary">Back to Dashboard</a>
          </div>
        </main>
        <Footer />
      </div>
    );
  }

  // Extract person ID from URL if present (e.g., /add-growth/123)
  const urlParts = route.split('/');
  const personIdFromUrl = urlParts.length > 2 ? urlParts[2] : undefined;

  const form = useAddGrowthForm(personIdFromUrl);

  return (
    <div>
      <Header isHome={false} />
      <main id="app" className="add-growth-container">
        <AddGrowthPage form={form} people={data.people} />
      </main>
      <Footer />
    </div>
  );
}

async function onSubmitGrowth(form: AddGrowthForm, people: server.Person[], event: Event) {
  event.preventDefault();
  form.loading = true;
  form.error = "";
  form.success = false;

  // Validation
  if (!form.selectedPersonId) {
    form.error = "Please select a family member";
    form.loading = false;
    vlens.scheduleRedraw();
    return;
  }

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
  const request: server.AddGrowthDataRequest = {
    personId: parseInt(form.selectedPersonId),
    measurementType: form.measurementType,
    value: parseFloat(form.value),
    unit: form.unit,
    inputType: form.inputType,
    measurementDate: form.inputType === 'date' ? form.measurementDate : null,
    ageYears: form.inputType === 'age' ? parseInt(form.ageYears) : null,
    ageMonths: form.inputType === 'age' && form.ageMonths ? parseInt(form.ageMonths) : null
  };

  try {
    let [resp, err] = await server.AddGrowthData(request);

    form.loading = false;

    if (resp) {
      form.success = true;

      // Reset form
      form.value = "";
      form.measurementDate = "";
      form.ageYears = "";
      form.ageMonths = "";

      vlens.scheduleRedraw();

      // Redirect to profile page after success
      setTimeout(() => {
        core.setRoute(`/profile/${form.selectedPersonId}`);
      }, 1500);
    } else {
      form.error = err || "Failed to save growth measurement";
    }
  } catch (error) {
    form.loading = false;
    form.error = "Network error. Please try again.";
  }

  vlens.scheduleRedraw();
}

function onMeasurementTypeChange(form: AddGrowthForm, newType: string) {
  form.measurementType = newType;
  form.unit = newType === 'height' ? 'in' : 'lbs';
  vlens.scheduleRedraw();
}

function onInputTypeChange(form: AddGrowthForm, newType: string) {
  form.inputType = newType;
  vlens.scheduleRedraw();
}

interface AddGrowthPageProps {
  form: AddGrowthForm;
  people: server.Person[];
}

const AddGrowthPage = ({ form, people }: AddGrowthPageProps) => {
  // Filter to only show children for growth tracking
  const children = people.filter(p => p.type === server.Child);
  const parents = people.filter(p => p.type === server.Parent);

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

  const selectedPerson = people.find(p => p.id === parseInt(form.selectedPersonId));

  return (
    <div className="add-growth-page">
      <div className="auth-card">
        <div className="auth-header">
          <h1>Add Growth Measurement</h1>
          <p>Track height or weight progress for your family</p>
        </div>

        {form.success && (
          <div className="success-message">
            {form.measurementType} measurement saved successfully!
          </div>
        )}

        {form.error && (
          <div className="error-message">{form.error}</div>
        )}

        <form className="auth-form" onSubmit={vlens.cachePartial(onSubmitGrowth, form, people)}>
          {/* Person Selection */}
          <div className="form-group">
            <label htmlFor="person">Family Member</label>
            <select
              id="person"
              {...vlens.attrsBindInput(vlens.ref(form, "selectedPersonId"))}
              required
              disabled={form.loading}
            >
              <option value="">Select a family member</option>
              {children.map(person => (
                <option key={person.id} value={person.id}>
                  {person.name} (Age {person.age})
                </option>
              ))}
              {parents.map(person => (
                <option key={person.id} value={person.id}>
                  {person.name} (Parent)
                </option>
              ))}
            </select>
          </div>

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
            <a href="/dashboard" className="btn btn-secondary">Cancel</a>
            <button
              type="submit"
              className="btn btn-primary auth-submit"
              disabled={form.loading}
            >
              {form.loading ? 'Saving...' : 'Save Measurement'}
            </button>
          </div>
        </form>

        {selectedPerson && form.value && (
          <div className="measurement-preview">
            <h3>Preview</h3>
            <p>
              <strong>{selectedPerson.name}</strong> - {form.measurementType}: {form.value} {form.unit}
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
