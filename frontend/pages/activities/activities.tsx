import { ChevronRight, Pencil, Trash2 } from "lucide-preact";
import { Icon } from "../../components/Icon";
import * as preact from "preact";
import * as vlens from "vlens";
import * as rpc from "vlens/rpc";
import * as server from "../../server";
import * as auth from "../../lib/authCache";
import { Header, Footer } from "../../layout";
import { requireAuthInView, ensureAuthInFetch } from "../../lib/authHelpers";
import { FamilySelect } from "../../components/FamilySelect";
import { formatDateRange, toDateInputValue } from "../../lib/dateUtils";
import { localDateString } from "../../lib/when";
import { dayKey, dayLabel } from "../../lib/daySummary";
import { EventFocus, seasonPhase, seasonProgress } from "../../lib/activityHome";
import {
  ActivityKindDance,
  ActivityLabels,
  activityKindIcon,
  activityKindName,
  activityKindOptions,
  labelsForKind,
} from "./labels";
import "./activities-styles";

const emptyHome: server.GetActivitiesHomeResponse = {
  familyId: 0,
  activities: [],
  seasons: [],
};

export async function fetch(
  route: string,
  prefix: string
): Promise<rpc.Response<server.GetActivitiesHomeResponse>> {
  if (!(await ensureAuthInFetch())) {
    return rpc.ok<server.GetActivitiesHomeResponse>(emptyHome);
  }
  return server.GetActivitiesHome({ familyId: 0 });
}

type SeasonForm = {
  name: string;
  start: string;
  end: string;
  notes: string;
};

const blankSeasonForm = (): SeasonForm => ({ name: "", start: "", end: "", notes: "" });

type ActivitiesState = {
  initialized: boolean;
  familyId: number;
  activities: server.Activity[];
  seasons: server.HomeSeason[];
  loading: boolean;
  managing: boolean;

  addingActivity: boolean;
  newActivityName: string;
  newActivityKind: string;

  editingActivityId: number;
  editActivityName: string;
  editActivityKind: string;

  addingSeasonFor: number;
  editingSeasonId: number;
  seasonForm: SeasonForm;

  error: string;
  saving: boolean;
};

const useActivitiesState = vlens.declareHook(
  (): ActivitiesState => ({
    initialized: false,
    familyId: 0,
    activities: [],
    seasons: [],
    loading: false,
    managing: false,

    addingActivity: false,
    newActivityName: "",
    newActivityKind: ActivityKindDance,

    editingActivityId: 0,
    editActivityName: "",
    editActivityKind: ActivityKindDance,

    addingSeasonFor: 0,
    editingSeasonId: 0,
    seasonForm: blankSeasonForm(),

    error: "",
    saving: false,
  })
);

function phaseOf(season: server.Season, today: string) {
  return seasonPhase(toDateInputValue(season.startDate), toDateInputValue(season.endDate), today);
}

