// The top of the activities feature: the programs a family tracks, and the
// seasons inside each one. Everything else in the feature hangs off a season,
// so this page is where a season is created and named.
//
// See docs/activities-plan.md, phase 6.

import * as preact from "preact";
import * as vlens from "vlens";
import * as rpc from "vlens/rpc";
import * as server from "../../server";
import { Header, Footer } from "../../layout";
import { requireAuthInView, ensureAuthInFetch } from "../../lib/authHelpers";
import { FamilySelect } from "../../components/FamilySelect";
import { formatDateRange, toDateInputValue } from "../../lib/dateUtils";
import { ActivityKindDance, activityKindName, activityKindOptions, labelsForKind } from "./labels";
import "./activities-styles";

type ActivitiesData = {
  familyId: number;
  activities: server.Activity[];
  // Seasons of selectedActivityId only. Switching activities reloads them
  // rather than fetching every season up front — a family with several
  // programs only ever looks at one at a time.
  selectedActivityId: number;
  seasons: server.Season[];
};

const emptyData: ActivitiesData = {
  familyId: 0,
  activities: [],
  selectedActivityId: 0,
  seasons: [],
};

export async function fetch(route: string, prefix: string): Promise<rpc.Response<ActivitiesData>> {
  if (!(await ensureAuthInFetch())) {
    return rpc.ok<ActivitiesData>(emptyData);
  }

  const [listed, listErr] = await server.ListActivities({ familyId: 0 });
  if (listErr || !listed) {
    return [null, listErr || "Failed to load activities"];
  }

  const activities = listed.activities ?? [];
  if (activities.length === 0) {
    return rpc.ok<ActivitiesData>({ ...emptyData, familyId: listed.familyId });
  }

  const [seasons, seasonsErr] = await server.ListSeasons({ activityId: activities[0].id });
  if (seasonsErr) {
    return [null, seasonsErr];
  }
  return rpc.ok<ActivitiesData>({
    familyId: listed.familyId,
    activities,
    selectedActivityId: activities[0].id,
    seasons: seasons?.seasons ?? [],
  });
}

type ActivitiesState = {
  initialized: boolean;
  familyId: number;
  activities: server.Activity[];
  selectedActivityId: number;
  seasons: server.Season[];
  loadingSeasons: boolean;

  addingActivity: boolean;
  newActivityName: string;
  newActivityKind: string;

  editingActivityId: number;
  editActivityName: string;
  editActivityKind: string;

  addingSeason: boolean;
  newSeasonName: string;
  newSeasonStart: string;
  newSeasonEnd: string;
  newSeasonNotes: string;

  editingSeasonId: number;
  editSeasonName: string;
  editSeasonStart: string;
  editSeasonEnd: string;
  editSeasonNotes: string;

  error: string;
  saving: boolean;
};

const useActivitiesState = vlens.declareHook(
  (): ActivitiesState => ({
    initialized: false,
    familyId: 0,
    activities: [],
    selectedActivityId: 0,
    seasons: [],
    loadingSeasons: false,

    addingActivity: false,
    newActivityName: "",
    newActivityKind: ActivityKindDance,

    editingActivityId: 0,
    editActivityName: "",
    editActivityKind: ActivityKindDance,

    addingSeason: false,
    newSeasonName: "",
    newSeasonStart: "",
    newSeasonEnd: "",
    newSeasonNotes: "",

    editingSeasonId: 0,
    editSeasonName: "",
    editSeasonStart: "",
    editSeasonEnd: "",
    editSeasonNotes: "",

    error: "",
    saving: false,
  })
);

function selectedActivity(state: ActivitiesState): server.Activity | null {
  return state.activities.find(a => a.id === state.selectedActivityId) ?? null;
}

