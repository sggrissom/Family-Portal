import * as preact from "preact";
import * as vlens from "vlens";
import * as rpc from "vlens/rpc";
import * as server from "../../server";
import { Header, Footer } from "../../layout";
import { requireAuthInView } from "../../lib/authHelpers";
import { canContribute } from "../../lib/authCache";
import { openAddSheet } from "../../components/AppNav";
import { ProfileImage, ThumbnailImage } from "../../components/ResponsiveImage";
import { FaceCrop } from "../../components/FaceCrop";
import { DaySummaryList } from "../../components/DaySummaryList";
import { MilestoneText, isQuote } from "../../components/MilestoneText";
import { SameAgeStrip } from "../../components/SameAgeRows";
import {
  AgeChart,
  AgeSeries,
  ChartZoom,
  SERIES_COLORS,
  newChartZoom,
} from "../../components/AgeChart";
import { SegmentedControl } from "../../components/SegmentedControl";
import { initial } from "../../components/PersonChips";
import { PersonActivities } from "../activities/person";
import { usePhotoStatus } from "../../hooks/usePhotoStatus";
import { getIdFromRoute } from "../../lib/routeHelpers";
import { timelineRequest } from "../../lib/photoPages";
import { summarizeDays } from "../../lib/daySummary";
import { chapterTitle, storyChapters } from "../../lib/story";
import { Metric, chartPoints, percentileBand } from "../../lib/ageChart";
import { latestOf, timeAgo } from "../../lib/checkup";
import { dueSummary } from "../../lib/familyStrip";
import { ageTitle, monthsOld } from "../../lib/sameAge";
import {
  ageInMonths,
  computePercentileLabel,
  formatAgeAtMeasurement,
  isValidBirthday,
} from "../../lib/growthPercentiles";
import { formatMeasurement } from "../../lib/weightFormat";
import { formatDate, formatLongDate } from "../../lib/dateUtils";
import { localDateString } from "../../lib/when";
import { copy } from "../../lib/copy";
import "./profile-styles";

type ProfileData = {
  person: server.GetPersonResponse;
  activities: server.GetPersonSeasonResponse;
  family: server.FamilyTimelineItem[];
  sameAge: server.GetSameAgeResponse | null;
  insights: server.GetPersonPhotoInsightsResponse;
};

const emptyInsights: server.GetPersonPhotoInsightsResponse = {
  growingUp: [],
  oftenWith: [],
  header: null,
};

const emptySeason: server.GetPersonSeasonResponse = {
  personId: 0,
  seasonId: 0,
  seasons: [],
  entries: [],
  appearances: [],
};

export async function fetch(route: string, prefix: string): Promise<rpc.Response<ProfileData>> {
  const personId = getIdFromRoute(route) || 0;
  const [[person, personErr], [activities], [family], [sameAge], [insights]] = await Promise.all([
    server.GetPerson({ id: personId }),
    server.GetPersonSeason({ personId, seasonId: 0 }),
    server.GetFamilyTimeline(timelineRequest({ skipMilestones: true, skipPhotos: true })),
    server.GetSameAge({
      ageMonths: null,
      fromPersonId: personId,
      today: localDateString(new Date()),
    }),
    server.GetPersonPhotoInsights({ personId }),
  ]);
  if (!person) return [null, personErr || "Failed to load person"];

  return rpc.ok<ProfileData>({
    person,
    activities: activities ?? { ...emptySeason, personId },
    family: family?.people ?? [],
    sameAge: sameAge?.fromPersonId === personId ? sameAge : null,
    insights: insights ?? emptyInsights,
  });
}

type Tab = "story" | "quotes" | "artwork" | "photos" | "growth" | "activities";
const TABS: Tab[] = ["story", "quotes", "artwork", "photos", "growth", "activities"];

type ProfileState = {
  tab: Tab;
  metric: Metric;
  showSiblings: boolean;
  zoom: ChartZoom;
};

