import * as preact from "preact";
import * as rpc from "vlens/rpc";
import * as auth from "./authCache";
import * as core from "vlens/core";
import { Header, Footer } from "./layout";

type Data = {};

export async function fetch(route: string, prefix: string) {
  // Check authentication on server fetch
  const currentAuth = auth.getAuth();
  if (!currentAuth || currentAuth.id <= 0) {
    core.setRoute('/login');
    return rpc.ok<Data>({});
  }
  return rpc.ok<Data>({});
}

export function view(
  route: string,
  prefix: string,
  data: Data,
): preact.ComponentChild {
  // Double-check authentication in view
  const currentAuth = auth.getAuth();
  if (!currentAuth || currentAuth.id <= 0) {
    core.setRoute('/login');
    return null;
  }

  return (
    <div>
      <Header isHome={false} />
      <main id="app" className="dashboard-container">
        <DashboardPage user={currentAuth} />
      </main>
      <Footer />
    </div>
  );
}

interface DashboardPageProps {
  user: auth.AuthCache;
}

const DashboardPage = ({ user }: DashboardPageProps) => (
  <div className="dashboard-page">
    <div className="dashboard-header">
      <h1>Welcome back, {user.name}!</h1>
      <p>Your family dashboard</p>
    </div>

    <div className="dashboard-grid">
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
        <div className="card-icon">💬</div>
        <h3>Messages</h3>
        <p>Chat with family members</p>
        <button className="btn btn-primary">Open Chat</button>
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