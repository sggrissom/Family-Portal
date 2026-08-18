// One competition: which routines performed there, and how each did.
//
// See docs/activities-plan.md, phase 6.

import * as preact from "preact";
import * as vlens from "vlens";
import * as rpc from "vlens/rpc";
import * as server from "../../server";
import { Header, Footer } from "../../layout";
import { requireAuthInView, ensureAuthInFetch } from "../../lib/authHelpers";
import { getIdFromRoute } from "../../lib/routeHelpers";
import { formatDate, formatDateRange, isRealDate, toDateInputValue } from "../../lib/dateUtils";
import { ActivityLabels, labelsFor } from "./labels";
import { ResultList } from "./results";
import { ResultRow, ResultsEditor, resultToRow, rowError, rowToInput } from "./results-editor";
import "./activities-styles";
import "./season-styles";
import "./competition-styles";

// CompetitionPageData is the competition plus what the add-performance form
// needs: the routines in this season, and the names a result can point at.
//
// The routines come from GetSeasonOverview because it is the only proc that
// lists them — GetEventDetail carries an entry per performance, which by
// definition excludes every routine that has not performed here yet, and those
// are exactly the ones the form is for.
export type CompetitionPageData = {
  detail: server.GetEventDetailResponse;
  activity: server.Activity;
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
  season: { id: 0, name: "", startDate: "", endDate: "" },
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
      entries: [],
      people: [],
      vocabulary: emptyVocabulary,
    });
  }

  const [detail, detailErr] = await server.GetEventDetail({ eventId: getIdFromRoute(route) || 0 });
  if (detailErr || !detail) {
    return [null, detailErr || "Failed to load competition"];
  }

  // None of these is worth failing the page over: without them the competition
  // still reads, only the forms are short of choices.
  const [overview] = await server.GetSeasonOverview({ seasonId: detail.season.id });
  const [people] = await server.ListPeople({});
  const [vocabulary] = await server.ListActivityVocabulary({
    activityId: overview?.activity.id ?? 0,
  });

  return rpc.ok<CompetitionPageData>({
    detail,
    activity: overview?.activity ?? emptyActivity,
    entries: overview?.entries ?? [],
    people: people?.people ?? [],
    vocabulary: vocabulary ?? emptyVocabulary,
  });
}

type CompetitionState = {
  initialized: boolean;
  eventId: number;
  appearances: server.AppearanceDetail[];

  adding: boolean;
  newEntryId: number;
  newOccurredAt: string;
  newNotes: string;

  editingId: number;
  editOccurredAt: string;
  editNotes: string;

  // The performance whose results are open for editing, and the rows being
  // edited. Only one is open at a time: results are replace-all, so two open
  // editors would be two pending overwrites of different sets.
  editingResultsFor: number;
  resultRows: ResultRow[];

  error: string;
  saving: boolean;
};

const useCompetitionState = vlens.declareHook(
  (): CompetitionState => ({
    initialized: false,
    eventId: 0,
    appearances: [],
    adding: false,
    newEntryId: 0,
    newOccurredAt: "",
    newNotes: "",
    editingId: 0,
    editOccurredAt: "",
    editNotes: "",
    editingResultsFor: 0,
    resultRows: [],
    error: "",
    saving: false,
  })
);