export function view(
  route: string,
  prefix: string,
  data: server.GetActivitiesHomeResponse
): preact.ComponentChild {
  const currentAuth = requireAuthInView();
  if (!currentAuth) return;

  const state = useActivitiesState();
  if (!state.initialized) {
    state.initialized = true;
    applyHome(state, data);
  }

  const editable = auth.canContribute(state.familyId);
  const today = localDateString(new Date());
  const activityOf = (season: server.Season) =>
    state.activities.find(a => a.id === season.activityId);
  const inSeason = state.seasons
    .filter(home => phaseOf(home.season, today) !== "past")
    .sort((a, b) => a.season.startDate.localeCompare(b.season.startDate));
  const past = state.seasons.filter(home => phaseOf(home.season, today) === "past");
  const idle = state.activities.filter(
    activity => !inSeason.some(home => home.season.activityId === activity.id)
  );

  return (
    <div>
      <Header isHome={false} />
      <main id="app" className="activities-container">
        <div className="activities-title">
          <h1>Activities</h1>
          {editable && !state.addingActivity && (
            <button
              className="btn btn-secondary btn-small"
              onClick={vlens.cachePartial(onShowActivityForm, state)}
              disabled={state.saving}
            >
              New program
            </button>
          )}
        </div>

        <FamilySelect
          id="activitiesFamilyId"
          minRole={auth.ROLE_VIEW}
          value={state.familyId}
          onChange={familyId => {
            void selectFamily(state, familyId);
          }}
          disabled={state.saving}
        />

        {state.error && (
          <div className="error-message" role="alert">
            {state.error}
          </div>
        )}

        {state.addingActivity && <ActivityForm state={state} />}

        {state.loading ? (
          <div className="muted">Loading…</div>
        ) : state.activities.length === 0 ? (
          <div className="empty-state">
            <p>
              Track a competitive season — the competitions or games, the routines or teams, and how
              each one did.
            </p>
            {editable && !state.addingActivity && (
              <button
                className="btn btn-primary"
                onClick={vlens.cachePartial(onShowActivityForm, state)}
              >
                Add a program
              </button>
            )}
          </div>
        ) : (
          <>
            {inSeason.length > 0 && (
              <section className="activities-section">
                <h2 className="activities-heading">In season</h2>
                <div className="home-season-cards">
                  {inSeason.map(home => (
                    <SeasonCard
                      key={home.season.id}
                      home={home}
                      activity={activityOf(home.season)}
                      today={today}
                      editable={editable}
                    />
                  ))}
                </div>
              </section>
            )}

            {idle.length > 0 && (
              <section className="activities-section">
                <h2 className="activities-heading">
                  {inSeason.length > 0 ? "Between seasons" : "Programs"}
                </h2>
                <ul className="idle-programs">
                  {idle.map(activity => (
                    <IdleProgram
                      key={activity.id}
                      state={state}
                      activity={activity}
                      editable={editable}
                    />
                  ))}
                </ul>
              </section>
            )}

            {past.length > 0 && (
              <section className="activities-section">
                <h2 className="activities-heading">Past seasons</h2>
                <ul className="past-seasons">
                  {past.map(home => (
                    <PastSeason
                      key={home.season.id}
                      home={home}
                      activity={activityOf(home.season)}
                    />
                  ))}
                </ul>
              </section>
            )}

            {editable && (
              <section className="activities-section">
                <button
                  className="manage-toggle"
                  onClick={vlens.cachePartial(onToggleManaging, state)}
                  aria-expanded={state.managing}
                >
                  <Icon symbol={ChevronRight} /> Manage programs and seasons
                </button>
                {state.managing && <ManagePrograms state={state} />}
              </section>
            )}
          </>
        )}
      </main>
      <Footer />
    </div>
  );
}

function focusLabel(focus: EventFocus): string {
  switch (focus) {
    case "latest":
      return "Latest";
    case "needs-results":
      return "No results yet";
    case "now":
      return "Happening now";
    case "next":
      return "Next";
  }
}

function countText(n: number, labels: ActivityLabels): string {
  return `${n} ${(n === 1 ? labels.appearance : labels.appearancePlural).toLowerCase()}`;
}

const SeasonCard = ({
  home,
  activity,
  today,
  editable,
}: {
  home: server.HomeSeason;
  activity: server.Activity | undefined;
  today: string;
  editable: boolean;
}) => {
  const season = home.season;
  const kind = activity?.kind ?? "";
  const labels = labelsForKind(kind);
  const entries = home.entries ?? [];
  const events = home.events ?? [];
  const progress = seasonProgress(events, entries.length > 0, today);
  const dates = formatDateRange(season.startDate, season.endDate);
  const starts = phaseOf(season, today) === "upcoming";

  return (
    <article className="home-season-card">
      <a className="home-season-card-head" href={`/season/${season.id}`}>
        <span className="home-season-program">
          <Icon symbol={activityKindIcon(kind)} /> {activity?.name ?? ""}
        </span>
        <span className="home-season-title">
          <strong>{season.name}</strong>
          <Icon symbol={ChevronRight} />
        </span>
        {dates && (
          <span className="home-season-dates">
            {starts ? "Starts " : ""}
            {dates}
          </span>
        )}
      </a>

      {entries.length > 0 && (
        <p className="home-season-entries">
          {entries.map(view => (
            <a key={view.entry.id} href={`/routine/${view.entry.id}`}>
              {view.entry.name}
            </a>
          ))}
        </p>
      )}

      {progress.focus.length > 0 && (
        <ul className="home-season-events">
          {progress.focus.map(({ item, focus }) => (
            <li key={item.event.id} className={`home-event home-event-${focus}`}>
              <a className="home-event-main" href={`/competition/${item.event.id}`}>
                <span className="home-event-focus">{focusLabel(focus)}</span>
                <strong>{item.event.name}</strong>
                <span className="home-event-when">
                  {dayLabel(dayKey(item.event.startDate), today)}
                </span>
              </a>
              {editable && (focus === "needs-results" || focus === "now") ? (
                <a className="btn btn-primary btn-small" href={`/competition/${item.event.id}`}>
                  Add results
                </a>
              ) : (
                focus === "latest" && (
                  <span className="home-event-count">{countText(item.appearances, labels)}</span>
                )
              )}
            </li>
          ))}
        </ul>
      )}

      <p className="home-season-foot">
        {events.length === 0
          ? `No ${labels.eventPlural.toLowerCase()} yet`
          : `${progress.recorded} of ${progress.held} ${labels.eventPlural.toLowerCase()} recorded · ${events.length} this season`}
      </p>
    </article>
  );
};

