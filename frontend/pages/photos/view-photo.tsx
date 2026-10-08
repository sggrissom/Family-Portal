import { formatDate as formatPhotoDate } from "../../lib/dateUtils";
import * as preact from "preact";
import * as vlens from "vlens";
import * as core from "vlens/core";
import * as rpc from "vlens/rpc";
import * as auth from "../../lib/authCache";
import * as server from "../../server";
import { Header, Footer } from "../../layout";
import { FullImage } from "../../components/ResponsiveImage";
import { CropSelector } from "../../components/CropSelector";
import { FaceCrop } from "../../components/FaceCrop";
import { usePhotoStatus } from "../../hooks/usePhotoStatus";
import { SameAgeStrip } from "../../components/SameAgeRows";
import { isValidBirthday } from "../../lib/growthPercentiles";
import { monthsOld, sameAgeForPhoto, sameAgePath } from "../../lib/sameAge";
import { copy } from "../../lib/copy";
import { localDateString } from "../../lib/when";
import "./view-photo-styles";

import { getIdFromRoute } from "../../lib/routeHelpers";
import {
  loadSequence,
  routeHasSequence,
  sequencePosition,
  swipeDirection,
  viewPhotoRoute,
  SequencePosition,
} from "../../lib/photoSequence";

type ViewPhotoData = {
  image: server.Image | null;
  people: server.Person[] | null;
  place: server.PhotoPlace | null;
  suggestions: server.SuggestedTag[];
  tags: server.Tag[];
  faces: server.GetPhotoFacesResponse | null;
  sameAge: server.GetSameAgeResponse | null;
  childId: number;
  compareHref: string;
};

const CHILD_MAX_MONTHS = 18 * 12;

function firstChildIn(people: server.Person[], photoDate: string): [number, number] {
  for (const person of people) {
    if (person.isPregnancy || !isValidBirthday(person.birthday)) continue;
    const age = monthsOld(person.birthday, photoDate);
    if (age >= 0 && age < CHILD_MAX_MONTHS) return [person.id, age];
  }
  return [0, 0];
}

function compareHref(route: string, people: server.Person[], photoDate: string): string {
  const personId = parseInt(new URLSearchParams(route.split("?")[1] ?? "").get("person") ?? "");
  const person = people.find(p => p.id === personId);
  if (!person || person.isPregnancy || !isValidBirthday(person.birthday)) return "";
  const age = sameAgeForPhoto(person.birthday, photoDate);
  return age === null ? "" : sameAgePath(age, person.id);
}

export async function fetch(route: string, prefix: string): Promise<rpc.Response<ViewPhotoData>> {
  const photoId = getIdFromRoute(route) || 0;
  const [photoResp, photoErr] = await server.GetPhoto({ id: photoId });
  if (photoErr) return [null, photoErr];
  const [childId, childAge] = photoResp
    ? firstChildIn(photoResp.people ?? [], photoResp.image.photoDate)
    : [0, 0];
  const [[tagsResp], [facesResp], [sameAge]] = await Promise.all([
    server.ListTags({}),
    server.GetPhotoFaces({ photoId }),
    childId
      ? server.GetSameAge({
          includeAvailableAges: false,
          details: false,
          ageMonths: childAge,
          fromPersonId: childId,
          today: localDateString(new Date()),
        })
      : Promise.resolve([null, ""] as rpc.Response<server.GetSameAgeResponse>),
  ]);
  return [
    {
      image: photoResp?.image ?? null,
      people: photoResp?.people ?? null,
      place: photoResp?.place ?? null,
      suggestions: photoResp?.suggestions ?? [],
      tags: tagsResp?.tags ?? [],
      faces: facesResp ?? null,
      sameAge: sameAge ?? null,
      childId,
      compareHref: photoResp
        ? compareHref(route, photoResp.people ?? [], photoResp.image.photoDate)
        : "",
    },
    "",
  ];
}

