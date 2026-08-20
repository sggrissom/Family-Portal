import * as preact from "preact";
import * as rpc from "vlens/rpc";
import * as auth from "../../lib/authCache";
import * as core from "vlens/core";
import * as server from "../../server";
import { Header, Footer } from "../../layout";
import { ensureAuthInFetch, requireAuthInView } from "../../lib/authHelpers";
import "./admin-styles";

type Data = {
  diagnostics: server.DiagnosticsResponse | null;
};

export async function fetch(route: string, prefix: string) {
  if (!(await ensureAuthInFetch())) {
    return rpc.ok<Data>({ diagnostics: null });
  }

  // A failure here must not take the dashboard down with it — the panel's other
  // cards are how you get to the logs that would explain the failure.
  const [diagnostics] = await server.GetDiagnostics({});
  return rpc.ok<Data>({ diagnostics: diagnostics ?? null });
}

export function view(route: string, prefix: string, data: Data): preact.ComponentChild {
  const currentAuth = requireAuthInView();
  if (!currentAuth) {
    return;
  }

  // Check if user is admin (ID == 1)
  if (!currentAuth.isAdmin) {
    return (
      <div>
        <Header isHome={false} />
        <main id="app" className="page-container">
          <div className="error-page">
            <h1>Access Denied</h1>
            <p>You do not have permission to access this page.</p>
            <a href="/dashboard" className="btn btn-primary">
              Return to Dashboard
            </a>
          </div>
        </main>
        <Footer />
      </div>
    );
  }

  return (
    <div>
      <Header isHome={false} />
      <main id="app" className="admin-container">
        <AdminPage user={currentAuth} diagnostics={data.diagnostics} />
      </main>
      <Footer />
    </div>
  );
}

interface AdminPageProps {
  user: auth.AuthCache;
  diagnostics: server.DiagnosticsResponse | null;
}

const AdminPage = ({ user, diagnostics }: AdminPageProps) => {
  return (
    <div className="admin-page">
      <div className="admin-header">
        <div className="admin-badge">
          <span className="admin-icon">⚡</span>
          <span>Admin Panel</span>
        </div>
        <h1>System Administration</h1>
        <p>Welcome, {user.name} - System Administrator</p>
      </div>

      {diagnostics && <Diagnostics info={diagnostics} />}

      <div className="admin-grid">
        <a href="/admin/analytics" className="admin-card admin-card-link">
          <div className="card-header">
            <div className="card-icon">📊</div>
            <h3>Site Analytics</h3>
          </div>
          <div className="card-content">
            <p>View site usage statistics, user activity, and performance metrics.</p>
            <div className="card-action">View Analytics Dashboard →</div>
          </div>
        </a>

        <a href="/admin/users" className="admin-card admin-card-link">
          <div className="card-header">
            <div className="card-icon">👥</div>
            <h3>User Management</h3>
          </div>
          <div className="card-content">
            <p>Manage user accounts, family groups, and permissions.</p>
            <div className="card-action">View All Users →</div>
          </div>
        </a>

        <a href="/admin/photos" className="admin-card admin-card-link">
          <div className="card-header">
            <div className="card-icon">🖼️</div>
            <h3>Photo Management</h3>
          </div>
          <div className="card-content">
            <p>Reprocess photos with modern formats and optimized sizes.</p>
            <div className="card-action">Manage Photos →</div>
          </div>
        </a>

        <div className="admin-card">
          <div className="card-header">
            <div className="card-icon">🛠️</div>
            <h3>System Settings</h3>
          </div>
          <div className="card-content">
            <p>Configure system-wide settings and maintenance options.</p>
            <div className="card-placeholder">Coming Soon</div>
          </div>
        </div>

        <a href="/admin/push" className="admin-card admin-card-link">
          <div className="card-header">
            <div className="card-icon">🔔</div>
            <h3>Push Notifications</h3>
          </div>
          <div className="card-content">
            <p>Check APNs configuration, registered devices, and delivery attempts.</p>
            <div className="card-action">Manage Push →</div>
          </div>
        </a>

        <a href="/admin/logs" className="admin-card admin-card-link">
          <div className="card-header">
            <div className="card-icon">📋</div>
            <h3>System Logs</h3>
          </div>
          <div className="card-content">
            <p>Review application logs and system events.</p>
            <div className="card-action">View System Logs →</div>
          </div>
        </a>
      </div>

      <div className="admin-section">
        <h2>Quick Actions</h2>
        <div className="admin-actions">
          <button className="admin-btn admin-btn-secondary" disabled>
            Export User Data
          </button>
          <button className="admin-btn admin-btn-secondary" disabled>
            System Health Check
          </button>
          <button className="admin-btn admin-btn-secondary" disabled>
            Clear Cache
          </button>
          <button className="admin-btn admin-btn-danger" disabled>
            Maintenance Mode
          </button>
        </div>
      </div>
    </div>
  );
};

// What is actually running, above everything else on the page. When something
// is wrong the first two questions are "which build is this" and "how long has
// it been up", and both have been answers you had to SSH for.
const Diagnostics = ({ info }: { info: server.DiagnosticsResponse }) => (
  <div className="diagnostics">
    <div className="diagnostics-primary">
      <span className="diagnostics-version">v{info.version}</span>
      <code className="diagnostics-commit">{info.commit}</code>
      <span className={info.release ? "diagnostics-tag" : "diagnostics-tag diagnostics-tag-warn"}>
        {info.release ? "release" : "local build"}
      </span>
    </div>
    <dl className="diagnostics-grid">
      <DiagnosticItem label="Uptime" value={formatUptime(info.uptimeSeconds)} />
      <DiagnosticItem label="Built" value={info.buildTime} />
      <DiagnosticItem label="Go" value={info.goVersion} />
      <DiagnosticItem
        label="Photo worker"
        value={info.photoRunning ? `running, ${info.photoQueue} queued` : "stopped"}
      />
      <DiagnosticItem
        label="Face analysis"
        value={info.analysisFaces ? `running, ${info.analysisQueue} queued` : "unavailable"}
      />
      <DiagnosticItem label="Mail queue" value={`${info.mailQueue} waiting`} />
      <DiagnosticItem label="Push" value={info.pushConfigured ? "configured" : "not configured"} />
    </dl>
  </div>
);

const DiagnosticItem = ({ label, value }: { label: string; value: string }) => (
  <div className="diagnostics-item">
    <dt>{label}</dt>
    <dd>{value}</dd>
  </div>
);

// Uptime is read at a glance, so it is rounded to whatever unit makes the
// number small. "3d 4h" beats "277,481 seconds" every time.
function formatUptime(seconds: number): string {
  if (seconds < 60) return `${seconds}s`;

  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return `${minutes}m`;

  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours}h ${minutes % 60}m`;

  const days = Math.floor(hours / 24);
  return `${days}d ${hours % 24}h`;
}
