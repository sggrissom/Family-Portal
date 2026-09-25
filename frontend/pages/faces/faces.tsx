import * as preact from "preact";
import * as vlens from "vlens";
import * as rpc from "vlens/rpc";
import * as server from "../../server";
import { Header, Footer } from "../../layout";
import { requireAuthInView, ensureAuthInFetch } from "../../lib/authHelpers";
import { FaceCrop } from "../../components/FaceCrop";
import "./faces-styles";

const emptyReview: server.GetFaceReviewResponse = {
  enabled: false,
  groups: [],
  autoTagged: [],
  families: [],
  unknownCount: 0,
  autoCount: 0,
};

export async function fetch(
  route: string,
  prefix: string
): Promise<rpc.Response<server.GetFaceReviewResponse>> {
  if (!(await ensureAuthInFetch())) {
    return rpc.ok(emptyReview);
  }
  return server.GetFaceReview({});
}

const GROUPS_PER_PAGE = 20;
const CROPS_PER_GROUP = 8;

type FacesState = {
  review: server.GetFaceReviewResponse | null;
  chosen: Record<number, number>;
  excluded: Record<number, boolean>;
  shownGroups: number;
  busy: boolean;
  error: string;
  notice: string;
};

const useFacesState = vlens.declareHook(
  (): FacesState => ({
    review: null,
    chosen: {},
    excluded: {},
    shownGroups: GROUPS_PER_PAGE,
    busy: false,
    error: "",
    notice: "",
  })
);

const groupKey = (group: server.FaceGroup) => group.faces[0]?.id ?? 0;

function peopleFor(review: server.GetFaceReviewResponse, familyId: number): server.Person[] {
  return review.families.find(f => f.familyId === familyId)?.people ?? [];
}

function personName(review: server.GetFaceReviewResponse, personId: number): string {
  for (const family of review.families) {
    const person = family.people.find(p => p.id === personId);
    if (person) return person.name;
  }
  return "Unknown";
}

function includedIds(state: FacesState, group: server.FaceGroup): number[] {
  return group.faces.filter(f => !state.excluded[f.id]).map(f => f.id);
}

export function view(
  route: string,
  prefix: string,
  data: server.GetFaceReviewResponse
): preact.ComponentChild {
  if (!requireAuthInView()) return;

  const state = useFacesState();
  if (state.review === null) {
    state.review = data;
  }
  const review = state.review;
  const multiFamily = review.families.length > 1;
  const nothingYet = review.unknownCount === 0 && review.autoCount === 0;

  return (
    <div>
      <Header isHome={false} />
      <main id="app" className="faces-container">
        <div className="faces-header">
          <div>
            <h1>Faces</h1>
            <p className="faces-subtitle">
              Tell the app who's who. Each face you name helps it recognize that person in other
              photos.
            </p>
          </div>
          <a href="/photos" className="btn btn-secondary">
            ← Photos
          </a>
        </div>

        {state.error && (
          <div className="faces-error" role="alert">
            {state.error}
          </div>
        )}
        {state.notice && (
          <div className="faces-notice" role="status">
            {state.notice}
          </div>
        )}

        {nothingYet && (
          <div className="faces-empty">
            {review.enabled ? (
              <p>
                No faces are waiting for review. New photos are scanned for faces after they upload.
              </p>
            ) : (
              <p>Face recognition isn't running on this server, so there's nothing to review.</p>
            )}
          </div>
        )}

        {review.groups.length > 0 && (
          <section className="faces-section">
            <h2>
              Who is this? <span className="faces-count">{review.unknownCount}</span>
            </h2>
            <p className="faces-hint">
              Faces that look alike are grouped together. Click a face to leave it out before you
              name the group.
            </p>
            {review.groups.slice(0, state.shownGroups).map(group => (
              <FaceGroupCard
                key={groupKey(group)}
                state={state}
                group={group}
                people={peopleFor(review, group.familyId)}
                familyName={
                  multiFamily
                    ? review.families.find(f => f.familyId === group.familyId)?.name
                    : undefined
                }
              />
            ))}
            {review.groups.length > state.shownGroups && (
              <button
                className="btn btn-secondary faces-more"
                onClick={vlens.cachePartial(onShowMore, state)}
              >
                Show more ({review.groups.length - state.shownGroups} more groups)
              </button>
            )}
            <p className="faces-hint">
              Someone missing from the list? <a href="/add-person">Add them</a> first, then come
              back.
            </p>
          </section>
        )}

        {review.autoTagged.length > 0 && (
          <section className="faces-section">
            <h2>
              Check automatic tags <span className="faces-count">{review.autoCount}</span>
            </h2>
            <p className="faces-hint">
              These were tagged automatically, least certain first. Confirming a match teaches the
              app what that person looks like.
            </p>
            <div className="auto-tag-grid">
              {review.autoTagged.map(face => (
                <div key={face.id} className="auto-tag-card">
                  <a href={`/view-photo/${face.photoId}`} title="Open photo">
                    <FaceCrop photoId={face.photoId} box={face.box} size={96} />
                  </a>
                  <div className="auto-tag-name">{personName(review, face.personId)}</div>
                  <div className="auto-tag-actions">
                    <button
                      className="btn btn-primary btn-small"
                      disabled={state.busy}
                      aria-label={`Confirm ${personName(review, face.personId)}`}
                      onClick={vlens.cachePartial(onConfirm, state, face)}
                    >
                      ✓
                    </button>
                    <button
                      className="btn btn-secondary btn-small"
                      disabled={state.busy}
                      aria-label={`Not ${personName(review, face.personId)}`}
                      onClick={vlens.cachePartial(onReject, state, face)}
                    >
                      ✗ Not them
                    </button>
                  </div>
                </div>
              ))}
            </div>
          </section>
        )}
      </main>
      <Footer />
    </div>
  );
}

