import { BookOpen, MapPin, Search, Trophy } from "lucide-preact";
import { Icon } from "../../components/Icon";
import * as preact from "preact";
import * as vlens from "vlens";
import * as core from "vlens/core";
import { navigate } from "../../lib/navigate";
import * as rpc from "vlens/rpc";
import * as auth from "../../lib/authCache";
import * as server from "../../server";
import { Header, Footer } from "../../layout";
import { ensureAuthInFetch, requireAuthInView } from "../../lib/authHelpers";
import { PersonChips, initial } from "../../components/PersonChips";
import { ProfileImage, ThumbnailImage } from "../../components/ResponsiveImage";
import { VideoBadge } from "../../components/VideoBadge";
import { InterviewCard } from "../../components/InterviewCard";
import { isQuote } from "../../components/MilestoneText";
import { chipOrder } from "../../lib/familyGroups";
import { groupInterviewAnswers, interviewsById, InterviewGroup } from "../../lib/interviews";
import { getCategoryIcon } from "../../lib/milestoneHelpers";
import { formatDate } from "../../lib/dateUtils";
import { matchStartMs, saveSequence, viewPhotoRoute } from "../../lib/photoSequence";
import { copy } from "../../lib/copy";
import {
  Chip,
  ChipKind,
  activityHref,
  addRecent,
  hasResults,
  highlightPieces,
  loadRecent,
  nextIndex,
  photosHref,
  queryOfRoute,
  saveRecent,
  searchRoute,
  searchTerms,
  understoodChips,
} from "../../lib/search";
import "../../components/day-summary-styles";
import "./search-styles";

const DEBOUNCE_MS = 150;
const SHOW_ALL_MOMENTS = 100;

type SearchData = {
  query: string;
  people: server.Person[];
  result: server.SearchResponse | null;
};

export async function fetch(route: string, prefix: string): Promise<rpc.Response<SearchData>> {
  const query = queryOfRoute(route);
  if (!(await ensureAuthInFetch())) {
    return rpc.ok<SearchData>({ query, people: [], result: null });
  }
  const [[people], [result]] = await Promise.all([
    server.ListPeople({}),
    query ? server.Search({ query, ignore: [], limit: 0 }) : Promise.resolve([null, ""] as const),
  ]);
  return rpc.ok<SearchData>({
    query,
    people: chipOrder(
      (people?.people ?? []).filter(p => !p.isPregnancy),
      people?.relations ?? [],
      auth.getAuth()?.familyId ?? 0
    ),
    result: result ?? null,
  });
}

export function view(route: string, prefix: string, data: SearchData): preact.ComponentChild {
  if (!requireAuthInView()) return;
  return (
    <div>
      <Header isHome={false} />
      <main id="app" className="search-container">
        <SearchPage data={data} />
      </main>
      <Footer />
    </div>
  );
}

type SearchState = {
  seed: SearchData | null;
  query: string;
  ignore: ChipKind[];
  limit: number;
  result: server.SearchResponse | null;
  loading: boolean;
  error: string;
  seq: number;
  timer: number;
  recent: string[];
  focusInput: boolean;
};

const useSearchState = vlens.declareHook(
  (): SearchState => ({
    seed: null,
    query: "",
    ignore: [],
    limit: 0,
    result: null,
    loading: false,
    error: "",
    seq: 0,
    timer: 0,
    recent: loadRecent(),
    focusInput: false,
  })
);

async function runSearch(state: SearchState) {
  window.clearTimeout(state.timer);
  const seq = ++state.seq;
  const query = state.query.trim();
  if (!query) {
    state.result = null;
    state.loading = false;
    state.error = "";
    vlens.scheduleRedraw();
    return;
  }
  state.loading = true;
  vlens.scheduleRedraw();
  const [result, err] = await server.Search({ query, ignore: state.ignore, limit: state.limit });
  if (seq !== state.seq) return;
  state.loading = false;
  state.error = result ? "" : err || copy.search.failed;
  if (result) state.result = result;
  vlens.scheduleRedraw();
}

