import { ChevronLeft, ChevronRight, X } from "lucide-preact";
import { Icon } from "../../components/Icon";
import * as preact from "preact";
import * as vlens from "vlens";
import * as rpc from "vlens/rpc";
import * as server from "../../server";
import * as auth from "../../lib/authCache";
import { Header, Footer } from "../../layout";
import { requireAuthInView, ensureAuthInFetch } from "../../lib/authHelpers";
import { getIdFromRoute } from "../../lib/routeHelpers";
import { formatDate, formatDateRange, isRealDate, toDateInputValue } from "../../lib/dateUtils";
import { localDateString } from "../../lib/when";
import { PagedPhotoPicker } from "../../components/PhotoPicker";
import {
  MediaUploadButton,
  MediaUploadError,
  MediaUploadState,
  newMediaUpload,
} from "../../components/MediaUpload";
import {
  MatchupDraft,
  commonRounds,
  draftError,
  draftToMatchup,
  emptyMatchupDraft,
  hasMatchup,
  matchupToDraft,
  outcomeFromScore,
  outcomeOptions,
  runSummary,
} from "../../lib/matchups";
import { eventStatus, quickLabels } from "../../lib/activityResults";
import { ActivityKindSport, ActivityLabels, labelsFor } from "./labels";
import { MatchupLine, MediaGrid, ResultBadges } from "./results";
import {
  ResultRow,
  ResultsEditor,
  resultRowsBlocked,
  resultToRow,
  rowToInput,
} from "./results-editor";
import "./activities-styles";
import "./season-styles";
import "./competition-styles";

export type CompetitionPageData = {
  detail: server.GetEventDetailResponse;
  activity: server.Activity;
  events: server.Event[];
  entries: server.EntryView[];
  people: server.Person[];
  vocabulary: server.ListActivityVocabularyResponse;
};

const emptyDetail: server.GetEventDetailResponse = {
  event: {
    id: 0,
    seasonId: 0,
    familyId: 0,
    name: "",
    host: "",
    location: "",
    startDate: "",
    endDate: "",
    notes: "",
    createdAt: "",
  },
  season: { id: 0, name: "", kind: "", startDate: "", endDate: "" },
  photoIds: [],
  appearances: [],
};

const emptyActivity: server.Activity = {
  id: 0,
  familyId: 0,
  name: "",
  kind: "",
  createdAt: "",
};

const emptyVocabulary: server.ListActivityVocabularyResponse = {
  activityId: 0,
  adjudications: [],
  awards: [],
  categories: [],
  styles: [],
  divisions: [],
  levels: [],
  formats: [],
  hosts: [],
};

export async function fetch(
  route: string,
  prefix: string
): Promise<rpc.Response<CompetitionPageData>> {
  if (!(await ensureAuthInFetch())) {
    return rpc.ok<CompetitionPageData>({
      detail: emptyDetail,
      activity: emptyActivity,
      events: [],
      entries: [],
      people: [],
      vocabulary: emptyVocabulary,
    });
  }

  const [detail, detailErr] = await server.GetEventDetail({ eventId: getIdFromRoute(route) || 0 });
  if (detailErr || !detail) {
    return [null, detailErr || "Failed to load competition"];
  }

  const [overview] = await server.GetSeasonOverview({ seasonId: detail.season.id });
  const [people] = await server.ListPeople({});
  const [vocabulary] = await server.ListActivityVocabulary({
    activityId: overview?.activity.id ?? 0,
  });

  return rpc.ok<CompetitionPageData>({
    detail,
    activity: overview?.activity ?? emptyActivity,
    events: overview?.events ?? [],
    entries: overview?.entries ?? [],
    people: people?.people ?? [],
    vocabulary: vocabulary ?? emptyVocabulary,
  });
}

type PerformanceDraft = {
  appearanceId: number;
  entryId: number;
  occurredAt: string;
  notes: string;
  matchup: MatchupDraft;
  showMatchup: boolean;
  photoIds: number[];
  browsing: boolean;
};