export function view(route: string, prefix: string, data: ViewPhotoData): preact.ComponentChild {
  const currentAuth = auth.getAuth();
  if (!currentAuth || currentAuth.id <= 0) {
    auth.clearAuth();
    core.setRoute("/login");
    return;
  }

  if (!data.image) {
    return (
      <div>
        <Header isHome={false} />
        <main id="app" className="view-photo-container">
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

  const sequence = routeHasSequence(route) ? loadSequence() : null;
  const position = sequence ? sequencePosition(sequence.ids, data.image.id) : null;

  return (
    <div>
      <Header isHome={false} />
      <main id="app" className="view-photo-container">
        <ViewPhotoPage
          photo={data.image}
          people={data.people || []}
          place={data.place}
          suggestions={data.suggestions}
          allTags={data.tags}
          faces={data.faces}
          sameAge={data.sameAge}
          childId={data.childId}
          compareHref={data.compareHref}
          position={position}
          backRoute={sequence?.backRoute || "/photos"}
        />
      </main>
      <Footer />
    </div>
  );
}

interface ViewPhotoPageProps {
  photo: server.Image;
  people: server.Person[];
  place: server.PhotoPlace | null;
  suggestions: server.SuggestedTag[];
  allTags: server.Tag[];
  faces: server.GetPhotoFacesResponse | null;
  sameAge: server.GetSameAgeResponse | null;
  childId: number;
  compareHref: string;
  position: SequencePosition | null;
  backRoute: string;
}

async function refreshAfter(photo: server.Image, call: Promise<[unknown, string]>) {
  const [, err] = await call;
  if (err) {
    alert(err);
    return;
  }
  core.replaceRoute(core.getRoute());
}

function goToPhoto(photoId: number) {
  if (photoId) core.replaceRoute(viewPhotoRoute(photoId, true));
}

let detachKeys: (() => void) | null = null;
core.registerCleanupFunction(() => {
  detachKeys?.();
  detachKeys = null;
});

function isTyping(target: EventTarget | null): boolean {
  return (
    target instanceof HTMLElement &&
    (target.isContentEditable || ["INPUT", "SELECT", "TEXTAREA"].includes(target.tagName))
  );
}

// View functions run on every redraw, so this replaces the listener rather
// than stacking another one.
function bindArrowKeys(position: SequencePosition, isBlocked: () => boolean) {
  detachKeys?.();
  const onKeyDown = (event: KeyboardEvent) => {
    if (event.defaultPrevented || event.altKey || event.ctrlKey || event.metaKey) return;
    if (isBlocked() || isTyping(event.target)) return;
    const photoId =
      event.key === "ArrowLeft"
        ? position.prevId
        : event.key === "ArrowRight"
          ? position.nextId
          : 0;
    if (!photoId) return;
    event.preventDefault();
    goToPhoto(photoId);
  };
  document.addEventListener("keydown", onKeyDown);
  detachKeys = () => document.removeEventListener("keydown", onKeyDown);
}

let touchStart: { x: number; y: number } | null = null;

function handleTouchStart(event: TouchEvent) {
  const touch = event.touches[0];
  touchStart = event.touches.length === 1 ? { x: touch.clientX, y: touch.clientY } : null;
}

function handleTouchEnd(position: SequencePosition, event: TouchEvent) {
  const start = touchStart;
  touchStart = null;
  const touch = event.changedTouches[0];
  if (!start || !touch) return;
  const direction = swipeDirection(touch.clientX - start.x, touch.clientY - start.y);
  if (direction === "prev") goToPhoto(position.prevId);
  if (direction === "next") goToPhoto(position.nextId);
}

function handleRemovePerson(photo: server.Image, person: server.Person) {
  if (!confirm(`Remove ${person.name} from this photo?`)) return;
  refreshAfter(photo, server.RemovePersonFromPhotoProc({ photoId: photo.id, personId: person.id }));
}

function handleAssignFace(photo: server.Image, face: server.PhotoFace, personId: number) {
  if (!personId) return;
  refreshAfter(photo, server.AssignFaces({ faceIds: [face.id], personId }));
}

function handleRejectFace(photo: server.Image, face: server.PhotoFace) {
  refreshAfter(photo, server.RejectFaces({ faceIds: [face.id] }));
}

function handleDismissFace(photo: server.Image, face: server.PhotoFace) {
  refreshAfter(photo, server.DismissFaces({ faceIds: [face.id] }));
}

const PhotoFaces = ({
  photo,
  faces,
}: {
  photo: server.Image;
  faces: server.GetPhotoFacesResponse;
}) => {
  const nameOf = (id: number) => faces.people.find(p => p.id === id)?.name ?? "Someone";
  return (
    <div className="photo-faces">
      <h3>Faces</h3>
      <div className="photo-faces-list">
        {faces.faces.map(face => (
          <div key={face.id} className="photo-face">
            <FaceCrop photoId={photo.id} box={face.box} size={72} />
            {face.personId > 0 ? (
              <div className="photo-face-info">
                <span className="photo-face-name">{nameOf(face.personId)}</span>
                {face.status === 1 && (
                  <div className="photo-face-actions">
                    <span className="photo-face-auto">auto</span>
                    <button
                      className="btn btn-outline btn-small"
                      aria-label={`Confirm ${nameOf(face.personId)}`}
                      onClick={() => handleAssignFace(photo, face, face.personId)}
                    >
                      ✓
                    </button>
                    <button
                      className="btn btn-outline btn-small"
                      aria-label={`Not ${nameOf(face.personId)}`}
                      onClick={() => handleRejectFace(photo, face)}
                    >
                      ✗
                    </button>
                  </div>
                )}
              </div>
            ) : (
              <div className="photo-face-info">
                <select
                  aria-label="Who is this?"
                  value={0}
                  onChange={e => handleAssignFace(photo, face, Number(e.currentTarget.value))}
                >
                  <option value={0}>Who is this?</option>
                  {faces.people.map(p => (
                    <option key={p.id} value={p.id}>
                      {p.name}
                    </option>
                  ))}
                </select>
                <button
                  className="photo-face-dismiss"
                  onClick={() => handleDismissFace(photo, face)}
                >
                  Not family
                </button>
              </div>
            )}
          </div>
        ))}
      </div>
      <a href="/faces" className="photo-faces-link">
        Review all faces →
      </a>
    </div>
  );
};

async function handleDeletePhoto(photo: server.Image, backRoute: string) {
  const confirmed = confirm(
    `Are you sure you want to delete "${photo.title}"? This action cannot be undone.`
  );
  if (!confirmed) return;

  try {
    const [resp, err] = await server.DeletePhoto({ id: photo.id });
    if (err) {
      alert(err || "Failed to delete photo");
      return;
    }

    if (resp && resp.success) {
      alert("Photo deleted successfully");
      core.setRoute(backRoute);
    } else {
      alert("Failed to delete photo");
    }
  } catch (error) {
    alert("Failed to delete photo");
  }
}

async function handleSetProfilePhoto(
  photo: server.Image,
  personId: number,
  cropX: number,
  cropY: number,
  cropScale: number
) {
  try {
    const [resp, err] = await server.SetProfilePhoto({
      personId: personId,
      photoId: photo.id,
      cropX: cropX,
      cropY: cropY,
      cropScale: cropScale,
    });

    if (err) {
      alert(err || "Failed to set profile photo");
      return;
    }

    if (resp && resp.person) {
      alert("Profile photo set successfully");
      core.replaceRoute(core.getRoute());
    } else {
      alert("Failed to set profile photo");
    }
  } catch (error) {
    alert("Failed to set profile photo");
  }
}

type PlaceFormState = {
  photoId: number;
  open: boolean;
  name: string;
  radius: number;
  saving: boolean;
};

const usePlaceForm = vlens.declareHook(
  (): PlaceFormState => ({ photoId: 0, open: false, name: "", radius: 250, saving: false })
);

const PLACE_RADII = [
  { meters: 100, label: "Just this spot" },
  { meters: 250, label: "A house or park" },
  { meters: 1000, label: "A neighborhood" },
  { meters: 5000, label: "A town" },
];

async function savePlace(state: PlaceFormState, photo: server.Image) {
  if (!state.name.trim() || state.saving) return;
  state.saving = true;
  vlens.scheduleRedraw();
  const [, err] = await server.SaveFamilyPlace({
    id: 0,
    photoId: photo.id,
    name: state.name.trim(),
    radiusMeters: state.radius,
  });
  state.saving = false;
  if (err) {
    alert(err);
    vlens.scheduleRedraw();
    return;
  }
  state.open = false;
  state.name = "";
  core.replaceRoute(core.getRoute());
}

const PhotoPlaceLine = ({ photo, place }: { photo: server.Image; place: server.PhotoPlace }) => {
  const form = usePlaceForm();
  if (form.photoId !== photo.id) {
    form.photoId = photo.id;
    form.open = false;
    form.name = "";
  }
  return (
    <div className="view-photo-place">
      <a href={`/photos?place=${place.key}`} className="view-photo-place-link">
        📍 {place.name}
      </a>
      {place.familyPlaceId === 0 && !form.open && auth.canContribute(photo.familyId) && (
        <button
          className="btn btn-outline btn-small"
          onClick={() => {
            form.open = true;
            vlens.scheduleRedraw();
          }}
        >
          Name this place
        </button>
      )}
      {form.open && (
        <form
          className="place-form"
          onSubmit={e => {
            e.preventDefault();
            savePlace(form, photo);
          }}
        >
          <input
            type="text"
            placeholder="Home, Grandma's house…"
            maxLength={80}
            value={form.name}
            onInput={e => (form.name = e.currentTarget.value)}
            aria-label="Place name"
          />
          <select
            value={form.radius}
            onChange={e => (form.radius = Number(e.currentTarget.value))}
            aria-label="How big is this place?"
          >
            {PLACE_RADII.map(r => (
              <option key={r.meters} value={r.meters}>
                {r.label}
              </option>
            ))}
          </select>
          <button type="submit" className="btn btn-primary btn-small" disabled={form.saving}>
            Save
          </button>
          <button
            type="button"
            className="btn btn-outline btn-small"
            onClick={() => {
              form.open = false;
              vlens.scheduleRedraw();
            }}
          >
            Cancel
          </button>
        </form>
      )}
    </div>
  );
};

type CropModalState = {
  isOpen: boolean;
  personId: number;
  personName: string;
  cropX: number;
  cropY: number;
  cropScale: number;
};

const useCropModalState = vlens.declareHook(
  (): CropModalState => ({
    isOpen: false,
    personId: 0,
    personName: "",
    cropX: 50,
    cropY: 50,
    cropScale: 1,
  })
);

function openCropModal(state: CropModalState, person: server.Person) {
  state.isOpen = true;
  state.personId = person.id;
  state.personName = person.name;
  state.cropX = person.profileCropX || 50;
  state.cropY = person.profileCropY || 50;
  state.cropScale = person.profileCropScale || 1;
  vlens.scheduleRedraw();
}

function closeCropModal(state: CropModalState) {
  state.isOpen = false;
  vlens.scheduleRedraw();
}

const ViewPhotoPage = ({
  photo,
  people,
  place,
  suggestions,
  allTags,
  faces,
  sameAge,
  childId,
  compareHref,
  position,
  backRoute,
}: ViewPhotoPageProps) => {
  const photoStatus = usePhotoStatus();
  const cropModalState = useCropModalState();
  const profileCandidates =
    auth.roleIn(photo.familyId) >= auth.ROLE_VIEW
      ? people.filter(person => auth.canContribute(person.familyId))
      : [];

  if (position) {
    bindArrowKeys(position, () => cropModalState.isOpen);
  }

  const handleSaveProfilePhoto = async () => {
    await handleSetProfilePhoto(
      photo,
      cropModalState.personId,
      cropModalState.cropX,
      cropModalState.cropY,
      cropModalState.cropScale
    );
    closeCropModal(cropModalState);
  };

  return (
    <div className="view-photo-page">
      <div className="photo-header">
        <a href={backRoute} className="back-link">
          ← Back to Photos
        </a>
        {position && (
          <span className="photo-position">
            {position.index + 1} of {position.total}
          </span>
        )}
      </div>

      <div
        className="photo-display"
        onTouchStart={position ? handleTouchStart : undefined}
        onTouchEnd={position ? e => handleTouchEnd(position, e) : undefined}
      >
        <FullImage
          photoId={photo.id}
          alt={photo.title}
          className="photo-main-image"
          status={photoStatus.getStatus(photo.id)}
        />
        {position && (
          <>
            <button
              className="photo-nav photo-nav-prev"
              aria-label="Previous photo"
              title="Previous (←)"
              disabled={!position.prevId}
              onClick={() => goToPhoto(position.prevId)}
            >
              ‹
            </button>
            <button
              className="photo-nav photo-nav-next"
              aria-label="Next photo"
              title="Next (→)"
              disabled={!position.nextId}
              onClick={() => goToPhoto(position.nextId)}
            >
              ›
            </button>
          </>
        )}
      </div>

      <div className="photo-info-panel">
        <div className="photo-metadata">
          <h1 className="view-photo-title">{photo.title}</h1>
          <div className="view-photo-date">📅 {formatPhotoDate(photo.photoDate)}</div>
          {compareHref && (
            <a href={compareHref} className="view-photo-compare">
              {copy.sameAge.compareAtThisAge} →
            </a>
          )}
          {place && <PhotoPlaceLine photo={photo} place={place} />}
          {photo.description && <div className="view-photo-description">{photo.description}</div>}

          <div className="photo-people">
            {people.length > 0 ? (
              <div>
                <h3>People in this photo:</h3>
                <div className="people-list">
                  {people.map(person => (
                    <div key={person.id} className="person-tag">
                      <a href={`/profile/${person.id}`} className="person-link">
                        {person.name}
                      </a>
                      {person.profilePhotoId === photo.id && (
                        <span className="profile-badge">Profile Photo</span>
                      )}
                      {faces?.canLabel && (
                        <button
                          className="person-tag-remove"
                          aria-label={`Remove ${person.name} from this photo`}
                          title="Remove from photo"
                          onClick={() => handleRemovePerson(photo, person)}
                        >
                          ×
                        </button>
                      )}
                    </div>
                  ))}
                </div>
              </div>
            ) : (
              <div>
                <h3>Family Photo</h3>
                <p>No specific people tagged</p>
              </div>
            )}
          </div>

          {faces && faces.faces.length > 0 && <PhotoFaces photo={photo} faces={faces} />}

          {photo.tagIds && photo.tagIds.length > 0 && (
            <div className="photo-tags">
              <h3>Tags</h3>
              <div className="tag-list">
                {photo.tagIds.map(tagId => {
                  const tag = allTags.find(t => t.id === tagId);
                  if (!tag) return null;
                  return (
                    <span key={tag.id} className="tag-pill-view" style={{ borderColor: tag.color }}>
                      <span className="tag-color-dot" style={{ background: tag.color }} />
                      {tag.name}
                    </span>
                  );
                })}
              </div>
            </div>
          )}

          {suggestions.length > 0 && (
            <div className="photo-tags">
              <h3>Suggested tags</h3>
              <div className="tag-list">
                {suggestions.map(s => (
                  <span
                    key={s.id}
                    className="tag-pill-view tag-suggestion"
                    style={{ borderColor: s.color }}
                  >
                    <span className="tag-color-dot" style={{ background: s.color }} />
                    {s.label}
                    <button
                      className="tag-suggestion-action"
                      aria-label={`Add tag ${s.label}`}
                      title="Add this tag"
                      onClick={() =>
                        refreshAfter(photo, server.AcceptTagSuggestions({ ids: [s.id] }))
                      }
                    >
                      ✓
                    </button>
                    <button
                      className="tag-suggestion-action"
                      aria-label={`Don't tag ${s.label}`}
                      title="Not this"
                      onClick={() =>
                        refreshAfter(photo, server.RejectTagSuggestions({ ids: [s.id] }))
                      }
                    >
                      ✗
                    </button>
                  </span>
                ))}
              </div>
            </div>
          )}

          <div className="photo-details">
            <small>
              Uploaded: {formatPhotoDate(photo.createdAt)} • {photo.originalFilename}
            </small>
          </div>
        </div>

        <div className="photo-actions">
          {auth.canContribute(photo.familyId) && (
            <a href={`/edit-photo/${photo.id}`} className="btn btn-secondary">
              ✏️ Edit
            </a>
          )}
          <a href={`/api/photo/${photo.id}/original?download=1`} className="btn btn-secondary">
            ⬇️ Download original
          </a>

          {profileCandidates.length > 0 && (
            <div className="profile-photo-actions">
              <h4>Set as Profile Photo:</h4>
              {profileCandidates.map(person => (
                <div key={person.id} className="profile-action">
                  {person.profilePhotoId === photo.id ? (
                    <button
                      className="btn btn-success btn-sm"
                      onClick={() => openCropModal(cropModalState, person)}
                    >
                      ✓ {person.name}'s Profile (Adjust)
                    </button>
                  ) : (
                    <button
                      className="btn btn-outline btn-sm"
                      onClick={() => openCropModal(cropModalState, person)}
                    >
                      👤 {person.name}
                    </button>
                  )}
                </div>
              ))}
            </div>
          )}

          {auth.canAdmin(photo.familyId) && (
            <button className="btn btn-danger" onClick={() => handleDeletePhoto(photo, backRoute)}>
              🗑️ Delete
            </button>
          )}
        </div>
      </div>

      {childId > 0 && (
        <SameAgeStrip data={sameAge} today={localDateString(new Date())} exceptPersonId={childId} />
      )}

      {cropModalState.isOpen && (
        <CropSelector
          photoId={photo.id}
          crop={cropModalState}
          onSave={handleSaveProfilePhoto}
          onCancel={() => closeCropModal(cropModalState)}
        />
      )}
    </div>
  );
};