export function view(route: string, prefix: string, data: ActivitiesData): preact.ComponentChild {
  const currentAuth = requireAuthInView();
  if (!currentAuth) return;

  const state = useActivitiesState();
  if (!state.initialized) {
    state.initialized = true;
    state.familyId = data.familyId;
    state.activities = [...data.activities];
    state.selectedActivityId = data.selectedActivityId;
    state.seasons = [...data.seasons];
  }

  const activity = selectedActivity(state);
  const labels = labelsForKind(activity?.kind ?? "");

  return (
    <div>
      <Header isHome={false} />
      <main id="app" className="activities-container">
        <h1>Activities</h1>
        <p className="activities-intro">
          Track a competitive season — {labels.eventPlural.toLowerCase()},{" "}
          {labels.entryPlural.toLowerCase()}, and how each one placed.
        </p>

        <FamilySelect
          id="activitiesFamilyId"
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

        <section className="activities-section">
          <div className="activities-section-head">
            <h2>Programs</h2>
            {!state.addingActivity && (
              <button
                className="btn btn-secondary"
                onClick={vlens.cachePartial(onShowActivityForm, state)}
                disabled={state.saving}
              >
                New program
              </button>
            )}
          </div>

          {state.addingActivity && <ActivityForm state={state} />}

          {state.activities.length === 0 ? (
            <div className="empty-state">
              <p>No programs yet. Add one — "Dance", "Soccer" — to start a season.</p>
            </div>
          ) : (
            <div className="activity-chips">
              {state.activities.map(a =>
                state.editingActivityId === a.id ? (
                  <ActivityEditForm key={a.id} state={state} activity={a} />
                ) : (
                  <div
                    key={a.id}
                    className={
                      a.id === state.selectedActivityId ? "activity-chip selected" : "activity-chip"
                    }
                  >
                    <button
                      className="activity-chip-name"
                      onClick={vlens.cachePartial(onSelectActivity, state, a.id)}
                      disabled={state.saving}
                    >
                      <strong>{a.name}</strong>
                      <small>{activityKindName(a.kind)}</small>
                    </button>
                    <span className="activity-chip-actions">
                      <button
                        className="icon-btn"
                        title="Rename program"
                        aria-label="Rename program"
                        onClick={vlens.cachePartial(onStartEditActivity, state, a)}
                        disabled={state.saving}
                      >
                        ✏️
                      </button>
                      <button
                        className="icon-btn"
                        title="Delete program"
                        aria-label="Delete program"
                        onClick={vlens.cachePartial(onDeleteActivity, state, a)}
                        disabled={state.saving}
                      >
                        🗑️
                      </button>
                    </span>
                  </div>
                )
              )}
            </div>
          )}
        </section>

        {activity && (
          <section className="activities-section">
            <div className="activities-section-head">
              <h2>{activity.name} seasons</h2>
              {!state.addingSeason && (
                <button
                  className="btn btn-primary"
                  onClick={vlens.cachePartial(onShowSeasonForm, state)}
                  disabled={state.saving}
                >
                  New season
                </button>
              )}
            </div>

            {state.addingSeason && <SeasonForm state={state} />}

            {state.loadingSeasons ? (
              <div className="muted">Loading seasons…</div>
            ) : state.seasons.length === 0 ? (
              <div className="empty-state">
                <p>
                  No seasons yet. A season holds the {labels.eventPlural.toLowerCase()} and{" "}
                  {labels.entryPlural.toLowerCase()} for one year.
                </p>
              </div>
            ) : (
              <ul className="season-list">
                {state.seasons.map(season =>
                  state.editingSeasonId === season.id ? (
                    <li key={season.id} className="season-item">
                      <SeasonEditForm state={state} season={season} />
                    </li>
                  ) : (
                    <li key={season.id} className="season-item">
                      <a className="season-item-main season-link" href={`/season/${season.id}`}>
                        <strong className="season-name">{season.name}</strong>
                        {formatDateRange(season.startDate, season.endDate) && (
                          <span className="season-dates">
                            {formatDateRange(season.startDate, season.endDate)}
                          </span>
                        )}
                        {season.notes && <p className="season-notes">{season.notes}</p>}
                        <span className="season-open">Open season →</span>
                      </a>
                      <span className="season-item-actions">
                        <button
                          className="icon-btn"
                          title="Edit season"
                          aria-label="Edit season"
                          onClick={vlens.cachePartial(onStartEditSeason, state, season)}
                          disabled={state.saving}
                        >
                          ✏️
                        </button>
                        <button
                          className="icon-btn"
                          title="Delete season"
                          aria-label="Delete season"
                          onClick={vlens.cachePartial(onDeleteSeason, state, season)}
                          disabled={state.saving}
                        >
                          🗑️
                        </button>
                      </span>
                    </li>
                  )
                )}
              </ul>
            )}
          </section>
        )}
      </main>
      <Footer />
    </div>
  );
}

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
  <div className="activities-form activity-chip-edit">
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

