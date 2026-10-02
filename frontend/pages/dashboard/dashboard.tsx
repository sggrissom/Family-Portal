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
import { localDateString } from "../../lib/when";
import { copy } from "../../lib/copy";
import "./dashboard-styles";

export async function fetch(route: string, prefix: string) {
  if (!(await ensureAuthInFetch())) {
    return rpc.ok<server.GetDashboardResponse | null>(null);
  }
  return server.GetDashboard({ today: localDateString(new Date()) });
}

export function view(
  route: string,
  prefix: string,
  data: server.GetDashboardResponse | null
): preact.ComponentChild {
  const currentAuth = requireAuthInView();
  if (!currentAuth || !data) {
    return;
  }

  return (
    <div>
      <Header isHome={false} />
      <main id="app" className="home-container">
        <HomePage user={currentAuth} data={data} />
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
    default:
      return `/profile/${nudge.personId}`;
  }
}

const KIND_ICONS: Record<string, string> = { dance: "🩰", sport: "🏅" };

const HomePage = ({ user, data }: { user: auth.AuthCache; data: server.GetDashboardResponse }) => {
  const people = data.people || [];
  const familyId = user.familyId;
  const strip = [
    ...people.filter(p => p.isPregnancy),
    ...chipOrder(people, data.relations || [], familyId),
  ];
  const names = chipLabels(people);
  const dismissed = readDismissed();
  const nudges = (data.nudges || []).filter(n => !dismissed.includes(n.key)).slice(0, MAX_NUDGES);
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
      <h1 className="home-greeting">Welcome back, {user.name}!</h1>

      <FamilyStrip people={strip} names={names} />

      {nudges.length > 0 && (
        <div className="home-nudges">
          {nudges.map(nudge => (
            <div key={nudge.key} className="home-nudge">
              <a href={nudgeAction(nudge)}>{nudge.text}</a>
              <button
                type="button"
                className="home-nudge-dismiss"
                aria-label={copy.home.dismiss}
                onClick={vlens.cachePartial(dismissNudge, nudge.key)}
              >
                ×
              </button>
            </div>
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
                  <span aria-hidden="true">{KIND_ICONS[s.season.kind] ?? "🏆"}</span>{" "}
                  {s.activityName} · {s.season.name}
                </a>
                {s.event && (
                  <>
                    <a href={`/competition/${s.event.id}`} className="home-season-event">
                      {copy.home.eventTiming[s.eventTiming] ?? ""}: {s.event.name} ·{" "}
                      {dayLabel(dayKey(s.event.startDate), data.today)}
                    </a>
                    {s.canContribute && (
                      <div className="home-season-actions">
                        <a href={`/competition/${s.event.id}`} className="btn btn-secondary">
                          {copy.home.addPhotos}
                        </a>
                        {s.canAddResults && (
                          <a href={`/competition/${s.event.id}`} className="btn btn-primary">
                            {copy.home.addResults}
                          </a>
                        )}
                      </div>
                    )}
                  </>
                )}
              </div>
            ))}
          </div>
        </section>
      )}

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
                {(year.milestones || []).map(m => (
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
        {recent.length === 0 ? (
          <p className="home-empty">{copy.home.nothingRecent}</p>
        ) : (
          <DaySummaryList days={recent} people={people} today={data.today} />
        )}
      </section>
    </div>
  );
};

const FamilyStrip = ({
  people,
  names,
}: {
  people: server.Person[];
  names: Map<number, string>;
}) => {
  const photoStatus = usePhotoStatus();
  const today = new Date();

  return (
    <nav className="family-strip" aria-label="Family">
      {people.map((person, index) => {
        const age = person.isPregnancy
          ? dueSummary(person.birthday, today)
          : compactAge(person.birthday, today);
        return (
          <a
            key={person.id}
            href={`/profile/${person.id}`}
            aria-label={age ? `${person.name}, ${age}` : person.name}
            className={person.isPregnancy ? "family-strip-person pregnancy" : "family-strip-person"}
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
                "🤰"
              ) : (
                initial(person.name)
              )}
            </span>
            <span className="family-strip-name">{names.get(person.id) ?? person.name}</span>
            <span className="family-strip-age">{age}</span>
          </a>
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
  );
};
