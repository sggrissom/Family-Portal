import * as preact from "preact";
import * as vlens from "vlens";
import * as rpc from "vlens/rpc";
import * as auth from "../../lib/authCache";
import * as core from "vlens/core";
import * as server from "../../server";
import { Header, Footer } from "../../layout";
import { requireAuthInView } from "../../lib/authHelpers";
import { usePhotoStatus } from "../../hooks/usePhotoStatus";
import { getIdFromRoute, personSubtitle } from "../../lib/routeHelpers";
import { NoFamilyMembersPage } from "../../components/NoFamilyMembersPage";
import {
  failureSummary,
  pendingPhotos,
  photoFileProblem,
  type QueuedPhoto,
} from "../../lib/photoUploadQueue";
import "./add-photo-styles";

type AddPhotoForm = {
  selectedPersonIds: string[];
  title: string;
  description: string;
  inputType: string;
  photoDate: string;
  ageYears: string;
  ageMonths: string;
  tagIds: number[];
  photos: QueuedPhoto[];
  batchTotal: number;
  batchDone: number;
  error: string;
  loading: boolean;
  dragActive: boolean;
};

const useAddPhotoForm = vlens.declareHook(
  (personId?: string): AddPhotoForm => ({
    selectedPersonIds: personId ? [personId] : [],
    title: "",
    description: "",
    inputType: "auto",
    photoDate: "",
    ageYears: "",
    ageMonths: "",
    tagIds: [],
    photos: [],
    batchTotal: 0,
    batchDone: 0,
    error: "",
    loading: false,
    dragActive: false,
  })
);

type AddPhotoData = {
  people: server.ListPeopleResponse;
  tags: server.Tag[];
};

export async function fetch(route: string, prefix: string): Promise<rpc.Response<AddPhotoData>> {
  const [people, peopleErr] = await server.ListPeople({});
  if (peopleErr) return [null, peopleErr];
  const [tags, tagsErr] = await server.ListTags({});
  if (tagsErr) return [null, tagsErr];
  return [{ people: people!, tags: tags!.tags }, ""];
}

export function view(route: string, prefix: string, data: AddPhotoData): preact.ComponentChild {
  const currentAuth = requireAuthInView();
  if (!currentAuth) {
    return;
  }

  if (!data.people.people || data.people.people.length === 0) {
    return (
      <NoFamilyMembersPage
        message="Please add family members before adding photos"
        containerClass="add-photo-container"
      />
    );
  }

  const personId = getIdFromRoute(route);
  const personIdFromUrl = personId ? personId.toString() : undefined;

  const form = useAddPhotoForm(personIdFromUrl);

  return (
    <div>
      <Header isHome={false} />
      <main id="app" className="add-photo-container">
        <AddPhotoPage form={form} people={data.people.people} tags={data.tags} />
      </main>
      <Footer />
    </div>
  );
}

async function uploadErrorMessage(response: Response): Promise<string> {
  const fallback = `Upload failed with status ${response.status}`;
  const body = await response.text();
  if (!body) return fallback;

  try {
    const parsed = JSON.parse(body);
    return parsed?.error?.message || fallback;
  } catch {
    return body;
  }
}

async function uploadPhoto(
  form: AddPhotoForm,
  personIds: number[],
  familyId: number | undefined,
  file: File
): Promise<void> {
  const formData = new FormData();
  formData.append("personIds", JSON.stringify(personIds));
  if (familyId !== undefined) {
    formData.append("familyId", String(familyId));
  }
  formData.append("title", form.title.trim());
  formData.append("description", form.description.trim());
  formData.append("inputType", form.inputType);
  formData.append("photo", file);

  if (form.inputType === "date") {
    formData.append("photoDate", form.photoDate);
  } else if (form.inputType === "age") {
    formData.append("ageYears", form.ageYears);
    if (form.ageMonths) {
      formData.append("ageMonths", form.ageMonths);
    }
  }

  const response = await window.fetch("/api/upload-photo", {
    method: "POST",
    credentials: "include",
    body: formData,
  });

  if (!response.ok) {
    throw new Error(await uploadErrorMessage(response));
  }

  const responseData = await response.json();

  if (responseData.image && responseData.image.status === 1) {
    usePhotoStatus().startMonitoring(responseData.image.id, responseData.image.status);
  }

  if (form.tagIds.length > 0 && responseData.image?.id) {
    await server.UpdatePhotoTags({ photoId: responseData.image.id, tagIds: form.tagIds });
  }
}

