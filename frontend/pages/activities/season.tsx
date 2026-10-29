import { Pencil, Trash2 } from "lucide-preact";
import { Icon } from "../../components/Icon";
import * as preact from "preact";
import * as vlens from "vlens";
import * as rpc from "vlens/rpc";
import * as server from "../../server";
import * as auth from "../../lib/authCache";
import { Header, Footer } from "../../layout";
import { requireAuthInView, ensureAuthInFetch } from "../../lib/authHelpers";
import { getIdFromRoute } from "../../lib/routeHelpers";
import { formatDateRange, isRealDate, toDateInputValue } from "../../lib/dateUtils";
import { localDateString } from "../../lib/when";
import { dayKey, dayLabel } from "../../lib/daySummary";
import { cellHeadline, eventStatus } from "../../lib/activityResults";
import { ActivityLabels, activityKindIcon, labelsFor } from "./labels";
import { DateBlock } from "./performance-card";
import { chipOrder } from "../../lib/familyGroups";
import "./activities-styles";
import "./season-styles";

export type SeasonPageData = {
  overview: server.GetSeasonOverviewResponse;
  people: server.Person[];
  rosterChoices: server.Person[];
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
      rosterChoices: [],
      vocabulary: emptyVocabulary,
    });
  }

  const [overview, overviewErr] = await server.GetSeasonOverview({
    seasonId: getIdFromRoute(route) || 0,
  });
  if (overviewErr || !overview) {
    return [null, overviewErr || "Failed to load season"];
  }

  const [people] = await server.ListPeople({});
  const [vocabulary] = await server.ListActivityVocabulary({
    activityId: overview.activity.id,
  });

  const familyId = overview.activity.familyId;
  return rpc.ok<SeasonPageData>({
    overview,
    people: people?.people ?? [],
    rosterChoices: chipOrder(
      (people?.people ?? []).filter(p => p.familyId === familyId),
      people?.relations ?? [],
      familyId
    ),
    vocabulary: vocabulary ?? emptyVocabulary,
  });
}

