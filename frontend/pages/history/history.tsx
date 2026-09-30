import * as preact from "preact";
import * as rpc from "vlens/rpc";
import * as vlens from "vlens";
import * as auth from "../../lib/authCache";
import * as server from "../../server";
import { Header, Footer } from "../../layout";
import { ensureAuthInFetch, requireAuthInView } from "../../lib/authHelpers";
import { LoadMore } from "../../components/LoadMore";
import { DaySummaryList } from "../../components/DaySummaryList";
import { PersonChips } from "../../components/PersonChips";
import { useTagCache } from "../../hooks/useTagCache";
import { chipOrder } from "../../lib/familyGroups";
import { groupByMonth, summarizeDays } from "../../lib/daySummary";
import { HISTORY_TYPES, HistoryType, historyView } from "../../lib/history";
import { getCategoryIcon } from "../../lib/milestoneHelpers";
import { formatDate } from "../../lib/dateUtils";
import { localDateString } from "../../lib/when";
import { copy } from "../../lib/copy";
import {
  firstUnloadedYear,
  mergeTimeline,
  timelineRequest,
  yearRange,
  yearsToReach,
} from "../../lib/photoPages";
import "./history-styles";

const FIRST_LOAD_ENTRIES = 50;

type HistoryData = {
  timeline: server.GetFamilyTimelineResponse;
  loaded: number[];
};

const emptyTimeline: server.GetFamilyTimelineResponse = {
  people: [],
  relations: [],
  years: [],
  appearances: [],
  untaggedPhotos: [],
};

const countEntries = (timeline: server.GetFamilyTimelineResponse) =>
  (timeline.people ?? []).reduce(
    (sum, item) =>
      sum +
      (item.milestones?.length ?? 0) +
      (item.growthData?.length ?? 0) +
      (item.photos?.length ?? 0),
    (timeline.appearances ?? []).length + (timeline.untaggedPhotos ?? []).length
  );

function yearRequest(from: number, to: number): server.GetFamilyTimelineRequest {
  return { ...yearRange(from, to), includeActivities: true, includeUntaggedPhotos: true };
}

export async function fetch(route: string, prefix: string): Promise<rpc.Response<HistoryData>> {
  if (!(await ensureAuthInFetch())) {
    return rpc.ok<HistoryData>({ timeline: emptyTimeline, loaded: [] });
  }

  const thisYear = new Date().getUTCFullYear();
  const [first, err] = await server.GetFamilyTimeline(
    timelineRequest({ from: `${thisYear}-01-01`, includeActivities: true })
  );
  if (!first) return [null, err];

  const data: HistoryData = {
    timeline: first,
    loaded: (first.years ?? []).filter(year => year >= thisYear),
  };
  while (countEntries(data.timeline) < FIRST_LOAD_ENTRIES) {
    const next = firstUnloadedYear(data.timeline.years ?? [], data.loaded, "newest");
    if (next === null) break;
    const [more, moreErr] = await server.GetFamilyTimeline(yearRequest(next, next));
    if (!more) return [null, moreErr];
    data.timeline = mergeTimeline(data.timeline, more);
    data.loaded.push(next);
  }
  return rpc.ok(data);
}

export function view(route: string, prefix: string, data: HistoryData): preact.ComponentChild {
  if (!requireAuthInView()) return;

  return (
    <div>
      <Header isHome={false} />
      <main id="app" className="history-container">
        <HistoryPage data={data} />
      </main>
      <Footer />
    </div>
  );
}

type HistoryState = {
  seed: HistoryData | null;
  timeline: server.GetFamilyTimelineResponse;
  loaded: number[];
  loading: boolean;
  error: string;
  personIds: number[];
  types: HistoryType[];
  tagIds: number[];
  query: string;
  results: server.Milestone[] | null;
  searching: boolean;
};

const useHistoryState = vlens.declareHook(
  (): HistoryState => ({
    seed: null,
    timeline: emptyTimeline,
    loaded: [],
    loading: false,
    error: "",
    personIds: [],
    types: [...HISTORY_TYPES],
    tagIds: [],
    query: "",
    results: null,
    searching: false,
  })
);