function setQuery(state: SearchState, query: string) {
  state.query = query;
  state.ignore = [];
  state.limit = 0;
  window.history.replaceState(window.history.state, "", searchRoute(query));
  window.clearTimeout(state.timer);
  state.timer = window.setTimeout(() => runSearch(state), DEBOUNCE_MS);
  vlens.scheduleRedraw();
}

function onInput(state: SearchState, event: Event) {
  setQuery(state, (event.target as HTMLInputElement).value);
}

function searchFor(state: SearchState, query: string) {
  setQuery(state, query);
  runSearch(state);
  document.getElementById("searchInput")?.focus();
}

function clearQuery(state: SearchState) {
  searchFor(state, "");
}

function removeChip(state: SearchState, kind: ChipKind) {
  if (!state.ignore.includes(kind)) state.ignore.push(kind);
  runSearch(state);
}

function showAllMoments(state: SearchState) {
  state.limit = SHOW_ALL_MOMENTS;
  runSearch(state);
}

function remember(state: SearchState) {
  state.recent = addRecent(state.recent, state.query);
  saveRecent(state.recent);
}

function clearRecent(state: SearchState) {
  state.recent = [];
  saveRecent([]);
  vlens.scheduleRedraw();
}

function resultLinks(): HTMLElement[] {
  return Array.from(
    document.querySelectorAll<HTMLElement>(
      ".search-results [data-result], .search-results .search-interview a"
    )
  );
}

function onKeyDown(state: SearchState, event: KeyboardEvent) {
  const links = resultLinks();
  const input = document.getElementById("searchInput");
  if (event.key === "Enter" && event.target === input) {
    event.preventDefault();
    window.clearTimeout(state.timer);
    if (links[0]) {
      remember(state);
      links[0].click();
    } else {
      runSearch(state);
    }
    return;
  }
  if (event.key !== "ArrowDown" && event.key !== "ArrowUp") return;
  event.preventDefault();
  const current = links.indexOf(document.activeElement as HTMLElement);
  if (event.key === "ArrowUp" && current <= 0) {
    input?.focus();
    return;
  }
  links[nextIndex(current, event.key === "ArrowDown" ? 1 : -1, links.length)]?.focus();
}

function onResultsClick(state: SearchState, event: MouseEvent) {
  if ((event.target as HTMLElement).closest("a")) remember(state);
}

function openPhoto(
  state: SearchState,
  photos: server.PhotoWithPeople[],
  photo: server.PhotoWithPeople
) {
  remember(state);
  saveSequence({ ids: photos.map(p => p.image.id), backRoute: core.getRoute() });
  navigate(viewPhotoRoute(photo.image.id, true, matchStartMs(photo)));
}

function focusOnMount(state: SearchState, el: HTMLInputElement | null) {
  if (el && state.focusInput) {
    state.focusInput = false;
    el.focus();
  }
}

const SearchPage = ({ data }: { data: SearchData }) => {
  const state = useSearchState();
  if (state.seed !== data) {
    state.seed = data;
    state.query = data.query;
    state.ignore = [];
    state.limit = 0;
    state.result = data.result;
    state.error = "";
    state.loading = false;
    state.seq++;
    state.focusInput = !data.query;
  }

  const query = state.query.trim();
  const result = query ? state.result : null;
  const chips = result ? understoodChips(result.understood, result.people) : [];
  const terms = result ? searchTerms(result.understood.text) : [];

  return (
    <div className="search-page" onKeyDown={vlens.cachePartial(onKeyDown, state)}>
      <h1 className="search-title">{copy.search.title}</h1>
      <form className="search-field" role="search" onSubmit={e => e.preventDefault()}>
        <span className="search-field-icon" aria-hidden="true">
          <Icon symbol={Search} />
        </span>
        <input
          id="searchInput"
          type="search"
          autoComplete="off"
          enterKeyHint="search"
          maxLength={200}
          aria-label={copy.search.label}
          placeholder={copy.search.placeholder}
          value={state.query}
          onInput={vlens.cachePartial(onInput, state)}
          ref={vlens.cachePartial(focusOnMount, state)}
        />
        {state.query && (
          <button
            type="button"
            className="search-clear"
            aria-label={copy.search.clear}
            onClick={vlens.cachePartial(clearQuery, state)}
          >
            ×
          </button>
        )}
      </form>

      {chips.length > 0 && <UnderstoodChips state={state} chips={chips} />}

      {!query ? (
        <EmptyQuery state={state} people={data.people} />
      ) : state.error ? (
        <p className="error-message" role="alert">
          {state.error}
        </p>
      ) : !result ? (
        <p className="search-status">{copy.search.searching}</p>
      ) : (
        <div
          className={state.loading ? "search-results stale" : "search-results"}
          onClick={vlens.cachePartial(onResultsClick, state)}
        >
          {result.searchMode === "text" && <p className="search-note">{copy.search.textOnly}</p>}
          {hasResults(result) ? (
            <Groups state={state} result={result} terms={terms} family={data.people} />
          ) : (
            <NothingMatched query={query} chips={chips} state={state} />
          )}
        </div>
      )}
    </div>
  );
};

