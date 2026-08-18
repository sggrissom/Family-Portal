import * as preact from "preact";
import * as vlens from "vlens";
import * as rpc from "vlens/rpc";
import * as server from "../../server";
import { Header, Footer } from "../../layout";
import { ensureAuthInFetch, requireAuthInView } from "../../lib/authHelpers";
import { formatDateRange } from "../../lib/dateUtils";
import { getIdFromRoute } from "../../lib/routeHelpers";
import { labelsForKind } from "./labels";
import "./season-overview-styles";

type SeasonData = server.GetSeasonOverviewResponse;

export async function fetch(route: string): Promise<rpc.Response<SeasonData>> {
  if (!(await ensureAuthInFetch())) return [null, "Authentication required"];
  const seasonId = getIdFromRoute(route, 3);
  if (!seasonId) return [null, "Invalid season"];
  return server.GetSeasonOverview({ seasonId });
}

type SeasonState = {
  initializedFor: number;
  events: server.Event[];
  entries: server.EntryView[];
  addingEvent: boolean;
  addingEntry: boolean;
  eventName: string;
  eventHost: string;
  eventLocation: string;
  eventStart: string;
  eventEnd: string;
  eventNotes: string;
  entryName: string;
  entryFormat: string;
  entryStyle: string;
  entryDivision: string;
  entryLevel: string;
  entryNotes: string;
  saving: boolean;
  error: string;
};

const useSeasonState = vlens.declareHook(
  (): SeasonState => ({
    initializedFor: 0,
    events: [],
    entries: [],
    addingEvent: false,
    addingEntry: false,
    eventName: "",
    eventHost: "",
    eventLocation: "",
    eventStart: "",
    eventEnd: "",
    eventNotes: "",
    entryName: "",
    entryFormat: "",
    entryStyle: "",
    entryDivision: "",
    entryLevel: "",
    entryNotes: "",
    saving: false,
    error: "",
  })
);

export function view(route: string, prefix: string, data: SeasonData): preact.ComponentChild {
  if (!requireAuthInView()) return;
  const state = useSeasonState();
  if (state.initializedFor !== data.season.id) {
    state.initializedFor = data.season.id;
    state.events = [...(data.events ?? [])];
    state.entries = [...(data.entries ?? [])];
    state.addingEvent = false;
    state.addingEntry = false;
    state.error = "";
  }
  const labels = labelsForKind(data.activity.kind);

  return (
    <div>
      <Header isHome={false} />
      <main id="app" className="season-container">
        <nav className="season-breadcrumb" aria-label="Breadcrumb">
          <a href="/activities">Activities</a>
          <span aria-hidden="true">/</span>
          <span>{data.activity.name}</span>
        </nav>
        <div className="season-title">
          <div>
            <p className="season-eyebrow">{data.activity.name}</p>
            <h1>{data.season.name}</h1>
            {formatDateRange(data.season.startDate, data.season.endDate) && (
              <p className="muted">{formatDateRange(data.season.startDate, data.season.endDate)}</p>
            )}
          </div>
          <a className="btn btn-secondary" href="/activities">
            Manage seasons
          </a>
        </div>
        {data.season.notes && <p className="season-description">{data.season.notes}</p>}
        {state.error && <div className="error-message">{state.error}</div>}

        <section className="season-panel">
          <div className="season-panel-head">
            <div>
              <h2>{labels.eventPlural}</h2>
              <p>Competitions, games, or meets in this season.</p>
            </div>
            <button
              className="btn btn-primary"
              onClick={() => toggleEventForm(state)}
              disabled={state.saving}
            >
              {state.addingEvent ? "Cancel" : `New ${labels.event.toLowerCase()}`}
            </button>
          </div>
          {state.addingEvent && <EventForm state={state} seasonId={data.season.id} />}
          {state.events.length ? (
            <div className="season-card-list">
              {state.events.map(event => (
                <article className="season-card" key={event.id}>
                  <div>
                    <h3>{event.name}</h3>
                    <p>
                      {[event.host, event.location].filter(Boolean).join(" · ") ||
                        "No host or location yet"}
                    </p>
                  </div>
                  <span>{formatDateRange(event.startDate, event.endDate)}</span>
                </article>
              ))}
            </div>
          ) : (
            <div className="empty-state">
              <p>No {labels.eventPlural.toLowerCase()} yet. Add the first one above.</p>
            </div>
          )}
        </section>

        <section className="season-panel">
          <div className="season-panel-head">
            <div>
              <h2>{labels.entryPlural}</h2>
              <p>The routines, teams, or events competing this season.</p>
            </div>
            <button
              className="btn btn-primary"
              onClick={() => toggleEntryForm(state)}
              disabled={state.saving}
            >
              {state.addingEntry ? "Cancel" : `New ${labels.entry.toLowerCase()}`}
            </button>
          </div>
          {state.addingEntry && <EntryForm state={state} seasonId={data.season.id} />}
          {state.entries.length ? (
            <div className="season-card-list">
              {state.entries.map(item => (
                <article className="season-card" key={item.entry.id}>
                  <div>
                    <h3>{item.entry.name}</h3>
                    <p>
                      {[item.entry.format, item.entry.style, item.entry.division, item.entry.level]
                        .filter(Boolean)
                        .join(" · ") || "No details yet"}
                    </p>
                  </div>
                  <span>
                    {item.personIds.length} roster member{item.personIds.length === 1 ? "" : "s"}
                  </span>
                </article>
              ))}
            </div>
          ) : (
            <div className="empty-state">
              <p>No {labels.entryPlural.toLowerCase()} yet. Add the first one above.</p>
            </div>
          )}
        </section>
      </main>
      <Footer />
    </div>
  );
}

