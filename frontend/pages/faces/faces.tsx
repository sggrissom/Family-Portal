import * as preact from "preact";
import * as vlens from "vlens";
import * as rpc from "vlens/rpc";
import * as server from "../../server";
import { Header, Footer } from "../../layout";
import { requireAuthInView, ensureAuthInFetch } from "../../lib/authHelpers";
import { FaceCrop } from "../../components/FaceCrop";
import { videoFrameOf } from "../../lib/faceCrop";
import { viewPhotoRoute } from "../../lib/photoSequence";
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

type GroupCard = {
  key: number;
  familyId: number;
  faces: server.PhotoFace[];
  suggestedPersonId: number;
};

type AutoCard = {
  personId: number;
  faces: server.PhotoFace[];
};

// Answers are applied to these local lists as they are given; the server's
// re-clustered view is only loaded when the user asks or the lists run out, so
// nothing moves under the pointer.
type FacesState = {
  review: server.GetFaceReviewResponse | null;
  groups: GroupCard[];
  autoCards: AutoCard[];
  chosen: Record<number, number>;
  excluded: Record<number, boolean>;
  expanded: Record<number, boolean>;
  shownGroups: number;
  pending: number;
  stale: boolean;
  reloading: boolean;
  error: string;
  notice: string;
};

const useFacesState = vlens.declareHook(
  (): FacesState => ({
    review: null,
    groups: [],
    autoCards: [],
    chosen: {},
    excluded: {},
    expanded: {},
    shownGroups: GROUPS_PER_PAGE,
    pending: 0,
    stale: false,
    reloading: false,
    error: "",
    notice: "",
  })
);

function load(state: FacesState, review: server.GetFaceReviewResponse) {
  state.review = review;
  state.groups = review.groups.map(g => ({
    key: g.faces[0]?.id ?? 0,
    familyId: g.familyId,
    faces: g.faces,
    suggestedPersonId: g.suggestedPersonId,
  }));
  const byPerson = new Map<number, AutoCard>();
  for (const face of review.autoTagged) {
    let card = byPerson.get(face.personId);
    if (!card) {
      card = { personId: face.personId, faces: [] };
      byPerson.set(face.personId, card);
    }
    card.faces.push(face);
  }
  state.autoCards = [...byPerson.values()];
  state.chosen = {};
  state.excluded = {};
  state.expanded = {};
  state.shownGroups = GROUPS_PER_PAGE;
  state.stale = false;
}

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

function includedIds(state: FacesState, faces: server.PhotoFace[]): number[] {
  return faces.filter(f => !state.excluded[f.id]).map(f => f.id);
}

const plural = (n: number, word: string) => `${n} ${word}${n !== 1 ? "s" : ""}`;

export function view(
  route: string,
  prefix: string,
  data: server.GetFaceReviewResponse
): preact.ComponentChild {
  if (!requireAuthInView()) return;

  const state = useFacesState();
  if (state.review === null) {
    load(state, data);
  }
  const review = state.review!;
  const multiFamily = review.families.length > 1;
  const empty = state.groups.length === 0 && state.autoCards.length === 0;

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
          <div className="faces-header-actions">
            {state.stale && !empty && (
              <button
                className="btn btn-secondary"
                disabled={state.pending > 0 || state.reloading}
                onClick={vlens.cachePartial(reload, state)}
              >
                Refresh
              </button>
            )}
            <a href="/photos" className="btn btn-secondary">
              ← Photos
            </a>
          </div>
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

        {empty && (
          <div className="faces-empty">
            {state.pending > 0 || state.reloading ? (
              <p>Checking for more faces…</p>
            ) : review.enabled ? (
              <p>
                No faces are waiting for review. New photos are scanned for faces after they upload.
              </p>
            ) : (
              <p>Face recognition isn't running on this server, so there's nothing to review.</p>
            )}
          </div>
        )}

        {state.groups.length > 0 && (
          <section className="faces-section">
            <h2>
              Who is this? <span className="faces-count">{Math.max(0, review.unknownCount)}</span>
            </h2>
            <p className="faces-hint">
              Faces that look alike are grouped together. Click a face to leave it out before you
              name the group.
            </p>
            {state.groups.slice(0, state.shownGroups).map(card => (
              <FaceGroupCard
                key={card.key}
                state={state}
                card={card}
                people={peopleFor(review, card.familyId)}
                familyName={
                  multiFamily
                    ? review.families.find(f => f.familyId === card.familyId)?.name
                    : undefined
                }
              />
            ))}
            {state.groups.length > state.shownGroups && (
              <button
                className="btn btn-secondary faces-more"
                onClick={vlens.cachePartial(onShowMore, state)}
              >
                Show more ({state.groups.length - state.shownGroups} more groups)
              </button>
            )}
            <p className="faces-hint">
              Someone missing from the list? <a href="/add-person">Add them</a> first, then come
              back.
            </p>
          </section>
        )}

        {state.autoCards.length > 0 && (
          <section className="faces-section">
            <h2>
              Check automatic tags{" "}
              <span className="faces-count">{Math.max(0, review.autoCount)}</span>
            </h2>
            <p className="faces-hint">
              These were tagged automatically, least certain first. Click any face that isn't the
              person named, then confirm the rest.
            </p>
            {state.autoCards.map(card => (
              <AutoTagCard
                key={card.personId}
                state={state}
                card={card}
                name={personName(review, card.personId)}
              />
            ))}
          </section>
        )}
      </main>
      <Footer />
    </div>
  );
}