type CompetitionState = {
  initialized: boolean;
  eventId: number;
  appearances: server.AppearanceDetail[];

  draft: PerformanceDraft | null;
  resultRows: ResultRow[];

  eventPhotoIds: number[];
  editingEventPhotos: boolean;
  eventPhotoDraft: number[];
  upload: MediaUploadState;

  error: string;
  saving: boolean;
};

const useCompetitionState = vlens.declareHook(
  (): CompetitionState => ({
    initialized: false,
    eventId: 0,
    appearances: [],
    draft: null,
    resultRows: [],
    eventPhotoIds: [],
    editingEventPhotos: false,
    eventPhotoDraft: [],
    upload: newMediaUpload(),
    error: "",
    saving: false,
  })
);

type EntryCard = {
  entry: server.Entry;
  personIds: number[];
  appearances: server.AppearanceDetail[];
};

function entryCards(
  entries: server.EntryView[],
  appearances: server.AppearanceDetail[]
): EntryCard[] {
  const cards = new Map<number, EntryCard>();
  for (const detail of appearances) {
    const card = cards.get(detail.entry.id);
    if (card) {
      card.appearances.push(detail);
      continue;
    }
    const view = entries.find(row => row.entry.id === detail.entry.id);
    cards.set(detail.entry.id, {
      entry: detail.entry,
      personIds: view?.personIds ?? [],
      appearances: [detail],
    });
  }
  for (const view of entries) {
    if (!cards.has(view.entry.id)) {
      cards.set(view.entry.id, {
        entry: view.entry,
        personIds: view.personIds ?? [],
        appearances: [],
      });
    }
  }
  return [...cards.values()];
}

function namesOf(people: server.Person[], personIds: number[]): string[] {
  return personIds
    .map(id => people.find(person => person.id === id)?.name)
    .filter((name): name is string => !!name);
}

export function view(
  route: string,
  prefix: string,
  data: CompetitionPageData
): preact.ComponentChild {
  const currentAuth = requireAuthInView();
  if (!currentAuth) return;

  const event = data.detail.event;
  const state = useCompetitionState();
  if (!state.initialized || state.eventId !== event.id) {
    state.initialized = true;
    state.eventId = event.id;
    state.appearances = [...(data.detail.appearances ?? [])];
    state.draft = null;
    state.resultRows = [];
    state.eventPhotoIds = [...(data.detail.photoIds ?? [])];
    state.editingEventPhotos = false;
    state.eventPhotoDraft = [];
    state.upload = newMediaUpload();
    state.error = "";
  }

  const labels = labelsFor(data.activity);
  const editable = auth.canContribute(event.familyId);
  const today = localDateString(new Date());
  const status = eventStatus(
    toDateInputValue(event.startDate),
    toDateInputValue(event.endDate),
    today
  );
  const dates = formatDateRange(event.startDate, event.endDate);
  const where = [event.location, event.host].filter(part => part).join(" · ");
  const cards = entryCards(data.entries, state.appearances).filter(
    card => editable || card.appearances.length > 0
  );
  const recorded = cards.filter(card => card.appearances.length > 0).length;

  return (
    <div>
      <Header isHome={false} />
      <main id="app" className="activities-container">
        <a className="back-link" href={`/season/${data.detail.season.id}`}>
          ←{" "}
          {[data.activity.name, data.detail.season.name].filter(part => part).join(" · ") ||
            "Season"}
        </a>

        <div className="comp-hero">
          <span className="season-eyebrow">
            {labels.event}
            {status !== "done" && (
              <span className={`comp-status comp-status-${status}`}>
                {status === "now" ? "Happening now" : "Upcoming"}
              </span>
            )}
          </span>
          <h1>{event.name}</h1>
          {dates && <p className="comp-when">{dates}</p>}
          {where && <p className="comp-where">{where}</p>}
          {event.notes && <p className="season-notes">{event.notes}</p>}
        </div>

        {state.error && (
          <div className="error-message" role="alert">
            {state.error}
          </div>
        )}

        <section className="activities-section">
          <div className="activities-section-head">
            <h2>Results</h2>
            {editable && data.entries.length > 0 && (
              <span className="comp-progress">
                {recorded} of {data.entries.length}{" "}
                {(data.entries.length === 1 ? labels.entry : labels.entryPlural).toLowerCase()}{" "}
                recorded
              </span>
            )}
          </div>

          {cards.length === 0 ? (
            <div className="empty-state">
              <p>
                {editable
                  ? `This season has no ${labels.entryPlural.toLowerCase()} yet — add one on the season page first.`
                  : `Nothing recorded for this ${labels.event.toLowerCase()} yet.`}
              </p>
            </div>
          ) : (
            <div className="perf-cards">
              {cards.map(card => (
                <EntryCardView
                  key={card.entry.id}
                  state={state}
                  data={data}
                  card={card}
                  labels={labels}
                  editable={editable}
                />
              ))}
            </div>
          )}
        </section>

        <EventPhotos state={state} data={data} labels={labels} editable={editable} />

        <SeasonNav data={data} labels={labels} />
      </main>
      <Footer />
    </div>
  );
}