type SeasonState = {
  initialized: boolean;
  seasonId: number;
  appearances: server.AppearanceView[];
  events: server.Event[];
  appearanceCounts: Record<number, number>;

  addingEvent: boolean;
  editingEventId: number;
  form: EventForm;

  entries: server.EntryView[];
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
    appearances: [],
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
  if (!state.initialized || state.seasonId !== overview.season.id) {
    const appearances = overview.appearances ?? [];
    state.initialized = true;
    state.seasonId = overview.season.id;
    state.appearances = appearances;
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
  const editable = auth.canContribute(overview.season.familyId);
  const today = localDateString(new Date());

  return (
    <div>
      <Header isHome={false} />
      <main id="app" className="activities-container">
        <a className="back-link" href="/activities">
          ← Activities
        </a>

        <div className="season-header">
          <span className="home-season-program">
            <Icon symbol={activityKindIcon(overview.activity.kind)} /> {overview.activity.name}
          </span>
          <h1>{overview.season.name}</h1>
          {dates && <p className="season-dates">{dates}</p>}
          {overview.season.notes && <p className="season-notes">{overview.season.notes}</p>}
        </div>

        {state.error && (
          <div className="error-message" role="alert">
            {state.error}
          </div>
        )}

        {state.events.length > 0 && state.entries.length > 0 && (
          <section className="activities-section">
            <h2 className="activities-heading">Results</h2>
            <ResultsGrid state={state} labels={labels} today={today} />
          </section>
        )}

        <section className="activities-section">
          <div className="activities-section-head">
            <h2>{labels.eventPlural}</h2>
            {editable && !state.addingEvent && (
              <button
                className="btn btn-secondary btn-small"
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
                No {labels.eventPlural.toLowerCase()} yet.
                {editable && " Add the first one to start recording results."}
              </p>
            </div>
          ) : (
            <ul className="schedule">
              {state.events.map(event =>
                state.editingEventId === event.id ? (
                  <li key={event.id}>
                    <EventFormFields
                      state={state}
                      labels={labels}
                      submitLabel="Save"
                      onSubmit={vlens.cachePartial(onSaveEvent, state, event.id)}
                      onCancel={vlens.cachePartial(onCancelEventForm, state)}
                    />
                  </li>
                ) : (
                  <EventRow
                    key={event.id}
                    state={state}
                    event={event}
                    labels={labels}
                    editable={editable}
                    today={today}
                  />
                )
              )}
            </ul>
          )}
        </section>

        <section className="activities-section">
          <div className="activities-section-head">
            <h2>{labels.entryPlural}</h2>
            {editable && !state.addingEntry && (
              <button
                className="btn btn-secondary btn-small"
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
            <div className="entry-cards">
              {state.entries.map(entryView =>
                state.editingEntryId === entryView.entry.id ? (
                  <EntryFormFields
                    key={entryView.entry.id}
                    state={state}
                    data={data}
                    labels={labels}
                    submitLabel="Save"
                    onSubmit={vlens.cachePartial(onSaveEntry, state, entryView)}
                    onCancel={vlens.cachePartial(onCancelEntryForm, state)}
                  />
                ) : (
                  <EntryRow
                    key={entryView.entry.id}
                    state={state}
                    data={data}
                    entryView={entryView}
                    labels={labels}
                    editable={editable}
                  />
                )
              )}
            </div>
          )}
        </section>
      </main>
      <Footer />
    </div>
  );
}

function statusOf(event: server.Event, today: string) {
  return eventStatus(toDateInputValue(event.startDate), toDateInputValue(event.endDate), today);
}

const ResultsGrid = ({
  state,
  labels,
  today,
}: {
  state: SeasonState;
  labels: ActivityLabels;
  today: string;
}) => (
  <div className="results-grid-scroll">
    <table className="results-grid">
      <thead>
        <tr>
          <th scope="col" className="results-grid-corner">
            {labels.entry}
          </th>
          {state.events.map(event => (
            <th
              key={event.id}
              scope="col"
              className={statusOf(event, today) === "upcoming" ? "results-grid-upcoming" : ""}
            >
              <a href={`/competition/${event.id}`}>{event.name}</a>
              {isRealDate(event.startDate) && (
                <small>{dayLabel(dayKey(event.startDate), today).replace(/^\w+, /, "")}</small>
              )}
            </th>
          ))}
        </tr>
      </thead>
      <tbody>
        {state.entries.map(entryView => (
          <tr key={entryView.entry.id}>
            <th scope="row">
              <a href={`/routine/${entryView.entry.id}`}>{entryView.entry.name}</a>
            </th>
            {state.events.map(event => {
              const views = state.appearances.filter(
                view =>
                  view.appearance.eventId === event.id &&
                  view.appearance.entryId === entryView.entry.id
              );
              const text = cellHeadline(views);
              return (
                <td key={event.id} className={text ? "results-grid-hit" : ""}>
                  {text ? (
                    <a href={`/competition/${event.id}`}>{text}</a>
                  ) : (
                    <span aria-label="Not recorded">·</span>
                  )}
                </td>
              );
            })}
          </tr>
        ))}
      </tbody>
    </table>
  </div>
);

const EventRow = ({
  state,
  event,
  labels,
  editable,
  today,
}: {
  state: SeasonState;
  event: server.Event;
  labels: ActivityLabels;
  editable: boolean;
  today: string;
}) => {
  const where = [event.location, event.host].filter(part => part).join(" · ");
  const count = state.appearanceCounts[event.id] ?? 0;
  const status = statusOf(event, today);
  const needsResults = status !== "upcoming" && count === 0 && state.entries.length > 0;

  return (
    <li className={`schedule-row schedule-${status}`}>
      <DateBlock date={event.startDate} />
      <a className="schedule-main" href={`/competition/${event.id}`}>
        <strong>{event.name}</strong>
        {where && <span className="event-meta">{where}</span>}
      </a>
      <span className="schedule-status">
        {status === "upcoming" ? (
          <span className="schedule-tag">Upcoming</span>
        ) : needsResults ? (
          editable ? (
            <a className="btn btn-primary btn-small" href={`/competition/${event.id}`}>
              Add results
            </a>
          ) : (
            <span className="schedule-tag schedule-tag-missing">No results yet</span>
          )
        ) : (
          <span className="event-count">
            {count === 1
              ? `1 ${labels.appearance.toLowerCase()}`
              : `${count} ${labels.appearancePlural.toLowerCase()}`}
          </span>
        )}
      </span>
      {editable && (
        <span className="event-item-actions">
          <button
            className="icon-btn"
            title={`Edit ${labels.event.toLowerCase()}`}
            aria-label={`Edit ${event.name}`}
            onClick={vlens.cachePartial(onStartEditEvent, state, event)}
            disabled={state.saving}
          >
            <Icon symbol={Pencil} />
          </button>
          <button
            className="icon-btn"
            title={`Delete ${labels.event.toLowerCase()}`}
            aria-label={`Delete ${event.name}`}
            onClick={vlens.cachePartial(onDeleteEvent, state, event, labels)}
            disabled={state.saving}
          >
            <Icon symbol={Trash2} />
          </button>
        </span>
      )}
    </li>
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
  editable,
}: {
  state: SeasonState;
  data: SeasonPageData;
  entryView: server.EntryView;
  labels: ActivityLabels;
  editable: boolean;
}) => {
  const entry = entryView.entry;
  const traits = [entry.format, entry.style, entry.division, entry.level]
    .filter(part => part)
    .join(" · ");
  const roster = rosterNames(data.people, entryView.personIds);
  const count = state.entryAppearanceCounts[entry.id] ?? 0;

  return (
    <article className="entry-card">
      <div className="entry-card-main">
        <a className="entry-card-name" href={`/routine/${entry.id}`}>
          {entry.name}
        </a>
        {traits && <span className="event-meta">{traits}</span>}
        <span className="entry-card-roster">
          {roster.length > 0 ? roster.join(", ") : `No ${labels.roster.toLowerCase()} yet`}
        </span>
        <span className="event-count">
          {count === 0
            ? `Not at any ${labels.event.toLowerCase()} yet`
            : count === 1
              ? `1 ${labels.appearance.toLowerCase()}`
              : `${count} ${labels.appearancePlural.toLowerCase()}`}
        </span>
        {entry.notes && <p className="event-notes">{entry.notes}</p>}
      </div>
      {editable && (
        <span className="event-item-actions">
          <button
            className="icon-btn"
            title={`Edit ${labels.entry.toLowerCase()}`}
            aria-label={`Edit ${entry.name}`}
            onClick={vlens.cachePartial(onStartEditEntry, state, entryView)}
            disabled={state.saving}
          >
            <Icon symbol={Pencil} />
          </button>
          <button
            className="icon-btn"
            title={`Delete ${labels.entry.toLowerCase()}`}
            aria-label={`Delete ${entry.name}`}
            onClick={vlens.cachePartial(onDeleteEntry, state, entryView, labels)}
            disabled={state.saving}
          >
            <Icon symbol={Trash2} />
          </button>
        </span>
      )}
    </article>
  );
};

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
    {(labels.entryTraits || state.entryForm.format || state.entryForm.style) && (
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
    )}
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
      {rosterChoices(data, state.entryForm.personIds).length === 0 ? (
        <p className="form-hint">No people to add yet.</p>
      ) : (
        <div className="roster-picker">
          {rosterChoices(data, state.entryForm.personIds).map(person => (
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
    state.appearances = state.appearances.filter(view => view.appearance.eventId !== event.id);
    if (state.editingEventId === event.id) state.editingEventId = 0;
  }
  state.saving = false;
  vlens.scheduleRedraw();
}

function rosterChoices(data: SeasonPageData, personIds: number[]): server.Person[] {
  const listed = new Set(data.rosterChoices.map(p => p.id));
  const extra = data.people.filter(p => personIds.includes(p.id) && !listed.has(p.id));
  return [...data.rosterChoices, ...extra];
}

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
    state.appearances = state.appearances.filter(
      view => view.appearance.entryId !== entryView.entry.id
    );
    if (state.editingEntryId === entryView.entry.id) state.editingEntryId = 0;
  }
  state.saving = false;
  vlens.scheduleRedraw();
}
