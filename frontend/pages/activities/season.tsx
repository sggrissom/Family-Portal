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

export async function fetch(
  route: string,
  prefix: string
): Promise<rpc.Response<server.GetSeasonOverviewResponse>> {
  if (!(await ensureAuthInFetch())) {
    return rpc.ok<server.GetSeasonOverviewResponse>(emptyOverview);
  }
  return server.GetSeasonOverview({ seasonId: getIdFromRoute(route) || 0 });
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

const useSeasonState = vlens.declareHook(
  (): SeasonState => ({
    initialized: false,
    seasonId: 0,
    events: [],
    appearanceCounts: {},
    addingEvent: false,
    editingEventId: 0,
    form: blankEventForm(),
    error: "",
    saving: false,
  })
);

function countAppearances(appearances: server.AppearanceView[]): Record<number, number> {
  const counts: Record<number, number> = {};
  for (const view of appearances) {
    counts[view.appearance.eventId] = (counts[view.appearance.eventId] ?? 0) + 1;
  }
  return counts;
}

export function view(
  route: string,
  prefix: string,
  data: server.GetSeasonOverviewResponse
): preact.ComponentChild {
  const currentAuth = requireAuthInView();
  if (!currentAuth) return;

  const state = useSeasonState();
  // The hook outlives a route change between two seasons, so reinitialize
  // whenever the season under it is a different one.
  if (!state.initialized || state.seasonId !== data.season.id) {
    state.initialized = true;
    state.seasonId = data.season.id;
    state.events = [...(data.events ?? [])];
    state.appearanceCounts = countAppearances(data.appearances ?? []);
    state.addingEvent = false;
    state.editingEventId = 0;
    state.error = "";
  }

  const labels = labelsFor(data.activity);
  const dates = formatDateRange(data.season.startDate, data.season.endDate);

  return (
    <div>
      <Header isHome={false} />
      <main id="app" className="activities-container">
        <a className="back-link" href="/activities">
          ← Activities
        </a>

        <div className="season-header">
          <span className="season-eyebrow">{data.activity.name}</span>
          <h1>{data.season.name}</h1>
          {dates && <p className="season-dates">{dates}</p>}
          {data.season.notes && <p className="season-notes">{data.season.notes}</p>}
        </div>

        {state.error && <div className="error-message">{state.error}</div>}

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
        <strong className="event-name">{event.name}</strong>
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
          onClick={vlens.cachePartial(onStartEditEvent, state, event)}
          disabled={state.saving}
        >
          ✏️
        </button>
        <button
          className="icon-btn"
          title={`Delete ${labels.event.toLowerCase()}`}
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