const EntryCardView = ({
  state,
  data,
  card,
  labels,
  editable,
}: {
  state: CompetitionState;
  data: CompetitionPageData;
  card: EntryCard;
  labels: ActivityLabels;
  editable: boolean;
}) => {
  const entry = card.entry;
  const traits = [entry.format, entry.style, entry.division, entry.level].filter(part => part);
  const meta = [...traits, ...namesOf(data.people, card.personIds)].join(" · ");
  const draft = state.draft;
  const addingHere = draft !== null && draft.appearanceId === 0 && draft.entryId === entry.id;
  const editingHere = draft !== null && draft.entryId === entry.id;
  const summary =
    card.appearances.length > 1 ? runSummary(card.appearances.map(d => d.appearance)) : "";
  const empty = card.appearances.length === 0;

  return (
    <article className={empty && !addingHere ? "perf-card perf-card-empty" : "perf-card"}>
      <header className="perf-card-head">
        <div className="perf-card-title">
          <a href={`/routine/${entry.id}`}>{entry.name}</a>
          {meta && <span className="perf-card-meta">{meta}</span>}
        </div>
        {editable && empty && !addingHere && (
          <button
            className="btn btn-primary btn-small"
            onClick={vlens.cachePartial(onStartNew, state, data, entry.id)}
            disabled={state.saving}
          >
            Record results
          </button>
        )}
      </header>

      {summary && <p className="perf-run-summary">{summary}</p>}

      {card.appearances.map(detail =>
        draft !== null && draft.appearanceId === detail.appearance.id ? (
          <PerformanceEditor
            key={detail.appearance.id}
            state={state}
            data={data}
            draft={draft}
            entry={card}
            labels={labels}
            existing={detail}
          />
        ) : (
          <PerformanceView
            key={detail.appearance.id}
            state={state}
            data={data}
            detail={detail}
            editable={editable}
          />
        )
      )}

      {addingHere && (
        <PerformanceEditor
          state={state}
          data={data}
          draft={draft}
          entry={card}
          labels={labels}
          existing={null}
        />
      )}

      {editable && !empty && !editingHere && (
        <button
          className="perf-another"
          onClick={vlens.cachePartial(onStartNew, state, data, entry.id)}
          disabled={state.saving}
        >
          + Another {labels.appearance.toLowerCase()}
        </button>
      )}
    </article>
  );
};

function showDate(detail: server.AppearanceDetail, event: server.Event): boolean {
  if (!isRealDate(detail.appearance.occurredAt)) return false;
  const multiDay =
    isRealDate(event.endDate) &&
    toDateInputValue(event.endDate) !== toDateInputValue(event.startDate);
  return (
    multiDay || toDateInputValue(detail.appearance.occurredAt) !== toDateInputValue(event.startDate)
  );
}

