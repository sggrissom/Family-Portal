import * as preact from "preact";
import * as vlens from "vlens";
import * as rpc from "vlens/rpc";
import * as auth from "../../lib/authCache";
import * as core from "vlens/core";
import * as server from "../../server";
import { Header, Footer } from "../../layout";
import { ensureAuthInFetch } from "../../lib/authHelpers";
import { adminView } from "../../components/AdminGuard";
import { formatBytes } from "../../lib/formatBytes";
import { formatRelativeTime } from "../../lib/dateUtils";
import "./admin-styles";

type Data = {
  diagnostics: server.DiagnosticsResponse | null;
  health: server.SystemHealthResponse | null;
  host: server.HostMetricsResponse | null;
  digest: server.WeeklyDigestResponse | null;
};

export async function fetch(route: string, prefix: string) {
  if (!(await ensureAuthInFetch())) {
    return rpc.ok<Data>({ diagnostics: null, health: null, host: null, digest: null });
  }

  const [[diagnostics], [health], [host], [digest]] = await Promise.all([
    server.GetDiagnostics({}),
    server.GetSystemHealth({}),
    server.GetHostMetrics({}),
    server.GetWeeklyDigest({}),
  ]);
  return rpc.ok<Data>({
    diagnostics: diagnostics ?? null,
    health: health ?? null,
    host: host ?? null,
    digest: digest ?? null,
  });
}

export function view(route: string, prefix: string, data: Data): preact.ComponentChild {
  return adminView(currentAuth => {
    return (
      <div>
        <Header isHome={false} />
        <main id="app" className="admin-container">
          <AdminPage
            user={currentAuth}
            diagnostics={data.diagnostics}
            health={data.health}
            host={data.host}
            digest={data.digest}
          />
        </main>
        <Footer />
      </div>
    );
  });
}

interface AdminPageProps {
  user: auth.AuthCache;
  diagnostics: server.DiagnosticsResponse | null;
  health: server.SystemHealthResponse | null;
  host: server.HostMetricsResponse | null;
  digest: server.WeeklyDigestResponse | null;
}

const AdminPage = ({ user, diagnostics, health, host, digest }: AdminPageProps) => {
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
      {host && host.available && <Releases releases={host.app.releases} />}
      {health && <Problems health={health} />}
      {host && host.configured && <Host metrics={host} />}
      {digest && <Digest digest={digest} />}

      <div className="admin-grid">
        <a href="/admin/analytics" className="admin-card admin-card-link">
          <div className="card-header">
            <div className="admin-card-icon">📊</div>
            <h3>Site Analytics</h3>
          </div>
          <div className="card-content">
            <p>View site usage statistics, user activity, and performance metrics.</p>
            <div className="card-action">View Analytics Dashboard →</div>
          </div>
        </a>

        <a href="/admin/users" className="admin-card admin-card-link">
          <div className="card-header">
            <div className="admin-card-icon">👥</div>
            <h3>User Management</h3>
          </div>
          <div className="card-content">
            <p>Manage user accounts, family groups, and permissions.</p>
            <div className="card-action">View All Users →</div>
          </div>
        </a>

        <a href="/admin/photos" className="admin-card admin-card-link">
          <div className="card-header">
            <div className="admin-card-icon">🖼️</div>
            <h3>Photo Management</h3>
          </div>
          <div className="card-content">
            <p>Reprocess photos with modern formats and optimized sizes.</p>
            <div className="card-action">Manage Photos →</div>
          </div>
        </a>

        <a href="/admin/push" className="admin-card admin-card-link">
          <div className="card-header">
            <div className="admin-card-icon">🔔</div>
            <h3>Push Notifications</h3>
          </div>
          <div className="card-content">
            <p>Check APNs configuration, registered devices, and delivery attempts.</p>
            <div className="card-action">Manage Push →</div>
          </div>
        </a>

        <a href="/admin/app-versions" className="admin-card admin-card-link">
          <div className="card-header">
            <div className="admin-card-icon">📱</div>
            <h3>App Versions</h3>
          </div>
          <div className="card-content">
            <p>
              Set the minimum and latest companion app builds, and where to send someone to update.
            </p>
            <div className="card-action">Manage App Versions →</div>
          </div>
        </a>

        <a href="/admin/seed" className="admin-card admin-card-link">
          <div className="card-header">
            <div className="admin-card-icon">🌱</div>
            <h3>Test Accounts</h3>
          </div>
          <div className="card-content">
            <p>
              Create the demo family so a reviewer has something to sign in to, and remove it again
              afterwards.
            </p>
            <div className="card-action">Manage Test Accounts →</div>
          </div>
        </a>

        <a href="/admin/logs" className="admin-card admin-card-link">
          <div className="card-header">
            <div className="admin-card-icon">📋</div>
            <h3>System Logs</h3>
          </div>
          <div className="card-content">
            <p>Review application logs and system events.</p>
            <div className="card-action">View System Logs →</div>
          </div>
        </a>
      </div>

      <Backups state={health ? health.backups : null} />
    </div>
  );
};