export function view(
  route: string,
  prefix: string,
  data: CompetitionPageData
): preact.ComponentChild {
  const currentAuth = requireAuthInView();
  if (!currentAuth) return;

  const event = data.detail.event;
  const state = useCompetitionState();
  // The hook outlives a route change between two competitions, so reinitialize
  // whenever the competition under it is a different one.
  if (!state.initialized || state.eventId !== event.id) {
    state.initialized = true;
    state.eventId = event.id;
    state.appearances = [...(data.detail.appearances ?? [])];
    state.adding = false;
    state.editingId = 0;
    state.editingResultsFor = 0;
    state.resultRows = [];
    state.error = "";
  }

  const labels = labelsFor(data.activity);
  const dates = formatDateRange(event.startDate, event.endDate);
  const where = [event.host, event.location].filter(part => part).join(" · ");

  return (
    <div>
      <Header isHome={false} />
      <main id="app" className="activities-container">
        <a className="back-link" href={`/season/${data.detail.season.id}`}>
          ← {data.detail.season.name || "Season"}
        </a>

        <div className="season-header">
          <span className="season-eyebrow">{labels.event}</span>
          <h1>{event.name}</h1>
          {(dates || where) && (
            <p className="season-dates">{[dates, where].filter(part => part).join(" — ")}</p>
          )}
          {event.notes && <p className="season-notes">{event.notes}</p>}
        </div>

        {state.error && <div className="error-message">{state.error}</div>}

        <section className="activities-section">
          <div className="activities-section-head">
            <h2>{labels.appearancePlural}</h2>
            {!state.adding && (
              <button
                className="btn btn-primary"
                onClick={vlens.cachePartial(onShowAddForm, state, data)}
                disabled={state.saving || data.entries.length === 0}
              >
                Add {labels.appearance.toLowerCase()}
              </button>
            )}
          </div>

          {state.adding && <AddAppearanceForm state={state} data={data} labels={labels} />}

          {data.entries.length === 0 && !state.adding && (
            <p className="form-hint">
              This season has no {labels.entryPlural.toLowerCase()} yet — add one on the season page
              first.
            </p>
          )}

          {state.appearances.length === 0 ? (
            <div className="empty-state">
              <p>Nothing recorded for this {labels.event.toLowerCase()} yet.</p>
            </div>
          ) : (
            <ul className="event-list">
              {state.appearances.map(detail => (
                <li key={detail.appearance.id} className="event-item appearance-item">
                  {state.editingId === detail.appearance.id ? (
                    <EditAppearanceForm state={state} detail={detail} labels={labels} />
                  ) : state.editingResultsFor === detail.appearance.id ? (
                    <div className="appearance-editing">
                      <strong className="event-name">{detail.entry.name}</strong>
                      <ResultsEditor
                        host={state}
                        vocabulary={data.vocabulary}
                        roster={rosterOf(data, detail.entry.id)}
                        onSave={vlens.cachePartial(onSaveResults, state, detail.appearance.id)}
                        onCancel={vlens.cachePartial(onCancelResults, state)}
                      />
                    </div>
                  ) : (
                    <AppearanceRow state={state} data={data} detail={detail} labels={labels} />
                  )}
                </li>
              ))}
            </ul>
          )}
        </section>
      </main>
      <Footer />
    </div>
  );
}

const AppearanceRow = ({
  state,
  data,
  detail,
  labels,
}: {
  state: CompetitionState;
  data: CompetitionPageData;
  detail: server.AppearanceDetail;
  labels: ActivityLabels;
}) => {
  const entry = detail.entry;
  const traits = [entry.format, entry.style, entry.division, entry.level]
    .filter(part => part)
    .join(" · ");

  return (
    <>
      <div className="event-item-main">
        <strong className="event-name">{entry.name}</strong>
        {traits && <span className="event-meta">{traits}</span>}
        {isRealDate(detail.appearance.occurredAt) && (
          <span className="event-count">{formatDate(detail.appearance.occurredAt)}</span>
        )}
        <ResultList results={detail.results} people={data.people} />
        {detail.appearance.notes && <p className="event-notes">{detail.appearance.notes}</p>}
      </div>
      <span className="event-item-actions">
        <button
          className="btn btn-secondary btn-small"
          onClick={vlens.cachePartial(onStartEditResults, state, detail)}
          disabled={state.saving}
        >
          {(detail.results ?? []).length === 0 ? "Add results" : "Edit results"}
        </button>
        <button
          className="icon-btn"
          title={`Edit ${labels.appearance.toLowerCase()}`}
          onClick={vlens.cachePartial(onStartEdit, state, detail)}
          disabled={state.saving}
        >
          ✏️
        </button>
        <button
          className="icon-btn"
          title={`Delete ${labels.appearance.toLowerCase()}`}
          onClick={vlens.cachePartial(onDeleteAppearance, state, detail, labels)}
          disabled={state.saving}
        >
          🗑️
        </button>
      </span>
    </>
  );
};

