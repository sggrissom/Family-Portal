import * as preact from "preact";
import * as vlens from "vlens";
import * as rpc from "vlens/rpc";
import * as auth from "../../lib/authCache";
import * as core from "vlens/core";
import * as server from "../../server";
import { Header, Footer } from "../../layout";
import { requireAuthInView } from "../../lib/authHelpers";
import "./add-milestone-styles";

type AddMilestoneForm = {
  selectedPersonId: string;
  description: string;
  category: string;
  inputType: string; // 'today' | 'date' | 'age'
  milestoneDate: string;
  ageYears: string;
  ageMonths: string;
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
    error: "",
    loading: false,
  })
);

export async function fetch(route: string, prefix: string) {
  // Fetch people list to populate the person selector
  return server.ListPeople({});
}

export function view(
  route: string,
  prefix: string,
  data: server.ListPeopleResponse
): preact.ComponentChild {
  const currentAuth = requireAuthInView();
  if (!currentAuth) {
    return;
  }

  if (!data.people || data.people.length === 0) {
    return (
      <div>
        <Header isHome={false} />
        <main id="app" className="add-milestone-container">
          <div className="error-page">
            <h1>No Family Members</h1>
            <p>Please add family members before adding milestones</p>
            <a href="/add-person" className="btn btn-primary">
              Add Family Member
            </a>
            <a href="/dashboard" className="btn btn-secondary">
              Back to Dashboard
            </a>
          </div>
        </main>
        <Footer />
      </div>
    );
  }

  // Extract person ID from URL if present (e.g., /add-milestone/123)
  const urlParts = route.split("/");
  const personIdFromUrl = urlParts.length > 2 ? urlParts[2] : undefined;

  const form = useAddMilestoneForm(personIdFromUrl);

  return (
    <div>
      <Header isHome={false} />
      <main id="app" className="add-milestone-container">
        <AddMilestonePage form={form} people={data.people} />
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
    // Prepare the request data
    const requestData: any = {
      personId: parseInt(form.selectedPersonId),
      description: form.description.trim(),
      category: form.category,
      inputType: form.inputType,
    };

    // Add date/age specific fields
    if (form.inputType === "date") {
      requestData.milestoneDate = form.milestoneDate;
    } else if (form.inputType === "age") {
      requestData.ageYears = parseInt(form.ageYears);
      if (form.ageMonths) {
        requestData.ageMonths = parseInt(form.ageMonths);
      }
    }

    // Call the backend API
    const response = await server.AddMilestone(requestData);

    // Redirect to profile page on success
    core.setRoute(`/profile/${form.selectedPersonId}`);
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

interface AddMilestonePageProps {
  form: AddMilestoneForm;
  people: server.Person[];
}

const AddMilestonePage = ({ form, people }: AddMilestonePageProps) => {
  // Filter to show all family members for milestones
  const children = people.filter(p => p.type === server.Child);
  const parents = people.filter(p => p.type === server.Parent);

  const categoryOptions = [
    { value: "development", label: "Development" },
    { value: "behavior", label: "Behavior" },
    { value: "health", label: "Health" },
    { value: "achievement", label: "Achievement" },
    { value: "first", label: "First Time" },
    { value: "other", label: "Other" },
  ];

  const selectedPerson = people.find(p => p.id === parseInt(form.selectedPersonId));

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
              {categoryOptions.map(option => (
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
