import * as preact from "preact";
import * as vlens from "vlens";
import * as rpc from "vlens/rpc";
import * as auth from "../../lib/authCache";
import * as core from "vlens/core";
import * as server from "../../server";
import { Header, Footer } from "../../layout";
import { requireAuthInView } from "../../lib/authHelpers";
import { MILESTONE_CATEGORIES } from "../../lib/milestoneHelpers";
import { getIdFromRoute } from "../../lib/routeHelpers";
import { ErrorPage } from "../../components/ErrorPage";
import "./add-milestone-styles";

type EditMilestoneForm = {
  selectedPersonId: string;
  description: string;
  category: string; // 'development' | 'behavior' | 'health' | 'achievement' | 'first' | 'other'
  inputType: string; // 'today' | 'date' | 'age'
  milestoneDate: string;
  ageYears: string;
  ageMonths: string;
  photoIds: number[];
  error: string;
  loading: boolean;
};

const useEditMilestoneForm = vlens.declareHook(
  (milestone?: server.Milestone): EditMilestoneForm => ({
    selectedPersonId: milestone?.personId?.toString() || "",
    description: milestone?.description || "",
    category: milestone?.category || "development",
    inputType: "date", // Default to date since we have the original date
    milestoneDate: milestone?.milestoneDate ? milestone.milestoneDate.split("T")[0] : "",
    ageYears: "",
    ageMonths: "",
    photoIds: milestone?.photoIds ?? [],
    error: "",
    loading: false,
  })
);

type EditMilestoneData = {
  milestone: server.GetMilestoneResponse;
  photos: server.ListFamilyPhotosResponse;
};

export async function fetch(route: string, prefix: string): Promise<rpc.Response<EditMilestoneData>> {
  const milestoneId = getIdFromRoute(route);

  if (!milestoneId) {
    return [null, "Milestone ID is required"];
  }

  const [milestone, milestoneErr] = await server.GetMilestone({ id: milestoneId });
  if (milestoneErr) return [null, milestoneErr];

  const [photos, photosErr] = await server.ListFamilyPhotos({
    personId: milestone?.milestone?.personId,
  });
  if (photosErr) return [null, photosErr];

  return [{ milestone: milestone!, photos: photos! }, ""];
}

export function view(
  route: string,
  prefix: string,
  data: EditMilestoneData
): preact.ComponentChild {
  const currentAuth = requireAuthInView();
  if (!currentAuth) {
    return;
  }

  const milestone = data.milestone.milestone;

  if (!milestone) {
    return (
      <ErrorPage
        title="Milestone Not Found"
        message="The milestone you're trying to edit could not be found"
        containerClass="add-milestone-container"
      />
    );
  }

  const form = useEditMilestoneForm(milestone);

  return (
    <div>
      <Header isHome={false} />
      <main id="app" className="add-milestone-container">
        <EditMilestonePage form={form} milestone={milestone} photos={data.photos.photos} />
      </main>
      <Footer />
    </div>
  );
}