const useProfileState = vlens.declareHook((personId: number): ProfileState => {
  const requested = new URLSearchParams(window.location.search).get("tab") as Tab | null;
  return {
    tab: requested && TABS.includes(requested) ? requested : "story",
    metric: "height",
    showSiblings: true,
    zoom: newChartZoom(),
  };
});

function chooseTab(state: ProfileState, personId: number, tab: Tab) {
  state.tab = tab;
  const query = tab === "story" ? "" : `?tab=${tab}`;
  window.history.replaceState(window.history.state, "", `/profile/${personId}${query}`);
  vlens.scheduleRedraw();
}

function chooseMetric(state: ProfileState, metric: Metric) {
  state.metric = metric;
  vlens.scheduleRedraw();
}

function toggleSiblings(state: ProfileState) {
  state.showSiblings = !state.showSiblings;
  vlens.scheduleRedraw();
}

export function view(route: string, prefix: string, data: ProfileData): preact.ComponentChild {
  if (!requireAuthInView()) return;

  return (
    <div>
      <Header isHome={false} />
      <main id="app" className="profile-container">
        <ProfilePage data={data} />
      </main>
      <Footer />
    </div>
  );
}

const ProfilePage = ({ data }: { data: ProfileData }) => {
  const person = data.person.person;
  const state = useProfileState(person.id);
  const today = localDateString(new Date());
  const hasBirthday = isValidBirthday(person.birthday) && !person.isPregnancy;
  const quotes = (data.person.milestones ?? []).filter(isQuote);
  const artwork = (data.person.milestones ?? []).filter(m => m.category === "artwork");
  const tabs = TABS.filter(
    t =>
      (t !== "quotes" || quotes.length > 0) &&
      (t !== "artwork" || artwork.length > 0) &&
      (!person.isPregnancy || t === "story" || t === "photos")
  );

  return (
    <div className="profile-page">
      <ProfileHeader person={person} fallback={data.insights.header} />

      {hasBirthday && (
        <Snapshot
          person={person}
          growth={data.person.growthData ?? []}
          sameAge={data.sameAge}
          activities={data.activities}
          today={today}
          onShowActivities={() => chooseTab(state, person.id, "activities")}
        />
      )}

      {hasBirthday && (
        <SameAgeStrip data={data.sameAge} today={today} exceptPersonId={person.id} hideWhenEmpty />
      )}

      <div className="profile-tabs" role="tablist" aria-label={person.name}>
        {tabs.map(tab => (
          <button
            key={tab}
            type="button"
            role="tab"
            aria-selected={state.tab === tab ? "true" : "false"}
            className={state.tab === tab ? "profile-tab active" : "profile-tab"}
            onClick={() => chooseTab(state, person.id, tab)}
          >
            {copy.person.tabs[tab]}
          </button>
        ))}
      </div>

      <div className="profile-tab-panel" role="tabpanel">
        {state.tab === "story" && <StoryTab data={data} today={today} />}
        {state.tab === "quotes" && <QuotesTab person={person} quotes={quotes} />}
        {state.tab === "artwork" && <ArtworkTab person={person} artwork={artwork} />}
        {state.tab === "photos" && (
          <PhotosTab person={person} photos={data.person.photos ?? []} insights={data.insights} />
        )}
        {state.tab === "growth" && hasBirthday && <GrowthTab data={data} state={state} />}
        {state.tab === "activities" && (
          <PersonActivities
            data={{ season: data.activities, people: data.family.map(item => item.person) }}
          />
        )}
      </div>
    </div>
  );
};

function ageLine(person: server.Person): string {
  if (person.isPregnancy) return dueSummary(person.birthday, new Date());
  if (!isValidBirthday(person.birthday)) return "";
  const months = monthsOld(person.birthday, localDateString(new Date()));
  const age = ageTitle(months >= 216 ? months - (months % 12) : months);
  return `${age} · ${copy.person.born(formatLongDate(person.birthday))}`;
}