const PerformanceView = ({
  state,
  data,
  detail,
  editable,
}: {
  state: CompetitionState;
  data: CompetitionPageData;
  detail: server.AppearanceDetail;
  editable: boolean;
}) => {
  const results = detail.results ?? [];
  const photos = detail.photoIds ?? [];
  const blank = results.length === 0 && photos.length === 0 && !hasMatchup(detail.appearance);

  return (
    <div className="perf">
      <div className="perf-top">
        <div className="perf-top-main">
          <MatchupLine appearance={detail.appearance} />
          {showDate(detail, data.detail.event) && (
            <span className="perf-date">{formatDate(detail.appearance.occurredAt)}</span>
          )}
          {blank && <span className="perf-date">Nothing recorded yet</span>}
          <ResultBadges results={results} people={data.people} />
        </div>
        {editable && (
          <button
            className="btn btn-secondary btn-small"
            onClick={vlens.cachePartial(onStartEdit, state, data, detail)}
            disabled={state.saving}
          >
            {blank ? "Add results" : "Edit"}
          </button>
        )}
      </div>
      {detail.appearance.notes && <p className="perf-notes">{detail.appearance.notes}</p>}
      <MediaGrid photoIds={photos} />
    </div>
  );
};

const PerformanceEditor = ({
  state,
  data,
  draft,
  entry,
  labels,
  existing,
}: {
  state: CompetitionState;
  data: CompetitionPageData;
  draft: PerformanceDraft;
  entry: EntryCard;
  labels: ActivityLabels;
  existing: server.AppearanceDetail | null;
}) => {
  const roster = data.people.filter(person => entry.personIds.includes(person.id));
  const busy = state.saving || state.upload.uploading;
  const idPrefix = existing ? `perf${existing.appearance.id}` : "perfNew";
  const sport = data.activity.kind === ActivityKindSport;
  const matchup = <MatchupFields state={state} draft={draft} idPrefix={idPrefix} labels={labels} />;

  return (
    <div className="perf-editor">
      {sport && matchup}
      <ResultsEditor
        host={state}
        vocabulary={data.vocabulary}
        roster={roster}
        quick={quickLabels(data.activity.kind, data.vocabulary.adjudications)}
      />
      {!sport && matchup}

      <div className="form-row">
        <div className="form-group flex-1">
          <label htmlFor={`${idPrefix}Date`}>Date</label>
          <input
            id={`${idPrefix}Date`}
            type="date"
            value={draft.occurredAt}
            onInput={e => {
              draft.occurredAt = e.currentTarget.value;
              vlens.scheduleRedraw();
            }}
            disabled={state.saving}
          />
        </div>
        <div className="form-group flex-2">
          <label htmlFor={`${idPrefix}Notes`}>Notes</label>
          <textarea
            id={`${idPrefix}Notes`}
            rows={2}
            value={draft.notes}
            onInput={e => {
              draft.notes = e.currentTarget.value;
              vlens.scheduleRedraw();
            }}
            disabled={state.saving}
          />
        </div>
      </div>

      <div className="form-group">
        <label>Photos &amp; video</label>
        <DraftMedia
          photoIds={draft.photoIds}
          disabled={state.saving}
          onRemove={photoId => onRemoveDraftPhoto(draft, photoId)}
        />
        <div className="perf-media-actions">
          <MediaUploadButton
            state={state.upload}
            personIds={entry.personIds}
            familyId={data.detail.event.familyId}
            disabled={state.saving}
            onUploaded={image => {
              if (state.draft === draft && !draft.photoIds.includes(image.id)) {
                draft.photoIds.push(image.id);
              }
            }}
          />
          <button
            type="button"
            className="btn btn-secondary"
            onClick={() => {
              draft.browsing = !draft.browsing;
              vlens.scheduleRedraw();
            }}
            disabled={state.saving}
          >
            {draft.browsing ? "Done choosing" : "Choose from library"}
          </button>
        </div>
        <MediaUploadError state={state.upload} />
        {draft.browsing && (
          <PagedPhotoPicker
            pageKey="competition-photos"
            filters={{}}
            selectedIds={draft.photoIds}
            onToggle={photoId => onToggleDraftPhoto(draft, photoId)}
            disabled={state.saving}
            emptyText="Nothing uploaded yet."
          />
        )}
      </div>

      <div className="form-actions perf-editor-actions">
        <button
          className="btn btn-primary"
          onClick={vlens.cachePartial(onSavePerformance, state, data)}
          disabled={busy || resultRowsBlocked(state)}
        >
          Save
        </button>
        <button
          className="btn btn-secondary"
          onClick={vlens.cachePartial(onCancelDraft, state)}
          disabled={busy}
        >
          Cancel
        </button>
        {existing && (
          <button
            className="perf-delete"
            onClick={vlens.cachePartial(onDeleteAppearance, state, existing, labels)}
            disabled={busy}
          >
            Delete this {labels.appearance.toLowerCase()}
          </button>
        )}
      </div>
    </div>
  );
};