interface FaceTileProps {
  state: FacesState;
  face: server.PhotoFace;
  markClass: string;
  includeTitle: string;
  excludeTitle: string;
}

const FaceTile = ({ state, face, markClass, includeTitle, excludeTitle }: FaceTileProps) => {
  const excluded = !!state.excluded[face.id];
  const frameMs = videoFrameOf(face);
  return (
    <div className="face-tile">
      <button
        type="button"
        className={`face-toggle ${excluded ? markClass : ""}`}
        title={excluded ? includeTitle : excludeTitle}
        aria-pressed={!excluded}
        onClick={vlens.cachePartial(onToggleFace, state, face.id)}
      >
        <FaceCrop photoId={face.photoId} box={face.box} frameMs={frameMs} />
      </button>
      <a
        className="face-open"
        href={viewPhotoRoute(face.photoId, false, frameMs)}
        title="Open the photo"
        aria-label="Open the photo"
      >
        ↗
      </a>
    </div>
  );
};

interface FaceGroupCardProps {
  state: FacesState;
  card: GroupCard;
  people: server.Person[];
  familyName?: string;
}

const FaceGroupCard = ({ state, card, people, familyName }: FaceGroupCardProps) => {
  const ids = includedIds(state, card.faces);
  const suggested = people.find(p => p.id === card.suggestedPersonId);
  const chosen = state.chosen[card.key] ?? 0;
  const shown = state.expanded[card.key] ? card.faces : card.faces.slice(0, CROPS_PER_GROUP);
  const hidden = card.faces.length - shown.length;

  return (
    <div className="face-group">
      <div className="face-group-crops">
        {shown.map(face => (
          <FaceTile
            key={face.id}
            state={state}
            face={face}
            markClass="excluded"
            includeTitle="Include this face"
            excludeTitle="Leave this face out"
          />
        ))}
        {hidden > 0 && (
          <button
            type="button"
            className="face-group-more"
            title="Show every face in this group"
            onClick={vlens.cachePartial(onExpand, state, card.key)}
          >
            +{hidden}
          </button>
        )}
      </div>
      <div className="face-group-controls">
        {familyName && <div className="face-group-family">{familyName}</div>}
        <div className="face-group-meta">
          {plural(ids.length, "face")}
          {ids.length !== card.faces.length && ` of ${card.faces.length}`}
        </div>
        {suggested && (
          <button
            className="btn btn-primary"
            disabled={ids.length === 0}
            onClick={vlens.cachePartial(onAssignGroup, state, card, suggested.id)}
          >
            It's {suggested.name}
          </button>
        )}
        <div className="face-group-assign">
          <select
            aria-label="Who is this?"
            value={chosen}
            onChange={e => {
              state.chosen[card.key] = Number(e.currentTarget.value);
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
            disabled={chosen === 0 || ids.length === 0}
            onClick={vlens.cachePartial(onAssignGroup, state, card, chosen)}
          >
            Save
          </button>
        </div>
        <button
          className="face-dismiss"
          disabled={ids.length === 0}
          onClick={vlens.cachePartial(onDismissGroup, state, card)}
        >
          Not someone in the family
        </button>
      </div>
    </div>
  );
};

interface AutoTagCardProps {
  state: FacesState;
  card: AutoCard;
  name: string;
}

const AutoTagCard = ({ state, card, name }: AutoTagCardProps) => {
  const ids = includedIds(state, card.faces);
  return (
    <div className="face-group">
      <div className="face-group-crops">
        {card.faces.map(face => (
          <FaceTile
            key={face.id}
            state={state}
            face={face}
            markClass="rejected"
            includeTitle={`This is ${name}`}
            excludeTitle={`Not ${name}`}
          />
        ))}
      </div>
      <div className="face-group-controls">
        <div className="auto-tag-name">{name}</div>
        <div className="face-group-meta">
          {ids.length === card.faces.length
            ? `Tagged in ${plural(card.faces.length, "photo")}`
            : `${ids.length} of ${card.faces.length} are ${name}`}
        </div>
        <button className="btn btn-primary" onClick={vlens.cachePartial(onReviewAuto, state, card)}>
          {ids.length > 0
            ? `Confirm ${ids.length === card.faces.length ? "all" : ids.length}`
            : `None of these are ${name}`}
        </button>
      </div>
    </div>
  );
};

function onShowMore(state: FacesState) {
  state.shownGroups += GROUPS_PER_PAGE;
  vlens.scheduleRedraw();
}

function onExpand(state: FacesState, key: number) {
  state.expanded[key] = true;
  vlens.scheduleRedraw();
}

function onToggleFace(state: FacesState, faceId: number) {
  state.excluded[faceId] = !state.excluded[faceId];
  vlens.scheduleRedraw();
}

async function reload(state: FacesState) {
  state.reloading = true;
  vlens.scheduleRedraw();
  const [review, err] = await server.GetFaceReview({});
  state.reloading = false;
  if (review) {
    load(state, review);
  } else {
    state.error = err || "Failed to reload faces";
  }
  vlens.scheduleRedraw();
}

function must<T>([resp, err]: rpc.Response<T>, fallback: string): T {
  if (!resp) throw new Error(err || fallback);
  return resp;
}

async function send(state: FacesState, action: () => Promise<string>) {
  state.pending++;
  state.error = "";
  vlens.scheduleRedraw();
  let failed = false;
  try {
    const notice = await action();
    if (notice) state.notice = notice;
  } catch (e) {
    failed = true;
    state.error = `${e instanceof Error ? e.message : e}. The list has been reloaded.`;
    state.stale = true;
  }
  state.pending--;
  const empty = state.groups.length === 0 && state.autoCards.length === 0;
  if (state.pending === 0 && state.stale && (failed || empty)) {
    await reload(state);
  }
  vlens.scheduleRedraw();
}

function removeFromGroup(state: FacesState, card: GroupCard, faceIds: number[]) {
  card.faces = card.faces.filter(f => !faceIds.includes(f.id));
  card.suggestedPersonId = 0;
  delete state.chosen[card.key];
  for (const face of card.faces) delete state.excluded[face.id];
  if (card.faces.length === 0) {
    state.groups = state.groups.filter(g => g !== card);
  }
  state.review!.unknownCount -= faceIds.length;
  state.stale = true;
}

function onAssignGroup(state: FacesState, card: GroupCard, personId: number) {
  const faceIds = includedIds(state, card.faces);
  if (faceIds.length === 0) return;
  const review = state.review!;
  const name = personName(review, personId);
  removeFromGroup(state, card, faceIds);
  send(state, async () => {
    const resp = must(await server.AssignFaces({ faceIds, personId }), "Failed to save");
    review.unknownCount += resp.skipped - resp.autoTagged;
    review.autoCount += resp.autoTagged;
    let notice = `Tagged ${name} in ${plural(resp.assigned, "photo")}.`;
    if (resp.skipped > 0) {
      notice += ` Left ${plural(resp.skipped, "face")} for review, since ${name} is already confirmed on ${resp.skipped !== 1 ? "those photos" : "that photo"}.`;
    }
    if (resp.autoTagged > 0) {
      notice += ` Found ${name} in ${plural(resp.autoTagged, "more photo")}; refresh to check them.`;
    }
    return notice;
  });
}

function onDismissGroup(state: FacesState, card: GroupCard) {
  const faceIds = includedIds(state, card.faces);
  if (faceIds.length === 0) return;
  removeFromGroup(state, card, faceIds);
  send(state, async () => {
    must(await server.DismissFaces({ faceIds }), "Failed to save");
    return "";
  });
}

function onReviewAuto(state: FacesState, card: AutoCard) {
  const review = state.review!;
  const name = personName(review, card.personId);
  const confirmed = includedIds(state, card.faces);
  const rejected = card.faces.filter(f => state.excluded[f.id]).map(f => f.id);
  state.autoCards = state.autoCards.filter(c => c !== card);
  review.autoCount -= card.faces.length;
  review.unknownCount += rejected.length;
  state.stale = true;
  send(state, async () => {
    const parts: string[] = [];
    if (confirmed.length > 0) {
      const resp = must(
        await server.AssignFaces({ faceIds: confirmed, personId: card.personId }),
        "Failed to save"
      );
      review.unknownCount -= resp.autoTagged;
      review.autoCount += resp.autoTagged;
      parts.push(`Confirmed ${name} in ${plural(resp.assigned, "photo")}.`);
      if (resp.autoTagged > 0) {
        parts.push(`Found ${name} in ${plural(resp.autoTagged, "more photo")}.`);
      }
    }
    if (rejected.length > 0) {
      must(await server.RejectFaces({ faceIds: rejected }), "Failed to save");
      parts.push(
        `Removed ${name} from ${plural(rejected.length, "face")}; they'll wait in “Who is this?”.`
      );
    }
    return parts.join(" ");
  });
}