const UnderstoodChips = ({ state, chips }: { state: SearchState; chips: Chip[] }) => (
  <div className="search-chips" aria-label={copy.search.understood}>
    {chips.map(chip => (
      <span key={chip.kind} className="search-chip">
        {chip.label}
        <button
          type="button"
          aria-label={copy.search.remove(chip.label)}
          onClick={vlens.cachePartial(removeChip, state, chip.kind)}
        >
          ×
        </button>
      </span>
    ))}
  </div>
);

const EmptyQuery = ({ state, people }: { state: SearchState; people: server.Person[] }) => (
  <div className="search-empty">
    {people.length > 0 && (
      <section>
        <h2 className="search-group-title">{copy.search.family}</h2>
        <PersonChips
          people={people}
          selected={[]}
          onToggle={(id: number) =>
            searchFor(state, people.find(p => p.id === id)?.name.split(" ")[0] ?? "")
          }
          label={copy.search.family}
        />
      </section>
    )}
    {state.recent.length > 0 && (
      <section>
        <div className="search-group-head">
          <h2 className="search-group-title">{copy.search.recent}</h2>
          <button
            type="button"
            className="link-button"
            onClick={vlens.cachePartial(clearRecent, state)}
          >
            {copy.search.clearRecent}
          </button>
        </div>
        <ul className="search-recent">
          {state.recent.map(q => (
            <li key={q}>
              <button type="button" onClick={() => searchFor(state, q)}>
                {q}
              </button>
            </li>
          ))}
        </ul>
      </section>
    )}
  </div>
);

const NothingMatched = ({
  query,
  chips,
  state,
}: {
  query: string;
  chips: Chip[];
  state: SearchState;
}) => (
  <div className="search-nothing">
    <p>{copy.search.nothing(query)}</p>
    {chips.map(chip => (
      <button
        key={chip.kind}
        type="button"
        className="link-button"
        onClick={vlens.cachePartial(removeChip, state, chip.kind)}
      >
        {copy.search.tryWithout(chip.label)}
      </button>
    ))}
  </div>
);

const Marked = ({ text, terms }: { text: string; terms: string[] }) => (
  <>
    {highlightPieces(text, terms).map((piece, i) =>
      piece.mark ? <mark key={i}>{piece.text}</mark> : piece.text
    )}
  </>
);

const Group = ({
  title,
  action,
  children,
}: {
  title: string;
  action?: preact.ComponentChild;
  children: preact.ComponentChildren;
}) => (
  <section className="search-group">
    <div className="search-group-head">
      <h2 className="search-group-title">{title}</h2>
      {action}
    </div>
    {children}
  </section>
);

