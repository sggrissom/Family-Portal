import { localDateString } from "../../lib/when";
import { formatDate as formatPhotoDate } from "../../lib/dateUtils";
import * as preact from "preact";
import * as vlens from "vlens";
import { navigate } from "../../lib/navigate";
import * as rpc from "vlens/rpc";
import * as auth from "../../lib/authCache";
import * as server from "../../server";
import { Header, Footer } from "../../layout";
import { ThumbnailImage } from "../../components/ResponsiveImage";
import { ViewOnlyPage } from "../../components/ErrorPage";
import "./edit-photo-styles";

import { getIdFromRoute } from "../../lib/routeHelpers";
import { recordDateLabel } from "../../lib/recordTime";
import {
  RecordWhen,
  newRecordWhen,
  recordWhenFrom,
  recordWhenProblem,
  recordWhenTime,
} from "../../lib/recordWhen";
import { approximateDatesEnabled } from "../../lib/recordTimeSettings";
import { RecordWhenControl } from "../../components/RecordWhenControl";

type EditPhotoData = {
  photo: server.GetPhotoResponse;
  tags: server.Tag[];
};

export async function fetch(route: string, prefix: string): Promise<rpc.Response<EditPhotoData>> {
  const photoId = getIdFromRoute(route) || 0;
  const [photo, photoErr] = await server.GetPhoto({ id: photoId });
  if (photoErr) return [null, photoErr];
  const [tagsResp] = await server.ListTags({});
  return [{ photo: photo!, tags: tagsResp?.tags ?? [] }, ""];
}

type EditPhotoForm = {
  title: string;
  description: string;
  inputType: string;
  photoDate: string;
  ageYears: string;
  ageMonths: string;
  when: RecordWhen;
  tagIds: number[];
  loading: boolean;
  error: string;
};

const useEditPhotoForm = vlens.declareHook((photo?: server.Image): EditPhotoForm => {
  if (!photo) {
    return {
      title: "",
      description: "",
      inputType: "auto",
      photoDate: "",
      ageYears: "",
      ageMonths: "",
      when: newRecordWhen(),
      tagIds: [],
      loading: false,
      error: "",
    };
  }

  const photoDate = photo.photoDate ? new Date(photo.photoDate).toISOString().split("T")[0] : "";

  return {
    title: photo.title || "",
    description: photo.description || "",
    inputType: "date",
    photoDate: photoDate,
    ageYears: "",
    ageMonths: "",
    when: recordWhenFrom(photo.recordTime, photo.photoDate),
    tagIds: photo.tagIds ?? [],
    loading: false,
    error: "",
  };
});

function onInputTypeChange(form: EditPhotoForm, inputType: string) {
  form.inputType = inputType;
  form.error = "";
  vlens.scheduleRedraw();
}

function onToggleTag(form: EditPhotoForm, tagId: number) {
  const idx = form.tagIds.indexOf(tagId);
  if (idx >= 0) form.tagIds.splice(idx, 1);
  else form.tagIds.push(tagId);
  vlens.scheduleRedraw();
}

export function view(route: string, prefix: string, data: EditPhotoData): preact.ComponentChild {
  const currentAuth = auth.getAuth();
  if (!currentAuth || currentAuth.id <= 0) {
    auth.clearAuth();
    navigate("/login");
    return;
  }

  if (!data.photo || !data.photo.image) {
    return (
      <div>
        <Header isHome={false} />
        <main id="app" className="edit-photo-container">
          <div className="error-page">
            <h1>Error</h1>
            <p>Photo not found or access denied</p>
            <a href="/dashboard" className="btn btn-primary">
              Back to Dashboard
            </a>
          </div>
        </main>
        <Footer />
      </div>
    );
  }

  if (!auth.canContribute(data.photo.image.familyId)) {
    return <ViewOnlyPage backLink={`/view-photo/${data.photo.image.id}`} />;
  }

  const form = useEditPhotoForm(data.photo.image);

  return (
    <div>
      <Header isHome={false} />
      <main id="app" className="edit-photo-container">
        <EditPhotoPage
          form={form}
          photo={data.photo.image}
          people={data.photo.people ?? []}
          allTags={data.tags}
        />
      </main>
      <Footer />
    </div>
  );
}

