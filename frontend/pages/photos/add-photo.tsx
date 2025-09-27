import * as preact from "preact";
import * as vlens from "vlens";
import * as rpc from "vlens/rpc";
import * as auth from "../../lib/authCache";
import * as core from "vlens/core";
import * as server from "../../server";
import { Header, Footer } from "../../layout";
import { requireAuthInView } from "../../lib/authHelpers";
import { usePhotoStatus } from "../../hooks/usePhotoStatus";
import "./add-photo-styles";

type AddPhotoForm = {
  selectedPersonId: string;
  title: string;
  description: string;
  inputType: string; // 'auto' | 'today' | 'date' | 'age'
  photoDate: string;
  ageYears: string;
  ageMonths: string;
  selectedFile: File | null;
  previewUrl: string;
  error: string;
  loading: boolean;
  dragActive: boolean;
};

const useAddPhotoForm = vlens.declareHook(
  (personId?: string): AddPhotoForm => ({
    selectedPersonId: personId || "",
    title: "",
    description: "",
    inputType: "auto",
    photoDate: "",
    ageYears: "",
    ageMonths: "",
    selectedFile: null,
    previewUrl: "",
    error: "",
    loading: false,
    dragActive: false,
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
        <main id="app" className="add-photo-container">
          <div className="error-page">
            <h1>No Family Members</h1>
            <p>Please add family members before adding photos</p>
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

  // Extract person ID from URL if present (e.g., /add-photo/123)
  const urlParts = route.split("/");
  const personIdFromUrl = urlParts.length > 2 ? urlParts[2] : undefined;

  const form = useAddPhotoForm(personIdFromUrl);

  return (
    <div>
      <Header isHome={false} />
      <main id="app" className="add-photo-container">
        <AddPhotoPage form={form} people={data.people} />
      </main>
      <Footer />
    </div>
  );
}

async function onSubmitPhoto(form: AddPhotoForm, people: server.Person[], event: Event) {
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

  if (!form.selectedFile) {
    form.error = "Please select a photo to upload";
    form.loading = false;
    vlens.scheduleRedraw();
    return;
  }

  if (form.inputType === "date" && !form.photoDate) {
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
    // Prepare FormData for multipart upload
    const formData = new FormData();
    formData.append("personId", form.selectedPersonId);
    formData.append("title", form.title.trim());
    formData.append("description", form.description.trim());
    formData.append("inputType", form.inputType);
    formData.append("photo", form.selectedFile);

    // Add date/age specific fields
    if (form.inputType === "date") {
      formData.append("photoDate", form.photoDate);
    } else if (form.inputType === "age") {
      formData.append("ageYears", form.ageYears);
      if (form.ageMonths) {
        formData.append("ageMonths", form.ageMonths);
      }
    }

    // Verify authentication (auth is handled via cookies)
    const currentAuth = auth.getAuth();
    if (!currentAuth) {
      throw new Error("Authentication required");
    }

    // Call the backend API
    const response = await window.fetch("/api/upload-photo", {
      method: "POST",
      credentials: "include",
      body: formData,
    });

    if (!response.ok) {
      const errorText = await response.text();
      throw new Error(errorText || `Upload failed with status ${response.status}`);
    }

    const responseData = await response.json();

    // Start monitoring the uploaded photo for processing status
    if (responseData.image && responseData.image.status === 1) {
      const photoStatus = usePhotoStatus();
      photoStatus.startMonitoring(responseData.image.id, responseData.image.status);
    }

    // Redirect to profile page on success
    core.setRoute(`/profile/${form.selectedPersonId}`);
  } catch (error) {
    form.loading = false;
    form.error =
      error instanceof Error ? error.message : "Failed to upload photo. Please try again.";
    vlens.scheduleRedraw();
  }
}

function onInputTypeChange(form: AddPhotoForm, newType: string) {
  form.inputType = newType;
  vlens.scheduleRedraw();
}

function onFileSelect(form: AddPhotoForm, event: Event) {
  const target = event.target as HTMLInputElement;
  const file = target.files?.[0];

  if (file) {
    handleFileSelection(form, file);
  }
}

function handleFileSelection(form: AddPhotoForm, file: File) {
  // Validate file type
  if (!file.type.startsWith("image/")) {
    form.error = "Please select a valid image file";
    vlens.scheduleRedraw();
    return;
  }

  // Validate file size (max 10MB)
  if (file.size > 10 * 1024 * 1024) {
    form.error = "File size too large. Please select an image under 10MB";
    vlens.scheduleRedraw();
    return;
  }

  form.selectedFile = file;
  form.error = "";

  // Create preview URL
  const reader = new FileReader();
  reader.onload = e => {
    form.previewUrl = e.target?.result as string;
    vlens.scheduleRedraw();
  };
  reader.readAsDataURL(file);

  vlens.scheduleRedraw();
}

function onDragOver(form: AddPhotoForm, event: DragEvent) {
  event.preventDefault();
  form.dragActive = true;
  vlens.scheduleRedraw();
}

function onDragLeave(form: AddPhotoForm, event: DragEvent) {
  event.preventDefault();
  form.dragActive = false;
  vlens.scheduleRedraw();
}

function onDrop(form: AddPhotoForm, event: DragEvent) {
  event.preventDefault();
  form.dragActive = false;

  const files = event.dataTransfer?.files;
  if (files && files.length > 0) {
    handleFileSelection(form, files[0]);
  }
  vlens.scheduleRedraw();
}

function removeSelectedFile(form: AddPhotoForm) {
  form.selectedFile = null;
  form.previewUrl = "";
  vlens.scheduleRedraw();
}

interface AddPhotoPageProps {
  form: AddPhotoForm;
  people: server.Person[];
}

const AddPhotoPage = ({ form, people }: AddPhotoPageProps) => {
  // Filter to show all family members for photos
  const children = people.filter(p => p.type === server.Child);
  const parents = people.filter(p => p.type === server.Parent);

  const selectedPerson = people.find(p => p.id === parseInt(form.selectedPersonId));

  return (
    <div className="add-photo-page">
      <div className="auth-card">
        <div className="auth-header">
          <h1>Add Photo</h1>
          <p>Upload and share precious moments with your family</p>
        </div>

        {form.error && <div className="error-message">{form.error}</div>}

        <form className="auth-form" onSubmit={vlens.cachePartial(onSubmitPhoto, form, people)}>
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

          {/* File Upload */}
          <div className="form-group">
            <label>Photo</label>
            <div
              className={`file-upload-area ${form.dragActive ? "drag-active" : ""} ${
                form.selectedFile ? "has-file" : ""
              }`}
              onDragOver={vlens.cachePartial(onDragOver, form)}
              onDragLeave={vlens.cachePartial(onDragLeave, form)}
              onDrop={vlens.cachePartial(onDrop, form)}
            >
              {!form.selectedFile ? (
                <div className="upload-prompt">
                  <div className="upload-icon">📸</div>
                  <p>
                    Drag and drop a photo here, or{" "}
                    <label htmlFor="photo-input" className="upload-link">
                      browse
                    </label>
                  </p>
                  <small>Supports JPG, PNG, GIF up to 10MB</small>
                </div>
              ) : (
                <div className="file-preview">
                  {form.previewUrl && (
                    <img src={form.previewUrl} alt="Preview" className="preview-image" />
                  )}
                  <div className="file-info">
                    <p className="file-name">{form.selectedFile.name}</p>
                    <p className="file-size">
                      {(form.selectedFile.size / 1024 / 1024).toFixed(2)} MB
                    </p>
                    <button
                      type="button"
                      onClick={() => removeSelectedFile(form)}
                      className="remove-file"
                    >
                      Remove
                    </button>
                  </div>
                </div>
              )}
              <input
                id="photo-input"
                type="file"
                accept="image/*"
                onChange={vlens.cachePartial(onFileSelect, form)}
                disabled={form.loading}
                style={{ display: "none" }}
              />
            </div>
          </div>

          {/* Title */}
          <div className="form-group">
            <label htmlFor="title">Photo Title (Optional)</label>
            <input
              id="title"
              type="text"
              {...vlens.attrsBindInput(vlens.ref(form, "title"))}
              placeholder="Leave empty to auto-generate from date or filename"
              disabled={form.loading}
            />
          </div>

          {/* Description */}
          <div className="form-group">
            <label htmlFor="description">Description (Optional)</label>
            <textarea
              id="description"
              {...vlens.attrsBindInput(vlens.ref(form, "description"))}
              placeholder="Add any details about this photo..."
              rows={3}
              disabled={form.loading}
            />
          </div>

          {/* Date or Age Toggle */}
          <div className="form-group">
            <label>When was this photo taken?</label>
            <div className="radio-group">
              <label className="radio-option">
                <input
                  type="radio"
                  name="inputType"
                  value="auto"
                  checked={form.inputType === "auto"}
                  onChange={() => onInputTypeChange(form, "auto")}
                  disabled={form.loading}
                />
                <span>Auto (from photo)</span>
              </label>
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
                {...vlens.attrsBindInput(vlens.ref(form, "photoDate"))}
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
            <button
              type="submit"
              className="btn btn-primary auth-submit"
              disabled={form.loading || !form.selectedFile}
            >
              {form.loading ? "Uploading..." : "Upload Photo"}
            </button>
          </div>
        </form>

        {selectedPerson && form.title && form.selectedFile && (
          <div className="photo-preview">
            <h3>Preview</h3>
            <p>
              <strong>{form.title}</strong> - {selectedPerson.name}
              {form.inputType === "today" && <span> (today)</span>}
              {form.inputType === "date" && form.photoDate && (
                <span> ({new Date(form.photoDate).toLocaleDateString()})</span>
              )}
              {form.inputType === "age" && form.ageYears && (
                <span>
                  {" "}
                  (age {form.ageYears}
                  {form.ageMonths ? `.${form.ageMonths}` : ""} years)
                </span>
              )}
            </p>
            {form.description && <p className="preview-description">{form.description}</p>}
          </div>
        )}
      </div>
    </div>
  );
};