type BackupState = {
  checking: boolean;
  result: server.VerifyBackupPathResponse | null;
  error: string;
};

const useBackupState = vlens.declareHook(
  (): BackupState => ({ checking: false, result: null, error: "" })
);

const Backups = ({ state: reported }: { state: server.BackupProblems | null }) => {
  const state = useBackupState();

  const verify = async () => {
    state.checking = true;
    state.error = "";
    vlens.scheduleRedraw();

    const [result, error] = await server.VerifyBackupPath({});
    state.checking = false;
    if (error) {
      state.error = error;
    } else if (result) {
      state.result = result;
    }
    vlens.scheduleRedraw();
  };

  return (
    <div className="admin-section">
      <h2>Backups</h2>
      {reported && reported.available && <BackupState state={reported} />}
      <p className="problem-note">
        Every night backupctl fetches a database snapshot over loopback, and the only evidence it
        worked is a file on a box this application cannot read. This sends the same request with the
        token this process would accept, reads the whole snapshot, and throws it away.
      </p>

      <div className="admin-actions">
        <button
          className="admin-btn admin-btn-secondary"
          onClick={verify}
          disabled={state.checking}
        >
          {state.checking ? "Checking…" : "Verify backup path"}
        </button>
      </div>

      {state.error && (
        <div className="admin-notice">
          <strong>The check could not run</strong> — {state.error}
        </div>
      )}
      {state.result && <BackupResult result={state.result} />}
    </div>
  );
};

const BackupState = ({ state }: { state: server.BackupProblems }) => {
  if (!state.registered) {
    return (
      <p className="problem-note">
        This app is not registered for backup on the host — there is no{" "}
        <code>shared/backup.conf</code>, so backupctl has never been asked to archive it.
      </p>
    );
  }
  if (state.neverRun) {
    return (
      <p className="problem-note">
        Registered for backup, but no run has ever succeeded. Nothing has been archived.
      </p>
    );
  }
  return (
    <p className="problem-note">
      Last backup {formatRelativeTime(state.lastSuccess)}
      {state.sizeKb > 0 && `, ${formatKb(state.sizeKb)} in the repository`}.
      {state.stale && " That is more than two nightly windows ago."}
    </p>
  );
};

const BackupResult = ({ result }: { result: server.VerifyBackupPathResponse }) => (
  <div className={result.ok ? "admin-notice admin-notice-ok" : "admin-notice"}>
    <strong>{result.ok ? "Verified" : "Not verified"}</strong> — {result.detail}
    {result.ok && (
      <p className="problem-note">
        {formatBytes(result.receivedBytes)} in {(result.durationMs / 1000).toFixed(1)}s.
      </p>
    )}
    {result.cached && (
      <p className="problem-note">
        Checked {formatRelativeTime(result.checkedAt)}. The snapshot endpoint allows ten requests an
        hour and the nightly backup spends from the same budget, so a result stands for ten minutes
        before another check runs.
      </p>
    )}
  </div>
);

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
      <BackupIssues backups={health.backups} />
      <HostIssues host={health.host} />
      <LogIssues logs={health.logs} />
      <PhotoIssues photos={health.photos} />
      <PushIssues push={health.push} />
      <MailIssues mail={health.mail} />
    </div>
  );
};

const ConfigIssues = ({ health }: { health: server.SystemHealthResponse }) => {
  const issues = health.configIssues ?? [];
  if (issues.length === 0) return null;

  return (
    <div className="problem-group">
      <h3>Configuration</h3>
      <p className="problem-note">
        {health.releaseBuild
          ? "This build refuses to start with these unset, so the environment has changed since it started."
          : "These would fail a release build. A development machine legitimately has no APNs key."}
      </p>
      <ul className="problem-list">
        {issues.map(issue => (
          <li key={issue.setting}>
            <code>{issue.setting}</code> {issue.detail}
          </li>
        ))}
      </ul>
    </div>
  );
};

