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
      when: newRecordWhen(),
      tagIds: [],
      loading: false,
      error: "",
    };
  }

  return {
    title: photo.title || "",
    description: photo.description || "",
    when: recordWhenFrom(photo.recordTime, photo.photoDate),
    tagIds: photo.tagIds ?? [],
    loading: false,
    error: "",
  };
});

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

  const anchor = people.find(p => p.id === form.when.anchorPersonId);
  const problem = recordWhenProblem(form.when, false, anchor?.birthday, new Date());
  if (problem) {
    form.error = problem;
    form.loading = false;
    vlens.scheduleRedraw();
    return;
  }
  await saveEdit(form, photo, recordWhenTime(form.when, new Date(), false));
}

async function saveEdit(form: EditPhotoForm, photo: server.Image, recordTime: server.RecordTime) {
  try {
    const updateRequest: server.UpdatePhotoRequest = {
      id: photo.id,
      title: form.title.trim(),
      description: form.description.trim(),
      recordTime,
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

          <div className="form-group">
            <span className="form-group-caption">When was this photo taken?</span>
            <RecordWhenControl
              when={form.when}
              isMoment={false}
              disabled={form.loading}
              birthday={anchor && !anchor.isPregnancy ? anchor.birthday : undefined}
              anchors={people}
            />
          </div>

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
