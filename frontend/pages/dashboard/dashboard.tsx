import * as preact from "preact";
import * as rpc from "vlens/rpc";
import * as auth from "../../lib/authCache";
import * as core from "vlens/core";
import * as server from "../../server";
import { Header, Footer } from "../../layout";
import { ensureAuthInFetch, requireAuthInView } from "../../lib/authHelpers";
import { ProfileImage } from "../../components/ResponsiveImage";
import { usePhotoStatus } from "../../hooks/usePhotoStatus";
import "./dashboard-styles";

export async function fetch(route: string, prefix: string) {
  if (!(await ensureAuthInFetch())) {
    return rpc.ok<server.ListPeopleResponse>({ people: [] });
  }

  return server.ListPeople({});
}

export function view(
  route: string,
  prefix: string,
  data: server.ListPeopleResponse
): preact.ComponentChild {
  const currentAuth = requireAuthInView();
  if (!currentAuth) {
    return;
  }

  return (
    <div>
      <Header isHome={false} />
      <main id="app" className="dashboard-container">
        <DashboardPage user={currentAuth} data={data} />
      </main>
      <Footer />
    </div>
  );
}

interface DashboardPageProps {
  user: auth.AuthCache;
  data: server.ListPeopleResponse;
}

const DashboardPage = ({ user, data }: DashboardPageProps) => {
  // Ensure people is always an array
  const people = data.people || [];
  const parents = people.filter(p => p.type === 0);
  const children = people.filter(p => p.type === 1);

  return (
    <div className="dashboard-page">
      <div className="dashboard-header">
        <h1>Welcome back, {user.name}!</h1>
        <p>Your family dashboard</p>
      </div>

      <div className="dashboard-content">
        {/* Main Family Members Section */}
        <div className="family-section">
          <div className="section-header">
            <h2>Your Family</h2>
          </div>

          {people.length === 0 ? (
            <div className="empty-state">
              <p>No family members added yet.</p>
              <a href="/add-person" className="btn btn-primary">
                Add Your First Family Member
              </a>
            </div>
          ) : (
            <div className="people-groups">
              {parents.length > 0 && (
                <div className="people-group">
                  <h3>Parents</h3>
                  <div className="people-grid">
                    {parents.map((person, index) => (
                      <PersonCard key={person.id} person={person} index={index} />
                    ))}
                  </div>
                </div>
              )}

              {children.length > 0 && (
                <div className="people-group">
                  <h3>Children</h3>
                  <div className="people-grid">
                    {children.map((person, index) => (
                      <PersonCard key={person.id} person={person} index={index + parents.length} />
                    ))}
                  </div>
                </div>
              )}
            </div>
          )}
        </div>

        <section className="quick-actions" aria-labelledby="quick-actions-title">
          <div className="quick-actions-heading">
            <div>
              <span className="section-kicker">Capture today</span>
              <h2 id="quick-actions-title">What would you like to add?</h2>
            </div>
            <p>Your most common family updates, one tap away.</p>
          </div>
          <div className="action-links">
            <a href="/add-milestone" className="action-card action-card-featured">
              <span className="action-icon" aria-hidden="true">
                ⭐
              </span>
              <span className="action-copy">
                <strong>Add a milestone</strong>
                <small>Celebrate a first, achievement, or favorite memory</small>
              </span>
              <span className="action-arrow" aria-hidden="true">
                →
              </span>
            </a>
            <a href="/add-growth" className="action-card">
              <span className="action-icon" aria-hidden="true">
                📏
              </span>
              <span className="action-copy">
                <strong>Record growth</strong>
                <small>Add a new height or weight measurement</small>
              </span>
              <span className="action-arrow" aria-hidden="true">
                →
              </span>
            </a>
            <a href="/add-photo" className="action-card">
              <span className="action-icon" aria-hidden="true">
                📷
              </span>
              <span className="action-copy">
                <strong>Share a photo</strong>
                <small>Keep a new family moment in one place</small>
              </span>
              <span className="action-arrow" aria-hidden="true">
                →
              </span>
            </a>
          </div>
          <div className="secondary-actions" aria-label="More family actions">
            <a href="/add-person">＋ Add family member</a>
            <a href="/family-timeline">View family timeline</a>
            <a href="/compare">Compare growth</a>
          </div>
        </section>
      </div>
    </div>
  );
};

