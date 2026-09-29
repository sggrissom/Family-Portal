import * as preact from "preact";
import * as vlens from "vlens";
import * as rpc from "vlens/rpc";
import * as server from "../../server";
import { Header, Footer } from "../../layout";
import { requireAuthInView, ensureAuthInFetch } from "../../lib/authHelpers";
import { ThumbnailImage } from "../../components/ResponsiveImage";
import "./suggestions-styles";

const emptyReview: server.GetTagSuggestionsResponse = { enabled: false, groups: [], total: 0 };

export async function fetch(
  route: string,
  prefix: string
): Promise<rpc.Response<server.GetTagSuggestionsResponse>> {
  if (!(await ensureAuthInFetch())) {
    return rpc.ok(emptyReview);
  }
  return server.GetTagSuggestions({});
}

const PHOTOS_PER_GROUP = 24;

type SuggestionsState = {
  review: server.GetTagSuggestionsResponse | null;
  excluded: Record<number, boolean>;
  busy: boolean;
  error: string;
  notice: string;
};

const useSuggestionsState = vlens.declareHook(
  (): SuggestionsState => ({ review: null, excluded: {}, busy: false, error: "", notice: "" })
);

const shownIn = (group: server.SuggestionGroup) => group.suggestions.slice(0, PHOTOS_PER_GROUP);

function includedIds(state: SuggestionsState, group: server.SuggestionGroup): number[] {
  return shownIn(group)
    .filter(s => !state.excluded[s.id])
    .map(s => s.id);
}

export function view(
  route: string,
  prefix: string,
  data: server.GetTagSuggestionsResponse
): preact.ComponentChild {
  if (!requireAuthInView()) return;

  const state = useSuggestionsState();
  if (state.review === null) state.review = data;
  const review = state.review;

  return (
    <div>
      <Header isHome={false} />
      <main id="app" className="suggestions-container">
        <div className="suggestions-header">
          <div>
            <h1>Suggested Tags</h1>
            <p className="suggestions-subtitle">
              Tags the app thinks fit your photos. Nothing is tagged until you say so, and anything
              you turn down won't be suggested again.
            </p>
          </div>
          <a href="/photos" className="btn btn-secondary">
            ← Photos
          </a>
        </div>

        {state.error && (
          <div className="suggestions-error" role="alert">
            {state.error}
          </div>
        )}
        {state.notice && (
          <div className="suggestions-notice" role="status">
            {state.notice}
          </div>
        )}

        {review.groups.length === 0 && (
          <div className="suggestions-empty">
            {review.enabled ? (
              <p>
                No suggestions right now. New photos are checked after they upload. You can teach
                the app your own tags by describing them on the <a href="/tags">tags page</a>.
              </p>
            ) : (
              <p>Photo analysis isn't running on this server, so there's nothing to suggest.</p>
            )}
          </div>
        )}

        {review.groups.map(group => (
          <SuggestionGroupCard key={group.key} state={state} group={group} />
        ))}
      </main>
      <Footer />
    </div>
  );
}

const SuggestionGroupCard = ({
  state,
  group,
}: {
  state: SuggestionsState;
  group: server.SuggestionGroup;
}) => {
  const ids = includedIds(state, group);
  const shown = shownIn(group);
  const rest = group.suggestions.length - shown.length;
  return (
    <section className="suggestion-group">
      <div className="suggestion-group-header">
        <h2>
          <span className="suggestion-swatch" style={{ background: group.color }} />
          {group.label}
          <span className="suggestions-count">{group.suggestions.length}</span>
        </h2>
        <div className="suggestion-group-actions">
          <button
            className="btn btn-primary"
            disabled={state.busy || ids.length === 0}
            onClick={vlens.cachePartial(onAccept, state, group, ids)}
          >
            Tag {ids.length} photo{ids.length !== 1 ? "s" : ""} “{group.label}”
          </button>
          <button
            className="btn btn-secondary"
            disabled={state.busy || ids.length === 0}
            onClick={vlens.cachePartial(onReject, state, ids)}
          >
            Not {ids.length === shown.length ? "these" : "the selected"}
          </button>
        </div>
      </div>
      <p className="suggestions-hint">Click a photo to leave it out.</p>
      <div className="suggestion-grid">
        {shown.map(s => (
          <button
            key={s.id}
            type="button"
            className={`suggestion-photo ${state.excluded[s.id] ? "excluded" : ""}`}
            aria-pressed={!state.excluded[s.id]}
            title={state.excluded[s.id] ? "Include this photo" : "Leave this photo out"}
            onClick={vlens.cachePartial(onToggle, state, s.id)}
          >
            <ThumbnailImage photoId={s.photoId} alt={group.label} className="suggestion-thumb" />
          </button>
        ))}
      </div>
      {rest > 0 && <p className="suggestions-hint">{rest} more after these.</p>}
    </section>
  );
};

function onToggle(state: SuggestionsState, id: number) {
  state.excluded[id] = !state.excluded[id];
  vlens.scheduleRedraw();
}

async function run(state: SuggestionsState, action: () => Promise<string>) {
  state.busy = true;
  state.error = "";
  state.notice = "";
  vlens.scheduleRedraw();
  const notice = await action();
  if (!state.error) {
    const [review, err] = await server.GetTagSuggestions({});
    if (review) {
      state.review = review;
      state.excluded = {};
    } else {
      state.error = err || "Failed to reload suggestions";
    }
    state.notice = notice;
  }
  state.busy = false;
  vlens.scheduleRedraw();
}

async function onAccept(state: SuggestionsState, group: server.SuggestionGroup, ids: number[]) {
  await run(state, async () => {
    const [resp, err] = await server.AcceptTagSuggestions({ ids });
    if (!resp) {
      state.error = err || "Failed to tag photos";
      return "";
    }
    return `Tagged ${resp.updated} photo${resp.updated !== 1 ? "s" : ""} “${group.label}”.`;
  });
}

async function onReject(state: SuggestionsState, ids: number[]) {
  await run(state, async () => {
    const [, err] = await server.RejectTagSuggestions({ ids });
    if (err) state.error = err;
    return "";
  });
}