async function onSubmitPhoto(form: AddPhotoForm, people: server.Person[], event: Event) {
  event.preventDefault();
  form.error = "";

  const queue = pendingPhotos(form.photos);
  if (queue.length === 0) {
    form.error = "Please select a photo to upload";
    vlens.scheduleRedraw();
    return;
  }

  if (form.inputType === "date" && !form.photoDate) {
    form.error = "Please select a date";
    vlens.scheduleRedraw();
    return;
  }

  if (form.inputType === "age" && (form.ageYears === "" || parseInt(form.ageYears) < 0)) {
    form.error = "Please enter a valid age";
    vlens.scheduleRedraw();
    return;
  }

  if (!auth.getAuth()) {
    form.error = "Authentication required";
    vlens.scheduleRedraw();
    return;
  }

  const personIds = form.selectedPersonIds.map(id => parseInt(id)).filter(id => !isNaN(id));
  const familyId = people.find(p => personIds.includes(p.id))?.familyId;

  form.loading = true;
  form.batchTotal = queue.length;
  form.batchDone = 0;

  for (const photo of queue) {
    photo.state = "uploading";
    photo.error = "";
    vlens.scheduleRedraw();

    try {
      await uploadPhoto(form, personIds, familyId, photo.file);
      photo.state = "done";
    } catch (error) {
      photo.state = "failed";
      photo.error =
        error instanceof Error ? error.message : "Failed to upload photo. Please try again.";
    }
    form.batchDone++;
  }

  form.loading = false;

  const firstFailure = form.photos.find(p => p.state === "failed");
  if (firstFailure) {
    form.error = failureSummary(form.photos) || firstFailure.error;
    vlens.scheduleRedraw();
    return;
  }

  form.photos.forEach(p => URL.revokeObjectURL(p.previewUrl));

  if (form.selectedPersonIds.length === 1) {
    core.setRoute(`/profile/${form.selectedPersonIds[0]}`);
  } else {
    core.setRoute("/photos");
  }
}

function onToggleTag(form: AddPhotoForm, tagId: number) {
  const idx = form.tagIds.indexOf(tagId);
  if (idx >= 0) form.tagIds.splice(idx, 1);
  else form.tagIds.push(tagId);
  vlens.scheduleRedraw();
}

function onInputTypeChange(form: AddPhotoForm, newType: string) {
  form.inputType = newType;
  vlens.scheduleRedraw();
}

function onPersonToggle(form: AddPhotoForm, personId: string) {
  const index = form.selectedPersonIds.indexOf(personId);

  if (index === -1) {
    form.selectedPersonIds.push(personId);
  } else {
    form.selectedPersonIds.splice(index, 1);
  }

  vlens.scheduleRedraw();
}

function onFileSelect(form: AddPhotoForm, event: Event) {
  const target = event.target as HTMLInputElement;
  if (target.files) {
    addFiles(form, target.files);
  }
  target.value = "";
}

function addFiles(form: AddPhotoForm, files: FileList) {
  const problems: string[] = [];

  for (const file of Array.from(files)) {
    const problem = photoFileProblem(file);
    if (problem) {
      problems.push(problem);
      continue;
    }
    form.photos.push({
      file,
      previewUrl: URL.createObjectURL(file),
      state: "queued",
      error: "",
    });
  }

  form.error = problems.length > 0 ? `Skipped: ${problems.join("; ")}` : "";
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
    addFiles(form, files);
  }
  vlens.scheduleRedraw();
}

function removePhoto(form: AddPhotoForm, photo: QueuedPhoto) {
  const index = form.photos.indexOf(photo);
  if (index >= 0) {
    form.photos.splice(index, 1);
    URL.revokeObjectURL(photo.previewUrl);
  }
  vlens.scheduleRedraw();
}

function uploadStatusLabel(photo: QueuedPhoto): string {
  switch (photo.state) {
    case "uploading":
      return "Uploading…";
    case "done":
      return "Uploaded";
    case "failed":
      return photo.error;
    default:
      return `${(photo.file.size / 1024 / 1024).toFixed(2)} MB`;
  }
}

function submitLabel(form: AddPhotoForm): string {
  if (form.loading) {
    return form.batchTotal > 1
      ? `Uploading ${Math.min(form.batchDone + 1, form.batchTotal)} of ${form.batchTotal}...`
      : "Uploading...";
  }
  const count = pendingPhotos(form.photos).length;
  return count > 1 ? `Upload ${count} Photos` : "Upload Photo";
}

interface AddPhotoPageProps {
  form: AddPhotoForm;
  people: server.Person[];
  tags: server.Tag[];
}