interface PersonCardProps {
  person: server.Person;
  index?: number;
}

const PersonCard = ({ person, index = 999 }: PersonCardProps) => {
  const photoStatus = usePhotoStatus();

  const getDueDateSummary = (birthday: string, isPregnancy: boolean): string | null => {
    const dueDate = new Date(birthday);
    if (!isPregnancy || isNaN(dueDate.getTime())) return null;

    const now = new Date();
    const dueDateUtc = Date.UTC(
      dueDate.getUTCFullYear(),
      dueDate.getUTCMonth(),
      dueDate.getUTCDate()
    );
    const nowUtc = Date.UTC(now.getUTCFullYear(), now.getUTCMonth(), now.getUTCDate());
    const msPerDay = 24 * 60 * 60 * 1000;

    if (dueDateUtc < nowUtc) {
      const daysPastDue = Math.ceil((nowUtc - dueDateUtc) / msPerDay);
      return `Due date passed ${daysPastDue} day${daysPastDue === 1 ? "" : "s"} ago`;
    }

    if (dueDateUtc === nowUtc) {
      return "Due today";
    }

    const daysUntilDue = Math.ceil((dueDateUtc - nowUtc) / msPerDay);
    const weeksUntilDue = Math.floor(daysUntilDue / 7);
    const extraDays = daysUntilDue % 7;

    if (weeksUntilDue > 0) {
      return `Baby due in ${weeksUntilDue}w ${extraDays}d`;
    }

    return `Baby due in ${daysUntilDue} day${daysUntilDue === 1 ? "" : "s"}`;
  };

  const getTrimester = (birthday: string, isPregnancy: boolean): string | null => {
    const dueDate = new Date(birthday);
    if (!isPregnancy || isNaN(dueDate.getTime())) return null;

    const now = new Date();
    const dueDateUtc = Date.UTC(
      dueDate.getUTCFullYear(),
      dueDate.getUTCMonth(),
      dueDate.getUTCDate()
    );
    const nowUtc = Date.UTC(now.getUTCFullYear(), now.getUTCMonth(), now.getUTCDate());
    if (dueDateUtc <= nowUtc) return null;

    const daysUntilDue = Math.ceil((dueDateUtc - nowUtc) / (24 * 60 * 60 * 1000));
    const gestationalWeeks = 40 - Math.ceil(daysUntilDue / 7);

    if (gestationalWeeks <= 12) return "1st trimester";
    if (gestationalWeeks <= 26) return "2nd trimester";
    return "3rd trimester";
  };

  const dueDateSummary = getDueDateSummary(person.birthday, person.isPregnancy);
  const trimester = getTrimester(person.birthday, person.isPregnancy);
  const getGenderIcon = (gender: number) => {
    switch (gender) {
      case 0:
        return "👨"; // Male
      case 1:
        return "👩"; // Female
      default:
        return "👤"; // Unknown
    }
  };

  const getTypeLabel = (type: number) => {
    return type === 0 ? "Parent" : "Child";
  };

  return (
    <a href={`/profile/${person.id}`} className="person-card clickable">
      <div className="person-avatar">
        {person.profilePhotoId ? (
          <ProfileImage
            photoId={person.profilePhotoId}
            alt={`${person.name}'s profile photo`}
            className="person-photo"
            loading={index < 3 ? "eager" : "lazy"}
            fetchpriority={index < 3 ? "high" : "auto"}
            status={photoStatus.getStatus(person.profilePhotoId)}
            cropX={person.profileCropX}
            cropY={person.profileCropY}
            cropScale={person.profileCropScale}
          />
        ) : (
          <span className="person-icon">{getGenderIcon(person.gender)}</span>
        )}
      </div>
      <div className="person-info">
        <h4>{person.name}</h4>
        <p className="person-details">
          {getTypeLabel(person.type)} • Age {person.age}
        </p>
        {dueDateSummary && <p className="person-due-date">{dueDateSummary}</p>}
        {trimester && <p className="person-trimester">{trimester}</p>}
        {person.isPregnancy && <p className="person-born-action">Edit to mark as born</p>}
      </div>
    </a>
  );
};
