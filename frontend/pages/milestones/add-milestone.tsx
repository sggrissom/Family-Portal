import * as preact from "preact";
import * as vlens from "vlens";
import * as rpc from "vlens/rpc";
import * as auth from "../../lib/authCache";
import * as core from "vlens/core";
import * as server from "../../server";
import { Header, Footer } from "../../layout";
import { requireAuthInView } from "../../lib/authHelpers";
import { MILESTONE_CATEGORIES } from "../../lib/milestoneHelpers";
import { getIdFromRoute, splitPeopleByType } from "../../lib/routeHelpers";
import { NoFamilyMembersPage } from "../../components/NoFamilyMembersPage";
import "./add-milestone-styles";

type AddMilestoneForm = {
  selectedPersonId: string;
  description: string;
  category: string;
  inputType: string; // 'today' | 'date' | 'age'
  milestoneDate: string;
  ageYears: string;
  ageMonths: string;
  photoIds: number[];
  error: string;
  loading: boolean;
};

const useAddMilestoneForm = vlens.declareHook(
  (personId?: string): AddMilestoneForm => ({
    selectedPersonId: personId || "",
    description: "",
    category: "development",
    inputType: "today",
    milestoneDate: "",
    ageYears: "",
    ageMonths: "",
    photoIds: [],
    error: "",
    loading: false,
  })
);

type AddMilestoneData = {
  people: server.ListPeopleResponse;
  photos: server.ListFamilyPhotosResponse;
};

export async function fetch(route: string, prefix: string): Promise<rpc.Response<AddMilestoneData>> {
  const [[people, peopleErr], [photos, photosErr]] = await Promise.all([
    server.ListPeople({}),
    server.ListFamilyPhotos({}),
  ]);
  if (peopleErr) return [null, peopleErr];
  if (photosErr) return [null, photosErr];
  return [{ people: people!, photos: photos! }, ""];
}

export function view(
  route: string,
  prefix: string,
  data: AddMilestoneData
): preact.ComponentChild {
  const currentAuth = requireAuthInView();
  if (!currentAuth) {
    return;
  }

  if (!data.people.people || data.people.people.length === 0) {
    return (
      <NoFamilyMembersPage
        message="Please add family members before adding milestones"
        containerClass="add-milestone-container"
      />
    );
  }

  // Extract person ID from URL if present (e.g., /add-milestone/123)
  const personId = getIdFromRoute(route);
  const personIdFromUrl = personId ? personId.toString() : undefined;

  const form = useAddMilestoneForm(personIdFromUrl);

  return (
    <div>
      <Header isHome={false} />
      <main id="app" className="add-milestone-container">
        <AddMilestonePage form={form} people={data.people.people} photos={data.photos.photos} />
      </main>
      <Footer />
    </div>
  );
}

async function onSubmitMilestone(form: AddMilestoneForm, people: server.Person[], event: Event) {
  event.preventDefault();
  form.loading = true;
  form.error = "";

  // Validation
  if (!form.selectedPersonId) {
    form.error = "Please select a family member";
    form.loading = false;
    vlens.scheduleRedraw();
    return;
  }

  if (!form.description.trim()) {
    form.error = "Please enter a milestone description";
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

  try {
    const requestData: server.AddMilestoneRequest = {
      personId: parseInt(form.selectedPersonId),
      description: form.description.trim(),
      category: form.category,
      inputType: form.inputType,
      milestoneDate: form.inputType === "date" ? form.milestoneDate : null,
      ageYears: form.inputType === "age" ? parseInt(form.ageYears) : null,
      ageMonths: form.inputType === "age" && form.ageMonths ? parseInt(form.ageMonths) : null,
      photoIds: form.photoIds,
    };

    const [resp, err] = await server.AddMilestone(requestData);

    if (resp) {
      core.setRoute(`/profile/${form.selectedPersonId}`);
    } else {
      form.loading = false;
      form.error = err || "Failed to save milestone. Please try again.";
      vlens.scheduleRedraw();
    }
  } catch (error) {
    form.loading = false;
    form.error =
      error instanceof Error ? error.message : "Failed to save milestone. Please try again.";
    vlens.scheduleRedraw();
  }
}

function onInputTypeChange(form: AddMilestoneForm, newType: string) {
  form.inputType = newType;
  vlens.scheduleRedraw();
}

function onTogglePhoto(form: AddMilestoneForm, photoId: number) {
  const idx = form.photoIds.indexOf(photoId);
  if (idx >= 0) {
    form.photoIds.splice(idx, 1);
  } else {
    form.photoIds.push(photoId);
  }
  vlens.scheduleRedraw();
}

interface AddMilestonePageProps {
  form: AddMilestoneForm;
  people: server.Person[];
  photos: server.PhotoWithPeople[];
}

const AddMilestonePage = ({ form, people, photos }: AddMilestonePageProps) => {
  // Filter to show all family members for milestones
  const { children, parents } = splitPeopleByType(people);

  const selectedPerson = people.find(p => p.id === parseInt(form.selectedPersonId));

  const selectedPersonIdNum = parseInt(form.selectedPersonId) || 0;
  const personPhotos = selectedPersonIdNum > 0
    ? photos.filter(p => p.people.some(person => person.id === selectedPersonIdNum))
    : [];

  return (
    <div className="add-milestone-page">
      <div className="auth-card">
        <div className="auth-header">
          <h1>Add Milestone</h1>
          <p>Capture special moments and developmental milestones</p>
        </div>

        {form.error && <div className="error-message">{form.error}</div>}

        <form className="auth-form" onSubmit={vlens.cachePartial(onSubmitMilestone, form, people)}>
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

          {/* Category */}
          <div className="form-group">
            <label htmlFor="category">Category</label>
            <select
              id="category"
              {...vlens.attrsBindInput(vlens.ref(form, "category"))}
              disabled={form.loading}
            >
              {MILESTONE_CATEGORIES.map(option => (
                <option key={option.value} value={option.value}>
                  {option.label}
                </option>
              ))}
            </select>
          </div>

          {/* Description */}
          <div className="form-group">
            <label htmlFor="description">Description</label>
            <textarea
              id="description"
              {...vlens.attrsBindInput(vlens.ref(form, "description"))}
              placeholder="e.g., 'only wears ballet shoes', 'can count to 100', 'first time swinging by herself'"
              rows={3}
              required
              disabled={form.loading}
            />
            <small className="form-hint">
              Describe what happened, what they said, or how they're developing
            </small>
          </div>

          {/* Photos */}
          <div className="form-group">
            <label>Photos (optional)</label>
            <div className="milestone-photo-picker">
              {personPhotos.length === 0 ? (
                <p className="milestone-photo-picker-empty">
                  {selectedPersonIdNum > 0
                    ? "No photos found for this person"
                    : "Select a family member to see their photos"}
                </p>
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
              <label htmlFor="date">Date</label>
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
            <a href="/dashboard" className="btn btn-secondary">
              Cancel
            </a>
            <button type="submit" className="btn btn-primary auth-submit" disabled={form.loading}>
              {form.loading ? "Saving..." : "Save Milestone"}
            </button>
          </div>
        </form>

        {selectedPerson && form.description && (
          <div className="milestone-preview">
            <h3>Preview</h3>
            <p>
              <strong>{selectedPerson.name}</strong> - {form.description}
              {form.inputType === "today" && <span> today</span>}
              {form.inputType === "date" && form.milestoneDate && (
                <span> on {new Date(form.milestoneDate).toLocaleDateString()}</span>
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