const IdleProgram = ({
  state,
  activity,
  editable,
}: {
  state: ActivitiesState;
  activity: server.Activity;
  editable: boolean;
}) => {
  const last = state.seasons.find(home => home.season.activityId === activity.id);
  return (
    <li className="idle-program">
      <div className="idle-program-main">
        <span className="home-season-program">
          <Icon symbol={activityKindIcon(activity.kind)} /> {activity.name}
        </span>
        <span className="home-season-dates">
          {last ? `Last season: ${last.season.name}` : "No seasons yet"}
        </span>
      </div>
      {editable && (state.addingSeasonFor !== activity.id || state.managing) && (
        <button
          className="btn btn-secondary btn-small"
          onClick={vlens.cachePartial(onShowSeasonForm, state, activity.id)}
          disabled={state.saving}
        >
          Start a season
        </button>
      )}
      {state.addingSeasonFor === activity.id && !state.managing && (
        <SeasonFormFields
          state={state}
          idPrefix={`newSeason${activity.id}`}
          submitLabel="Add season"
          onSubmit={vlens.cachePartial(onCreateSeason, state)}
        />
      )}
    </li>
  );
};

const PastSeason = ({
  home,
  activity,
}: {
  home: server.HomeSeason;
  activity: server.Activity | undefined;
}) => {
  const labels = labelsForKind(activity?.kind ?? "");
  const events = (home.events ?? []).length;
  const entries = (home.entries ?? []).length;
  const dates = formatDateRange(home.season.startDate, home.season.endDate);
  const counts = [
    `${events} ${(events === 1 ? labels.event : labels.eventPlural).toLowerCase()}`,
    `${entries} ${(entries === 1 ? labels.entry : labels.entryPlural).toLowerCase()}`,
  ].join(" · ");

  return (
    <li>
      <a className="past-season" href={`/season/${home.season.id}`}>
        <span className="past-season-main">
          <strong>{home.season.name}</strong>
          <span className="home-season-dates">
            {[activity?.name, dates].filter(part => part).join(" · ")}
          </span>
        </span>
        <span className="past-season-counts">{counts}</span>
        <Icon symbol={ChevronRight} />
      </a>
    </li>
  );
};

const ManagePrograms = ({ state }: { state: ActivitiesState }) => (
  <div className="manage-programs">
    {state.activities.map(activity => (
      <div key={activity.id} className="manage-program">
        {state.editingActivityId === activity.id ? (
          <ActivityEditForm state={state} activity={activity} />
        ) : (
          <div className="manage-row">
            <span className="manage-row-main">
              <strong>{activity.name}</strong>
              <small>{activityKindName(activity.kind)}</small>
            </span>
            <span className="season-item-actions">
              <button
                className="icon-btn"
                title="Rename program"
                aria-label={`Rename ${activity.name}`}
                onClick={vlens.cachePartial(onStartEditActivity, state, activity)}
                disabled={state.saving}
              >
                <Icon symbol={Pencil} />
              </button>
              <button
                className="icon-btn"
                title="Delete program"
                aria-label={`Delete ${activity.name}`}
                onClick={vlens.cachePartial(onDeleteActivity, state, activity)}
                disabled={state.saving}
              >
                <Icon symbol={Trash2} />
              </button>
            </span>
          </div>
        )}

        <ul className="manage-seasons">
          {state.seasons
            .filter(home => home.season.activityId === activity.id)
            .map(home =>
              state.editingSeasonId === home.season.id ? (
                <li key={home.season.id}>
                  <SeasonFormFields
                    state={state}
                    idPrefix={`editSeason${home.season.id}`}
                    submitLabel="Save"
                    onSubmit={vlens.cachePartial(onSaveSeason, state, home.season.id)}
                  />
                </li>
              ) : (
                <li key={home.season.id} className="manage-row">
                  <span className="manage-row-main">
                    <span>{home.season.name}</span>
                    <small>{formatDateRange(home.season.startDate, home.season.endDate)}</small>
                  </span>
                  <span className="season-item-actions">
                    <button
                      className="icon-btn"
                      title="Edit season"
                      aria-label={`Edit ${home.season.name}`}
                      onClick={vlens.cachePartial(onStartEditSeason, state, home.season)}
                      disabled={state.saving}
                    >
                      <Icon symbol={Pencil} />
                    </button>
                    <button
                      className="icon-btn"
                      title="Delete season"
                      aria-label={`Delete ${home.season.name}`}
                      onClick={vlens.cachePartial(onDeleteSeason, state, home.season)}
                      disabled={state.saving}
                    >
                      <Icon symbol={Trash2} />
                    </button>
                  </span>
                </li>
              )
            )}
        </ul>

        {state.addingSeasonFor === activity.id ? (
          <SeasonFormFields
            state={state}
            idPrefix={`manageSeason${activity.id}`}
            submitLabel="Add season"
            onSubmit={vlens.cachePartial(onCreateSeason, state)}
          />
        ) : (
          <button
            className="link-action"
            onClick={vlens.cachePartial(onShowSeasonForm, state, activity.id)}
            disabled={state.saving}
          >
            + New season
          </button>
        )}
      </div>
    ))}
  </div>
);