async function onSubmitMilestone(
  form: EditMilestoneForm,
  milestone: server.Milestone,
  event: Event
) {
  event.preventDefault();
  form.loading = true;
  form.error = "";

  // Validation
  if (!form.description.trim()) {
    form.error = "Please enter a description";
    form.loading = false;
    vlens.scheduleRedraw();
    return;
  }

  if (form.inputType === "date" && !form.milestoneDate) {
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

  // Prepare API request
  const request: server.UpdateMilestoneRequest = {
    id: milestone.id,
    description: form.description.trim(),
    category: form.category,
    inputType: form.inputType,
    milestoneDate: form.inputType === "date" ? form.milestoneDate : null,
    ageYears: form.inputType === "age" ? parseInt(form.ageYears) : null,
    ageMonths: form.inputType === "age" && form.ageMonths ? parseInt(form.ageMonths) : null,
    photoIds: form.photoIds,
  };

  try {
    let [resp, err] = await server.UpdateMilestone(request);

    if (resp) {
      // Redirect immediately to profile page
      core.setRoute(`/profile/${form.selectedPersonId}`);
    } else {
      form.loading = false;
      form.error = err || "Failed to update milestone";
      vlens.scheduleRedraw();
    }
  } catch (error) {
    form.loading = false;
    form.error = "Network error. Please try again.";
    vlens.scheduleRedraw();
  }
}

function onCategoryChange(form: EditMilestoneForm, newCategory: string) {
  form.category = newCategory;
  vlens.scheduleRedraw();
}

function onInputTypeChange(form: EditMilestoneForm, newType: string) {
  form.inputType = newType;
  vlens.scheduleRedraw();
}

function onTogglePhoto(form: EditMilestoneForm, photoId: number) {
  const idx = form.photoIds.indexOf(photoId);
  if (idx >= 0) {
    form.photoIds.splice(idx, 1);
  } else {
    form.photoIds.push(photoId);
  }
  vlens.scheduleRedraw();
}

interface EditMilestonePageProps {
  form: EditMilestoneForm;
  milestone: server.Milestone;
  photos: server.PhotoWithPeople[];
}

const EditMilestonePage = ({ form, milestone, photos }: EditMilestonePageProps) => {
  const personPhotos = photos.filter(p => p.people.some(person => person.id === milestone.personId));
  return (
    <div className="add-milestone-page">
      <div className="auth-card">
        <div className="auth-header">
          <h1>Edit Milestone</h1>
          <p>Update this milestone record</p>
        </div>

        {form.error && <div className="error-message">{form.error}</div>}

        <form
          className="auth-form"
          onSubmit={vlens.cachePartial(onSubmitMilestone, form, milestone)}
        >
          {/* Description */}
          <div className="form-group">
            <label htmlFor="description">Description</label>
            <textarea
              id="description"
              {...vlens.attrsBindInput(vlens.ref(form, "description"))}
              placeholder="Describe what happened..."
              rows={3}
              required
              disabled={form.loading}
            />
          </div>

          {/* Category */}
          <div className="form-group">
            <label>Category</label>
            <div className="category-grid">
              {MILESTONE_CATEGORIES.map(option => (
                <label key={option.value} className="category-option">
                  <input
                    type="radio"
                    name="category"
                    value={option.value}
                    checked={form.category === option.value}
                    onChange={() => onCategoryChange(form, option.value)}
                    disabled={form.loading}
                  />
                  <span className="category-card">
                    <span className="category-icon">{option.icon}</span>
                    <span className="category-label">{option.label}</span>
                  </span>
                </label>
              ))}
            </div>
          </div>

          {/* Photos */}
          <div className="form-group">
            <label>Photos (optional)</label>
            <div className="milestone-photo-picker">
              {personPhotos.length === 0 ? (
                <p className="milestone-photo-picker-empty">No photos found for this person</p>
              ) : (
                personPhotos.map(p => {
                  const isSelected = form.photoIds.includes(p.image.id);
                  return (
                    <div
                      key={p.image.id}
                      className={`milestone-photo-picker-item${isSelected ? " selected" : ""}`}
                      onClick={form.loading ? undefined : vlens.cachePartial(onTogglePhoto, form, p.image.id)}
                    >
                      <img
                        src={`/api/photo/${p.image.id}/thumb`}
                        className="milestone-photo-picker-img"
                        alt=""
                        loading="lazy"
                      />
                      {isSelected && <div className="milestone-photo-picker-check">✓</div>}
                    </div>
                  );
                })
              )}
            </div>
          </div>

          {/* Date or Age Toggle */}
          <div className="form-group">
            <label>When did this happen?</label>
            <div className="radio-group">
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
          </div>

          {/* Date Input */}
          {form.inputType === "date" && (
            <div className="form-group">
              <label htmlFor="date">Milestone Date</label>
              <input
                id="date"
                type="date"
                {...vlens.attrsBindInput(vlens.ref(form, "milestoneDate"))}
                max={new Date().toISOString().split("T")[0]}
                required
                disabled={form.loading}
              />
            </div>
          )}

          {/* Age Input */}
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

          {/* Submit Button */}
          <div className="form-actions">
            <a href={`/profile/${form.selectedPersonId}`} className="btn btn-secondary">
              Cancel
            </a>
            <button type="submit" className="btn btn-primary auth-submit" disabled={form.loading}>
              {form.loading ? "Saving..." : "Update Milestone"}
            </button>
          </div>
        </form>

        {form.description && (
          <div className="milestone-preview">
            <h3>Preview</h3>
            <p>
              <strong>{form.category.charAt(0).toUpperCase() + form.category.slice(1)}:</strong>{" "}
              {form.description}
              {form.inputType === "today" && <span> (today)</span>}
              {form.inputType === "date" && form.milestoneDate && (
                <span> ({new Date(form.milestoneDate).toLocaleDateString()})</span>
              )}
              {form.inputType === "age" && form.ageYears && (
                <span>
                  {" "}
                  (at age {form.ageYears}
                  {form.ageMonths ? `.${form.ageMonths}` : ""} years)
                </span>
              )}
            </p>
          </div>
        )}
      </div>
    </div>
  );
};
