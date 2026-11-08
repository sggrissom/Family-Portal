import { Baby, Cake, ChevronLeft, ChevronRight, Mic } from "lucide-preact";
import { Icon } from "../../components/Icon";
import * as preact from "preact";
import * as vlens from "vlens";
import * as rpc from "vlens/rpc";
import * as auth from "../../lib/authCache";
import * as server from "../../server";
import { Header, Footer } from "../../layout";
import { ensureAuthInFetch, requireAuthInView } from "../../lib/authHelpers";
import { ProfileImage } from "../../components/ResponsiveImage";
import { DaySummaryList } from "../../components/DaySummaryList";
import { initial } from "../../components/PersonChips";
import { usePhotoStatus } from "../../hooks/usePhotoStatus";
import { chipLabels, chipOrder } from "../../lib/familyGroups";
import { compactAge, dueSummary } from "../../lib/familyStrip";
import { dayKey, dayLabel, summarizeDays } from "../../lib/daySummary";
import { getCategoryIcon } from "../../lib/milestoneHelpers";
import { MilestoneText } from "../../components/MilestoneText";
import { InterviewCards } from "../../components/InterviewCard";
import { activityKindIcon } from "../activities/labels";
import { groupInterviewAnswers, interviewsById } from "../../lib/interviews";
import { localDateString } from "../../lib/dateUtils";
import { copy } from "../../lib/copy";
import { FeaturedMemory, MemoryTiles } from "../../components/MemoryCard";
import { currentVisit, markSeen, saveVisit } from "../../lib/memories";
import "./dashboard-styles";

type HomeData = {
  dashboard: server.GetDashboardResponse;
  memories: server.GetMemoriesResponse;
};

const FEATURED_MEMORIES = 12;

export async function fetch(route: string, prefix: string): Promise<rpc.Response<HomeData | null>> {
  if (!(await ensureAuthInFetch())) {
    return rpc.ok<HomeData | null>(null);
  }
  const today = localDateString(new Date());
  const visit = currentVisit();
  const [[dashboard, err], [memories]] = await Promise.all([
    server.GetDashboard({ today }),
    server.GetMemories({
      today,
      seed: visit.seed,
      seen: visit.seen,
      limit: FEATURED_MEMORIES,
      kinds: [],
      personId: 0,
      recentlyAdded: true,
    }),
  ]);
  if (!dashboard) return [null, err];
  return rpc.ok<HomeData | null>({
    dashboard,
    memories: memories ?? { memories: [], recentlyAdded: [] },
  });
}

export function view(route: string, prefix: string, data: HomeData | null): preact.ComponentChild {
  const currentAuth = requireAuthInView();
  if (!currentAuth || !data) {
    return;
  }

  return (
    <div>
      <Header isHome={false} />
      <main id="app" className="home-container">
        <HomePage user={currentAuth} data={data.dashboard} memories={data.memories} />
      </main>
      <Footer />
    </div>
  );
}

const DISMISSED_KEY = "dismissed-nudges";
const MAX_NUDGES = 2;

function readDismissed(): string[] {
  try {
    const stored = JSON.parse(localStorage.getItem(DISMISSED_KEY) ?? "[]");
    return Array.isArray(stored) ? stored : [];
  } catch {
    return [];
  }
}

function dismissNudge(key: string) {
  const dismissed = readDismissed().filter(k => k !== key);
  dismissed.push(key);
  try {
    localStorage.setItem(DISMISSED_KEY, JSON.stringify(dismissed.slice(-100)));
  } catch {}
  vlens.scheduleRedraw();
}

function nudgeAction(nudge: server.DashboardNudge): string {
  switch (nudge.kind) {
    case "faces":
      return "/faces";
    case "measure":
      return `/add-growth/${nudge.personId}`;
    case "interview":
      return `/interview/new/${nudge.personId}`;
    case "interview-continue":
      return `/interview/${nudge.interviewId}/ask`;
    default:
      return `/profile/${nudge.personId}`;
  }
}

const TIMELY_NUDGES = ["birthday"];

function anotherMemory(count: number) {
  const visit = currentVisit();
  saveVisit({ ...visit, index: (visit.index + 1) % count });
  vlens.scheduleRedraw();
}

function featuredMemory(memories: server.Memory[]): server.Memory | null {
  return memories.length > 0 ? memories[currentVisit().index % memories.length] : null;
}

const Featured = ({
  memory,
  count,
  people,
}: {
  memory: server.Memory | null;
  count: number;
  people: Map<number, server.Person>;
}) => {
  if (!memory) return null;
  markSeen(memory.key);
  return (
    <section className="home-section" aria-label={copy.memory.featured}>
      <FeaturedMemory
        memory={memory}
        people={people}
        onAnother={count > 1 ? vlens.cachePartial(anotherMemory, count) : undefined}
      />
    </section>
  );
};