const DraftMedia = ({
  photoIds,
  disabled,
  onRemove,
}: {
  photoIds: number[];
  disabled: boolean;
  onRemove: (photoId: number) => void;
}) => {
  if (photoIds.length === 0) return null;
  return (
    <div className="media-grid media-grid-small draft-media">
      {photoIds.map(id => (
        <div key={id} className="media-grid-item draft-media-item">
          <img src={`/api/photo/${id}/thumb`} alt="" loading="lazy" />
          <button
            type="button"
            className="draft-media-remove"
            title="Remove"
            aria-label="Remove"
            onClick={() => onRemove(id)}
            disabled={disabled}
          >
            <Icon symbol={X} />
          </button>
        </div>
      ))}
    </div>
  );
};

function usedValues(state: CompetitionState, pick: (a: server.Appearance) => string): string[] {
  const seen = new Set<string>();
  for (const detail of state.appearances) {
    const value = pick(detail.appearance);
    if (value) seen.add(value);
  }
  return [...seen].sort((a, b) => a.localeCompare(b));
}

function onScoreInput(draft: MatchupDraft, field: "scoreFor" | "scoreAgainst", value: string) {
  const derivedBefore = outcomeFromScore(draft);
  draft[field] = value;
  if (draft.outcome === "" || draft.outcome === derivedBefore) {
    draft.outcome = outcomeFromScore(draft);
  }
  vlens.scheduleRedraw();
}