const ActivityForm = ({ state }: { state: ActivitiesState }) => (
  <div className="activities-form">
    <div className="form-row">
      <div className="form-group flex-2">
        <label htmlFor="newActivityName">Name</label>
        <input
          id="newActivityName"
          type="text"
          placeholder="Dance"
          value={state.newActivityName}
          onInput={e => {
            state.newActivityName = e.currentTarget.value;
            vlens.scheduleRedraw();
          }}
          disabled={state.saving}
        />
      </div>
      <div className="form-group flex-1">
        <label htmlFor="newActivityKind">Kind</label>
        <select
          id="newActivityKind"
          value={state.newActivityKind}
          onInput={e => {
            state.newActivityKind = e.currentTarget.value;
            vlens.scheduleRedraw();
          }}
          disabled={state.saving}
        >
          {activityKindOptions.map(option => (
            <option key={option.value} value={option.value}>
              {option.label}
            </option>
          ))}
        </select>
      </div>
    </div>
    <p className="form-hint">
      Kind only picks the wording — "routine" for dance, "team" for a sport.
    </p>
    <div className="form-actions">
      <button
        className="btn btn-primary"
        onClick={vlens.cachePartial(onCreateActivity, state)}
        disabled={state.saving || !state.newActivityName.trim()}
      >
        Add program
      </button>
      <button
        className="btn btn-secondary"
        onClick={vlens.cachePartial(onCancelActivityForm, state)}
        disabled={state.saving}
      >
        Cancel
      </button>
    </div>
  </div>
);

const ActivityEditForm = ({
  state,
  activity,
}: {
  state: ActivitiesState;
  activity: server.Activity;
}) => (
  <div className="activities-form">
    <div className="form-row">
      <div className="form-group flex-2">
        <label htmlFor={`editActivityName${activity.id}`}>Name</label>
        <input
          id={`editActivityName${activity.id}`}
          type="text"
          value={state.editActivityName}
          onInput={e => {
            state.editActivityName = e.currentTarget.value;
            vlens.scheduleRedraw();
          }}
          disabled={state.saving}
        />
      </div>
      <div className="form-group flex-1">
        <label htmlFor={`editActivityKind${activity.id}`}>Kind</label>
        <select
          id={`editActivityKind${activity.id}`}
          value={state.editActivityKind}
          onInput={e => {
            state.editActivityKind = e.currentTarget.value;
            vlens.scheduleRedraw();
          }}
          disabled={state.saving}
        >
          {activityKindOptions.map(option => (
            <option key={option.value} value={option.value}>
              {option.label}
            </option>
          ))}
        </select>
      </div>
    </div>
    <div className="form-actions">
      <button
        className="btn btn-primary"
        onClick={vlens.cachePartial(onSaveActivity, state, activity.id)}
        disabled={state.saving || !state.editActivityName.trim()}
      >
        Save
      </button>
      <button
        className="btn btn-secondary"
        onClick={vlens.cachePartial(onCancelEditActivity, state)}
        disabled={state.saving}
      >
        Cancel
      </button>
    </div>
  </div>
);

