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

        {/* Simple Navigation Sidebar */}
        <div className="quick-actions">
          <h3>Quick Actions</h3>
          <div className="action-links">
            <div className="action-group">
              <h4>Family</h4>
              <a href="/add-person" className="action-link">
                ➕ Add Family Member
              </a>
              <a href="/compare" className="action-link">
                📊 Compare People
              </a>
            </div>

            <div className="action-group">
              <h4>Photos & Memories</h4>
              <a href="/photos" className="action-link">
                📸 View Photos
              </a>
              <a href="/add-photo" className="action-link">
                📷 Add Photo
              </a>
            </div>

            <div className="action-group">
              <h4>Growth & Milestones</h4>
              <a href="/add-growth" className="action-link">
                📏 Track Growth
              </a>
              <a href="/add-milestone" className="action-link">
                ⭐ Add Milestone
              </a>
            </div>

            <div className="action-group">
              <h4>More</h4>
              <a href="/chat" className="action-link">
                💬 Family Chat
              </a>
              <a href="/settings" className="action-link">
                📥📤 Import/Export Data
              </a>
              <a href="/settings" className="action-link">
                ⚙️ Settings
              </a>
            </div>
          </div>
        </div>
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
      </div>
    </a>
  );
};