const AddAppearanceForm = ({
  state,
  data,
  labels,
}: {
  state: CompetitionState;
  data: CompetitionPageData;
  labels: ActivityLabels;
}) => (
  <div className="activities-form">
    <div className="form-group">
      <label htmlFor="appearanceEntry">{labels.entry}</label>
      <select
        id="appearanceEntry"
        value={String(state.newEntryId)}
        onInput={e => {
          state.newEntryId = Number(e.currentTarget.value);
          vlens.scheduleRedraw();
        }}
        disabled={state.saving}
      >
        {data.entries.map(entryView => (
          <option key={entryView.entry.id} value={String(entryView.entry.id)}>
            {entryView.entry.name}
          </option>
        ))}
      </select>
    </div>
    <div className="form-group">
      <label htmlFor="appearanceDate">Date</label>
      <input
        id="appearanceDate"
        type="date"
        value={state.newOccurredAt}
        onInput={e => {
          state.newOccurredAt = e.currentTarget.value;
          vlens.scheduleRedraw();
        }}
        disabled={state.saving}
      />
      <p className="form-hint">
        Leave empty for "sometime that {labels.event === "Competition" ? "weekend" : "day"}".
      </p>
    </div>
    <div className="form-group">
      <label htmlFor="appearanceNotes">Notes</label>
      <textarea
        id="appearanceNotes"
        rows={2}
        value={state.newNotes}
        onInput={e => {
          state.newNotes = e.currentTarget.value;
          vlens.scheduleRedraw();
        }}
        disabled={state.saving}
      />
    </div>
    <div className="form-actions">
      <button
        className="btn btn-primary"
        onClick={vlens.cachePartial(onCreateAppearance, state)}
        disabled={state.saving || state.newEntryId === 0}
      >
        Add {labels.appearance.toLowerCase()}
      </button>
      <button
        className="btn btn-secondary"
        onClick={vlens.cachePartial(onCancelForm, state)}
        disabled={state.saving}
      >
        Cancel
      </button>
    </div>
  </div>
);

const EditAppearanceForm = ({
  state,
  detail,
  labels,
}: {
  state: CompetitionState;
  detail: server.AppearanceDetail;
  labels: ActivityLabels;
}) => (
  <div className="activities-form">
    <p className="form-hint">
      {detail.entry.name} — which {labels.entry.toLowerCase()} performed here is the identity of
      this record, so it cannot be changed. Delete and re-enter to move it.
    </p>
    <div className="form-group">
      <label htmlFor="editAppearanceDate">Date</label>
      <input
        id="editAppearanceDate"
        type="date"
        value={state.editOccurredAt}
        onInput={e => {
          state.editOccurredAt = e.currentTarget.value;
          vlens.scheduleRedraw();
        }}
        disabled={state.saving}
      />
    </div>
    <div className="form-group">
      <label htmlFor="editAppearanceNotes">Notes</label>
      <textarea
        id="editAppearanceNotes"
        rows={2}
        value={state.editNotes}
        onInput={e => {
          state.editNotes = e.currentTarget.value;
          vlens.scheduleRedraw();
        }}
        disabled={state.saving}
      />
    </div>
    <div className="form-actions">
      <button
        className="btn btn-primary"
        onClick={vlens.cachePartial(onSaveAppearance, state, detail.appearance.id)}
        disabled={state.saving}
      >
        Save
      </button>
      <button
        className="btn btn-secondary"
        onClick={vlens.cachePartial(onCancelForm, state)}
        disabled={state.saving}
      >
        Cancel
      </button>
    </div>
  </div>
);

// ── handlers ─────────────────────────────────────────────────────────────────

function dateOrNull(value: string): string | null {
  const trimmed = value.trim();
  return trimmed === "" ? null : trimmed;
}

function onShowAddForm(state: CompetitionState, data: CompetitionPageData) {
  state.adding = true;
  state.editingId = 0;
  state.newEntryId = data.entries.length > 0 ? data.entries[0].entry.id : 0;
  // Most performances happen on the competition's own dates, and a single-day
  // competition has only one candidate. Prefilling saves the common case a
  // click without hiding that the field is optional.
  state.newOccurredAt = toDateInputValue(data.detail.event.startDate);
  state.newNotes = "";
  vlens.scheduleRedraw();
}

function onStartEdit(state: CompetitionState, detail: server.AppearanceDetail) {
  state.adding = false;
  state.editingResultsFor = 0;
  state.editingId = detail.appearance.id;
  state.editOccurredAt = toDateInputValue(detail.appearance.occurredAt);
  state.editNotes = detail.appearance.notes;
  vlens.scheduleRedraw();
}