const SeasonFormFields = ({
  state,
  idPrefix,
  submitLabel,
  onSubmit,
}: {
  state: ActivitiesState;
  idPrefix: string;
  submitLabel: string;
  onSubmit: () => void;
}) => {
  const form = state.seasonForm;
  return (
    <div className="activities-form season-form">
      <div className="form-group">
        <label htmlFor={`${idPrefix}Name`}>Season name</label>
        <input
          id={`${idPrefix}Name`}
          type="text"
          placeholder="2025–26 Competition Season"
          value={form.name}
          onInput={e => {
            form.name = e.currentTarget.value;
            vlens.scheduleRedraw();
          }}
          disabled={state.saving}
        />
      </div>
      <div className="form-row">
        <div className="form-group flex-1">
          <label htmlFor={`${idPrefix}Start`}>Starts</label>
          <input
            id={`${idPrefix}Start`}
            type="date"
            value={form.start}
            onInput={e => {
              form.start = e.currentTarget.value;
              vlens.scheduleRedraw();
            }}
            disabled={state.saving}
          />
        </div>
        <div className="form-group flex-1">
          <label htmlFor={`${idPrefix}End`}>Ends</label>
          <input
            id={`${idPrefix}End`}
            type="date"
            value={form.end}
            onInput={e => {
              form.end = e.currentTarget.value;
              vlens.scheduleRedraw();
            }}
            disabled={state.saving}
          />
        </div>
      </div>
      <p className="form-hint">
        The dates decide whether the season shows as in season or past. Leave the end empty until
        it's over.
      </p>
      <div className="form-group">
        <label htmlFor={`${idPrefix}Notes`}>Notes</label>
        <textarea
          id={`${idPrefix}Notes`}
          rows={2}
          value={form.notes}
          onInput={e => {
            form.notes = e.currentTarget.value;
            vlens.scheduleRedraw();
          }}
          disabled={state.saving}
        />
      </div>
      <div className="form-actions">
        <button
          className="btn btn-primary"
          onClick={onSubmit}
          disabled={state.saving || !form.name.trim()}
        >
          {submitLabel}
        </button>
        <button
          className="btn btn-secondary"
          onClick={vlens.cachePartial(onCancelSeasonForm, state)}
          disabled={state.saving}
        >
          Cancel
        </button>
      </div>
    </div>
  );
};

function applyHome(state: ActivitiesState, home: server.GetActivitiesHomeResponse) {
  state.familyId = home.familyId;
  state.activities = [...(home.activities ?? [])];
  state.seasons = [...(home.seasons ?? [])];
}

async function reload(state: ActivitiesState) {
  const familyId = state.familyId;
  const [resp, err] = await server.GetActivitiesHome({ familyId });
  if (state.familyId !== familyId) return;
  if (err || !resp) {
    state.error = err || "Failed to load activities";
  } else {
    applyHome(state, resp);
  }
  state.loading = false;
  vlens.scheduleRedraw();
}

async function selectFamily(state: ActivitiesState, familyId: number) {
  if (state.familyId === familyId) return;
  state.familyId = familyId;
  state.activities = [];
  state.seasons = [];
  state.addingActivity = false;
  state.editingActivityId = 0;
  state.addingSeasonFor = 0;
  state.editingSeasonId = 0;
  state.error = "";
  state.loading = true;
  vlens.scheduleRedraw();
  await reload(state);
}

function onToggleManaging(state: ActivitiesState) {
  state.managing = !state.managing;
  vlens.scheduleRedraw();
}

function onShowActivityForm(state: ActivitiesState) {
  state.addingActivity = true;
  state.editingActivityId = 0;
  state.newActivityName = "";
  state.newActivityKind = ActivityKindDance;
  vlens.scheduleRedraw();
}

function onCancelActivityForm(state: ActivitiesState) {
  state.addingActivity = false;
  state.newActivityName = "";
  vlens.scheduleRedraw();
}

async function onCreateActivity(state: ActivitiesState) {
  const name = state.newActivityName.trim();
  if (!name) return;
  state.saving = true;
  state.error = "";
  vlens.scheduleRedraw();

  const [resp, err] = await server.CreateActivity({
    familyId: state.familyId,
    name,
    kind: state.newActivityKind,
  });
  state.saving = false;
  if (err || !resp) {
    state.error = err || "Failed to create program";
    vlens.scheduleRedraw();
    return;
  }

  state.activities.push(resp.activity);
  state.addingActivity = false;
  state.newActivityName = "";
  onShowSeasonForm(state, resp.activity.id);
}

function onStartEditActivity(state: ActivitiesState, activity: server.Activity) {
  state.editingActivityId = activity.id;
  state.editActivityName = activity.name;
  state.editActivityKind = activity.kind;
  state.addingActivity = false;
  vlens.scheduleRedraw();
}

