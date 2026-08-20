// A season's overview: the competitions in it, and later the routines and how
// each one placed. Everything below a program hangs off this page.
//
// See docs/activities-plan.md, phase 6.

import * as preact from "preact";
import * as vlens from "vlens";
import * as rpc from "vlens/rpc";
import * as server from "../../server";
import { Header, Footer } from "../../layout";
import { requireAuthInView, ensureAuthInFetch } from "../../lib/authHelpers";
import { getIdFromRoute } from "../../lib/routeHelpers";
import { formatDateRange, toDateInputValue } from "../../lib/dateUtils";
import { ActivityLabels, labelsFor } from "./labels";
import "./activities-styles";
import "./season-styles";

// SeasonPageData is the overview plus the two lists the forms on this page
// need: who can be rostered, and what the family has typed into these fields
// before. Fetching them here rather than per-form keeps the page to one load.
export type SeasonPageData = {
  overview: server.GetSeasonOverviewResponse;
  people: server.Person[];
  vocabulary: server.ListActivityVocabularyResponse;
};

const emptyOverview: server.GetSeasonOverviewResponse = {
  activity: { id: 0, familyId: 0, name: "", kind: "", createdAt: "" },
  season: {
    id: 0,
    activityId: 0,
    familyId: 0,
    name: "",
    startDate: "",
    endDate: "",
    notes: "",
    createdAt: "",
  },
  events: [],
  entries: [],
  appearances: [],
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

export async function fetch(route: string, prefix: string): Promise<rpc.Response<SeasonPageData>> {
  if (!(await ensureAuthInFetch())) {
    return rpc.ok<SeasonPageData>({
      overview: emptyOverview,
      people: [],
      vocabulary: emptyVocabulary,
    });
  }

  const [overview, overviewErr] = await server.GetSeasonOverview({
    seasonId: getIdFromRoute(route) || 0,
  });
  if (overviewErr || !overview) {
    return [null, overviewErr || "Failed to load season"];
  }

  // Neither of these is worth failing the page over. A roster picker with no
  // names or a field with no suggestions is degraded, not broken.
  const [people] = await server.ListPeople({});
  const [vocabulary] = await server.ListActivityVocabulary({
    activityId: overview.activity.id,
  });

  return rpc.ok<SeasonPageData>({
    overview,
    people: people?.people ?? [],
    vocabulary: vocabulary ?? emptyVocabulary,
  });
}

type SeasonState = {
  initialized: boolean;
  seasonId: number;
  events: server.Event[];
  // Performance counts per event, so a competition row can say whether
  // anything has been recorded for it yet. Kept as a plain map rebuilt on
  // load: appearances are only added on the competition page, which is a
  // separate route, so this never goes stale under us.
  appearanceCounts: Record<number, number>;

  addingEvent: boolean;
  editingEventId: number;
  form: EventForm;

  entries: server.EntryView[];
  // Performance counts per entry, same reason as the per-event counts.
  entryAppearanceCounts: Record<number, number>;
  addingEntry: boolean;
  editingEntryId: number;
  entryForm: EntryForm;

  error: string;
  saving: boolean;
};

type EventForm = {
  name: string;
  host: string;
  location: string;
  startDate: string;
  endDate: string;
  notes: string;
};

const blankEventForm = (): EventForm => ({
  name: "",
  host: "",
  location: "",
  startDate: "",
  endDate: "",
  notes: "",
});

type EntryForm = {
  name: string;
  format: string;
  style: string;
  division: string;
  level: string;
  notes: string;
  personIds: number[];
};

const blankEntryForm = (): EntryForm => ({
  name: "",
  format: "",
  style: "",
  division: "",
  level: "",
  notes: "",
  personIds: [],
});

const useSeasonState = vlens.declareHook(
  (): SeasonState => ({
    initialized: false,
    seasonId: 0,
    events: [],
    appearanceCounts: {},
    addingEvent: false,
    editingEventId: 0,
    form: blankEventForm(),
    entries: [],
    entryAppearanceCounts: {},
    addingEntry: false,
    editingEntryId: 0,
    entryForm: blankEntryForm(),
    error: "",
    saving: false,
  })
);

function countBy(
  appearances: server.AppearanceView[],
  key: (view: server.AppearanceView) => number
): Record<number, number> {
  const counts: Record<number, number> = {};
  for (const view of appearances) {
    const id = key(view);
    counts[id] = (counts[id] ?? 0) + 1;
  }
  return counts;
}

export function view(route: string, prefix: string, data: SeasonPageData): preact.ComponentChild {
  const currentAuth = requireAuthInView();
  if (!currentAuth) return;

  const overview = data.overview;
  const state = useSeasonState();
  // The hook outlives a route change between two seasons, so reinitialize
  // whenever the season under it is a different one.
  if (!state.initialized || state.seasonId !== overview.season.id) {
    const appearances = overview.appearances ?? [];
    state.initialized = true;
    state.seasonId = overview.season.id;
    state.events = [...(overview.events ?? [])];
    state.appearanceCounts = countBy(appearances, a => a.appearance.eventId);
    state.entries = [...(overview.entries ?? [])];
    state.entryAppearanceCounts = countBy(appearances, a => a.appearance.entryId);
    state.addingEvent = false;
    state.editingEventId = 0;
    state.addingEntry = false;
    state.editingEntryId = 0;
    state.error = "";
  }

  const labels = labelsFor(overview.activity);
  const dates = formatDateRange(overview.season.startDate, overview.season.endDate);

  return (
    <div>
      <Header isHome={false} />
      <main id="app" className="activities-container">
        <a className="back-link" href="/activities">
          ← Activities
        </a>

        <div className="season-header">
          <span className="season-eyebrow">{overview.activity.name}</span>
          <h1>{overview.season.name}</h1>
          {dates && <p className="season-dates">{dates}</p>}
          {overview.season.notes && <p className="season-notes">{overview.season.notes}</p>}
        </div>

        {state.error && (
          <div className="error-message" role="alert">
            {state.error}
          </div>
        )}

        <section className="activities-section">
          <div className="activities-section-head">
            <h2>{labels.eventPlural}</h2>
            {!state.addingEvent && (
              <button
                className="btn btn-primary"
                onClick={vlens.cachePartial(onShowEventForm, state)}
                disabled={state.saving}
              >
                Add {labels.event.toLowerCase()}
              </button>
            )}
          </div>

          {state.addingEvent && (
            <EventFormFields
              state={state}
              labels={labels}
              submitLabel={`Add ${labels.event.toLowerCase()}`}
              onSubmit={vlens.cachePartial(onCreateEvent, state)}
              onCancel={vlens.cachePartial(onCancelEventForm, state)}
            />
          )}

          {state.events.length === 0 ? (
            <div className="empty-state">
              <p>
                No {labels.eventPlural.toLowerCase()} yet. Add the first one to start recording
                results.
              </p>
            </div>
          ) : (
            <ul className="event-list">
              {state.events.map(event =>
                state.editingEventId === event.id ? (
                  <li key={event.id} className="event-item">
                    <EventFormFields
                      state={state}
                      labels={labels}
                      submitLabel="Save"
                      onSubmit={vlens.cachePartial(onSaveEvent, state, event.id)}
                      onCancel={vlens.cachePartial(onCancelEventForm, state)}
                    />
                  </li>
                ) : (
                  <li key={event.id} className="event-item">
                    <EventRow state={state} event={event} labels={labels} />
                  </li>
                )
              )}
            </ul>
          )}
        </section>

        <section className="activities-section">
          <div className="activities-section-head">
            <h2>{labels.entryPlural}</h2>
            {!state.addingEntry && (
              <button
                className="btn btn-primary"
                onClick={vlens.cachePartial(onShowEntryForm, state)}
                disabled={state.saving}
              >
                Add {labels.entry.toLowerCase()}
              </button>
            )}
          </div>

          {state.addingEntry && (
            <EntryFormFields
              state={state}
              data={data}
              labels={labels}
              submitLabel={`Add ${labels.entry.toLowerCase()}`}
              onSubmit={vlens.cachePartial(onCreateEntry, state)}
              onCancel={vlens.cachePartial(onCancelEntryForm, state)}
            />
          )}

          {state.entries.length === 0 ? (
            <div className="empty-state">
              <p>
                No {labels.entryPlural.toLowerCase()} yet. A {labels.entry.toLowerCase()} belongs to
                this season — one that carries over from last year is entered again.
              </p>
            </div>
          ) : (
            <ul className="event-list">
              {state.entries.map(entryView =>
                state.editingEntryId === entryView.entry.id ? (
                  <li key={entryView.entry.id} className="event-item">
                    <EntryFormFields
                      state={state}
                      data={data}
                      labels={labels}
                      submitLabel="Save"
                      onSubmit={vlens.cachePartial(onSaveEntry, state, entryView)}
                      onCancel={vlens.cachePartial(onCancelEntryForm, state)}
                    />
                  </li>
                ) : (
                  <li key={entryView.entry.id} className="event-item">
                    <EntryRow state={state} data={data} entryView={entryView} labels={labels} />
                  </li>
                )
              )}
            </ul>
          )}
        </section>
      </main>
      <Footer />
    </div>
  );
}

const EventRow = ({
  state,
  event,
  labels,
}: {
  state: SeasonState;
  event: server.Event;
  labels: ActivityLabels;
}) => {
  const dates = formatDateRange(event.startDate, event.endDate);
  const where = [event.host, event.location].filter(part => part).join(" · ");
  const count = state.appearanceCounts[event.id] ?? 0;

  return (
    <>
      <div className="event-item-main">
        <a className="event-name" href={`/competition/${event.id}`}>
          {event.name}
        </a>
        {(dates || where) && (
          <span className="event-meta">{[dates, where].filter(part => part).join(" — ")}</span>
        )}
        <span className="event-count">
          {count === 0
            ? `No ${labels.appearancePlural.toLowerCase()} recorded`
            : count === 1
              ? `1 ${labels.appearance.toLowerCase()}`
              : `${count} ${labels.appearancePlural.toLowerCase()}`}
        </span>
        {event.notes && <p className="event-notes">{event.notes}</p>}
      </div>
      <span className="event-item-actions">
        <button
          className="icon-btn"
          title={`Edit ${labels.event.toLowerCase()}`}
          aria-label={`Edit ${labels.event.toLowerCase()}`}
          onClick={vlens.cachePartial(onStartEditEvent, state, event)}
          disabled={state.saving}
        >
          ✏️
        </button>
        <button
          className="icon-btn"
          title={`Delete ${labels.event.toLowerCase()}`}
          aria-label={`Delete ${labels.event.toLowerCase()}`}
          onClick={vlens.cachePartial(onDeleteEvent, state, event, labels)}
          disabled={state.saving}
        >
          🗑️
        </button>
      </span>
    </>
  );
};

const EventFormFields = ({
  state,
  labels,
  submitLabel,
  onSubmit,
  onCancel,
}: {
  state: SeasonState;
  labels: ActivityLabels;
  submitLabel: string;
  onSubmit: () => void;
  onCancel: () => void;
}) => (
  <div className="activities-form">
    <div className="form-group">
      <label htmlFor="eventName">Name</label>
      <input
        id="eventName"
        type="text"
        placeholder={labels.event === "Competition" ? "Nuvo Nashville" : ""}
        value={state.form.name}
        onInput={e => {
          state.form.name = e.currentTarget.value;
          vlens.scheduleRedraw();
        }}
        disabled={state.saving}
      />
    </div>
    <div className="form-row">
      <div className="form-group flex-1">
        <label htmlFor="eventHost">Host</label>
        <input
          id="eventHost"
          type="text"
          value={state.form.host}
          onInput={e => {
            state.form.host = e.currentTarget.value;
            vlens.scheduleRedraw();
          }}
          disabled={state.saving}
        />
      </div>
      <div className="form-group flex-1">
        <label htmlFor="eventLocation">Location</label>
        <input
          id="eventLocation"
          type="text"
          value={state.form.location}
          onInput={e => {
            state.form.location = e.currentTarget.value;
            vlens.scheduleRedraw();
          }}
          disabled={state.saving}
        />
      </div>
    </div>
    <div className="form-row">
      <div className="form-group flex-1">
        <label htmlFor="eventStart">Starts</label>
        <input
          id="eventStart"
          type="date"
          value={state.form.startDate}
          onInput={e => {
            state.form.startDate = e.currentTarget.value;
            vlens.scheduleRedraw();
          }}
          disabled={state.saving}
        />
      </div>
      <div className="form-group flex-1">
        <label htmlFor="eventEnd">Ends</label>
        <input
          id="eventEnd"
          type="date"
          value={state.form.endDate}
          onInput={e => {
            state.form.endDate = e.currentTarget.value;
            vlens.scheduleRedraw();
          }}
          disabled={state.saving}
        />
      </div>
    </div>
    <p className="form-hint">
      Leave the end date empty for a single-day {labels.event.toLowerCase()}.
    </p>
    <div className="form-group">
      <label htmlFor="eventNotes">Notes</label>
      <textarea
        id="eventNotes"
        rows={2}
        value={state.form.notes}
        onInput={e => {
          state.form.notes = e.currentTarget.value;
          vlens.scheduleRedraw();
        }}
        disabled={state.saving}
      />
    </div>
    <div className="form-actions">
      <button
        className="btn btn-primary"
        onClick={onSubmit}
        disabled={state.saving || !state.form.name.trim()}
      >
        {submitLabel}
      </button>
      <button className="btn btn-secondary" onClick={onCancel} disabled={state.saving}>
        Cancel
      </button>
    </div>
  </div>
);

const EntryRow = ({
  state,
  data,
  entryView,
  labels,
}: {
  state: SeasonState;
  data: SeasonPageData;
  entryView: server.EntryView;
  labels: ActivityLabels;
}) => {
  const entry = entryView.entry;
  const traits = [entry.format, entry.style, entry.division, entry.level]
    .filter(part => part)
    .join(" · ");
  const roster = rosterNames(data.people, entryView.personIds);
  const count = state.entryAppearanceCounts[entry.id] ?? 0;

  return (
    <>
      <div className="event-item-main">
        <a className="event-name" href={`/routine/${entry.id}`}>
          {entry.name}
        </a>
        {traits && <span className="event-meta">{traits}</span>}
        <span className="event-meta">
          {roster.length > 0 ? roster.join(", ") : `No ${labels.roster.toLowerCase()} yet`}
        </span>
        <span className="event-count">
          {count === 0
            ? `Not yet at a ${labels.event.toLowerCase()}`
            : count === 1
              ? `1 ${labels.appearance.toLowerCase()}`
              : `${count} ${labels.appearancePlural.toLowerCase()}`}
        </span>
        {entry.notes && <p className="event-notes">{entry.notes}</p>}
      </div>
      <span className="event-item-actions">
        <button
          className="icon-btn"
          title={`Edit ${labels.entry.toLowerCase()}`}
          aria-label={`Edit ${labels.entry.toLowerCase()}`}
          onClick={vlens.cachePartial(onStartEditEntry, state, entryView)}
          disabled={state.saving}
        >
          ✏️
        </button>
        <button
          className="icon-btn"
          title={`Delete ${labels.entry.toLowerCase()}`}
          aria-label={`Delete ${labels.entry.toLowerCase()}`}
          onClick={vlens.cachePartial(onDeleteEntry, state, entryView, labels)}
          disabled={state.saving}
        >
          🗑️
        </button>
      </span>
    </>
  );
};

// Suggestions come from what this family has already typed into the same
// field. Without them "High Gold" becomes "high gold" and "Hi-Gold", and the
// season view can no longer even count labels, let alone group them.
const Suggestions = ({ id, values }: { id: string; values: string[] }) => (
  <datalist id={id}>
    {values.map(value => (
      <option key={value} value={value} />
    ))}
  </datalist>
);

const EntryFormFields = ({
  state,
  data,
  labels,
  submitLabel,
  onSubmit,
  onCancel,
}: {
  state: SeasonState;
  data: SeasonPageData;
  labels: ActivityLabels;
  submitLabel: string;
  onSubmit: () => void;
  onCancel: () => void;
}) => (
  <div className="activities-form">
    <div className="form-group">
      <label htmlFor="entryName">Name</label>
      <input
        id="entryName"
        type="text"
        placeholder={labels.entry === "Routine" ? "Rise Up" : ""}
        value={state.entryForm.name}
        onInput={e => {
          state.entryForm.name = e.currentTarget.value;
          vlens.scheduleRedraw();
        }}
        disabled={state.saving}
      />
    </div>
    <div className="form-row">
      <div className="form-group flex-1">
        <label htmlFor="entryFormat">Format</label>
        <input
          id="entryFormat"
          type="text"
          list="entryFormatOptions"
          placeholder={labels.entry === "Routine" ? "solo, duet, group" : ""}
          value={state.entryForm.format}
          onInput={e => {
            state.entryForm.format = e.currentTarget.value;
            vlens.scheduleRedraw();
          }}
          disabled={state.saving}
        />
        <Suggestions id="entryFormatOptions" values={data.vocabulary.formats ?? []} />
      </div>
      <div className="form-group flex-1">
        <label htmlFor="entryStyle">Style</label>
        <input
          id="entryStyle"
          type="text"
          list="entryStyleOptions"
          placeholder={labels.entry === "Routine" ? "Jazz, Lyrical" : ""}
          value={state.entryForm.style}
          onInput={e => {
            state.entryForm.style = e.currentTarget.value;
            vlens.scheduleRedraw();
          }}
          disabled={state.saving}
        />
        <Suggestions id="entryStyleOptions" values={data.vocabulary.styles ?? []} />
      </div>
    </div>
    <div className="form-row">
      <div className="form-group flex-1">
        <label htmlFor="entryDivision">Division</label>
        <input
          id="entryDivision"
          type="text"
          list="entryDivisionOptions"
          value={state.entryForm.division}
          onInput={e => {
            state.entryForm.division = e.currentTarget.value;
            vlens.scheduleRedraw();
          }}
          disabled={state.saving}
        />
        <Suggestions id="entryDivisionOptions" values={data.vocabulary.divisions ?? []} />
      </div>
      <div className="form-group flex-1">
        <label htmlFor="entryLevel">Level</label>
        <input
          id="entryLevel"
          type="text"
          list="entryLevelOptions"
          value={state.entryForm.level}
          onInput={e => {
            state.entryForm.level = e.currentTarget.value;
            vlens.scheduleRedraw();
          }}
          disabled={state.saving}
        />
        <Suggestions id="entryLevelOptions" values={data.vocabulary.levels ?? []} />
      </div>
    </div>

    <div className="form-group">
      <label>{labels.roster}</label>
      {data.people.length === 0 ? (
        <p className="form-hint">No people to add yet.</p>
      ) : (
        <div className="roster-picker">
          {data.people.map(person => (
            <label key={person.id} className="roster-option">
              <input
                type="checkbox"
                checked={state.entryForm.personIds.includes(person.id)}
                onChange={vlens.cachePartial(onToggleRosterMember, state, person.id)}
                disabled={state.saving}
              />
              <span>{person.name}</span>
            </label>
          ))}
        </div>
      )}
    </div>

    <div className="form-group">
      <label htmlFor="entryNotes">Notes</label>
      <textarea
        id="entryNotes"
        rows={2}
        value={state.entryForm.notes}
        onInput={e => {
          state.entryForm.notes = e.currentTarget.value;
          vlens.scheduleRedraw();
        }}
        disabled={state.saving}
      />
    </div>
    <div className="form-actions">
      <button
        className="btn btn-primary"
        onClick={onSubmit}
        disabled={state.saving || !state.entryForm.name.trim()}
      >
        {submitLabel}
      </button>
      <button className="btn btn-secondary" onClick={onCancel} disabled={state.saving}>
        Cancel
      </button>
    </div>
  </div>
);

// ── handlers ─────────────────────────────────────────────────────────────────

// Empty date inputs go over as null rather than "": the backend reads a nil
// pointer as "not known yet" and stores the zero time.
function dateOrNull(value: string): string | null {
  const trimmed = value.trim();
  return trimmed === "" ? null : trimmed;
}

function onShowEventForm(state: SeasonState) {
  state.addingEvent = true;
  state.editingEventId = 0;
  state.form = blankEventForm();
  vlens.scheduleRedraw();
}

function onStartEditEvent(state: SeasonState, event: server.Event) {
  state.addingEvent = false;
  state.editingEventId = event.id;
  state.form = {
    name: event.name,
    host: event.host,
    location: event.location,
    startDate: toDateInputValue(event.startDate),
    endDate: toDateInputValue(event.endDate),
    notes: event.notes,
  };
  vlens.scheduleRedraw();
}

function onCancelEventForm(state: SeasonState) {
  state.addingEvent = false;
  state.editingEventId = 0;
  vlens.scheduleRedraw();
}

// sortEvents keeps the client list in the order GetSeasonOverview returns:
// chronological, undated events first, ties broken by id.
function sortEvents(events: server.Event[]) {
  events.sort((a, b) => {
    if (a.startDate !== b.startDate) return a.startDate < b.startDate ? -1 : 1;
    return a.id - b.id;
  });
}

async function onCreateEvent(state: SeasonState) {
  const name = state.form.name.trim();
  if (!name || state.seasonId === 0) return;
  state.saving = true;
  state.error = "";
  vlens.scheduleRedraw();

  const [resp, err] = await server.CreateEvent({
    seasonId: state.seasonId,
    name,
    host: state.form.host.trim(),
    location: state.form.location.trim(),
    startDate: dateOrNull(state.form.startDate),
    endDate: dateOrNull(state.form.endDate),
    notes: state.form.notes.trim(),
  });
  if (err || !resp) {
    state.error = err || "Failed to add";
  } else {
    state.events.push(resp.event);
    sortEvents(state.events);
    state.addingEvent = false;
  }
  state.saving = false;
  vlens.scheduleRedraw();
}

async function onSaveEvent(state: SeasonState, eventId: number) {
  const name = state.form.name.trim();
  if (!name) return;
  state.saving = true;
  state.error = "";
  vlens.scheduleRedraw();

  const [resp, err] = await server.UpdateEvent({
    id: eventId,
    name,
    host: state.form.host.trim(),
    location: state.form.location.trim(),
    startDate: dateOrNull(state.form.startDate),
    endDate: dateOrNull(state.form.endDate),
    notes: state.form.notes.trim(),
  });
  if (err || !resp) {
    state.error = err || "Failed to save";
  } else {
    const idx = state.events.findIndex(e => e.id === eventId);
    if (idx >= 0) state.events[idx] = resp.event;
    sortEvents(state.events);
    state.editingEventId = 0;
  }
  state.saving = false;
  vlens.scheduleRedraw();
}

async function onDeleteEvent(state: SeasonState, event: server.Event, labels: ActivityLabels) {
  const count = state.appearanceCounts[event.id] ?? 0;
  const tail =
    count === 0
      ? "This cannot be undone."
      : `Its ${count} ${count === 1 ? labels.appearance.toLowerCase() : labels.appearancePlural.toLowerCase()} and their results go with it. This cannot be undone.`;
  if (!confirm(`Delete "${event.name}"? ${tail}`)) return;

  state.saving = true;
  state.error = "";
  vlens.scheduleRedraw();

  const [, err] = await server.DeleteEvent({ id: event.id });
  if (err) {
    state.error = err || "Failed to delete";
  } else {
    const idx = state.events.findIndex(e => e.id === event.id);
    if (idx >= 0) state.events.splice(idx, 1);
    delete state.appearanceCounts[event.id];
    if (state.editingEventId === event.id) state.editingEventId = 0;
  }
  state.saving = false;
  vlens.scheduleRedraw();
}

// ── routine handlers ─────────────────────────────────────────────────────────

// rosterNames resolves ids to names in the order the roster was stored, and
// drops any it cannot resolve. ListPeople is scoped to what the caller can
// see, so a routine can name someone this viewer has no access to.
function rosterNames(people: server.Person[], personIds: number[] | null): string[] {
  return (personIds ?? [])
    .map(id => people.find(person => person.id === id)?.name)
    .filter((name): name is string => !!name);
}

function onShowEntryForm(state: SeasonState) {
  state.addingEntry = true;
  state.editingEntryId = 0;
  state.entryForm = blankEntryForm();
  vlens.scheduleRedraw();
}

function onStartEditEntry(state: SeasonState, entryView: server.EntryView) {
  state.addingEntry = false;
  state.editingEntryId = entryView.entry.id;
  state.entryForm = {
    name: entryView.entry.name,
    format: entryView.entry.format,
    style: entryView.entry.style,
    division: entryView.entry.division,
    level: entryView.entry.level,
    notes: entryView.entry.notes,
    personIds: [...(entryView.personIds ?? [])],
  };
  vlens.scheduleRedraw();
}

function onCancelEntryForm(state: SeasonState) {
  state.addingEntry = false;
  state.editingEntryId = 0;
  vlens.scheduleRedraw();
}

function onToggleRosterMember(state: SeasonState, personId: number) {
  const idx = state.entryForm.personIds.indexOf(personId);
  if (idx >= 0) {
    state.entryForm.personIds.splice(idx, 1);
  } else {
    state.entryForm.personIds.push(personId);
  }
  vlens.scheduleRedraw();
}

// sortEntries keeps the client list in the order GetSeasonOverview returns:
// by name, ties broken by id.
function sortEntries(entries: server.EntryView[]) {
  entries.sort((a, b) => {
    if (a.entry.name !== b.entry.name) return a.entry.name < b.entry.name ? -1 : 1;
    return a.entry.id - b.entry.id;
  });
}

async function onCreateEntry(state: SeasonState) {
  const name = state.entryForm.name.trim();
  if (!name || state.seasonId === 0) return;
  state.saving = true;
  state.error = "";
  vlens.scheduleRedraw();

  const [resp, err] = await server.CreateEntry({
    seasonId: state.seasonId,
    name,
    format: state.entryForm.format.trim(),
    style: state.entryForm.style.trim(),
    division: state.entryForm.division.trim(),
    level: state.entryForm.level.trim(),
    notes: state.entryForm.notes.trim(),
    personIds: state.entryForm.personIds,
  });
  if (err || !resp) {
    state.error = err || "Failed to add";
  } else {
    state.entries.push(resp.entry);
    sortEntries(state.entries);
    state.addingEntry = false;
  }
  state.saving = false;
  vlens.scheduleRedraw();
}

function sameRoster(a: number[], b: number[]): boolean {
  if (a.length !== b.length) return false;
  return a.every((id, idx) => id === b[idx]);
}

// Saving an edit is two calls because the backend splits them: UpdateEntry
// carries the fields, SetEntryRoster replaces the roster. The roster call is
// skipped when nothing about it changed, which is the common edit.
async function onSaveEntry(state: SeasonState, original: server.EntryView) {
  const name = state.entryForm.name.trim();
  if (!name) return;
  state.saving = true;
  state.error = "";
  vlens.scheduleRedraw();

  const entryId = original.entry.id;
  let [resp, err] = await server.UpdateEntry({
    id: entryId,
    name,
    format: state.entryForm.format.trim(),
    style: state.entryForm.style.trim(),
    division: state.entryForm.division.trim(),
    level: state.entryForm.level.trim(),
    notes: state.entryForm.notes.trim(),
  });

  if (!err && resp && !sameRoster(state.entryForm.personIds, original.personIds ?? [])) {
    [resp, err] = await server.SetEntryRoster({
      entryId,
      personIds: state.entryForm.personIds,
    });
  }

  if (err || !resp) {
    state.error = err || "Failed to save";
  } else {
    const idx = state.entries.findIndex(view => view.entry.id === entryId);
    if (idx >= 0) state.entries[idx] = resp.entry;
    sortEntries(state.entries);
    state.editingEntryId = 0;
  }
  state.saving = false;
  vlens.scheduleRedraw();
}

async function onDeleteEntry(
  state: SeasonState,
  entryView: server.EntryView,
  labels: ActivityLabels
) {
  const count = state.entryAppearanceCounts[entryView.entry.id] ?? 0;
  const tail =
    count === 0
      ? "This cannot be undone."
      : `Its ${count} ${count === 1 ? labels.appearance.toLowerCase() : labels.appearancePlural.toLowerCase()} and their results go with it. This cannot be undone.`;
  if (!confirm(`Delete "${entryView.entry.name}"? ${tail}`)) return;

  state.saving = true;
  state.error = "";
  vlens.scheduleRedraw();

  const [, err] = await server.DeleteEntry({ id: entryView.entry.id });
  if (err) {
    state.error = err || "Failed to delete";
  } else {
    const idx = state.entries.findIndex(row => row.entry.id === entryView.entry.id);
    if (idx >= 0) state.entries.splice(idx, 1);
    delete state.entryAppearanceCounts[entryView.entry.id];
    if (state.editingEntryId === entryView.entry.id) state.editingEntryId = 0;
  }
  state.saving = false;
  vlens.scheduleRedraw();
}