const ProfileHeader = ({
  person,
  fallback,
}: {
  person: server.Person;
  fallback: server.PortraitPhoto | null;
}) => {
  const photoStatus = usePhotoStatus();
  return (
    <header className="profile-header">
      <span className="profile-avatar">
        {person.profilePhotoId ? (
          <ProfileImage
            photoId={person.profilePhotoId}
            alt={`${person.name}'s profile photo`}
            className="profile-photo"
            loading="eager"
            fetchpriority="high"
            status={photoStatus.getStatus(person.profilePhotoId)}
            cropX={person.profileCropX}
            cropY={person.profileCropY}
            cropScale={person.profileCropScale}
          />
        ) : fallback ? (
          <FaceCrop
            photoId={fallback.photoId}
            box={fallback.box}
            size={84}
            alt={`${person.name}, from a recent photo`}
            className="profile-face"
          />
        ) : (
          <span className="profile-initial">
            {person.isPregnancy ? "🤰" : initial(person.name)}
          </span>
        )}
      </span>
      <div className="profile-info">
        <h1>{person.name}</h1>
        <p className="profile-age">
          {person.relationship && (
            <span className="profile-relationship">{person.relationship} · </span>
          )}
          {ageLine(person)}
        </p>
      </div>
      <div className="profile-actions">
        {canContribute(person.familyId) && (
          <a href={`/edit-person/${person.id}`} className="btn btn-secondary">
            ✏️ Edit
          </a>
        )}
        <a href="/books" className="btn btn-secondary">
          📖 Books
        </a>
        {canContribute(person.familyId) && (
          <button
            type="button"
            className="btn btn-primary profile-add-button"
            onClick={openAddSheet}
          >
            + {copy.nav.add}
          </button>
        )}
      </div>
    </header>
  );
};

const Sparkline = ({ values }: { values: number[] }) => {
  if (values.length < 2) return null;
  const min = Math.min(...values);
  const max = Math.max(...values);
  const span = Math.max(max - min, 1e-6);
  const points = values
    .map((v, i) => `${(i / (values.length - 1)) * 80},${20 - ((v - min) / span) * 18 - 1}`)
    .join(" ");
  return (
    <svg className="sparkline" viewBox="0 0 80 20" aria-hidden="true">
      <polyline points={points} />
    </svg>
  );
};

function isActive(season: server.SeasonSummary, today: string): boolean {
  const start = season.startDate.slice(0, 10);
  const end = season.endDate.slice(0, 10);
  const open = !end || end.startsWith("0001");
  return start <= today && (open || end >= today);
}

const STALE_METRIC_MONTHS = 12;