interface FaceGroupCardProps {
  state: FacesState;
  group: server.FaceGroup;
  people: server.Person[];
  familyName?: string;
}

const FaceGroupCard = ({ state, group, people, familyName }: FaceGroupCardProps) => {
  const key = groupKey(group);
  const ids = includedIds(state, group);
  const suggested = people.find(p => p.id === group.suggestedPersonId);
  const chosen = state.chosen[key] ?? 0;
  const hidden = group.faces.length - CROPS_PER_GROUP;

  return (
    <div className="face-group">
      <div className="face-group-crops">
        {group.faces.slice(0, CROPS_PER_GROUP).map(face => (
          <button
            key={face.id}
            type="button"
            className={`face-toggle ${state.excluded[face.id] ? "excluded" : ""}`}
            title={state.excluded[face.id] ? "Include this face" : "Leave this face out"}
            aria-pressed={!state.excluded[face.id]}
            onClick={vlens.cachePartial(onToggleFace, state, face.id)}
          >
            <FaceCrop photoId={face.photoId} box={face.box} />
          </button>
        ))}
        {hidden > 0 && <div className="face-group-more">+{hidden}</div>}
      </div>
      <div className="face-group-controls">
        {familyName && <div className="face-group-family">{familyName}</div>}
        <div className="face-group-meta">
          {ids.length} face{ids.length !== 1 ? "s" : ""}
          {ids.length !== group.faces.length && ` of ${group.faces.length}`}
        </div>
        {suggested && (
          <button
            className="btn btn-primary"
            disabled={state.busy || ids.length === 0}
            onClick={vlens.cachePartial(onAssign, state, ids, suggested.id)}
          >
            It's {suggested.name}
          </button>
        )}
        <div className="face-group-assign">
          <select
            aria-label="Who is this?"
            value={chosen}
            disabled={state.busy}
            onChange={e => {
              state.chosen[key] = Number(e.currentTarget.value);
              vlens.scheduleRedraw();
            }}
          >
            <option value={0}>{suggested ? "Someone else…" : "Who is this?"}</option>
            {people.map(p => (
              <option key={p.id} value={p.id}>
                {p.name}
              </option>
            ))}
          </select>
          <button
            className="btn btn-secondary"
            disabled={state.busy || chosen === 0 || ids.length === 0}
            onClick={vlens.cachePartial(onAssign, state, ids, chosen)}
          >
            Save
          </button>
        </div>
        <button
          className="face-dismiss"
          disabled={state.busy || ids.length === 0}
          onClick={vlens.cachePartial(onDismiss, state, ids)}
        >
          Not someone in the family
        </button>
      </div>
    </div>
  );
};

function onShowMore(state: FacesState) {
  state.shownGroups += GROUPS_PER_PAGE;
  vlens.scheduleRedraw();
}

function onToggleFace(state: FacesState, faceId: number) {
  state.excluded[faceId] = !state.excluded[faceId];
  vlens.scheduleRedraw();
}

async function run(state: FacesState, action: () => Promise<string>) {
  state.busy = true;
  state.error = "";
  state.notice = "";
  vlens.scheduleRedraw();

  const notice = await action();
  if (!state.error) {
    const [review, err] = await server.GetFaceReview({});
    if (review) {
      state.review = review;
      state.excluded = {};
      state.chosen = {};
    } else {
      state.error = err || "Failed to reload faces";
    }
    state.notice = notice;
  }
  state.busy = false;
  vlens.scheduleRedraw();
}

async function onAssign(state: FacesState, faceIds: number[], personId: number) {
  await run(state, async () => {
    const [resp, err] = await server.AssignFaces({ faceIds, personId });
    if (!resp) {
      state.error = err || "Failed to save";
      return "";
    }
    const name = state.review ? personName(state.review, personId) : "them";
    const more =
      resp.autoTagged > 0
        ? ` and found ${name} in ${resp.autoTagged} more photo${resp.autoTagged !== 1 ? "s" : ""}`
        : "";
    return `Tagged ${name} in ${resp.assigned} photo${resp.assigned !== 1 ? "s" : ""}${more}.`;
  });
}

async function onDismiss(state: FacesState, faceIds: number[]) {
  await run(state, async () => {
    const [, err] = await server.DismissFaces({ faceIds });
    if (err) state.error = err;
    return "";
  });
}

async function onConfirm(state: FacesState, face: server.PhotoFace) {
  await onAssign(state, [face.id], face.personId);
}

async function onReject(state: FacesState, face: server.PhotoFace) {
  await run(state, async () => {
    const [, err] = await server.RejectFaces({ faceIds: [face.id] });
    if (err) state.error = err;
    return "Removed the tag. The face is back in “Who is this?”.";
  });
}