const Groups = ({
  state,
  result,
  terms,
  family,
}: {
  state: SearchState;
  result: server.SearchResponse;
  terms: string[];
  family: server.Person[];
}) => {
  const g = copy.search.groups;
  const people = new Map([...family, ...(result.people ?? [])].map(p => [p.id, p]));
  const photos = result.photos ?? [];
  return (
    <>
      {result.people.length > 0 && (
        <Group title={g.people}>
          <ul className="search-list">
            {result.people.map(p => (
              <PersonRow key={p.id} person={p} terms={terms} />
            ))}
          </ul>
        </Group>
      )}

      {result.moments.length > 0 && (
        <Group
          title={momentsTitle(result.moments)}
          action={
            result.momentCount > result.moments.length && (
              <button
                type="button"
                className="link-button"
                onClick={vlens.cachePartial(showAllMoments, state)}
              >
                {copy.search.showAllMoments(result.momentCount)}
              </button>
            )
          }
        >
          <Moments result={result} people={people} terms={terms} />
        </Group>
      )}

      {photos.length > 0 && (
        <Group
          title={g.photos}
          action={
            result.morePhotos && (
              <a className="search-see-all" href={photosHref(result.understood)} data-result>
                {copy.search.seeAllPhotos}
              </a>
            )
          }
        >
          <div className="search-photos">
            {photos.map(p => (
              <a
                key={p.image.id}
                href={viewPhotoRoute(p.image.id, true, matchStartMs(p))}
                className="search-photo has-video-badge"
                data-result
                title={p.match?.snippet || p.image.title || undefined}
                onClick={e => {
                  e.preventDefault();
                  openPhoto(state, photos, p);
                }}
              >
                <ThumbnailImage
                  photoId={p.image.id}
                  alt={p.image.title || ""}
                  frameMs={matchStartMs(p)}
                />
                <VideoBadge image={p.image} />
                {p.match?.snippet && (
                  <span className="search-photo-snippet">“{p.match.snippet}”</span>
                )}
              </a>
            ))}
          </div>
        </Group>
      )}

      {result.tags.length > 0 && (
        <Group title={g.tags}>
          <ul className="search-list">
            {result.tags.map(tag => (
              <li key={tag.id}>
                <a className="search-row" href={`/photos?tags=${tag.id}`} data-result>
                  <span
                    className="search-row-icon search-tag-dot"
                    style={{ background: tag.color || "var(--accent)" }}
                    aria-hidden="true"
                  />
                  <span className="search-row-main">
                    <Marked text={tag.name} terms={terms} />
                    <small>{copy.search.tagPhotos}</small>
                  </span>
                </a>
              </li>
            ))}
          </ul>
        </Group>
      )}

      {result.places.length > 0 && (
        <Group title={g.places}>
          <ul className="search-list">
            {result.places.map(place => (
              <li key={place.key}>
                <a
                  className="search-row"
                  href={`/photos?place=${encodeURIComponent(place.key)}`}
                  data-result
                >
                  <span className="search-row-icon" aria-hidden="true">
                    <Icon symbol={MapPin} />
                  </span>
                  <span className="search-row-main">
                    <Marked text={place.name} terms={terms} />
                  </span>
                </a>
              </li>
            ))}
          </ul>
        </Group>
      )}

      {result.activities.length > 0 && (
        <Group title={g.activities}>
          <ul className="search-list">
            {result.activities.map(hit => (
              <li key={`${hit.kind}${hit.id}`}>
                <a className="search-row" href={activityHref(hit)} data-result>
                  <span className="search-row-icon" aria-hidden="true">
                    <Icon symbol={Trophy} />
                  </span>
                  <span className="search-row-main">
                    <Marked text={hit.name} terms={terms} />
                    <small>
                      {[hit.detail, hit.kind !== "activity" && hit.date ? formatDate(hit.date) : ""]
                        .filter(Boolean)
                        .join(" · ")}
                    </small>
                  </span>
                </a>
              </li>
            ))}
          </ul>
        </Group>
      )}

      {result.books.length > 0 && (
        <Group title={g.books}>
          <ul className="search-list">
            {result.books.map(book => (
              <li key={book.id}>
                <a className="search-row" href={`/book/${book.id}`} data-result>
                  <span className="search-row-icon" aria-hidden="true">
                    <Icon symbol={BookOpen} />
                  </span>
                  <span className="search-row-main">
                    <Marked text={book.title} terms={terms} />
                    <small>{copy.search.by(book.personNames ?? [])}</small>
                  </span>
                </a>
              </li>
            ))}
          </ul>
        </Group>
      )}
    </>
  );
};

