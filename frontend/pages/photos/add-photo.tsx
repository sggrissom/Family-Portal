import * as preact from "preact";
import * as vlens from "vlens";
import * as rpc from "vlens/rpc";
import * as auth from "../../lib/authCache";
import * as core from "vlens/core";
import * as server from "../../server";
import { Header, Footer } from "../../layout";
import { requireAuthInView } from "../../lib/authHelpers";
import { usePhotoStatus } from "../../hooks/usePhotoStatus";
import { getIdFromRoute } from "../../lib/routeHelpers";
import { chipOrder } from "../../lib/familyGroups";
import { copy } from "../../lib/copy";
import { localDateString } from "../../lib/when";
import { readLastPerson, returnPath, takeHandedOffPhotos, takeReturnPath } from "../../lib/addFlow";
import { NoFamilyMembersPage } from "../../components/NoFamilyMembersPage";
import { PersonChips, scrollSelectedChipIntoView } from "../../components/PersonChips";
import {
  peopleChanges,
  photoFileProblem,
  takenLabel,
  type QueuedPhoto,
} from "../../lib/photoUploadQueue";
import "./add-photo-styles";
import "../../components/entry-form-styles";

type UploadItem = QueuedPhoto & {
  image: server.Image | null;
  taggedAtUpload: number[];
};

type AddPhotoForm = {
  personIds: number[];
  caption: string;
  tagIds: number[];
  items: UploadItem[];
  pumping: boolean;
  editingDate: boolean;
  newDate: string;
  finishing: boolean;
  dragActive: boolean;
  handedOff: boolean;
  error: string;
};

const useAddPhotoForm = vlens.declareHook((personId: number | null): AddPhotoForm => {
  scrollSelectedChipIntoView();
  return {
    personIds: personId ? [personId] : [],
    caption: "",
    tagIds: [],
    items: [],
    pumping: false,
    editingDate: false,
    newDate: "",
    finishing: false,
    dragActive: false,
    handedOff: false,
    error: "",
  };
});

type AddPhotoData = {
  people: server.Person[];
  tags: server.Tag[];
};

export async function fetch(route: string, prefix: string): Promise<rpc.Response<AddPhotoData>> {
  const [people, peopleErr] = await server.ListPeople({});
  if (!people) return [null, peopleErr];
  const [tags, tagsErr] = await server.ListTags({});
  if (!tags) return [null, tagsErr];
  const ordered = chipOrder(
    people.people || [],
    people.relations || [],
    auth.getAuth()?.familyId ?? 0
  );
  return [{ people: ordered, tags: tags.tags }, ""];
}