const MatchupFields = ({
  state,
  draft,
  idPrefix,
  labels,
}: {
  state: CompetitionState;
  draft: PerformanceDraft;
  idPrefix: string;
  labels: ActivityLabels;
}) => {
  if (!draft.showMatchup) {
    return (
      <div className="perf-editor-extra">
        <button
          type="button"
          className="result-chip"
          onClick={() => {
            draft.showMatchup = true;
            vlens.scheduleRedraw();
          }}
          disabled={state.saving}
          title={`For a bracket or a game: the round, who it was against, and whether the ${labels.entry.toLowerCase()} won.`}
        >
          + Head-to-head
        </button>
      </div>
    );
  }

  const matchup = draft.matchup;
  const rounds = [...new Set([...commonRounds, ...usedValues(state, a => a.round)])];
  const opponents = usedValues(state, a => a.opponent);
  return (
    <fieldset className="matchup-fields">
      <legend>Head-to-head</legend>
      <div className="form-row">
        <div className="form-group">
          <label htmlFor={`${idPrefix}Round`}>Round</label>
          <input
            id={`${idPrefix}Round`}
            type="text"
            list={`${idPrefix}Rounds`}
            maxLength={100}
            value={matchup.round}
            onInput={e => {
              matchup.round = e.currentTarget.value;
              vlens.scheduleRedraw();
            }}
            disabled={state.saving}
          />
          <datalist id={`${idPrefix}Rounds`}>
            {rounds.map(round => (
              <option key={round} value={round} />
            ))}
          </datalist>
        </div>
        <div className="form-group">
          <label htmlFor={`${idPrefix}Opponent`}>Against</label>
          <input
            id={`${idPrefix}Opponent`}
            type="text"
            list={`${idPrefix}Opponents`}
            maxLength={100}
            value={matchup.opponent}
            onInput={e => {
              matchup.opponent = e.currentTarget.value;
              vlens.scheduleRedraw();
            }}
            disabled={state.saving}
          />
          <datalist id={`${idPrefix}Opponents`}>
            {opponents.map(opponent => (
              <option key={opponent} value={opponent} />
            ))}
          </datalist>
        </div>
      </div>
      <div className="form-row">
        <div className="form-group">
          <label htmlFor={`${idPrefix}Outcome`}>Outcome</label>
          <select
            id={`${idPrefix}Outcome`}
            value={matchup.outcome}
            onInput={e => {
              matchup.outcome = e.currentTarget.value;
              vlens.scheduleRedraw();
            }}
            disabled={state.saving}
          >
            {outcomeOptions.map(option => (
              <option key={option.value} value={option.value}>
                {option.label}
              </option>
            ))}
          </select>
        </div>
        <div className="form-group">
          <label htmlFor={`${idPrefix}ScoreFor`}>Score</label>
          <div className="matchup-score">
            <input
              id={`${idPrefix}ScoreFor`}
              type="text"
              inputMode="numeric"
              aria-label="Our score"
              placeholder="Us"
              value={matchup.scoreFor}
              onInput={e => onScoreInput(matchup, "scoreFor", e.currentTarget.value)}
              disabled={state.saving}
            />
            <span aria-hidden="true">–</span>
            <input
              id={`${idPrefix}ScoreAgainst`}
              type="text"
              inputMode="numeric"
              aria-label="Their score"
              placeholder="Them"
              value={matchup.scoreAgainst}
              onInput={e => onScoreInput(matchup, "scoreAgainst", e.currentTarget.value)}
              disabled={state.saving}
            />
          </div>
          <p className="form-hint">Optional — leave empty when there is only a winner.</p>
        </div>
      </div>
    </fieldset>
  );
};

const EventPhotos = ({
  state,
  data,
  labels,
  editable,
}: {
  state: CompetitionState;
  data: CompetitionPageData;
  labels: ActivityLabels;
  editable: boolean;
}) => {
  const event = data.detail.event;
  if (!editable && state.eventPhotoIds.length === 0) return null;
  const busy = state.saving || state.upload.uploading;

  return (
    <section className="activities-section">
      <div className="activities-section-head">
        <h2>Around the {labels.event.toLowerCase()}</h2>
        {editable && !state.editingEventPhotos && (
          <button
            className="btn btn-secondary btn-small"
            onClick={vlens.cachePartial(onStartEventPhotos, state)}
            disabled={state.saving}
          >
            {state.eventPhotoIds.length === 0 ? "Add photos" : "Edit photos"}
          </button>
        )}
      </div>

      {state.editingEventPhotos ? (
        <div className="activities-form">
          <p className="form-hint">
            The shots that are not of any one {labels.entry.toLowerCase()} — the venue, the team,
            the trophy table. A {labels.entry.toLowerCase()}'s own photos go on its results above.
          </p>
          <DraftMedia
            photoIds={state.eventPhotoDraft}
            disabled={state.saving}
            onRemove={photoId => onToggleEventPhoto(state, photoId)}
          />
          <div className="perf-media-actions">
            <MediaUploadButton
              state={state.upload}
              personIds={[]}
              familyId={event.familyId}
              disabled={state.saving}
              onUploaded={image => {
                if (state.editingEventPhotos && !state.eventPhotoDraft.includes(image.id)) {
                  state.eventPhotoDraft.push(image.id);
                }
              }}
            />
          </div>
          <MediaUploadError state={state.upload} />
          <PagedPhotoPicker
            pageKey="competition-photos"
            filters={{}}
            selectedIds={state.eventPhotoDraft}
            onToggle={photoId => onToggleEventPhoto(state, photoId)}
            disabled={state.saving}
            emptyText="Nothing uploaded yet."
          />
          <div className="form-actions">
            <button
              className="btn btn-primary"
              onClick={vlens.cachePartial(onSaveEventPhotos, state)}
              disabled={busy}
            >
              Save
            </button>
            <button
              className="btn btn-secondary"
              onClick={vlens.cachePartial(onCancelEventPhotos, state)}
              disabled={busy}
            >
              Cancel
            </button>
          </div>
        </div>
      ) : state.eventPhotoIds.length === 0 ? (
        <p className="form-hint">No photos of the {labels.event.toLowerCase()} itself yet.</p>
      ) : (
        <MediaGrid photoIds={state.eventPhotoIds} />
      )}
    </section>
  );
};