const SeasonForm = ({ state }: { state: ActivitiesState }) => (
  <div className="activities-form">
    <div className="form-group">
      <label htmlFor="newSeasonName">Name</label>
      <input
        id="newSeasonName"
        type="text"
        placeholder="2025–26 Competition Season"
        value={state.newSeasonName}
        onInput={e => {
          state.newSeasonName = e.currentTarget.value;
          vlens.scheduleRedraw();
        }}
        disabled={state.saving}
      />
    </div>
    <div className="form-row">
      <div className="form-group flex-1">
        <label htmlFor="newSeasonStart">Starts</label>
        <input
          id="newSeasonStart"
          type="date"
          value={state.newSeasonStart}
          onInput={e => {
            state.newSeasonStart = e.currentTarget.value;
            vlens.scheduleRedraw();
          }}
          disabled={state.saving}
        />
      </div>
      <div className="form-group flex-1">
        <label htmlFor="newSeasonEnd">Ends</label>
        <input
          id="newSeasonEnd"
          type="date"
          value={state.newSeasonEnd}
          onInput={e => {
            state.newSeasonEnd = e.currentTarget.value;
            vlens.scheduleRedraw();
          }}
          disabled={state.saving}
        />
      </div>
    </div>
    <div className="form-group">
      <label htmlFor="newSeasonNotes">Notes</label>
      <textarea
        id="newSeasonNotes"
        rows={2}
        value={state.newSeasonNotes}
        onInput={e => {
          state.newSeasonNotes = e.currentTarget.value;
          vlens.scheduleRedraw();
        }}
        disabled={state.saving}
      />
    </div>
    <div className="form-actions">
      <button
        className="btn btn-primary"
        onClick={vlens.cachePartial(onCreateSeason, state)}
        disabled={state.saving || !state.newSeasonName.trim()}
      >
        Add season
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

const SeasonEditForm = ({ state, season }: { state: ActivitiesState; season: server.Season }) => (
  <div className="activities-form">
    <div className="form-group">
      <label htmlFor={`editSeasonName${season.id}`}>Name</label>
      <input
        id={`editSeasonName${season.id}`}
        type="text"
        value={state.editSeasonName}
        onInput={e => {
          state.editSeasonName = e.currentTarget.value;
          vlens.scheduleRedraw();
        }}
        disabled={state.saving}
      />
    </div>
    <div className="form-row">
      <div className="form-group flex-1">
        <label htmlFor={`editSeasonStart${season.id}`}>Starts</label>
        <input
          id={`editSeasonStart${season.id}`}
          type="date"
          value={state.editSeasonStart}
          onInput={e => {
            state.editSeasonStart = e.currentTarget.value;
            vlens.scheduleRedraw();
          }}
          disabled={state.saving}
        />
      </div>
      <div className="form-group flex-1">
        <label htmlFor={`editSeasonEnd${season.id}`}>Ends</label>
        <input
          id={`editSeasonEnd${season.id}`}
          type="date"
          value={state.editSeasonEnd}
          onInput={e => {
            state.editSeasonEnd = e.currentTarget.value;
            vlens.scheduleRedraw();
          }}
          disabled={state.saving}
        />
      </div>
    </div>
    <div className="form-group">
      <label htmlFor={`editSeasonNotes${season.id}`}>Notes</label>
      <textarea
        id={`editSeasonNotes${season.id}`}
        rows={2}
        value={state.editSeasonNotes}
        onInput={e => {
          state.editSeasonNotes = e.currentTarget.value;
          vlens.scheduleRedraw();
        }}
        disabled={state.saving}
      />
    </div>
    <div className="form-actions">
      <button
        className="btn btn-primary"
        onClick={vlens.cachePartial(onSaveSeason, state, season.id)}
        disabled={state.saving || !state.editSeasonName.trim()}
      >
        Save
      </button>
      <button
        className="btn btn-secondary"
        onClick={vlens.cachePartial(onCancelEditSeason, state)}
        disabled={state.saving}
      >
        Cancel
      </button>
    </div>
  </div>
);

// ── family switching ────────────────────────────────────────────

// selectFamily reloads the whole page for a user in more than one family.
// Activities are family-scoped, so the list, the selection, and the season
// list underneath it all belong to whichever family is showing.
async function selectFamily(state: ActivitiesState, familyId: number) {
  if (state.familyId === familyId) return;
  state.familyId = familyId;
  state.activities = [];
  state.seasons = [];
  state.selectedActivityId = 0;
  state.addingActivity = false;
  state.editingActivityId = 0;
  state.addingSeason = false;
  state.editingSeasonId = 0;
  state.error = "";
  state.loadingSeasons = true;
  vlens.scheduleRedraw();

  const [resp, err] = await server.ListActivities({ familyId });
  if (state.familyId !== familyId) return;
  if (err || !resp) {
    state.error = err || "Failed to load activities";
    state.loadingSeasons = false;
    vlens.scheduleRedraw();
    return;
  }

  state.activities = resp.activities ?? [];
  state.loadingSeasons = false;
  await selectActivity(state, state.activities.length > 0 ? state.activities[0].id : 0);
}

// ── activity handlers ────────────────────────────────────────────────────────

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
  if (err || !resp) {
    state.error = err || "Failed to create program";
    state.saving = false;
    vlens.scheduleRedraw();
    return;
  }

  state.activities.push(resp.activity);
  state.addingActivity = false;
  state.newActivityName = "";
  state.saving = false;
  await selectActivity(state, resp.activity.id);
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
  if (err) {
    state.error = err || "Failed to delete program";
    state.saving = false;
    vlens.scheduleRedraw();
    return;
  }

  const idx = state.activities.findIndex(a => a.id === activity.id);
  if (idx >= 0) state.activities.splice(idx, 1);
  state.saving = false;
  if (state.selectedActivityId === activity.id) {
    await selectActivity(state, state.activities.length > 0 ? state.activities[0].id : 0);
    return;
  }
  vlens.scheduleRedraw();
}