const AddPhotoPage = ({ form, people, tags }: AddPhotoPageProps) => {
  const selectedPersonIds = new Set(form.selectedPersonIds.map(id => parseInt(id)));
  const hasPhotos = form.photos.length > 0;
  const isBatch = form.photos.length > 1;

  return (
    <div className="add-photo-page">
      <div className="auth-card">
        <div className="auth-header">
          <h1>Add Photos</h1>
          <p>Upload and share precious moments with your family</p>
        </div>

        {form.error && (
          <div className="error-message" role="alert">
            {form.error}
          </div>
        )}

        <form className="auth-form" onSubmit={vlens.cachePartial(onSubmitPhoto, form, people)}>
          <div className="form-group">
            <label>Who's in this photo? (Optional)</label>
            <p className="form-hint">
              Select family members who appear in this photo. Leave unchecked for general family
              photos.
            </p>

            <div className="photo-person-group">
              {people.map(person => (
                <label key={person.id} className="photo-person-option">
                  <input
                    type="checkbox"
                    checked={selectedPersonIds.has(person.id)}
                    onChange={() => onPersonToggle(form, person.id.toString())}
                    disabled={form.loading}
                  />
                  <span>
                    {person.name} ({personSubtitle(person)})
                  </span>
                </label>
              ))}
            </div>
          </div>

          <div className="form-group">
            <label>Photos</label>
            <div
              className={`file-upload-area ${form.dragActive ? "drag-active" : ""} ${
                hasPhotos ? "has-file" : ""
              }`}
              onDragOver={vlens.cachePartial(onDragOver, form)}
              onDragLeave={vlens.cachePartial(onDragLeave, form)}
              onDrop={vlens.cachePartial(onDrop, form)}
            >
              {!hasPhotos ? (
                <div className="upload-prompt">
                  <div className="upload-icon">📸</div>
                  <p>
                    Drag and drop photos here, or{" "}
                    <label htmlFor="photo-input" className="upload-link">
                      browse
                    </label>
                  </p>
                  <small>Supports JPG, PNG, GIF up to 10MB each</small>
                </div>
              ) : (
                <div className="file-preview-list">
                  {form.photos.map(photo => (
                    <div key={photo.previewUrl} className={`file-preview upload-${photo.state}`}>
                      <img src={photo.previewUrl} alt="" className="preview-image" />
                      <div className="file-info">
                        <p className="file-name">{photo.file.name}</p>
                        <p className="file-size">{uploadStatusLabel(photo)}</p>
                        {photo.state !== "done" && (
                          <button
                            type="button"
                            onClick={() => removePhoto(form, photo)}
                            className="remove-file"
                            disabled={form.loading}
                          >
                            Remove
                          </button>
                        )}
                      </div>
                    </div>
                  ))}
                  {!form.loading && (
                    <label htmlFor="photo-input" className="upload-link add-more-photos">
                      Add more photos
                    </label>
                  )}
                </div>
              )}
              <input
                id="photo-input"
                type="file"
                accept="image/*"
                multiple
                onChange={vlens.cachePartial(onFileSelect, form)}
                disabled={form.loading}
                style={{ display: "none" }}
              />
            </div>
            {isBatch && (
              <p className="form-hint">Everything else on this form applies to every photo.</p>
            )}
          </div>

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

          {tags.length > 0 && (
            <div className="form-group">
              <span className="form-group-caption" id="tagPickerLabel">
                Tags
              </span>
              <div className="tag-picker" role="group" aria-labelledby="tagPickerLabel">
                {tags.map(tag => {
                  const selected = form.tagIds.includes(tag.id);
                  return (
                    <button
                      key={tag.id}
                      type="button"
                      className={`tag-pill${selected ? " selected" : ""}`}
                      style={{ borderColor: tag.color }}
                      aria-pressed={selected}
                      onClick={vlens.cachePartial(onToggleTag, form, tag.id)}
                    >
                      <span className="tag-color-dot" style={{ background: tag.color }} />
                      {tag.name}
                    </button>
                  );
                })}
              </div>
            </div>
          )}

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
                  disabled={form.loading || form.selectedPersonIds.length === 0}
                />
                <span>
                  At Age {form.selectedPersonIds.length === 0 && "(requires person selection)"}
                </span>
              </label>
            </div>
          </div>

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
            <a href="/dashboard" className="btn btn-secondary">
              Cancel
            </a>
            <button
              type="submit"
              className="btn btn-primary auth-submit"
              disabled={form.loading || pendingPhotos(form.photos).length === 0}
            >
              {submitLabel(form)}
            </button>
          </div>
        </form>

        {form.title && hasPhotos && (
          <div className="photo-preview">
            <h3>Preview</h3>
            <p>
              <strong>{form.title}</strong>
              {form.selectedPersonIds.length > 0 && (
                <span>
                  {" "}
                  -{" "}
                  {form.selectedPersonIds
                    .map(id => {
                      const person = people.find(p => p.id === parseInt(id));
                      return person?.name;
                    })
                    .filter(Boolean)
                    .join(", ")}
                </span>
              )}
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