const BackupIssues = ({ backups }: { backups: server.BackupProblems }) => {
  if (!backups.available) return null;
  if (backups.registered && !backups.neverRun && !backups.stale) return null;

  return (
    <div className="problem-group">
      <h3>Backups</h3>
      <ul className="problem-list">
        {!backups.registered && (
          <li>
            This app is not registered for backup — no <code>shared/backup.conf</code> on the host,
            so nothing is being archived
          </li>
        )}
        {backups.neverRun && <li>Registered for backup, but no run has ever succeeded</li>}
        {backups.stale && (
          <li>
            Last successful backup was {formatRelativeTime(backups.lastSuccess)}, more than two
            nightly windows ago
          </li>
        )}
      </ul>
    </div>
  );
};

const HostIssues = ({ host }: { host: server.HostProblems }) => {
  if (!host.available || (!host.diskLow && host.proxy5xx === 0)) return null;

  const windowMinutes = Math.round(host.windowSeconds / 60);

  return (
    <div className="problem-group">
      <h3>Host</h3>
      <ul className="problem-list">
        {host.diskLow && <li>Disk is {host.diskUsedPct.toFixed(0)}% full</li>}
        {host.proxy5xx > 0 && (
          <li>
            Caddy answered {host.proxy5xx} request{host.proxy5xx === 1 ? "" : "s"} with 5xx in the
            last {windowMinutes}m{host.proxy4xx > 0 && `, and ${host.proxy4xx} with 4xx`}
          </li>
        )}
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

      {(logs.recentErrors ?? []).length > 0 && (
        <div className="problem-errors">
          {(logs.recentErrors ?? []).map((entry, index) => (
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

const ErrorLine = ({ entry }: { entry: server.PublicLogEntry }) => {
  const reference = referenceOf(entry);

  return (
    <div className="problem-error">
      <span className="problem-error-time">{formatRelativeTime(entry.timestamp)}</span>
      <span className="problem-error-message">{entry.message}</span>
      {reference && (
        <a className="problem-error-ref" href={`/admin/logs?ref=${encodeURIComponent(reference)}`}>
          {reference}
        </a>
      )}
    </div>
  );
};

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
          Last error {formatRelativeTime(push.lastErrorAt)}: {push.lastError}
        </li>
        {push.failed > 0 && <li>{push.failed} failed since this process started</li>}
      </ul>
      <a className="problem-action" href="/admin/push">
        Open push notifications →
      </a>
    </div>
  );
};

const MailIssues = ({ mail }: { mail: server.MailProblems }) => {
  if (!mail.lastError) return null;

  return (
    <div className="problem-group">
      <h3>Email</h3>
      <ul className="problem-list">
        <li>
          Last error {formatRelativeTime(mail.lastErrorAt)}: {mail.lastError}
        </li>
        {mail.failed > 0 && <li>{mail.failed} failed since this process started</li>}
        {mail.queueLength > 0 && <li>{mail.queueLength} still waiting in the queue</li>}
      </ul>
      <a className="problem-action" href="/admin/users">
        Open mail delivery →
      </a>
    </div>
  );
};

const Releases = ({ releases }: { releases: server.HostRelease[] }) => {
  const list = releases ?? [];
  if (list.length === 0) return null;

  return (
    <div className="releases">
      <h3 className="releases-title">Recent deploys</h3>
      <ol className="releases-list">
        {list.map(release => (
          <li
            key={release.name}
            className={release.current ? "release release-current" : "release"}
          >
            <code className="release-sha">{release.sha}</code>
            <span className="release-when">
              {formatRelativeTime(release.deployed_at, "unknown")}
            </span>
            {release.current && <span className="release-tag">running</span>}
          </li>
        ))}
      </ol>
    </div>
  );
};

const Digest = ({ digest }: { digest: server.WeeklyDigestResponse }) => {
  if (digest.quiet) {
    return (
      <div className="admin-section">
        <h2>The last {digest.windowDays} days</h2>
        <p className="problem-note">
          Nobody signed in, and nothing was added. All {digest.accounts} accounts were quiet.
        </p>
      </div>
    );
  }

  return (
    <div className="admin-section">
      <h2>The last {digest.windowDays} days</h2>

      <dl className="diagnostics-grid">
        <DiagnosticItem label="Photos" value={countOf(digest.photos, "photo", "photos")} />
        <DiagnosticItem
          label="Milestones"
          value={countOf(digest.milestones, "milestone", "milestones")}
        />
        <DiagnosticItem
          label="Measurements"
          value={countOf(digest.measurements, "measurement", "measurements")}
        />
        <DiagnosticItem label="Chat" value={countOf(digest.messages, "message", "messages")} />
      </dl>

      {(digest.people ?? []).length > 0 && (
        <ul className="digest-people">
          {(digest.people ?? []).map(person => (
            <DigestPerson key={person.name} person={person} />
          ))}
        </ul>
      )}

      {digest.absent > 0 && (
        <p className="problem-note">
          {digest.absent} of {digest.accounts} accounts neither signed in nor added anything.
        </p>
      )}
    </div>
  );
};

const DigestPerson = ({ person }: { person: server.DigestPerson }) => {
  const added: string[] = [];
  if (person.photos > 0) added.push(countOf(person.photos, "photo", "photos"));
  if (person.messages > 0) added.push(countOf(person.messages, "message", "messages"));

  return (
    <li className="digest-person">
      <span className="digest-person-name">{person.name}</span>
      <span className="digest-person-what">
        {added.length > 0 ? added.join(", ") : "nothing added"}
      </span>
      <span className="digest-person-when">
        {person.joined
          ? "joined this week"
          : person.signedIn
            ? `signed in ${formatRelativeTime(person.lastLogin)}`
            : "no sign-in this week"}
      </span>
    </li>
  );
};

function countOf(value: number, one: string, many: string): string {
  return `${value} ${value === 1 ? one : many}`;
}

const Host = ({ metrics }: { metrics: server.HostMetricsResponse }) => {
  if (!metrics.available) {
    return (
      <div className="diagnostics">
        <dl className="diagnostics-grid">
          <DiagnosticItem label="Host metrics" value={metrics.error || "unavailable"} />
        </dl>
      </div>
    );
  }

  const traffic = metrics.app.traffic;
  const windowMinutes = Math.round(traffic.window_seconds / 60);

  return (
    <div className="diagnostics">
      <dl className="diagnostics-grid">
        <DiagnosticItem
          label="Disk"
          value={`${metrics.system.disk.used_pct.toFixed(0)}% used, ${formatKb(metrics.system.disk.free_kb)} free`}
        />
        <DiagnosticItem label="This app" value={formatKb(metrics.app.disk_kb)} />
        <DiagnosticItem
          label="Memory"
          value={`${metrics.system.memory.used_pct.toFixed(0)}% used`}
        />
        <DiagnosticItem
          label="CPU"
          value={`${(100 - metrics.system.cpu.idle_pct).toFixed(0)}% busy, ${metrics.system.cpu.iowait_pct.toFixed(0)}% iowait`}
        />
        <DiagnosticItem
          label="Load"
          value={`${metrics.system.load_avg.one.toFixed(2)} / ${metrics.system.load_avg.five.toFixed(2)} / ${metrics.system.load_avg.fifteen.toFixed(2)}`}
        />
        <DiagnosticItem
          label={`Traffic (${windowMinutes}m)`}
          value={`${traffic.requests_total} requests, ${traffic.requests_per_min.toFixed(1)}/min`}
        />
        <DiagnosticItem
          label="Proxy errors"
          value={`${traffic.error_5xx} 5xx, ${traffic.error_4xx} 4xx`}
        />
      </dl>
    </div>
  );
};

function formatKb(kb: number): string {
  const units = ["KB", "MB", "GB", "TB"];
  let size = kb;
  let unit = 0;
  while (size >= 1024 && unit < units.length - 1) {
    size /= 1024;
    unit++;
  }
  return `${size.toFixed(size >= 10 || unit === 0 ? 0 : 1)} ${units[unit]}`;
}

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

function formatUptime(seconds: number): string {
  if (seconds < 60) return `${seconds}s`;

  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return `${minutes}m`;

  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours}h ${minutes % 60}m`;

  const days = Math.floor(hours / 24);
  return `${days}d ${hours % 24}h`;
}
