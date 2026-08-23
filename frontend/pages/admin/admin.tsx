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
  health: server.SystemHealthResponse | null;
};

export async function fetch(route: string, prefix: string) {
  if (!(await ensureAuthInFetch())) {
    return rpc.ok<Data>({ diagnostics: null, health: null });
  }

  // A failure in either must not take the dashboard down with it — the panel's
  // other cards are how you get to the logs that would explain the failure.
  const [[diagnostics], [health]] = await Promise.all([
    server.GetDiagnostics({}),
    server.GetSystemHealth({}),
  ]);
  return rpc.ok<Data>({ diagnostics: diagnostics ?? null, health: health ?? null });
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
        <AdminPage user={currentAuth} diagnostics={data.diagnostics} health={data.health} />
      </main>
      <Footer />
    </div>
  );
}

interface AdminPageProps {
  user: auth.AuthCache;
  diagnostics: server.DiagnosticsResponse | null;
  health: server.SystemHealthResponse | null;
}

const AdminPage = ({ user, diagnostics, health }: AdminPageProps) => {
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
      {health && <Problems health={health} />}

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

        <a href="/admin/app-versions" className="admin-card admin-card-link">
          <div className="card-header">
            <div className="card-icon">📱</div>
            <h3>App Versions</h3>
          </div>
          <div className="card-content">
            <p>
              Set the minimum and latest companion app builds, and where to send someone to update.
            </p>
            <div className="card-action">Manage App Versions →</div>
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
    </div>
  );
};

// The problems feed. Everything in it was already available somewhere in the
// panel; what was missing was one place that asks all of it at once. It stays
// quiet when there is nothing to say, because a green page only means something
// if it is capable of being red.
const Problems = ({ health }: { health: server.SystemHealthResponse }) => {
  if (health.healthy) {
    return (
      <div className="problems problems-clear">
        <span className="problems-icon">✅</span>
        <span>
          Nothing to report. No errors in the last {health.logs.windowHours}h, no failed photos, no
          configuration problems.
        </span>
      </div>
    );
  }

  return (
    <div className="problems">
      <h2 className="problems-title">Needs a look</h2>

      <ConfigIssues health={health} />
      <LogIssues logs={health.logs} />
      <PhotoIssues photos={health.photos} />
      <PushIssues push={health.push} />
    </div>
  );
};

const ConfigIssues = ({ health }: { health: server.SystemHealthResponse }) => {
  if (health.configIssues.length === 0) return null;

  return (
    <div className="problem-group">
      <h3>Configuration</h3>
      {/* A release build refuses to start with any of these, so seeing one on
          production means the environment changed under a running process. A
          local build logs them and carries on. */}
      <p className="problem-note">
        {health.releaseBuild
          ? "This build refuses to start with these unset, so the environment has changed since it started."
          : "These would fail a release build. A development machine legitimately has no APNs key."}
      </p>
      <ul className="problem-list">
        {health.configIssues.map(issue => (
          <li key={issue.setting}>
            <code>{issue.setting}</code> {issue.detail}
          </li>
        ))}
      </ul>
    </div>
  );
};

const LogIssues = ({ logs }: { logs: server.LogProblems }) => {
  if (logs.unavailable) {
    return (
      <div className="problem-group">
        <h3>Logs</h3>
        <p className="problem-note">
          No log file has been written in the last {logs.windowHours}h. Either this is a fresh
          deploy, or the logger is writing somewhere nothing is reading.
        </p>
      </div>
    );
  }

  if (logs.errors === 0 && logs.requests5xx === 0) return null;

  return (
    <div className="problem-group">
      <h3>Logs, last {logs.windowHours}h</h3>
      <ul className="problem-list">
        {logs.errors > 0 && (
          <li>
            {logs.errors} error{logs.errors === 1 ? "" : "s"}
          </li>
        )}
        {logs.requests5xx > 0 && (
          <li>
            {logs.requests5xx} request{logs.requests5xx === 1 ? "" : "s"} answered 5xx
            {logs.requests4xx > 0 && `, ${logs.requests4xx} answered 4xx`}
          </li>
        )}
      </ul>

      {logs.recentErrors.length > 0 && (
        <div className="problem-errors">
          {logs.recentErrors.map((entry, index) => (
            <ErrorLine key={index} entry={entry} />
          ))}
        </div>
      )}

      <a className="problem-action" href="/admin/logs">
        Open the log viewer →
      </a>
    </div>
  );
};

// One error, with its reference code as a link straight into the log viewer's
// lookup. That code is the join key the whole error design is built around;
// making it clickable is what turns the feed into a starting point rather than
// a thing to read and then go searching manually.
const ErrorLine = ({ entry }: { entry: server.PublicLogEntry }) => {
  const reference = referenceOf(entry);

  return (
    <div className="problem-error">
      <span className="problem-error-time">{formatWhen(entry.timestamp)}</span>
      <span className="problem-error-message">{entry.message}</span>
      {reference && (
        <a className="problem-error-ref" href={`/admin/logs?ref=${encodeURIComponent(reference)}`}>
          {reference}
        </a>
      )}
    </div>
  );
};

// ProcError writes the correlation id to data.requestId. The payload is
// whatever JSON was logged, so this checks rather than assumes.
function referenceOf(entry: server.PublicLogEntry): string | null {
  const data = entry.data;
  if (data && typeof data === "object" && !Array.isArray(data)) {
    const id = (data as Record<string, unknown>)["requestId"];
    if (typeof id === "string" && id !== "") return id;
  }
  return null;
}

const PhotoIssues = ({ photos }: { photos: server.PhotoProblems }) => {
  const anything =
    photos.failed > 0 || photos.stuck > 0 || photos.analysisFailed > 0 || photos.workerStopped;
  if (!anything) return null;

  return (
    <div className="problem-group">
      <h3>Photos</h3>
      <ul className="problem-list">
        {photos.workerStopped && (
          <li>
            The photo worker is not running
            {photos.queueLength > 0 && `, and ${photos.queueLength} jobs are queued behind it`}
          </li>
        )}
        {photos.failed > 0 && <li>{photos.failed} failed to process</li>}
        {/* Stranded rows are the ones nothing will ever retry, which is why
            they are called out separately from ordinary failures. */}
        {photos.stuck > 0 && (
          <li>{photos.stuck} stuck in processing for over an hour with nothing attending them</li>
        )}
        {photos.analysisFailed > 0 && <li>{photos.analysisFailed} failed face analysis</li>}
      </ul>
      <a className="problem-action" href="/admin/photos">
        Open photo management →
      </a>
    </div>
  );
};

const PushIssues = ({ push }: { push: server.PushProblems }) => {
  if (!push.lastError) return null;

  return (
    <div className="problem-group">
      <h3>Push notifications</h3>
      <ul className="problem-list">
        <li>
          Last error {formatWhen(push.lastErrorAt)}: {push.lastError}
        </li>
        {push.failed > 0 && <li>{push.failed} failed since this process started</li>}
      </ul>
      <a className="problem-action" href="/admin/push">
        Open push notifications →
      </a>
    </div>
  );
};

// Relative time, because in this context "40m ago" is the useful form and a
// wall-clock timestamp is one subtraction away from being useful.
function formatWhen(timestamp: string): string {
  const then = new Date(timestamp).getTime();
  if (!Number.isFinite(then)) return timestamp;

  const seconds = Math.max(0, Math.round((Date.now() - then) / 1000));
  if (seconds < 60) return "just now";

  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return `${minutes}m ago`;

  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours}h ago`;

  return `${Math.floor(hours / 24)}d ago`;
}

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