export function view(route: string, prefix: string, data: AddPhotoData): preact.ComponentChild {
  const currentAuth = requireAuthInView();
  if (!currentAuth) {
    return;
  }

  if (data.people.length === 0) {
    return (
      <NoFamilyMembersPage
        message="Please add family members before adding photos"
        containerClass="entry-container"
      />
    );
  }

  const fromRoute = getIdFromRoute(route);
  const initial = [fromRoute, readLastPerson()].find(
    id => id !== null && data.people.some(p => p.id === id)
  );
  const form = useAddPhotoForm(initial ?? null);

  if (!form.handedOff) {
    form.handedOff = true;
    const files = takeHandedOffPhotos();
    if (files.length > 0) addFiles(form, data.people, files);
  }

  return (
    <div>
      <Header isHome={false} />
      <main id="app" className="entry-container">
        <AddPhotoPage form={form} people={data.people} tags={data.tags} />
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
  file: File,
  personIds: number[],
  familyId: number | undefined
): Promise<server.Image> {
  const formData = new FormData();
  formData.append("personIds", JSON.stringify(personIds));
  if (familyId !== undefined) {
    formData.append("familyId", String(familyId));
  }
  formData.append("inputType", "auto");
  formData.append("photo", file);

  const response = await window.fetch("/api/upload-photo", {
    method: "POST",
    credentials: "include",
    body: formData,
  });

  if (!response.ok) {
    throw new Error(await uploadErrorMessage(response));
  }

  const image: server.Image = (await response.json()).image;
  if (image.status === 1) {
    usePhotoStatus().startMonitoring(image.id, image.status);
  }
  return image;
}

function addFiles(form: AddPhotoForm, people: server.Person[], files: File[]) {
  const problems: string[] = [];
  for (const file of files) {
    const problem = photoFileProblem(file);
    if (problem) {
      problems.push(problem);
      continue;
    }
    form.items.push({
      file,
      previewUrl: URL.createObjectURL(file),
      state: "queued",
      error: "",
      image: null,
      taggedAtUpload: [],
    });
  }
  form.error = problems.length > 0 ? `Skipped: ${problems.join("; ")}` : "";
  vlens.scheduleRedraw();
  pump(form, people);
}

async function pump(form: AddPhotoForm, people: server.Person[]) {
  if (form.pumping) return;
  form.pumping = true;

  let next: UploadItem | undefined;
  while ((next = form.items.find(item => item.state === "queued"))) {
    const item = next;
    item.state = "uploading";
    item.taggedAtUpload = [...form.personIds];
    vlens.scheduleRedraw();

    const familyId = people.find(p => item.taggedAtUpload.includes(p.id))?.familyId;
    try {
      item.image = await uploadPhoto(item.file, item.taggedAtUpload, familyId);
      item.state = "done";
    } catch (error) {
      item.state = "failed";
      item.error = error instanceof Error ? error.message : "The upload failed.";
    }
    vlens.scheduleRedraw();
  }

  form.pumping = false;
  if (form.finishing) {
    finish(form);
  }
}

function onFileSelect(form: AddPhotoForm, people: server.Person[], event: Event) {
  const target = event.target as HTMLInputElement;
  const files = Array.from(target.files ?? []);
  target.value = "";
  if (files.length > 0) addFiles(form, people, files);
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

function onDrop(form: AddPhotoForm, people: server.Person[], event: DragEvent) {
  event.preventDefault();
  form.dragActive = false;
  const files = Array.from(event.dataTransfer?.files ?? []);
  if (files.length > 0) addFiles(form, people, files);
  vlens.scheduleRedraw();
}

function retry(form: AddPhotoForm, people: server.Person[], item: UploadItem) {
  item.state = "queued";
  item.error = "";
  form.error = "";
  pump(form, people);
}

function remove(form: AddPhotoForm, item: UploadItem) {
  const index = form.items.indexOf(item);
  if (index >= 0) form.items.splice(index, 1);
  URL.revokeObjectURL(item.previewUrl);
  vlens.scheduleRedraw();
}

function togglePerson(form: AddPhotoForm, personId: number) {
  const index = form.personIds.indexOf(personId);
  if (index >= 0) form.personIds.splice(index, 1);
  else form.personIds.push(personId);
  vlens.scheduleRedraw();
}

function toggleTag(form: AddPhotoForm, tagId: number) {
  const index = form.tagIds.indexOf(tagId);
  if (index >= 0) form.tagIds.splice(index, 1);
  else form.tagIds.push(tagId);
  vlens.scheduleRedraw();
}

function editDate(form: AddPhotoForm) {
  form.editingDate = true;
  const first = form.items.find(item => item.image)?.image;
  form.newDate = first ? first.photoDate.split("T")[0] : localDateString(new Date());
  vlens.scheduleRedraw();
}

function onDone(form: AddPhotoForm, event: Event) {
  event.preventDefault();
  if (form.finishing) return;
  if (form.items.some(item => item.state === "failed")) {
    form.error = "Some photos did not upload. Retry or remove them first.";
    vlens.scheduleRedraw();
    return;
  }
  if (form.editingDate && !form.newDate) {
    form.error = "Pick the date these were taken";
    vlens.scheduleRedraw();
    return;
  }
  form.finishing = true;
  form.error = "";
  vlens.scheduleRedraw();
  if (!form.pumping) finish(form);
}

async function finish(form: AddPhotoForm) {
  const caption = form.caption.trim();
  const problems: string[] = [];

  for (const item of form.items) {
    const image = item.image;
    if (!image) continue;

    const { add, remove } = peopleChanges(item.taggedAtUpload, form.personIds);
    if (add.length > 0) {
      const [, err] = await server.AddPeopleToPhoto({ photoId: image.id, personIds: add });
      if (err) problems.push(err);
    }
    for (const personId of remove) {
      const [, err] = await server.RemovePersonFromPhotoProc({ photoId: image.id, personId });
      if (err) problems.push(err);
    }
    item.taggedAtUpload = [...form.personIds];

    if (caption || form.editingDate) {
      const [resp, err] = await server.UpdatePhoto({
        id: image.id,
        title: caption || (form.editingDate ? "" : image.title),
        description: image.description,
        inputType: form.editingDate ? "date" : "keep",
        photoDate: form.editingDate ? form.newDate : "",
        ageYears: null,
        ageMonths: null,
      });
      if (resp) item.image = resp.image;
      else problems.push(err);
    }

    if (form.tagIds.length > 0) {
      const [, err] = await server.UpdatePhotoTags({ photoId: image.id, tagIds: form.tagIds });
      if (err) problems.push(err);
    }
  }

  if (problems.length > 0) {
    form.finishing = false;
    form.error = problems[0];
    vlens.scheduleRedraw();
    return;
  }

  form.items.forEach(item => URL.revokeObjectURL(item.previewUrl));
  const fallback = form.personIds.length === 1 ? `/profile/${form.personIds[0]}` : "/photos";
  core.setRoute(takeReturnPath(fallback));
}

function cancel(event: Event) {
  event.preventDefault();
  core.setRoute(takeReturnPath("/dashboard"));
}

function statusLabel(item: UploadItem): string {
  switch (item.state) {
    case "uploading":
    case "queued":
      return copy.photos.uploading;
    case "done":
      return copy.photos.uploaded;
    case "failed":
      return item.error || copy.photos.failed;
  }
}

interface AddPhotoPageProps {
  form: AddPhotoForm;
  people: server.Person[];
  tags: server.Tag[];
}

const AddPhotoPage = ({ form, people, tags }: AddPhotoPageProps) => {
  const hasItems = form.items.length > 0;
  const uploaded = form.items.filter(item => item.image).map(item => item.image!.photoDate);
  const taken = takenLabel(uploaded);
  const busy = form.items.some(item => item.state === "queued" || item.state === "uploading");

  return (
    <div className="entry-card">
      <h1 className="entry-title">{copy.photos.title}</h1>

      <input
        id="photo-input"
        type="file"
        accept="image/*"
        multiple
        hidden
        onChange={vlens.cachePartial(onFileSelect, form, people)}
      />

      <div
        className={`photo-drop${form.dragActive ? " drag-active" : ""}${hasItems ? " has-items" : ""}`}
        onDragOver={vlens.cachePartial(onDragOver, form)}
        onDragLeave={vlens.cachePartial(onDragLeave, form)}
        onDrop={vlens.cachePartial(onDrop, form, people)}
      >
        {!hasItems ? (
          <div className="photo-drop-empty">
            <label htmlFor="photo-input" className="btn btn-primary">
              {copy.photos.choose}
            </label>
            <small>{copy.photos.dropHint}</small>
          </div>
        ) : (
          <div className="upload-grid">
            {form.items.map(item => (
              <div key={item.previewUrl} className={`upload-tile upload-${item.state}`}>
                <img src={item.previewUrl} alt="" />
                <span className="upload-tile-status">{statusLabel(item)}</span>
                {item.state === "failed" && (
                  <span className="upload-tile-actions">
                    <button type="button" onClick={() => retry(form, people, item)}>
                      Retry
                    </button>
                    <button type="button" onClick={() => remove(form, item)}>
                      Remove
                    </button>
                  </span>
                )}
              </div>
            ))}
            {!form.finishing && (
              <label htmlFor="photo-input" className="upload-tile upload-add">
                <span aria-hidden="true">+</span>
                {copy.photos.addMore}
              </label>
            )}
          </div>
        )}
      </div>

      <form className="entry-form" onSubmit={vlens.cachePartial(onDone, form)} noValidate>
        <div className="entry-field">
          <span className="entry-label">{copy.photos.whoIsIn}</span>
          <PersonChips
            people={people}
            selected={form.personIds}
            onToggle={vlens.cachePartial(togglePerson, form)}
            label={copy.photos.whoIsIn}
            disabled={form.finishing}
          />
        </div>

        <div className="entry-field">
          <label htmlFor="caption">{copy.photos.caption}</label>
          <input
            id="caption"
            type="text"
            placeholder={copy.photos.captionPlaceholder}
            disabled={form.finishing}
            {...vlens.attrsBindInput(vlens.ref(form, "caption"))}
          />
        </div>

        {tags.length > 0 && (
          <div className="entry-field">
            <span className="entry-label" id="tagPickerLabel">
              {copy.photos.tags}
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
                    disabled={form.finishing}
                    onClick={vlens.cachePartial(toggleTag, form, tag.id)}
                  >
                    <span className="tag-color-dot" style={{ background: tag.color }} />
                    {tag.name}
                  </button>
                );
              })}
            </div>
          </div>
        )}

        {(taken || form.editingDate) && (
          <div className="photo-taken">
            <span className="entry-label">{copy.photos.taken}</span>
            {form.editingDate ? (
              <input
                type="date"
                className="when-date"
                aria-label={copy.photos.taken}
                max={localDateString(new Date())}
                disabled={form.finishing}
                {...vlens.attrsBindInput(vlens.ref(form, "newDate"))}
              />
            ) : (
              <span>
                {taken} ·{" "}
                <button
                  type="button"
                  className="link-button"
                  onClick={vlens.cachePartial(editDate, form)}
                >
                  {copy.photos.change}
                </button>
              </span>
            )}
          </div>
        )}

        {form.error && (
          <div className="error-message" role="alert">
            {form.error}
          </div>
        )}

        <div className="entry-actions">
          {!hasItems && (
            <a href={returnPath("/dashboard")} className="btn btn-secondary" onClick={cancel}>
              {copy.measurement.cancel}
            </a>
          )}
          <button type="submit" className="btn btn-primary" disabled={!hasItems || form.finishing}>
            {form.finishing
              ? busy
                ? `${copy.photos.uploading}…`
                : copy.photos.finishing
              : copy.photos.done}
          </button>
        </div>
      </form>
    </div>
  );
};