const Snapshot = ({
  person,
  growth,
  sameAge,
  activities,
  today,
  onShowActivities,
}: {
  person: server.Person;
  growth: server.GrowthData[];
  sameAge: server.GetSameAgeResponse | null;
  activities: server.GetPersonSeasonResponse;
  today: string;
  onShowActivities: () => void;
}) => {
  const now = new Date();
  const metrics: { metric: Metric; type: server.MeasurementType; label: string }[] = [
    { metric: "height", type: server.Height, label: copy.measurement.height },
    { metric: "weight", type: server.Weight, label: copy.measurement.weight },
  ];
  const readings = metrics
    .map(m => ({ ...m, record: latestOf(growth, m.type) }))
    .filter(m => m.record);
  const lastMeasured = readings
    .map(m => m.record!.measurementDate)
    .sort()
    .pop();
  const latest = readings.filter(
    m => monthsOld(m.record!.measurementDate, lastMeasured!) <= STALE_METRIC_MONTHS
  );
  const sibling = (sameAge?.rows ?? []).find(r => r.person.id !== person.id && r.height);
  const seasons = (activities.seasons ?? []).filter(s => isActive(s, today));

  if (latest.length === 0 && seasons.length === 0) return null;

  return (
    <section className="profile-snapshot">
      {latest.map(({ metric, label, record }) => {
        const g = record!;
        const age = monthsOld(person.birthday, g.measurementDate);
        const pct =
          age <= 240
            ? computePercentileLabel(
                g.value,
                g.unit,
                ageInMonths(person.birthday, g.measurementDate),
                person.gender,
                metric
              )
            : null;
        return (
          <a key={metric} href={`/view-growth/${g.id}`} className="snapshot-row">
            <span className="snapshot-label">{label}</span>
            <strong>{formatMeasurement(g.value, g.unit, age)}</strong>
            {pct && <span className="snapshot-pct">{pct}</span>}
            <Sparkline values={chartPoints(growth, person.birthday, metric).map(p => p.value)} />
          </a>
        );
      })}
      {lastMeasured && (
        <p className="snapshot-note">
          {copy.person.measured(timeAgo(lastMeasured, now))}
          {sibling?.height &&
            ` · ${copy.person.siblingThen(
              sibling.person.name.split(" ")[0],
              formatMeasurement(
                sibling.height.value,
                sibling.height.unit,
                monthsOld(sibling.person.birthday, sibling.height.measurementDate)
              )
            )}`}
        </p>
      )}
      {seasons.map(season => {
        const entries = (activities.entries ?? []).filter(e => e.entry.seasonId === season.id);
        const next = (activities.appearances ?? [])
          .filter(a => a.entry.seasonId === season.id && a.event.startDate.slice(0, 10) >= today)
          .sort((a, b) => a.event.startDate.localeCompare(b.event.startDate))[0];
        return (
          <button
            key={season.id}
            type="button"
            className="snapshot-season"
            onClick={onShowActivities}
          >
            🏆 {season.name} · {copy.person.entries(entries.length)}
            {next && ` · ${copy.person.next(next.event.name)}`}
          </button>
        );
      })}
    </section>
  );
};

const StoryTab = ({ data, today }: { data: ProfileData; today: string }) => {
  const person = data.person.person;
  const days = summarizeDays(
    {
      photos: (data.person.photos ?? []).map(image => ({ image, people: [], similar: [] })),
      growth: data.person.growthData ?? [],
      milestones: data.person.milestones ?? [],
      appearances: (data.activities.appearances ?? []).map(detail => ({
        detail,
        personIds: [person.id],
      })),
    },
    [person],
    isValidBirthday(person.birthday) && !person.isPregnancy
      ? { from: person.birthday.slice(0, 10), to: today }
      : undefined
  );

  if (days.length === 0) {
    return <p className="profile-empty">{copy.person.nothingYet(person.name.split(" ")[0])}</p>;
  }

  const chapters = isValidBirthday(person.birthday)
    ? storyChapters(days, person.birthday)
    : [{ age: null, days, grew: "" }];

  return (
    <div className="story">
      {chapters.map(chapter => (
        <section key={String(chapter.age)} className="story-chapter">
          <div className="story-chapter-head">
            <h2>{chapterTitle(chapter.age)}</h2>
            {chapter.grew && <span className="story-grew">{chapter.grew}</span>}
          </div>
          <DaySummaryList
            days={chapter.days}
            people={[person]}
            today={today}
            subjectId={person.id}
          />
        </section>
      ))}
    </div>
  );
};

const QuotesTab = ({ person, quotes }: { person: server.Person; quotes: server.Milestone[] }) => {
  if (quotes.length === 0) {
    return <p className="profile-empty">{copy.person.noQuotes(firstName(person.name))}</p>;
  }
  const sorted = [...quotes].sort((a, b) => a.milestoneDate.localeCompare(b.milestoneDate));
  return (
    <div className="profile-quotes">
      {sorted.map(quote => {
        const age = isValidBirthday(person.birthday)
          ? monthsOld(person.birthday, quote.milestoneDate)
          : -1;
        return (
          <a key={quote.id} href={`/milestone/${quote.id}`} className="profile-quote">
            <blockquote>
              <MilestoneText milestone={quote} />
            </blockquote>
            {quote.context && <p className="profile-quote-context">{quote.context}</p>}
            <small className="profile-quote-when">
              {age >= 0 && `${formatAgeAtMeasurement(age)} · `}
              {formatDate(quote.milestoneDate)}
            </small>
          </a>
        );
      })}
    </div>
  );
};