const Nudge = ({ nudge }: { nudge: server.DashboardNudge }) => (
  <div className="home-nudge">
    <a href={nudgeAction(nudge)}>
      {nudge.kind === "birthday" ? (
        <>
          <Icon symbol={Cake} /> {nudge.text.replace(/^🎂\s*/, "")}
        </>
      ) : nudge.kind === "interview" ? (
        <>
          <Icon symbol={Mic} /> {nudge.text.replace(/^🎤\s*/, "")}
        </>
      ) : (
        nudge.text
      )}
    </a>
    <button
      type="button"
      className="home-nudge-dismiss"
      aria-label={copy.home.dismiss}
      onClick={vlens.cachePartial(dismissNudge, nudge.key)}
    >
      ×
    </button>
  </div>
);

const HomePage = ({
  user,
  data,
  memories,
}: {
  user: auth.AuthCache;
  data: server.GetDashboardResponse;
  memories: server.GetMemoriesResponse;
}) => {
  const people = data.people || [];
  const familyId = user.familyId;
  const strip = chipOrder(people, data.relations || [], familyId, { includeUnborn: true });
  const names = chipLabels(people);
  const peopleById = new Map(people.map(p => [p.id, p]));
  const dismissed = readDismissed();
  const nudges = (data.nudges || []).filter(n => !dismissed.includes(n.key));
  const timely = nudges.filter(n => TIMELY_NUDGES.includes(n.kind));
  const featured = featuredMemory(memories.memories || []);
  const added = (memories.recentlyAdded || []).filter(m => m.key !== featured?.key);
  const chores = nudges.filter(n => !TIMELY_NUDGES.includes(n.kind)).slice(0, MAX_NUDGES);
  const recent = summarizeDays(
    {
      photos: data.recent.photos || [],
      growth: data.recent.growth || [],
      milestones: data.recent.milestones || [],
    },
    people,
    { from: data.recent.from, to: data.today }
  );

  return (
    <div className="home-page">
      <h1 className="home-greeting">
        {auth.isNewAccount() ? "Welcome" : "Welcome back"}, {user.name}!
      </h1>

      <FamilyStrip people={strip} names={names} families={data.families ?? []} />

      {timely.length > 0 && (
        <div className="home-nudges">
          {timely.map(nudge => (
            <Nudge key={nudge.key} nudge={nudge} />
          ))}
        </div>
      )}

      {(data.seasons || []).length > 0 && (
        <section className="home-section">
          <h2>{copy.home.inSeason}</h2>
          <div className="home-seasons">
            {data.seasons.map(s => (
              <div key={s.season.id} className="home-season">
                <a href={`/season/${s.season.id}`} className="home-season-name">
                  <span aria-hidden="true">
                    <Icon symbol={activityKindIcon(s.season.kind)} />
                  </span>{" "}
                  {s.activityName} · {s.season.name}
                </a>
                {s.event && (
                  <>
                    <a href={`/competition/${s.event.id}`} className="home-season-event">
                      {copy.home.eventTiming[s.eventTiming] ?? ""}: {s.event.name} ·{" "}
                      {dayLabel(dayKey(s.event.startDate), data.today)}
                    </a>
                    {s.canContribute && s.canAddResults ? (
                      <div className="home-season-actions">
                        <a href={`/competition/${s.event.id}`} className="btn btn-secondary">
                          {copy.home.addPhotos}
                        </a>
                        <a href={`/competition/${s.event.id}`} className="btn btn-primary">
                          {copy.home.addResults}
                        </a>
                      </div>
                    ) : (
                      <div className="home-season-actions">
                        <a href={`/competition/${s.event.id}`} className="btn btn-secondary">
                          {copy.home.viewEvent}
                        </a>
                      </div>
                    )}
                  </>
                )}
              </div>
            ))}
          </div>
        </section>
      )}

      <Featured memory={featured} count={(memories.memories || []).length} people={peopleById} />

      {(data.onThisDay || []).length > 0 && (
        <section className="home-section">
          <h2>{copy.home.onThisDay}</h2>
          <div className="home-on-this-day">
            {data.onThisDay.map(year => (
              <div key={year.yearsAgo} className="home-year">
                <h3>{copy.home.yearsAgo(year.yearsAgo)}</h3>
                {(year.photos || []).length > 0 && (
                  <div className="home-year-photos">
                    {year.photos.map(photo => (
                      <a key={photo.id} href={`/view-photo/${photo.id}`} aria-label="View photo">
                        <img src={`/api/photo/${photo.id}/thumb`} alt="" loading="lazy" />
                      </a>
                    ))}
                  </div>
                )}
                <InterviewCards
                  groups={groupInterviewAnswers(year.milestones || []).groups}
                  people={peopleById}
                  interviews={interviewsById(year.interviews)}
                />
                {groupInterviewAnswers(year.milestones || []).loose.map(m => (
                  <a key={m.id} href={`/milestone/${m.id}`} className="day-milestone">
                    <span className="day-milestone-icon" aria-hidden="true">
                      {getCategoryIcon(m.category)}
                    </span>
                    <span>
                      <strong>{names.get(m.personId) ?? ""}</strong> <MilestoneText milestone={m} />
                    </span>
                  </a>
                ))}
              </div>
            ))}
          </div>
        </section>
      )}

      <section className="home-section">
        <div className="home-section-head">
          <h2>{copy.home.recent}</h2>
          <a href="/history">{copy.home.seeAll}</a>
        </div>
        {recent.length > 0 && (
          <DaySummaryList
            days={recent}
            people={people}
            today={data.today}
            interviews={interviewsById(data.recent.interviews)}
          />
        )}
        {recent.length === 0 && added.length === 0 && (
          <p className="home-empty">{copy.home.nothingRecent}</p>
        )}
        {added.length > 0 && (
          <div className="home-added">
            <h3>{copy.memory.addedFromEarlier}</h3>
            <MemoryTiles memories={added} people={peopleById} />
          </div>
        )}
      </section>

      {chores.length > 0 && (
        <section className="home-section home-chores">
          <h2>{copy.home.whenYouHaveAMinute}</h2>
          <div className="home-nudges">
            {chores.map(nudge => (
              <Nudge key={nudge.key} nudge={nudge} />
            ))}
          </div>
        </section>
      )}
    </div>
  );
};