async function loadYears(state: HistoryState, years: number[]): Promise<boolean> {
  if (state.loading || years.length === 0) return false;
  state.loading = true;
  vlens.scheduleRedraw();

  const from = Math.min(...years);
  const to = Math.max(...years);
  const [more, err] = await server.GetFamilyTimeline(yearRequest(from, to));
  state.loading = false;
  if (more) {
    state.timeline = mergeTimeline(state.timeline, more);
    for (const year of state.timeline.years ?? []) {
      if (year >= from && year <= to && !state.loaded.includes(year)) state.loaded.push(year);
    }
    state.error = "";
  } else {
    state.error = err || "Failed to load history";
  }
  vlens.scheduleRedraw();
  return !!more;
}

async function jumpToYear(state: HistoryState, year: number) {
  const missing = yearsToReach(year, state.timeline.years ?? [], state.loaded, "newest");
  if (missing.length > 0 && !(await loadYears(state, missing))) return;
  requestAnimationFrame(() =>
    document.getElementById(`year-${year}`)?.scrollIntoView({ behavior: "smooth", block: "start" })
  );
}

function toggle<T>(list: T[], value: T) {
  const index = list.indexOf(value);
  if (index >= 0) list.splice(index, 1);
  else list.push(value);
  vlens.scheduleRedraw();
}

function togglePerson(state: HistoryState, personId: number) {
  toggle(state.personIds, personId);
}

function toggleType(state: HistoryState, type: HistoryType) {
  toggle(state.types, type);
}

function toggleTag(state: HistoryState, tagId: number) {
  toggle(state.tagIds, tagId);
}

function resetFilters(state: HistoryState) {
  state.personIds = [];
  state.types = [...HISTORY_TYPES];
  state.tagIds = [];
  vlens.scheduleRedraw();
}

async function search(state: HistoryState, event: Event) {
  event.preventDefault();
  const query = state.query.trim();
  if (!query) return;
  state.searching = true;
  vlens.scheduleRedraw();
  const [result] = await server.SearchMilestones({ query, limit: 50 });
  state.results = result?.milestones ?? [];
  state.searching = false;
  vlens.scheduleRedraw();
}

function clearSearch(state: HistoryState) {
  state.query = "";
  state.results = null;
  vlens.scheduleRedraw();
}