const ArtworkTab = ({
  person,
  artwork,
}: {
  person: server.Person;
  artwork: server.Milestone[];
}) => {
  if (artwork.length === 0) {
    return <p className="profile-empty">{copy.person.noArtwork(firstName(person.name))}</p>;
  }
  const sorted = [...artwork].sort((a, b) => b.milestoneDate.localeCompare(a.milestoneDate));
  return (
    <div className="profile-artwork">
      {sorted.map(piece => {
        const age = isValidBirthday(person.birthday)
          ? monthsOld(person.birthday, piece.milestoneDate)
          : -1;
        const cover = piece.photoIds?.[0];
        const more = (piece.photoIds?.length ?? 0) - 1;
        return (
          <a key={piece.id} href={`/milestone/${piece.id}`} className="profile-artwork-piece">
            <div className="profile-artwork-frame">
              {cover ? (
                <ThumbnailImage photoId={cover} alt={piece.description} />
              ) : (
                <span aria-hidden="true">🎨</span>
              )}
              {more > 0 && <span className="profile-artwork-more">+{more}</span>}
            </div>
            <span className="profile-artwork-title">{piece.description}</span>
            <small className="profile-artwork-when">
              {age >= 0 ? formatAgeAtMeasurement(age) : formatDate(piece.milestoneDate)}
            </small>
          </a>
        );
      })}
    </div>
  );
};

const firstName = (name: string) => name.split(" ")[0];

function timelineLabel(portrait: server.PortraitPhoto): string {
  if (portrait.ageMonths < 0) return String(portrait.year);
  if (portrait.ageMonths < 24) return formatAgeAtMeasurement(portrait.ageMonths);
  return `${portrait.ageMonths / 12} yr`;
}

const PhotosTab = ({
  person,
  photos,
  insights,
}: {
  person: server.Person;
  photos: server.Image[];
  insights: server.GetPersonPhotoInsightsResponse;
}) => {
  const sorted = [...photos].sort((a, b) => b.photoDate.localeCompare(a.photoDate));
  if (sorted.length === 0) {
    return <p className="profile-empty">{copy.person.noPhotos}</p>;
  }
  return (
    <div className="profile-photos">
      {insights.growingUp.length > 1 && (
        <section className="profile-growing-up">
          <h3>{copy.person.growingUp}</h3>
          <div className="growing-up-strip">
            {insights.growingUp.map(portrait => (
              <a
                key={portrait.photoId}
                href={`/view-photo/${portrait.photoId}`}
                className="growing-up-item"
              >
                <FaceCrop photoId={portrait.photoId} box={portrait.box} size={88} alt="" />
                <span>{timelineLabel(portrait)}</span>
              </a>
            ))}
          </div>
        </section>
      )}
      {insights.oftenWith.length > 0 && (
        <section className="profile-often-with">
          <h3>{copy.person.oftenWith}</h3>
          <div className="often-with-chips">
            {insights.oftenWith.map(w => (
              <a
                key={w.person.id}
                className="often-with-chip"
                href={`/photos?q=${encodeURIComponent(`${firstName(person.name)} ${firstName(w.person.name)}`)}`}
              >
                {w.person.name}
                <span className="often-with-count">{w.count}</span>
              </a>
            ))}
          </div>
        </section>
      )}
      <a href={`/photos?people=${person.id}`} className="profile-photos-open">
        {copy.person.openInPhotos}
      </a>
      <div className="profile-photo-grid">
        {sorted.map(photo => (
          <a key={photo.id} href={`/view-photo/${photo.id}`} aria-label={photo.title}>
            <ThumbnailImage photoId={photo.id} alt={photo.title} />
          </a>
        ))}
      </div>
    </div>
  );
};