const FamilyStrip = ({
  people,
  names,
  families,
}: {
  people: server.Person[];
  names: Map<number, string>;
  families: server.FamilyLabel[];
}) => {
  const photoStatus = usePhotoStatus();
  const today = new Date();
  const labelFamilies = new Set(people.map(p => p.familyId)).size > 1;
  const familyName = (id: number) => families.find(f => f.id === id)?.name || "Family";

  return (
    <div className="family-strip-wrap">
      <nav className="family-strip" aria-label="Family" ref={watchStrip} onScroll={stripScrolled}>
        {people.map((person, index) => {
          const age = person.isPregnancy
            ? dueSummary(person.birthday, today)
            : compactAge(person.birthday, today);
          const startsFamily =
            labelFamilies && (index === 0 || people[index - 1].familyId !== person.familyId);
          return (
            <preact.Fragment key={person.id}>
              {startsFamily && (
                <span className="family-strip-family">{familyName(person.familyId)}</span>
              )}
              <a
                href={`/profile/${person.id}`}
                aria-label={age ? `${person.name}, ${age}` : person.name}
                className={
                  person.isPregnancy ? "family-strip-person pregnancy" : "family-strip-person"
                }
              >
                <span className="family-strip-avatar" aria-hidden="true">
                  {person.profilePhotoId ? (
                    <ProfileImage
                      photoId={person.profilePhotoId}
                      alt=""
                      loading={index < 6 ? "eager" : "lazy"}
                      status={photoStatus.getStatus(person.profilePhotoId)}
                      cropX={person.profileCropX}
                      cropY={person.profileCropY}
                      cropScale={person.profileCropScale}
                    />
                  ) : person.isPregnancy ? (
                    <Icon symbol={Baby} />
                  ) : (
                    initial(person.name)
                  )}
                </span>
                <span className="family-strip-name">{names.get(person.id) ?? person.name}</span>
                <span className="family-strip-age">{age}</span>
              </a>
            </preact.Fragment>
          );
        })}
        {auth.canContributeAnywhere() && (
          <a href="/add-person" className="family-strip-person family-strip-add">
            <span className="family-strip-avatar" aria-hidden="true">
              +
            </span>
            <span className="family-strip-name">{copy.home.addPerson}</span>
          </a>
        )}
      </nav>
      <button
        type="button"
        className="family-strip-scroll prev"
        aria-label={copy.home.scrollFamilyBack}
        tabIndex={-1}
        onClick={vlens.cachePartial(scrollStrip, -1)}
      >
        <Icon symbol={ChevronLeft} />
      </button>
      <button
        type="button"
        className="family-strip-scroll next"
        aria-label={copy.home.scrollFamilyOn}
        tabIndex={-1}
        onClick={vlens.cachePartial(scrollStrip, 1)}
      >
        <Icon symbol={ChevronRight} />
      </button>
    </div>
  );
};

function markStripEdges(strip: HTMLElement) {
  const wrap = strip.parentElement;
  if (!wrap) return;
  const end = strip.scrollWidth - strip.clientWidth;
  wrap.classList.toggle("more-before", strip.scrollLeft > 4);
  wrap.classList.toggle("more-after", strip.scrollLeft < end - 4);
}

let stripResizeWatched = false;

function watchStrip(strip: HTMLElement | null) {
  if (!strip) return;
  requestAnimationFrame(() => markStripEdges(strip));
  if (stripResizeWatched) return;
  stripResizeWatched = true;
  window.addEventListener("resize", () => {
    const current = document.querySelector<HTMLElement>(".family-strip");
    if (current) markStripEdges(current);
  });
}

function stripScrolled(event: Event) {
  markStripEdges(event.currentTarget as HTMLElement);
}

function scrollStrip(direction: number) {
  const strip = document.querySelector<HTMLElement>(".family-strip");
  strip?.scrollBy({ left: direction * strip.clientWidth * 0.8, behavior: "smooth" });
}
