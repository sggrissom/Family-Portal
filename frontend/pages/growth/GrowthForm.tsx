import * as preact from "preact";
import * as vlens from "vlens";
import * as server from "../../server";
import { formatLbOz, lbOzToLbs, OZ_PER_LB, prefersLbOz, splitLbOz } from "../../lib/weightFormat";

type GrowthFormData = {
  measurementType: string;
  value: string;
  unit: string;
  heightInputMode: string;
  feet: string;
  inches: string;
  weightInputMode: string;
  pounds: string;
  ounces: string;
  inputType: string;
  measurementDate: string;
  ageYears: string;
  ageMonths: string;
  error: string;
  loading: boolean;
};

const editLbOz = (growthData: server.GrowthData) =>
  growthData.measurementType === server.Weight && prefersLbOz(growthData.value, growthData.unit);

const useGrowthForm = vlens.declareHook(
  (growthData: server.GrowthData): GrowthFormData => ({
    measurementType: growthData.measurementType === server.Height ? "height" : "weight",
    value: growthData.value.toString(),
    unit: growthData.unit || "in",
    heightInputMode: "decimal",
    feet: "",
    inches: "",
    weightInputMode: editLbOz(growthData) ? "lb-oz" : "decimal",
    pounds: editLbOz(growthData) ? splitLbOz(growthData.value).lb.toString() : "",
    ounces: editLbOz(growthData) ? splitLbOz(growthData.value).oz.toString() : "",
    inputType: "date",
    measurementDate: growthData.measurementDate ? growthData.measurementDate.split("T")[0] : "",
    ageYears: "",
    ageMonths: "",
    error: "",
    loading: false,
  })
);