const HistoryPage = ({ data }: { data: HistoryData }) => {
  const state = useHistoryState();
  if (state.seed !== data) {
    state.seed = data;
    state.timeline = data.timeline;
    state.loaded = [...data.loaded];
    state.error = "";
  }
  const tagCache = useTagCache();
  tagCache.loadTags();
  const tags = tagCache.tags;

  const today = localDateString(new Date());
  const everyone = (state.timeline.people ?? []).map(item => item.person);
  const people = chipOrder(everyone, state.timeline.relations ?? [], auth.getAuth()?.familyId ?? 0);
  const years = state.timeline.years ?? [];
  const stopYear = firstUnloadedYear(years, state.loaded, "newest");

  const shown = historyView(
    state.timeline,
    { personIds: state.personIds, types: state.types, tagIds: state.tagIds },
    stopYear,
    today
  );
  const days = summarizeDays(shown.records, shown.birthdayPeople, shown.range ?? undefined);
  const months = groupByMonth(days);
  const filtered =
    state.personIds.length > 0 ||
    state.tagIds.length > 0 ||
    state.types.length < HISTORY_TYPES.length;
  const moreCount = HISTORY_TYPES.length - state.types.length + state.tagIds.length;

  const firstMonthOfYear = new Set<string>();
  const seenYears = new Set<string>();
  for (const m of months) {
    const year = m.month.slice(0, 4);
    if (!seenYears.has(year)) {
      seenYears.add(year);
      firstMonthOfYear.add(m.month);
    }
  }

  return (
    <div className="history-page">
      <h1>{copy.nav.history}</h1>

      <div className="history-filters">
        <PersonChips
          people={people}
          selected={state.personIds}
          onToggle={vlens.cachePartial(togglePerson, state)}
          label={copy.history.people}
        />
        <div className="history-filter-row">
          <details className="history-more">
            <summary>
              {copy.history.more}
              {moreCount > 0 && <span className="count-badge">{moreCount}</span>}
            </summary>
            <div className="history-more-panel">
              <span className="history-more-label">{copy.history.show}</span>
              <div className="history-pills">
                {HISTORY_TYPES.map(type => (
                  <button
                    key={type}
                    type="button"
                    className={
                      state.types.includes(type) ? "history-pill selected" : "history-pill"
                    }
                    aria-pressed={state.types.includes(type) ? "true" : "false"}
                    onClick={vlens.cachePartial(toggleType, state, type)}
                  >
                    {copy.history.types[type]}
                  </button>
                ))}
              </div>
              {tags.length > 0 && (
                <>
                  <span className="history-more-label">{copy.history.tags}</span>
                  <div className="history-pills">
                    {tags.map(tag => (
                      <button
                        key={tag.id}
                        type="button"
                        className={
                          state.tagIds.includes(tag.id) ? "history-pill selected" : "history-pill"
                        }
                        style={{ borderColor: tag.color }}
                        aria-pressed={state.tagIds.includes(tag.id) ? "true" : "false"}
                        onClick={vlens.cachePartial(toggleTag, state, tag.id)}
                      >
                        {tag.name}
                      </button>
                    ))}
                  </div>
                </>
              )}
              <a href="/manage-tags" className="history-manage-tags">
                {copy.history.manageTags}
              </a>
            </div>
          </details>
          {filtered && (
            <button type="button" className="link-button" onClick={() => resetFilters(state)}>
              {copy.history.clear}
            </button>
          )}
          <form className="history-search" onSubmit={vlens.cachePartial(search, state)}>
            <input
              type="search"
              placeholder={copy.history.searchPlaceholder}
              aria-label={copy.history.search}
              {...vlens.attrsBindInput(vlens.ref(state, "query"))}
            />
          </form>
        </div>
      </div>

      {state.results !== null ? (
        <SearchResults state={state} people={everyone} />
      ) : years.length === 0 ? (
        <p className="history-empty">{copy.history.empty}</p>
      ) : (
        <>
          {years.length > 1 && (
            <nav className="history-years" aria-label={copy.history.years}>
              {years.map(year => (
                <button key={year} type="button" onClick={() => jumpToYear(state, year)}>
                  {year}
                </button>
              ))}
            </nav>
          )}

          {months.map(month => (
            <section key={month.month} className="history-month">
              {firstMonthOfYear.has(month.month) && <span id={`year-${month.month.slice(0, 4)}`} />}
              <h2 className="history-month-label">{month.label}</h2>
              <DaySummaryList days={month.days} people={everyone} today={today} />
            </section>
          ))}

          {months.length === 0 && stopYear === null && (
            <p className="history-empty">{copy.history.noMatches}</p>
          )}

          {state.error && (
            <div className="error-message" role="alert">
              {state.error}
            </div>
          )}

          {stopYear !== null && (
            <LoadMore
              loading={state.loading}
              onLoad={() => loadYears(state, [stopYear])}
              label={copy.history.loadYear(stopYear)}
            />
          )}
        </>
      )}
    </div>
  );
};

const SearchResults = ({ state, people }: { state: HistoryState; people: server.Person[] }) => {
  const names = new Map(people.map(p => [p.id, p.name]));
  const results = state.results ?? [];
  return (
    <section className="history-results">
      <div className="history-results-head">
        <h2>{copy.history.resultsFor(state.query.trim(), results.length)}</h2>
        <button type="button" className="link-button" onClick={() => clearSearch(state)}>
          {copy.history.clear}
        </button>
      </div>
      {results.map(m => (
        <a key={m.id} href={`/milestone/${m.id}`} className="day-milestone">
          <span className="day-milestone-icon" aria-hidden="true">
            {getCategoryIcon(m.category)}
          </span>
          <span>
            <strong>{names.get(m.personId) ?? ""}</strong> {m.description}
            <small className="history-result-date"> · {formatDate(m.milestoneDate)}</small>
          </span>
        </a>
      ))}
    </section>
  );
};