const Field = ({
  label,
  value,
  onInput,
  type = "text",
}: {
  label: string;
  value: string;
  onInput: (value: string) => void;
  type?: string;
}) => (
  <label className="season-field">
    <span>{label}</span>
    <input type={type} value={value} onInput={e => onInput(e.currentTarget.value)} />
  </label>
);

const EventForm = ({ state, seasonId }: { state: SeasonState; seasonId: number }) => (
  <div className="season-form">
    <Field label="Name" value={state.eventName} onInput={v => update(state, "eventName", v)} />
    <Field label="Host" value={state.eventHost} onInput={v => update(state, "eventHost", v)} />
    <Field
      label="Location"
      value={state.eventLocation}
      onInput={v => update(state, "eventLocation", v)}
    />
    <Field
      label="Starts"
      type="date"
      value={state.eventStart}
      onInput={v => update(state, "eventStart", v)}
    />
    <Field
      label="Ends"
      type="date"
      value={state.eventEnd}
      onInput={v => update(state, "eventEnd", v)}
    />
    <label className="season-field full">
      <span>Notes</span>
      <textarea
        rows={2}
        value={state.eventNotes}
        onInput={e => update(state, "eventNotes", e.currentTarget.value)}
      />
    </label>
    <button
      className="btn btn-primary"
      disabled={state.saving || !state.eventName.trim()}
      onClick={() => void createEvent(state, seasonId)}
    >
      Add event
    </button>
  </div>
);

const EntryForm = ({ state, seasonId }: { state: SeasonState; seasonId: number }) => (
  <div className="season-form">
    <Field label="Name" value={state.entryName} onInput={v => update(state, "entryName", v)} />
    <Field
      label="Format"
      value={state.entryFormat}
      onInput={v => update(state, "entryFormat", v)}
    />
    <Field label="Style" value={state.entryStyle} onInput={v => update(state, "entryStyle", v)} />
    <Field
      label="Division"
      value={state.entryDivision}
      onInput={v => update(state, "entryDivision", v)}
    />
    <Field label="Level" value={state.entryLevel} onInput={v => update(state, "entryLevel", v)} />
    <label className="season-field full">
      <span>Notes</span>
      <textarea
        rows={2}
        value={state.entryNotes}
        onInput={e => update(state, "entryNotes", e.currentTarget.value)}
      />
    </label>
    <p className="form-hint full">
      You can add roster members when routine details are expanded in a later UI pass.
    </p>
    <button
      className="btn btn-primary"
      disabled={state.saving || !state.entryName.trim()}
      onClick={() => void createEntry(state, seasonId)}
    >
      Add entry
    </button>
  </div>
);

function update<K extends keyof SeasonState>(state: SeasonState, key: K, value: SeasonState[K]) {
  state[key] = value;
  vlens.scheduleRedraw();
}
function toggleEventForm(state: SeasonState) {
  state.addingEvent = !state.addingEvent;
  state.error = "";
  vlens.scheduleRedraw();
}
function toggleEntryForm(state: SeasonState) {
  state.addingEntry = !state.addingEntry;
  state.error = "";
  vlens.scheduleRedraw();
}
const dateOrNull = (value: string) => value || null;

async function createEvent(state: SeasonState, seasonId: number) {
  state.saving = true;
  state.error = "";
  vlens.scheduleRedraw();
  const [resp, err] = await server.CreateEvent({
    seasonId,
    name: state.eventName.trim(),
    host: state.eventHost.trim(),
    location: state.eventLocation.trim(),
    startDate: dateOrNull(state.eventStart),
    endDate: dateOrNull(state.eventEnd),
    notes: state.eventNotes.trim(),
  });
  if (err || !resp) state.error = err || "Failed to create event";
  else {
    state.events.push(resp.event);
    state.addingEvent = false;
    state.eventName =
      state.eventHost =
      state.eventLocation =
      state.eventStart =
      state.eventEnd =
      state.eventNotes =
        "";
  }
  state.saving = false;
  vlens.scheduleRedraw();
}

async function createEntry(state: SeasonState, seasonId: number) {
  state.saving = true;
  state.error = "";
  vlens.scheduleRedraw();
  const [resp, err] = await server.CreateEntry({
    seasonId,
    name: state.entryName.trim(),
    format: state.entryFormat.trim(),
    style: state.entryStyle.trim(),
    division: state.entryDivision.trim(),
    level: state.entryLevel.trim(),
    notes: state.entryNotes.trim(),
    personIds: [],
  });
  if (err || !resp) state.error = err || "Failed to create entry";
  else {
    state.entries.push(resp.entry);
    state.addingEntry = false;
    state.entryName =
      state.entryFormat =
      state.entryStyle =
      state.entryDivision =
      state.entryLevel =
      state.entryNotes =
        "";
  }
  state.saving = false;
  vlens.scheduleRedraw();
}