async function onSubmitGrowth(
  form: GrowthFormData,
  growthData: server.GrowthData,
  onSuccess: (personId: number) => void,
  event: Event
) {
  event.preventDefault();
  form.loading = true;
  form.error = "";

  let actualValue: number;
  if (
    form.measurementType === "height" &&
    form.unit === "in" &&
    form.heightInputMode === "feet-inches"
  ) {
    const feet = parseFloat(form.feet) || 0;
    const inches = parseFloat(form.inches) || 0;
    if (feet <= 0 && inches <= 0) {
      form.error = "Please enter a valid height (feet and/or inches)";
      form.loading = false;
      vlens.scheduleRedraw();
      return;
    }
    actualValue = feet * 12 + inches;
  } else if (isLbOzEntry(form)) {
    const pounds = parseFloat(form.pounds) || 0;
    const ounces = parseFloat(form.ounces) || 0;
    if ((pounds <= 0 && ounces <= 0) || pounds < 0 || ounces < 0 || ounces >= OZ_PER_LB) {
      form.error = "Please enter a valid weight (pounds and ounces under 16)";
      form.loading = false;
      vlens.scheduleRedraw();
      return;
    }
    actualValue = lbOzToLbs(pounds, ounces);
  } else {
    actualValue = parseFloat(form.value);
    if (!form.value || actualValue <= 0) {
      form.error = "Please enter a valid measurement value";
      form.loading = false;
      vlens.scheduleRedraw();
      return;
    }
  }

  if (form.inputType === "date" && !form.measurementDate) {
    form.error = "Please select a date";
    form.loading = false;
    vlens.scheduleRedraw();
    return;
  }

  if (form.inputType === "age" && (form.ageYears === "" || parseInt(form.ageYears) < 0)) {
    form.error = "Please enter a valid age";
    form.loading = false;
    vlens.scheduleRedraw();
    return;
  }

  try {
    const request: server.UpdateGrowthDataRequest = {
      id: growthData.id,
      measurementType: form.measurementType,
      value: actualValue,
      unit: form.unit,
      inputType: form.inputType,
      measurementDate: form.inputType === "date" ? form.measurementDate : null,
      ageYears: form.inputType === "age" ? parseInt(form.ageYears) : null,
      ageMonths: form.inputType === "age" && form.ageMonths ? parseInt(form.ageMonths) : null,
    };

    const [resp, err] = await server.UpdateGrowthData(request);

    if (resp) {
      onSuccess(growthData.personId);
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

function isLbOzEntry(form: GrowthFormData): boolean {
  return (
    form.measurementType === "weight" && form.unit === "lbs" && form.weightInputMode === "lb-oz"
  );
}

function onMeasurementTypeChange(form: GrowthFormData, newType: string) {
  form.measurementType = newType;
  form.unit = newType === "height" ? "in" : "lbs";
  form.heightInputMode = "decimal";
  form.weightInputMode = "decimal";
  form.value = "";
  form.feet = "";
  form.inches = "";
  form.pounds = "";
  form.ounces = "";
  vlens.scheduleRedraw();
}

function onWeightInputModeChange(form: GrowthFormData, newMode: string) {
  form.weightInputMode = newMode;
  form.value = "";
  form.pounds = "";
  form.ounces = "";
  vlens.scheduleRedraw();
}

function onInputTypeChange(form: GrowthFormData, newType: string) {
  form.inputType = newType;
  vlens.scheduleRedraw();
}

function onHeightInputModeChange(form: GrowthFormData, newMode: string) {
  form.heightInputMode = newMode;
  form.value = "";
  form.feet = "";
  form.inches = "";
  vlens.scheduleRedraw();
}

interface GrowthFormProps {
  growthData: server.GrowthData;
  onCancel: () => void;
  onSuccess: (personId: number) => void;
}

export const GrowthForm = ({ growthData, onCancel, onSuccess }: GrowthFormProps) => {
  const form = useGrowthForm(growthData);

  const getUnitOptions = () => {
    if (form.measurementType === "height") {
      return [
        { value: "in", label: "inches" },
        { value: "cm", label: "cm" },
      ];
    } else {
      return [{ value: "lbs", label: "lbs" }];
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
          <div className="error-message" role="alert">
            {form.error}
          </div>
        )}

        <form
          className="auth-form growth-form"
          onSubmit={vlens.cachePartial(onSubmitGrowth, form, growthData, onSuccess)}
        >
          <fieldset className="form-group growth-choice-group">
            <legend>Measurement Type</legend>
            <div className="radio-group growth-radio-group">
              <label className="radio-option">
                <input
                  type="radio"
                  name="measurementType"
                  value="height"
                  checked={form.measurementType === "height"}
                  onChange={() => onMeasurementTypeChange(form, "height")}
                  disabled={form.loading}
                />
                <span>Height</span>
              </label>
              <label className="radio-option">
                <input
                  type="radio"
                  name="measurementType"
                  value="weight"
                  checked={form.measurementType === "weight"}
                  onChange={() => onMeasurementTypeChange(form, "weight")}
                  disabled={form.loading}
                />
                <span>Weight</span>
              </label>
            </div>
          </fieldset>

          {form.measurementType === "height" && form.unit === "in" && (
            <fieldset className="form-group growth-choice-group">
              <legend>Height Input Mode</legend>
              <div className="radio-group growth-radio-group">
                <label className="radio-option">
                  <input
                    type="radio"
                    name="heightInputMode"
                    value="decimal"
                    checked={form.heightInputMode === "decimal"}
                    onChange={() => onHeightInputModeChange(form, "decimal")}
                    disabled={form.loading}
                  />
                  <span>Decimal (inches)</span>
                </label>
                <label className="radio-option">
                  <input
                    type="radio"
                    name="heightInputMode"
                    value="feet-inches"
                    checked={form.heightInputMode === "feet-inches"}
                    onChange={() => onHeightInputModeChange(form, "feet-inches")}
                    disabled={form.loading}
                  />
                  <span>Feet & Inches</span>
                </label>
              </div>
            </fieldset>
          )}

          {form.measurementType === "weight" && form.unit === "lbs" && (
            <fieldset className="form-group growth-choice-group">
              <legend>Weight Input Mode</legend>
              <div className="radio-group growth-radio-group">
                <label className="radio-option">
                  <input
                    type="radio"
                    name="weightInputMode"
                    value="decimal"
                    checked={form.weightInputMode === "decimal"}
                    onChange={() => onWeightInputModeChange(form, "decimal")}
                    disabled={form.loading}
                  />
                  <span>Decimal (lbs)</span>
                </label>
                <label className="radio-option">
                  <input
                    type="radio"
                    name="weightInputMode"
                    value="lb-oz"
                    checked={form.weightInputMode === "lb-oz"}
                    onChange={() => onWeightInputModeChange(form, "lb-oz")}
                    disabled={form.loading}
                  />
                  <span>Pounds & Ounces</span>
                </label>
              </div>
            </fieldset>
          )}

          {isLbOzEntry(form) && (
            <div className="form-row">
              <div className="form-group flex-2">
                <label htmlFor="pounds">Pounds</label>
                <input
                  id="pounds"
                  type="number"
                  min="0"
                  step="1"
                  inputmode="numeric"
                  {...vlens.attrsBindInput(vlens.ref(form, "pounds"))}
                  placeholder="7"
                  disabled={form.loading}
                />
              </div>
              <div className="form-group flex-2">
                <label htmlFor="ounces">Ounces</label>
                <input
                  id="ounces"
                  type="text"
                  inputmode="decimal"
                  pattern="[0-9]*\.?[0-9]*"
                  {...vlens.attrsBindInput(vlens.ref(form, "ounces"))}
                  placeholder="8"
                  disabled={form.loading}
                />
              </div>
            </div>
          )}

          {!(
            form.measurementType === "height" &&
            form.unit === "in" &&
            form.heightInputMode === "feet-inches"
          ) &&
            !isLbOzEntry(form) && (
              <div className="form-row">
                <div className="form-group flex-2">
                  <label htmlFor="value">
                    {form.measurementType === "height" ? "Height" : "Weight"}
                  </label>
                  <input
                    id="value"
                    type="text"
                    inputmode="decimal"
                    pattern="[0-9]*\.?[0-9]*"
                    {...vlens.attrsBindInput(vlens.ref(form, "value"))}
                    placeholder={form.measurementType === "height" ? "67.50" : "45.25"}
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
            )}

          {form.measurementType === "height" &&
            form.unit === "in" &&
            form.heightInputMode === "feet-inches" && (
              <div className="form-row">
                <div className="form-group flex-2">
                  <label htmlFor="feet">Feet</label>
                  <input
                    id="feet"
                    type="number"
                    min="0"
                    max="8"
                    step="1"
                    {...vlens.attrsBindInput(vlens.ref(form, "feet"))}
                    placeholder="5"
                    disabled={form.loading}
                  />
                </div>
                <div className="form-group flex-2">
                  <label htmlFor="inches">Inches</label>
                  <input
                    id="inches"
                    type="text"
                    inputmode="decimal"
                    pattern="[0-9]*\.?[0-9]*"
                    {...vlens.attrsBindInput(vlens.ref(form, "inches"))}
                    placeholder="7.50"
                    disabled={form.loading}
                  />
                </div>
                <div className="form-group flex-1">
                  <label htmlFor="unit-display">Unit</label>
                  <input
                    id="unit-display"
                    type="text"
                    value="in"
                    disabled
                    style="background: var(--surface); opacity: 0.6;"
                  />
                </div>
              </div>
            )}

          <fieldset className="form-group growth-choice-group">
            <legend>When was this measured?</legend>
            <div className="radio-group growth-radio-group">
              <label className="radio-option">
                <input
                  type="radio"
                  name="inputType"
                  value="today"
                  checked={form.inputType === "today"}
                  onChange={() => onInputTypeChange(form, "today")}
                  disabled={form.loading}
                />
                <span>Today</span>
              </label>
              <label className="radio-option">
                <input
                  type="radio"
                  name="inputType"
                  value="date"
                  checked={form.inputType === "date"}
                  onChange={() => onInputTypeChange(form, "date")}
                  disabled={form.loading}
                />
                <span>Specific Date</span>
              </label>
              <label className="radio-option">
                <input
                  type="radio"
                  name="inputType"
                  value="age"
                  checked={form.inputType === "age"}
                  onChange={() => onInputTypeChange(form, "age")}
                  disabled={form.loading}
                />
                <span>At Age</span>
              </label>
            </div>
          </fieldset>

          {form.inputType === "date" && (
            <div className="form-group">
              <label htmlFor="date">Measurement Date</label>
              <input
                id="date"
                type="date"
                {...vlens.attrsBindInput(vlens.ref(form, "measurementDate"))}
                max={new Date().toISOString().split("T")[0]}
                required
                disabled={form.loading}
              />
            </div>
          )}

          {form.inputType === "age" && (
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

          <div className="form-actions">
            <button
              type="button"
              className="btn btn-secondary"
              onClick={onCancel}
              disabled={form.loading}
            >
              Cancel
            </button>
            <button type="submit" className="btn btn-primary auth-submit" disabled={form.loading}>
              {form.loading ? "Saving..." : "Update Measurement"}
            </button>
          </div>
        </form>

        {(form.value || form.feet || form.inches || form.pounds || form.ounces) && (
          <div className="measurement-preview">
            <h3>Preview</h3>
            <p>
              Updated
              {form.measurementType}:{" "}
              {form.measurementType === "height" &&
              form.unit === "in" &&
              form.heightInputMode === "feet-inches" ? (
                <>
                  {form.feet || "0"} ft {form.inches || "0"} in
                  {form.feet || form.inches ? (
                    <span style="opacity: 0.7">
                      {" "}
                      (
                      {((parseFloat(form.feet) || 0) * 12 + (parseFloat(form.inches) || 0)).toFixed(
                        2
                      )}{" "}
                      in total)
                    </span>
                  ) : null}
                </>
              ) : isLbOzEntry(form) ? (
                <>
                  {formatLbOz(
                    lbOzToLbs(parseFloat(form.pounds) || 0, parseFloat(form.ounces) || 0)
                  )}
                </>
              ) : (
                <>
                  {form.value} {form.unit}
                </>
              )}
              {form.inputType === "today" && <span> today</span>}
              {form.inputType === "date" && form.measurementDate && (
                <span> on {new Date(form.measurementDate).toLocaleDateString()}</span>
              )}
              {form.inputType === "age" && form.ageYears && (
                <span>
                  {" "}
                  at age {form.ageYears}
                  {form.ageMonths ? `.${form.ageMonths}` : ""} years
                </span>
              )}
            </p>
          </div>
        )}
      </div>
    </div>
  );
};