function onCancelForm(state: CompetitionState) {
  state.adding = false;
  state.editingId = 0;
  vlens.scheduleRedraw();
}

// ── result handlers ──────────────────────────────────────────────────────────

// rosterOf finds the people a result on this entry may narrow to. The season
// overview carries the roster ids; ListPeople carries the names, and drops
// anyone this viewer cannot see.
function rosterOf(data: CompetitionPageData, entryId: number): server.Person[] {
  const entryView = data.entries.find(row => row.entry.id === entryId);
  const ids = entryView?.personIds ?? [];
  return ids
    .map(id => data.people.find(person => person.id === id))
    .filter((person): person is server.Person => !!person);
}

function onStartEditResults(state: CompetitionState, detail: server.AppearanceDetail) {
  state.adding = false;
  state.editingId = 0;
  state.editingResultsFor = detail.appearance.id;
  state.resultRows = (detail.results ?? []).map(resultToRow);
  vlens.scheduleRedraw();
}

function onCancelResults(state: CompetitionState) {
  state.editingResultsFor = 0;
  state.resultRows = [];
  vlens.scheduleRedraw();
}

async function onSaveResults(state: CompetitionState, appearanceId: number) {
  if (state.resultRows.some(row => rowError(row) !== "")) return;

  state.saving = true;
  state.error = "";
  vlens.scheduleRedraw();

  const [resp, err] = await server.SetAppearanceResults({
    appearanceId,
    results: state.resultRows.map(rowToInput),
  });
  if (err || !resp) {
    state.error = err || "Failed to save results";
  } else {
    const idx = state.appearances.findIndex(row => row.appearance.id === appearanceId);
    if (idx >= 0) {
      state.appearances[idx] = {
        ...state.appearances[idx],
        appearance: resp.appearance.appearance,
        results: resp.appearance.results ?? [],
      };
    }
    state.editingResultsFor = 0;
    state.resultRows = [];
  }
  state.saving = false;
  vlens.scheduleRedraw();
}

// sortAppearances mirrors appearanceOrder on the server: by the performance's
// own time, falling back to the competition's start date when it has none,
// ties broken by id so two entered off the same sheet keep their order.
function sortAppearances(appearances: server.AppearanceDetail[]) {
  appearances.sort((a, b) => {
    const at = isRealDate(a.appearance.occurredAt) ? a.appearance.occurredAt : a.event.startDate;
    const bt = isRealDate(b.appearance.occurredAt) ? b.appearance.occurredAt : b.event.startDate;
    if (at !== bt) return at < bt ? -1 : 1;
    return a.appearance.id - b.appearance.id;
  });
}

async function onCreateAppearance(state: CompetitionState) {
  if (state.newEntryId === 0) return;
  state.saving = true;
  state.error = "";
  vlens.scheduleRedraw();

  const [resp, err] = await server.CreateAppearance({
    eventId: state.eventId,
    entryId: state.newEntryId,
    occurredAt: dateOrNull(state.newOccurredAt),
    notes: state.newNotes.trim(),
  });
  if (err || !resp) {
    state.error = err || "Failed to add";
  } else {
    state.adding = false;
  }
  state.saving = false;
  // A new performance needs the entry and event rows the list renders, which
  // the create response does not carry. Reloading is one call and keeps the
  // ordering the server's.
  if (!err && resp) {
    await reload(state);
    return;
  }
  vlens.scheduleRedraw();
}

async function onSaveAppearance(state: CompetitionState, appearanceId: number) {
  state.saving = true;
  state.error = "";
  vlens.scheduleRedraw();

  const [resp, err] = await server.UpdateAppearance({
    id: appearanceId,
    occurredAt: dateOrNull(state.editOccurredAt),
    notes: state.editNotes.trim(),
  });
  if (err || !resp) {
    state.error = err || "Failed to save";
  } else {
    const idx = state.appearances.findIndex(row => row.appearance.id === appearanceId);
    if (idx >= 0) {
      const existing = state.appearances[idx];
      state.appearances[idx] = {
        ...existing,
        appearance: resp.appearance.appearance,
        results: resp.appearance.results ?? existing.results,
      };
      sortAppearances(state.appearances);
    }
    state.editingId = 0;
  }
  state.saving = false;
  vlens.scheduleRedraw();
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
    if (state.editingId === detail.appearance.id) state.editingId = 0;
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
