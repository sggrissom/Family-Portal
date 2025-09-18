import * as preact from "preact";
import * as rpc from "vlens/rpc";
import * as auth from "./authCache";
import * as core from "vlens/core";
import * as server from "./server";
import { Header, Footer } from "./layout";

export async function fetch(route: string, prefix: string) {
  return server.ListPeople({})
}

export function view(
  route: string,
  prefix: string,
  data: server.ListPeopleResponse,
): preact.ComponentChild {
  const currentAuth = auth.getAuth();
  if (!currentAuth || currentAuth.id <= 0) {
    auth.clearAuth();
    core.setRoute('/login');
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

      {/* Family Members Section */}
      <div className="family-section">
        <div className="section-header">
          <h2>Family Members</h2>
          <a href="/add-person" className="btn btn-primary">Add Family Member</a>
        </div>

        {people.length === 0 ? (
          <div className="empty-state">
            <p>No family members added yet.</p>
            <a href="/add-person" className="btn btn-primary">Add Your First Family Member</a>
          </div>
        ) : (
          <div className="people-grid">
            {parents.length > 0 && (
              <div className="people-group">
                <h3>Parents</h3>
                <div className="people-list">
                  {parents.map(person => (
                    <PersonCard key={person.id} person={person} />
                  ))}
                </div>
              </div>
            )}

            {children.length > 0 && (
              <div className="people-group">
                <h3>Children</h3>
                <div className="people-list">
                  {children.map(person => (
                    <PersonCard key={person.id} person={person} />
                  ))}
                </div>
              </div>
            )}
          </div>
        )}
      </div>

      {/* Quick Actions */}
      <div className="dashboard-grid">
        <div className="dashboard-card">
          <div className="card-icon">📏</div>
          <h3>Growth Tracking</h3>
          <p>Track height and weight measurements</p>
          <a href="/add-growth" className="btn btn-primary">Add Measurement</a>
        </div>

        <div className="dashboard-card">
          <div className="card-icon">📸</div>
          <h3>Photo Albums</h3>
          <p>Share and organize family photos</p>
          <button className="btn btn-primary">View Photos</button>
        </div>

        <div className="dashboard-card">
          <div className="card-icon">📅</div>
          <h3>Family Calendar</h3>
          <p>Keep track of events and schedules</p>
          <button className="btn btn-primary">View Calendar</button>
        </div>

        <div className="dashboard-card">
          <div className="card-icon">⚙️</div>
          <h3>Settings</h3>
          <p>Manage your family portal</p>
          <button className="btn btn-secondary">Settings</button>
        </div>
      </div>
    </div>
  );
};

interface PersonCardProps {
  person: server.Person;
}

const PersonCard = ({ person }: PersonCardProps) => {
  const getGenderIcon = (gender: number) => {
    switch (gender) {
      case 0: return "👨"; // Male
      case 1: return "👩"; // Female
      default: return "👤"; // Unknown
    }
  };

  const getTypeLabel = (type: number) => {
    return type === 0 ? "Parent" : "Child";
  };

  return (
    <a href={`/profile/${person.id}`} className="person-card clickable">
      <div className="person-avatar">
        {getGenderIcon(person.gender)}
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