function onSelectActivity(state: ActivitiesState, activityId: number) {
  if (state.selectedActivityId === activityId) return;
  void selectActivity(state, activityId);
}

// selectActivity switches the season list over. Seasons for the previous
// activity are dropped immediately so a slow load never shows one activity's
// heading above another's seasons.
async function selectActivity(state: ActivitiesState, activityId: number) {
  state.selectedActivityId = activityId;
  state.seasons = [];
  state.addingSeason = false;
  state.editingSeasonId = 0;
  if (activityId === 0) {
    state.loadingSeasons = false;
    vlens.scheduleRedraw();
    return;
  }

  state.loadingSeasons = true;
  vlens.scheduleRedraw();

  const [resp, err] = await server.ListSeasons({ activityId });
  // A second click while this one was in flight wins; drop the stale answer.
  if (state.selectedActivityId !== activityId) return;
  if (err || !resp) {
    state.error = err || "Failed to load seasons";
  } else {
    state.seasons = resp.seasons ?? [];
  }
  state.loadingSeasons = false;
  vlens.scheduleRedraw();
}

// ── season handlers ──────────────────────────────────────────────────────────

function onShowSeasonForm(state: ActivitiesState) {
  state.addingSeason = true;
  state.editingSeasonId = 0;
  state.newSeasonName = "";
  state.newSeasonStart = "";
  state.newSeasonEnd = "";
  state.newSeasonNotes = "";
  vlens.scheduleRedraw();
}

function onCancelSeasonForm(state: ActivitiesState) {
  state.addingSeason = false;
  vlens.scheduleRedraw();
}

// Empty date inputs go over as null rather than "": the backend reads a nil
// pointer as "not known yet" and stores the zero time.
function dateOrNull(value: string): string | null {
  const trimmed = value.trim();
  return trimmed === "" ? null : trimmed;
}

async function onCreateSeason(state: ActivitiesState) {
  const name = state.newSeasonName.trim();
  if (!name || state.selectedActivityId === 0) return;
  state.saving = true;
  state.error = "";
  vlens.scheduleRedraw();

  const [resp, err] = await server.CreateSeason({
    activityId: state.selectedActivityId,
    name,
    startDate: dateOrNull(state.newSeasonStart),
    endDate: dateOrNull(state.newSeasonEnd),
    notes: state.newSeasonNotes.trim(),
  });
  if (err || !resp) {
    state.error = err || "Failed to create season";
  } else {
    state.seasons.push(resp.season);
    state.addingSeason = false;
  }
  state.saving = false;
  vlens.scheduleRedraw();
}

function onStartEditSeason(state: ActivitiesState, season: server.Season) {
  state.editingSeasonId = season.id;
  state.editSeasonName = season.name;
  state.editSeasonStart = toDateInputValue(season.startDate);
  state.editSeasonEnd = toDateInputValue(season.endDate);
  state.editSeasonNotes = season.notes;
  state.addingSeason = false;
  vlens.scheduleRedraw();
}

function onCancelEditSeason(state: ActivitiesState) {
  state.editingSeasonId = 0;
  vlens.scheduleRedraw();
}

async function onSaveSeason(state: ActivitiesState, seasonId: number) {
  const name = state.editSeasonName.trim();
  if (!name) return;
  state.saving = true;
  state.error = "";
  vlens.scheduleRedraw();

  const [resp, err] = await server.UpdateSeason({
    id: seasonId,
    name,
    startDate: dateOrNull(state.editSeasonStart),
    endDate: dateOrNull(state.editSeasonEnd),
    notes: state.editSeasonNotes.trim(),
  });
  if (err || !resp) {
    state.error = err || "Failed to update season";
  } else {
    const idx = state.seasons.findIndex(s => s.id === seasonId);
    if (idx >= 0) state.seasons[idx] = resp.season;
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
    const idx = state.seasons.findIndex(s => s.id === season.id);
    if (idx >= 0) state.seasons.splice(idx, 1);
    if (state.editingSeasonId === season.id) state.editingSeasonId = 0;
  }
  state.saving = false;
  vlens.scheduleRedraw();
}
