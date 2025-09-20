import * as preact from "preact";
import * as vlens from "vlens";
import * as core from "vlens/core";
import * as auth from "./authCache";
import * as server from "./server";
import { Header, Footer } from "./layout";
import "./edit-photo-styles";

export async function fetch(route: string, prefix: string) {
  const photoId = parseInt(route.split('/')[2]);
  return server.GetPhoto({ id: photoId });
}

type EditPhotoData = server.GetPhotoResponse | { image: null };

type EditPhotoForm = {
  title: string;
  description: string;
  inputType: string; // 'auto' | 'today' | 'date' | 'age'
  photoDate: string;
  ageYears: string;
  ageMonths: string;
  loading: boolean;
  error: string;
}

const useEditPhotoForm = vlens.declareHook((photo?: server.Image): EditPhotoForm => {
  if (!photo) {
    return {
      title: "",
      description: "",
      inputType: "auto",
      photoDate: "",
      ageYears: "",
      ageMonths: "",
      loading: false,
      error: ""
    };
  }

  // Convert photo date to form format
  const photoDate = photo.photoDate ? new Date(photo.photoDate).toISOString().split('T')[0] : "";

  return {
    title: photo.title || "",
    description: photo.description || "",
    inputType: "date", // Default to date since we have a known date
    photoDate: photoDate,
    ageYears: "",
    ageMonths: "",
    loading: false,
    error: ""
  };
});

function onInputTypeChange(form: EditPhotoForm, inputType: string) {
  form.inputType = inputType;
  form.error = "";
  vlens.scheduleRedraw();
}

export function view(
  route: string,
  prefix: string,
  data: EditPhotoData,
): preact.ComponentChild {
  const currentAuth = auth.getAuth();
  if (!currentAuth || currentAuth.id <= 0) {
    auth.clearAuth();
    core.setRoute('/login');
    return;
  }

  if (!data.image) {
    return (
      <div>
        <Header isHome={false} />
        <main id="app" className="edit-photo-container">
          <div className="error-page">
            <h1>Error</h1>
            <p>Photo not found or access denied</p>
            <a href="/dashboard" className="btn btn-primary">Back to Dashboard</a>
          </div>
        </main>
        <Footer />
      </div>
    );
  }

  const form = useEditPhotoForm(data.image);

  return (
    <div>
      <Header isHome={false} />
      <main id="app" className="edit-photo-container">
        <EditPhotoPage form={form} photo={data.image} />
      </main>
      <Footer />
    </div>
  );
}

async function onSubmitEdit(form: EditPhotoForm, photo: server.Image, event: Event) {
  event.preventDefault();
  form.loading = true;
  form.error = "";

  // Validation
  if (form.inputType === 'date' && !form.photoDate) {
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

  try {
    const updateRequest: server.UpdatePhotoRequest = {
      id: photo.id,
      title: form.title.trim(),
      description: form.description.trim(),
      inputType: form.inputType,
      photoDate: form.photoDate,
      ageYears: form.ageYears ? parseInt(form.ageYears) : null,
      ageMonths: form.ageMonths ? parseInt(form.ageMonths) : null,
    };

    const [resp, err] = await server.UpdatePhoto(updateRequest);

    if (err) {
      form.error = err;
      form.loading = false;
      vlens.scheduleRedraw();
      return;
    }

    if (resp && resp.image) {
      // Success! Navigate to view photo page
      core.setRoute(`/view-photo/${photo.id}`);
    } else {
      form.error = "Failed to update photo";
      form.loading = false;
      vlens.scheduleRedraw();
    }
  } catch (error) {
    form.error = "Failed to update photo";
    form.loading = false;
    vlens.scheduleRedraw();
  }
}

interface EditPhotoPageProps {
  form: EditPhotoForm;
  photo: server.Image;
}

const formatPhotoDate = (dateString: string) => {
  if (!dateString) return '';
  if (dateString.includes('T') && dateString.endsWith('Z')) {
    const dateParts = dateString.split('T')[0].split('-');
    const year = parseInt(dateParts[0]);
    const month = parseInt(dateParts[1]) - 1; // Month is 0-indexed
    const day = parseInt(dateParts[2]);
    return new Date(year, month, day).toLocaleDateString();
  }
  return new Date(dateString).toLocaleDateString();
};

const EditPhotoPage = ({ form, photo }: EditPhotoPageProps) => {
  return (
    <div className="edit-photo-page">
      <div className="auth-card">
        <div className="auth-header">
          <h1>Edit Photo</h1>
          <p>Update photo details and information</p>
        </div>

        {/* Photo preview */}
        <div className="photo-preview">
          <img
            src={`/api/photo/${photo.id}/thumb`}
            alt={photo.title}
            className="preview-image"
          />
          <div className="photo-info">
            <div><strong>Current Date:</strong> {formatPhotoDate(photo.photoDate)}</div>
            <div><strong>Uploaded:</strong> {formatPhotoDate(photo.createdAt)}</div>
          </div>
        </div>

        {form.error && (
          <div className="error-message">{form.error}</div>
        )}

        <form className="auth-form" onSubmit={vlens.cachePartial(onSubmitEdit, form, photo)}>
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
                  checked={form.inputType === 'auto'}
                  onChange={() => onInputTypeChange(form, 'auto')}
                  disabled={form.loading}
                />
                <span>Auto (from photo)</span>
              </label>
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
              <label htmlFor="date">Date</label>
              <input
                id="date"
                type="date"
                {...vlens.attrsBindInput(vlens.ref(form, "photoDate"))}
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
                  placeholder="6"
                  disabled={form.loading}
                />
              </div>
            </div>
          )}

          {/* Action Buttons */}
          <div className="form-actions">
            <a href={`/view-photo/${photo.id}`} className="btn btn-secondary">
              Cancel
            </a>
            <button type="submit" className="btn btn-primary auth-submit" disabled={form.loading}>
              {form.loading ? "Saving..." : "Save Changes"}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
};