function onCancelEditActivity(state: ActivitiesState) {
  state.editingActivityId = 0;
  vlens.scheduleRedraw();
}

async function onSaveActivity(state: ActivitiesState, activityId: number) {
  const name = state.editActivityName.trim();
  if (!name) return;
  state.saving = true;
  state.error = "";
  vlens.scheduleRedraw();

  const [resp, err] = await server.UpdateActivity({
    id: activityId,
    name,
    kind: state.editActivityKind,
  });
  if (err || !resp) {
    state.error = err || "Failed to update program";
  } else {
    const idx = state.activities.findIndex(a => a.id === activityId);
    if (idx >= 0) state.activities[idx] = resp.activity;
    state.editingActivityId = 0;
  }
  state.saving = false;
  vlens.scheduleRedraw();
}

async function onDeleteActivity(state: ActivitiesState, activity: server.Activity) {
  if (
    !confirm(
      `Delete "${activity.name}"? Its seasons and everything in them are deleted too. This cannot be undone.`
    )
  ) {
    return;
  }
  state.saving = true;
  state.error = "";
  vlens.scheduleRedraw();

  const [, err] = await server.DeleteActivity({ id: activity.id });
  state.saving = false;
  if (err) {
    state.error = err || "Failed to delete program";
    vlens.scheduleRedraw();
    return;
  }
  await reload(state);
}

function onShowSeasonForm(state: ActivitiesState, activityId: number) {
  state.addingSeasonFor = activityId;
  state.editingSeasonId = 0;
  state.seasonForm = blankSeasonForm();
  vlens.scheduleRedraw();
}

function onCancelSeasonForm(state: ActivitiesState) {
  state.addingSeasonFor = 0;
  state.editingSeasonId = 0;
  vlens.scheduleRedraw();
}

function dateOrNull(value: string): string | null {
  const trimmed = value.trim();
  return trimmed === "" ? null : trimmed;
}

async function onCreateSeason(state: ActivitiesState) {
  const form = state.seasonForm;
  const name = form.name.trim();
  if (!name || state.addingSeasonFor === 0) return;
  state.saving = true;
  state.error = "";
  vlens.scheduleRedraw();

  const [resp, err] = await server.CreateSeason({
    activityId: state.addingSeasonFor,
    name,
    startDate: dateOrNull(form.start),
    endDate: dateOrNull(form.end),
    notes: form.notes.trim(),
  });
  state.saving = false;
  if (err || !resp) {
    state.error = err || "Failed to create season";
    vlens.scheduleRedraw();
    return;
  }
  state.addingSeasonFor = 0;
  await reload(state);
}

function onStartEditSeason(state: ActivitiesState, season: server.Season) {
  state.editingSeasonId = season.id;
  state.addingSeasonFor = 0;
  state.seasonForm = {
    name: season.name,
    start: toDateInputValue(season.startDate),
    end: toDateInputValue(season.endDate),
    notes: season.notes,
  };
  vlens.scheduleRedraw();
}

async function onSaveSeason(state: ActivitiesState, seasonId: number) {
  const form = state.seasonForm;
  const name = form.name.trim();
  if (!name) return;
  state.saving = true;
  state.error = "";
  vlens.scheduleRedraw();

  const [resp, err] = await server.UpdateSeason({
    id: seasonId,
    name,
    startDate: dateOrNull(form.start),
    endDate: dateOrNull(form.end),
    notes: form.notes.trim(),
  });
  if (err || !resp) {
    state.error = err || "Failed to update season";
  } else {
    const home = state.seasons.find(row => row.season.id === seasonId);
    if (home) home.season = resp.season;
    state.editingSeasonId = 0;
  }
  state.saving = false;
  vlens.scheduleRedraw();
}

async function onDeleteSeason(state: ActivitiesState, season: server.Season) {
  if (
    !confirm(
      `Delete "${season.name}"? Everything recorded in this season is deleted too. This cannot be undone.`
    )
  ) {
    return;
  }
  state.saving = true;
  state.error = "";
  vlens.scheduleRedraw();

  const [, err] = await server.DeleteSeason({ id: season.id });
  if (err) {
    state.error = err || "Failed to delete season";
  } else {
    state.seasons = state.seasons.filter(home => home.season.id !== season.id);
    if (state.editingSeasonId === season.id) state.editingSeasonId = 0;
  }
  state.saving = false;
  vlens.scheduleRedraw();
}