const PersonRow = ({ person, terms }: { person: server.Person; terms: string[] }) => {
  const q = copy.search.quick;
  return (
    <li className="search-person">
      <a className="search-row" href={`/profile/${person.id}`} data-result>
        <span className="search-avatar" aria-hidden="true">
          {person.profilePhotoId ? (
            <ProfileImage
              photoId={person.profilePhotoId}
              alt=""
              cropX={person.profileCropX}
              cropY={person.profileCropY}
              cropScale={person.profileCropScale}
            />
          ) : (
            initial(person.name)
          )}
        </span>
        <span className="search-row-main">
          <Marked text={person.name} terms={terms} />
          <small>{[person.relationship, person.age].filter(Boolean).join(" · ")}</small>
        </span>
      </a>
      <span className="search-quick">
        <a href={`/profile/${person.id}`}>{q.moments}</a>
        <a href={`/profile/${person.id}?tab=photos`}>{q.photos}</a>
        {!person.isPregnancy && <a href={`/profile/${person.id}?tab=growth`}>{q.growth}</a>}
      </span>
    </li>
  );
};

function momentsTitle(moments: server.Milestone[]): string {
  const g = copy.search.groups;
  const interviews = moments.filter(m => m.interviewId).length;
  if (interviews === 0) return g.moments;
  return interviews === moments.length ? g.interviews : g.momentsAndInterviews;
}

type MomentItem =
  | { kind: "moment"; moment: server.Milestone }
  | { kind: "interview"; group: InterviewGroup };

function momentItems(moments: server.Milestone[]): MomentItem[] {
  const { groups } = groupInterviewAnswers(moments);
  const byInterview = new Map(groups.map(g => [g.interviewId, g]));
  const items: MomentItem[] = [];
  const shown = new Set<number>();
  for (const m of moments) {
    if (!m.interviewId) {
      items.push({ kind: "moment", moment: m });
    } else if (!shown.has(m.interviewId)) {
      shown.add(m.interviewId);
      items.push({ kind: "interview", group: byInterview.get(m.interviewId)! });
    }
  }
  return items;
}

const Moments = ({
  result,
  people,
  terms,
}: {
  result: server.SearchResponse;
  people: Map<number, server.Person>;
  terms: string[];
}) => {
  const interviews = interviewsById(result.interviews);
  return (
    <div className="search-moments">
      {momentItems(result.moments).map(item =>
        item.kind === "interview" ? (
          <div key={`i${item.group.interviewId}`} className="search-interview">
            <InterviewCard
              group={item.group}
              person={people.get(item.group.personId)}
              interview={interviews.get(item.group.interviewId)}
              matchesOnly
            />
          </div>
        ) : (
          <MomentRow
            key={item.moment.id}
            moment={item.moment}
            person={people.get(item.moment.personId)}
            terms={terms}
          />
        )
      )}
    </div>
  );
};

const MomentRow = ({
  moment,
  person,
  terms,
}: {
  moment: server.Milestone;
  person: server.Person | undefined;
  terms: string[];
}) => (
  <a href={`/milestone/${moment.id}`} className="day-milestone" data-result>
    <span className="day-milestone-icon" aria-hidden="true">
      {getCategoryIcon(moment.category)}
    </span>
    <span>
      {person && <strong>{person.name.split(" ")[0]} </strong>}
      {isQuote(moment) ? (
        <q className="milestone-quote">
          <Marked text={moment.description} terms={terms} />
        </q>
      ) : (
        <Marked text={moment.description} terms={terms} />
      )}
      {isQuote(moment) && moment.context && (
        <small className="search-moment-context">
          {" "}
          — <Marked text={moment.context} terms={terms} />
        </small>
      )}
      <small className="search-moment-date"> · {formatDate(moment.milestoneDate)}</small>
    </span>
  </a>
);
