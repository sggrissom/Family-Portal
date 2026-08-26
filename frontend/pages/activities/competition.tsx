import * as preact from "preact";
import * as vlens from "vlens";
import * as rpc from "vlens/rpc";
import * as server from "../../server";
import { Header, Footer } from "../../layout";
import { requireAuthInView, ensureAuthInFetch } from "../../lib/authHelpers";
import { getIdFromRoute } from "../../lib/routeHelpers";
import { formatDate, formatDateRange, isRealDate, toDateInputValue } from "../../lib/dateUtils";
import { PhotoPicker, PhotoStrip } from "../../components/PhotoPicker";
import { ActivityLabels, labelsFor } from "./labels";
import { ResultList } from "./results";
import { ResultRow, ResultsEditor, resultToRow, rowError, rowToInput } from "./results-editor";
import "./activities-styles";
import "./season-styles";
import "./competition-styles";

export type CompetitionPageData = {
  detail: server.GetEventDetailResponse;
  activity: server.Activity;
  entries: server.EntryView[];
  people: server.Person[];
  vocabulary: server.ListActivityVocabularyResponse;
  photos: server.PhotoWithPeople[];
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
      entries: [],
      people: [],
      vocabulary: emptyVocabulary,
      photos: [],
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
  const [photos] = await server.ListFamilyPhotos({ personId: 0 });

  return rpc.ok<CompetitionPageData>({
    detail,
    activity: overview?.activity ?? emptyActivity,
    entries: overview?.entries ?? [],
    people: people?.people ?? [],
    vocabulary: vocabulary ?? emptyVocabulary,
    photos: photos?.photos ?? [],
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

  editingResultsFor: number;
  resultRows: ResultRow[];

  eventPhotoIds: number[];
  editingEventPhotos: boolean;
  editingPhotosFor: number;
  photoDraft: number[];

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
    eventPhotoIds: [],
    editingEventPhotos: false,
    editingPhotosFor: 0,
    photoDraft: [],
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
  if (!state.initialized || state.eventId !== event.id) {
    state.initialized = true;
    state.eventId = event.id;
    state.appearances = [...(data.detail.appearances ?? [])];
    state.adding = false;
    state.editingId = 0;
    state.editingResultsFor = 0;
    state.resultRows = [];
    state.eventPhotoIds = [...(data.detail.photoIds ?? [])];
    state.editingEventPhotos = false;
    state.editingPhotosFor = 0;
    state.photoDraft = [];
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

        {state.error && (
          <div className="error-message" role="alert">
            {state.error}
          </div>
        )}

        <section className="activities-section">
          <div className="activities-section-head">
            <h2>Photos</h2>
            {!state.editingEventPhotos && (
              <button
                className="btn btn-secondary"
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
                Photos of the {labels.event.toLowerCase()} itself — the weekend shots that are not
                of any one {labels.entry.toLowerCase()}. A {labels.entry.toLowerCase()}'s own photos
                hang off its {labels.appearance.toLowerCase()} below.
              </p>
              <PhotoPicker
                photos={data.photos}
                selectedIds={state.photoDraft}
                onToggle={photoId => onTogglePhotoDraft(state, photoId)}
                disabled={state.saving}
                emptyText="No photos yet — upload some first."
              />
              <div className="form-actions">
                <button
                  className="btn btn-primary"
                  onClick={vlens.cachePartial(onSaveEventPhotos, state)}
                  disabled={state.saving}
                >
                  Save
                </button>
                <button
                  className="btn btn-secondary"
                  onClick={vlens.cachePartial(onCancelPhotos, state)}
                  disabled={state.saving}
                >
                  Cancel
                </button>
              </div>
            </div>
          ) : state.eventPhotoIds.length === 0 ? (
            <p className="form-hint">No photos of this {labels.event.toLowerCase()} yet.</p>
          ) : (
            <PhotoStrip photoIds={state.eventPhotoIds} />
          )}
        </section>

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
                  ) : state.editingPhotosFor === detail.appearance.id ? (
                    <div className="appearance-editing">
                      <strong className="event-name">{detail.entry.name}</strong>
                      <PhotoPicker
                        photos={data.photos}
                        selectedIds={state.photoDraft}
                        onToggle={photoId => onTogglePhotoDraft(state, photoId)}
                        disabled={state.saving}
                        emptyText="No photos yet — upload some first."
                      />
                      <div className="form-actions">
                        <button
                          className="btn btn-primary"
                          onClick={vlens.cachePartial(
                            onSaveAppearancePhotos,
                            state,
                            detail.appearance.id
                          )}
                          disabled={state.saving}
                        >
                          Save
                        </button>
                        <button
                          className="btn btn-secondary"
                          onClick={vlens.cachePartial(onCancelPhotos, state)}
                          disabled={state.saving}
                        >
                          Cancel
                        </button>
                      </div>
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
        <PhotoStrip photoIds={detail.photoIds} />
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
          title={`Photos of this ${labels.appearance.toLowerCase()}`}
          aria-label={`Photos of this ${labels.appearance.toLowerCase()}`}
          onClick={vlens.cachePartial(onStartAppearancePhotos, state, detail)}
          disabled={state.saving}
        >
          📸
        </button>
        <button
          className="icon-btn"
          title={`Edit ${labels.appearance.toLowerCase()}`}
          aria-label={`Edit ${labels.appearance.toLowerCase()}`}
          onClick={vlens.cachePartial(onStartEdit, state, detail)}
          disabled={state.saving}
        >
          ✏️
        </button>
        <button
          className="icon-btn"
          title={`Delete ${labels.appearance.toLowerCase()}`}
          aria-label={`Delete ${labels.appearance.toLowerCase()}`}
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

function dateOrNull(value: string): string | null {
  const trimmed = value.trim();
  return trimmed === "" ? null : trimmed;
}

function onShowAddForm(state: CompetitionState, data: CompetitionPageData) {
  state.adding = true;
  state.editingId = 0;
  closePhotoEditors(state);
  state.newEntryId = data.entries.length > 0 ? data.entries[0].entry.id : 0;
  state.newOccurredAt = toDateInputValue(data.detail.event.startDate);
  state.newNotes = "";
  vlens.scheduleRedraw();
}

function onStartEdit(state: CompetitionState, detail: server.AppearanceDetail) {
  state.adding = false;
  state.editingResultsFor = 0;
  closePhotoEditors(state);
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

function closePhotoEditors(state: CompetitionState) {
  state.editingEventPhotos = false;
  state.editingPhotosFor = 0;
  state.photoDraft = [];
}

function onStartEventPhotos(state: CompetitionState) {
  state.adding = false;
  state.editingId = 0;
  state.editingResultsFor = 0;
  state.editingPhotosFor = 0;
  state.editingEventPhotos = true;
  state.photoDraft = [...state.eventPhotoIds];
  vlens.scheduleRedraw();
}

function onStartAppearancePhotos(state: CompetitionState, detail: server.AppearanceDetail) {
  state.adding = false;
  state.editingId = 0;
  state.editingResultsFor = 0;
  state.editingEventPhotos = false;
  state.editingPhotosFor = detail.appearance.id;
  state.photoDraft = [...(detail.photoIds ?? [])];
  vlens.scheduleRedraw();
}

function onCancelPhotos(state: CompetitionState) {
  closePhotoEditors(state);
  vlens.scheduleRedraw();
}

function onTogglePhotoDraft(state: CompetitionState, photoId: number) {
  const idx = state.photoDraft.indexOf(photoId);
  if (idx >= 0) state.photoDraft.splice(idx, 1);
  else state.photoDraft.push(photoId);
  vlens.scheduleRedraw();
}

async function onSaveEventPhotos(state: CompetitionState) {
  state.saving = true;
  state.error = "";
  vlens.scheduleRedraw();

  const [resp, err] = await server.SetEventPhotos({
    eventId: state.eventId,
    photoIds: state.photoDraft,
  });
  if (err || !resp) {
    state.error = err || "Failed to save photos";
  } else {
    state.eventPhotoIds = resp.photoIds ?? [];
    closePhotoEditors(state);
  }
  state.saving = false;
  vlens.scheduleRedraw();
}

async function onSaveAppearancePhotos(state: CompetitionState, appearanceId: number) {
  state.saving = true;
  state.error = "";
  vlens.scheduleRedraw();

  const [resp, err] = await server.SetAppearancePhotos({
    appearanceId,
    photoIds: state.photoDraft,
  });
  if (err || !resp) {
    state.error = err || "Failed to save photos";
  } else {
    const idx = state.appearances.findIndex(row => row.appearance.id === appearanceId);
    if (idx >= 0) {
      state.appearances[idx] = {
        ...state.appearances[idx],
        photoIds: resp.appearance.photoIds ?? [],
      };
    }
    closePhotoEditors(state);
  }
  state.saving = false;
  vlens.scheduleRedraw();
}

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
  closePhotoEditors(state);
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