const SeasonNav = ({ data, labels }: { data: CompetitionPageData; labels: ActivityLabels }) => {
  const idx = data.events.findIndex(e => e.id === data.detail.event.id);
  if (idx < 0) return null;
  const prev = idx > 0 ? data.events[idx - 1] : null;
  const next = idx < data.events.length - 1 ? data.events[idx + 1] : null;
  if (!prev && !next) return null;

  return (
    <nav className="comp-nav" aria-label={`Other ${labels.eventPlural.toLowerCase()}`}>
      {prev ? (
        <a className="comp-nav-link" href={`/competition/${prev.id}`}>
          <Icon symbol={ChevronLeft} />
          <span>
            <small>Previous</small>
            {prev.name}
          </span>
        </a>
      ) : (
        <span />
      )}
      {next && (
        <a className="comp-nav-link comp-nav-next" href={`/competition/${next.id}`}>
          <span>
            <small>Next</small>
            {next.name}
          </span>
          <Icon symbol={ChevronRight} />
        </a>
      )}
    </nav>
  );
};

function dateOrNull(value: string): string | null {
  const trimmed = value.trim();
  return trimmed === "" ? null : trimmed;
}

function closeEditors(state: CompetitionState) {
  state.draft = null;
  state.resultRows = [];
  state.editingEventPhotos = false;
  state.eventPhotoDraft = [];
  state.upload.uploadError = "";
}

function onStartNew(state: CompetitionState, data: CompetitionPageData, entryId: number) {
  closeEditors(state);
  const sport = data.activity.kind === ActivityKindSport;
  state.draft = {
    appearanceId: 0,
    entryId,
    occurredAt: toDateInputValue(data.detail.event.startDate),
    notes: "",
    matchup: emptyMatchupDraft(),
    showMatchup: sport,
    photoIds: [],
    browsing: false,
  };
  vlens.scheduleRedraw();
}

function onStartEdit(
  state: CompetitionState,
  data: CompetitionPageData,
  detail: server.AppearanceDetail
) {
  closeEditors(state);
  state.draft = {
    appearanceId: detail.appearance.id,
    entryId: detail.entry.id,
    occurredAt: toDateInputValue(detail.appearance.occurredAt),
    notes: detail.appearance.notes,
    matchup: matchupToDraft(detail.appearance),
    showMatchup: data.activity.kind === ActivityKindSport || hasMatchup(detail.appearance),
    photoIds: [...(detail.photoIds ?? [])],
    browsing: false,
  };
  state.resultRows = (detail.results ?? []).map(resultToRow);
  vlens.scheduleRedraw();
}

function onCancelDraft(state: CompetitionState) {
  closeEditors(state);
  vlens.scheduleRedraw();
}

function onToggleDraftPhoto(draft: PerformanceDraft, photoId: number) {
  const idx = draft.photoIds.indexOf(photoId);
  if (idx >= 0) draft.photoIds.splice(idx, 1);
  else draft.photoIds.push(photoId);
  vlens.scheduleRedraw();
}