async function onSubmitEdit(
  form: EditPhotoForm,
  photo: server.Image,
  people: server.Person[],
  event: Event
) {
  event.preventDefault();
  form.loading = true;
  form.error = "";

  if (approximateDatesEnabled()) {
    const anchor = people.find(p => p.id === form.when.anchorPersonId);
    const problem = recordWhenProblem(form.when, false, anchor?.birthday, new Date());
    if (problem) {
      form.error = problem;
      form.loading = false;
      vlens.scheduleRedraw();
      return;
    }
    await saveEdit(form, photo, {
      inputType: "keep",
      photoDate: "",
      ageYears: null,
      ageMonths: null,
      recordTime: recordWhenTime(form.when, new Date(), false),
    });
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

  await saveEdit(form, photo, {
    inputType: form.inputType === "today" ? "date" : form.inputType,
    photoDate: form.inputType === "today" ? localDateString(new Date()) : form.photoDate,
    ageYears: form.ageYears ? parseInt(form.ageYears) : null,
    ageMonths: form.ageMonths ? parseInt(form.ageMonths) : null,
    recordTime: null,
  });
}

type DateFields = Omit<server.UpdatePhotoRequest, "id" | "title" | "description">;

async function saveEdit(form: EditPhotoForm, photo: server.Image, date: DateFields) {
  try {
    const updateRequest: server.UpdatePhotoRequest = {
      id: photo.id,
      title: form.title.trim(),
      description: form.description.trim(),
      ...date,
    };

    const [resp, err] = await server.UpdatePhoto(updateRequest);

    if (err) {
      form.error = err;
      form.loading = false;
      vlens.scheduleRedraw();
      return;
    }

    if (resp && resp.image) {
      await server.UpdatePhotoTags({ photoId: photo.id, tagIds: form.tagIds });
      navigate(`/view-photo/${photo.id}`);
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
  people: server.Person[];
  allTags: server.Tag[];
}

const EditPhotoPage = ({ form, photo, people, allTags }: EditPhotoPageProps) => {
  const approximate = approximateDatesEnabled();
  if (form.when.anchorPersonId === 0 && people.length === 1)
    form.when.anchorPersonId = people[0].id;
  const anchor = people.find(p => p.id === form.when.anchorPersonId);
  return (
    <div className="edit-photo-page">
      <div className="auth-card">
        <div className="auth-header">
          <h1>Edit Photo</h1>
          <p>Update photo details and information</p>
        </div>

        <div className="photo-preview">
          <ThumbnailImage photoId={photo.id} alt={photo.title} className="preview-image" />
          <div className="photo-info">
            <div>
              <strong>Current Date:</strong>{" "}
              {recordDateLabel(photo.recordTime, photo.photoDate, formatPhotoDate)}
            </div>
            <div>
              <strong>Uploaded:</strong> {formatPhotoDate(photo.createdAt)}
            </div>
          </div>
        </div>

        {form.error && (
          <div className="error-message" role="alert">
            {form.error}
          </div>
        )}

        <form
          className="auth-form"
          onSubmit={vlens.cachePartial(onSubmitEdit, form, photo, people)}
        >
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

          {approximate && (
            <div className="form-group">
              <span className="form-group-caption">When was this photo taken?</span>
              <RecordWhenControl
                when={form.when}
                approximate={true}
                isMoment={false}
                disabled={form.loading}
                birthday={anchor && !anchor.isPregnancy ? anchor.birthday : undefined}
                anchors={people}
              />
            </div>
          )}

          {!approximate && (
            <>
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

              {form.inputType === "date" && (
                <div className="form-group">
                  <label htmlFor="date">Date</label>
                  <input
                    id="date"
                    type="date"
                    {...vlens.attrsBindInput(vlens.ref(form, "photoDate"))}
                    max={localDateString(new Date())}
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
                      placeholder="6"
                      disabled={form.loading}
                    />
                  </div>
                </div>
              )}
            </>
          )}

          {allTags.length > 0 && (
            <div className="form-group">
              <span className="form-group-caption" id="tagPickerLabel">
                Tags
              </span>
              <div className="tag-picker" role="group" aria-labelledby="tagPickerLabel">
                {allTags.map(tag => {
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