const CHILD_MONTHS = 18 * 12;

const GrowthTab = ({ data, state }: { data: ProfileData; state: ProfileState }) => {
  const person = data.person.person;
  const growth = data.person.growthData ?? [];
  const nowMonths = monthsOld(person.birthday, localDateString(new Date()));

  const siblings = data.family.filter(
    item =>
      item.person.id !== person.id &&
      !item.person.isPregnancy &&
      isValidBirthday(item.person.birthday) &&
      monthsOld(item.person.birthday, localDateString(new Date())) < CHILD_MONTHS
  );

  const main: AgeSeries = {
    key: person.id,
    label: person.name.split(" ")[0],
    color: SERIES_COLORS[0],
    points: chartPoints(growth, person.birthday, state.metric),
  };
  const others: AgeSeries[] = state.showSiblings
    ? siblings.map((item, i) => ({
        key: item.person.id,
        label: item.person.name.split(" ")[0],
        color: SERIES_COLORS[(i + 1) % SERIES_COLORS.length],
        points: chartPoints(item.growthData ?? [], item.person.birthday, state.metric),
        faint: true,
      }))
    : [];
  const maxAge = Math.max(nowMonths + 1, 12);
  for (const series of others) {
    series.points = series.points.filter(p => p.ageMonths <= maxAge);
  }
  const shown = others.filter(s => s.points.length > 0);
  const band = nowMonths <= 240 ? percentileBand(person.gender, state.metric, 0, maxAge) : [];

  const records = [...growth].sort((a, b) => b.measurementDate.localeCompare(a.measurementDate));

  return (
    <div className="profile-growth">
      <div className="profile-growth-controls">
        <SegmentedControl
          label={copy.person.metric}
          options={[
            { value: "height", label: copy.measurement.height },
            { value: "weight", label: copy.measurement.weight },
          ]}
          value={state.metric}
          onChange={vlens.cachePartial(chooseMetric, state)}
        />
        {siblings.length > 0 && (
          <label className="profile-growth-toggle">
            <input
              type="checkbox"
              checked={state.showSiblings}
              onChange={vlens.cachePartial(toggleSiblings, state)}
            />
            {copy.person.showSiblings}
          </label>
        )}
        {canContribute(person.familyId) && (
          <a href={`/add-growth/${person.id}`} className="btn btn-primary">
            {copy.person.measure}
          </a>
        )}
      </div>

      {main.points.length === 0 ? (
        <p className="profile-empty">{copy.person.noMeasurements}</p>
      ) : (
        <AgeChart
          series={[main, ...shown]}
          metric={state.metric}
          band={band}
          label={`${person.name} ${state.metric}`}
          zoom={state.zoom}
        />
      )}

      {records.length > 0 && (
        <ul className="profile-measurements">
          {records.map(g => {
            const age = monthsOld(person.birthday, g.measurementDate);
            const metric: Metric = g.measurementType === server.Height ? "height" : "weight";
            const pct =
              age >= 0 && age <= 240
                ? computePercentileLabel(
                    g.value,
                    g.unit,
                    ageInMonths(person.birthday, g.measurementDate),
                    person.gender,
                    metric
                  )
                : null;
            return (
              <li key={g.id}>
                <a href={`/view-growth/${g.id}`}>
                  <span className="measurement-date">{formatDate(g.measurementDate)}</span>
                  <span className="measurement-kind">
                    {metric === "height" ? copy.measurement.height : copy.measurement.weight}
                  </span>
                  <strong>{formatMeasurement(g.value, g.unit, age)}</strong>
                  {pct && <span className="snapshot-pct">{pct}</span>}
                </a>
              </li>
            );
          })}
        </ul>
      )}
    </div>
  );
};