function onRemoveDraftPhoto(draft: PerformanceDraft, photoId: number) {
  const idx = draft.photoIds.indexOf(photoId);
  if (idx >= 0) draft.photoIds.splice(idx, 1);
  vlens.scheduleRedraw();
}

async function onSavePerformance(state: CompetitionState, data: CompetitionPageData) {
  const draft = state.draft;
  if (!draft || resultRowsBlocked(state)) return;
  if (draft.showMatchup) {
    state.error = draftError(draft.matchup);
    if (state.error) {
      vlens.scheduleRedraw();
      return;
    }
  }

  state.saving = true;
  state.error = "";
  vlens.scheduleRedraw();

  const [resp, err] = await server.SaveAppearance({
    id: draft.appearanceId,
    eventId: data.detail.event.id,
    entryId: draft.entryId,
    occurredAt: dateOrNull(draft.occurredAt),
    notes: draft.notes.trim(),
    matchup: draft.showMatchup ? draftToMatchup(draft.matchup) : null,
    results: state.resultRows.map(rowToInput),
    photoIds: draft.photoIds,
  });
  state.saving = false;
  if (err || !resp) {
    state.error = err || "Failed to save";
    vlens.scheduleRedraw();
    return;
  }
  closeEditors(state);
  await reload(state);
}

async function onDeleteAppearance(
  state: CompetitionState,
  detail: server.AppearanceDetail,
  labels: ActivityLabels
) {
  const count = (detail.results ?? []).length;
  const tail =
    count === 0
      ? "This cannot be undone."
      : `Its ${count === 1 ? "result" : `${count} results`} go with it. This cannot be undone.`;
  if (
    !confirm(
      `Delete ${detail.entry.name}'s ${labels.appearance.toLowerCase()} at this ${labels.event.toLowerCase()}? ${tail}`
    )
  ) {
    return;
  }

  state.saving = true;
  state.error = "";
  vlens.scheduleRedraw();

  const [, err] = await server.DeleteAppearance({ id: detail.appearance.id });
  if (err) {
    state.error = err || "Failed to delete";
  } else {
    const idx = state.appearances.findIndex(row => row.appearance.id === detail.appearance.id);
    if (idx >= 0) state.appearances.splice(idx, 1);
    closeEditors(state);
  }
  state.saving = false;
  vlens.scheduleRedraw();
}

function onStartEventPhotos(state: CompetitionState) {
  closeEditors(state);
  state.editingEventPhotos = true;
  state.eventPhotoDraft = [...state.eventPhotoIds];
  vlens.scheduleRedraw();
}

function onCancelEventPhotos(state: CompetitionState) {
  closeEditors(state);
  vlens.scheduleRedraw();
}

function onToggleEventPhoto(state: CompetitionState, photoId: number) {
  const idx = state.eventPhotoDraft.indexOf(photoId);
  if (idx >= 0) state.eventPhotoDraft.splice(idx, 1);
  else state.eventPhotoDraft.push(photoId);
  vlens.scheduleRedraw();
}

async function onSaveEventPhotos(state: CompetitionState) {
  state.saving = true;
  state.error = "";
  vlens.scheduleRedraw();

  const [resp, err] = await server.SetEventPhotos({
    eventId: state.eventId,
    photoIds: state.eventPhotoDraft,
  });
  if (err || !resp) {
    state.error = err || "Failed to save photos";
  } else {
    state.eventPhotoIds = resp.photoIds ?? [];
    closeEditors(state);
  }
  state.saving = false;
  vlens.scheduleRedraw();
}

async function reload(state: CompetitionState) {
  const [resp, err] = await server.GetEventDetail({ eventId: state.eventId });
  if (err || !resp) {
    state.error = err || "Failed to reload";
  } else {
    state.appearances = [...(resp.appearances ?? [])];
  }
  vlens.scheduleRedraw();
}
