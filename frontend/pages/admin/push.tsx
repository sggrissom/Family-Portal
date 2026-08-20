import * as preact from "preact";
import * as vlens from "vlens";
import * as rpc from "vlens/rpc";
import * as server from "../../server";
import { Header, Footer } from "../../layout";
import { ensureAuthInFetch, requireAuthInView } from "../../lib/authHelpers";
import { logWarn } from "../../lib/logger";
import "./admin-styles";
import "./push-styles";

// The last N delivery attempts the server keeps in memory. Mirrors
// maxRecentPushAttempts in push_worker.go; used only for the note above the table.
const RECENT_ATTEMPT_LIMIT = 50;

type PushPageState = {
  // status starts null and is replaced by each poll, so the counters and
  // delivery history stay live while you wait for a test push to land.
  status: server.GetPushStatusResponse | null;
  devices: server.AdminPushDevice[] | null;
  isSending: boolean;
  sendResult: string | null;
  sendError: string | null;
  testMessage: string;
  pollingStarted: boolean;
};

const usePushPageState = vlens.declareHook(
  (): PushPageState => ({
    status: null,
    devices: null,
    isSending: false,
    sendResult: null,
    sendError: null,
    testMessage: "",
    pollingStarted: false,
  })
);

const emptyStatus: server.GetPushStatusResponse = {
  config: {
    configured: false,
    teamId: "",
    keyId: "",
    bundleId: "",
    keyPath: "",
    environment: "",
    keyLoaded: false,
    loadError: "",
  },
  stats: {
    enabled: false,
    isRunning: false,
    queueLength: 0,
    sent: 0,
    failed: 0,
    deactivated: 0,
    lastSentAt: "",
    lastError: "",
    lastErrorAt: "",
    recentAttempts: [],
  },
  issues: [],
  totalDevices: 0,
  activeDevices: 0,
  inactiveDevices: 0,
};

export async function fetch(route: string, prefix: string) {
  if (!(await ensureAuthInFetch())) {
    return rpc.ok<server.GetPushStatusResponse>(emptyStatus);
  }

  return server.GetPushStatus({});
}

export function view(
  route: string,
  prefix: string,
  data: server.GetPushStatusResponse
): preact.ComponentChild {
  const currentAuth = requireAuthInView();
  if (!currentAuth) {
    return;
  }

  if (!currentAuth.isAdmin) {
    return (
      <div>
        <Header isHome={false} />
        <main id="app" className="page-container">
          <div className="error-page">
            <h1>Access Denied</h1>
            <p>You do not have permission to access this page.</p>
            <a href="/admin" className="btn btn-primary">
              Return to Admin Dashboard
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
        <PushPage data={data} />
      </main>
      <Footer />
    </div>
  );
}

// Go's zero time serializes as year 1; treat anything at or before it as "never
// happened" rather than rendering a 1-1-1 date.
function formatTimestamp(value: string): string {
  if (!value) return "Never";
  const date = new Date(value);
  if (isNaN(date.getTime()) || date.getUTCFullYear() < 2000) return "Never";
  return (
    date.toLocaleDateString() +
    " " +
    date.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit", second: "2-digit" })
  );
}

interface PushPageProps {
  data: server.GetPushStatusResponse;
}

const PushPage = ({ data }: PushPageProps) => {
  const state = usePushPageState();

  const loadDevices = async () => {
    try {
      const [result, error] = await server.ListPushDevices({});
      if (result && !error) {
        state.devices = result.devices || [];
        vlens.scheduleRedraw();
      }
    } catch (err) {
      logWarn("admin", "Failed to load push devices", err);
    }
  };

  const loadStatus = async () => {
    try {
      const [result, error] = await server.GetPushStatus({});
      if (result && !error) {
        state.status = result;
        vlens.scheduleRedraw();
      }
    } catch (err) {
      logWarn("admin", "Failed to load push status", err);
    }
  };

  if (!state.pollingStarted) {
    state.pollingStarted = true;
    loadDevices();
    // A test send lands asynchronously, so the counters and history below only
    // become useful if they refresh on their own while you watch the phone.
    setInterval(() => {
      loadStatus();
      loadDevices();
    }, 5000);
  }

  const sendTest = async () => {
    state.isSending = true;
    state.sendResult = null;
    state.sendError = null;
    vlens.scheduleRedraw();

    try {
      const [result, error] = await server.SendTestPushNotification({
        userId: 0,
        message: state.testMessage,
      });

      if (error) {
        state.sendError = error;
      } else if (result) {
        state.sendResult =
          result.deviceCount > 0
            ? `Queued for ${result.deviceCount} device${result.deviceCount === 1 ? "" : "s"} registered to ${result.targetName}. Watch the delivery history below.`
            : `Queued, but ${result.targetName} has no active registered devices — nothing will be sent.`;
      }
    } catch (err) {
      state.sendError = "Failed to send test notification: " + String(err);
    }

    state.isSending = false;
    vlens.scheduleRedraw();
  };

  const status = state.status || data;
  const config = status.config;
  const stats = status.stats;
  const devices = state.devices || [];

  return (
    <div className="admin-page">
      <div className="admin-breadcrumb">
        <a href="/admin">Admin Dashboard</a>
        <span className="breadcrumb-separator">›</span>
        <span>Push Notifications</span>
      </div>

      <div className="admin-header">
        <div className="admin-badge">
          <span className="admin-icon">🔔</span>
          <span>Push Notifications</span>
        </div>
        <h1>Push Notifications</h1>
        <p>Verify APNs configuration, registered devices, and recent delivery attempts.</p>
      </div>

      {status.issues.length > 0 && (
        <div className="admin-card error-card">
          <div className="card-header">
            <div className="card-icon">⚠️</div>
            <h3>Configuration Problems</h3>
          </div>
          <div className="card-content">
            <ul className="error-list">
              {status.issues.map((issue, index) => (
                <li key={index}>
                  <strong>{issue.setting}</strong> {issue.detail}
                </li>
              ))}
            </ul>
          </div>
        </div>
      )}

      <div className="photo-stats-grid">
        <div className="stat-card">
          <div className="stat-icon">{stats.enabled ? "🟢" : "🔴"}</div>
          <div className="stat-content">
            <h3>Worker</h3>
            <div className="stat-value">{stats.enabled ? "Running" : "Off"}</div>
            <div className="stat-label">
              {stats.enabled ? "Accepting notifications" : "APNs not configured"}
            </div>
          </div>
        </div>

        <div className="stat-card">
          <div className="stat-icon">📮</div>
          <div className="stat-content">
            <h3>Queue</h3>
            <div className="stat-value">{stats.queueLength}</div>
            <div className="stat-label">Jobs awaiting send</div>
          </div>
        </div>

        <div className="stat-card">
          <div className="stat-icon">✅</div>
          <div className="stat-content">
            <h3>Delivered</h3>
            <div className="stat-value">{stats.sent.toLocaleString()}</div>
            <div className="stat-label">Accepted by APNs since restart</div>
          </div>
        </div>

        <div className="stat-card">
          <div className="stat-icon">❌</div>
          <div className="stat-content">
            <h3>Failed</h3>
            <div className="stat-value">{stats.failed.toLocaleString()}</div>
            <div className="stat-label">Rejected or unreachable</div>
          </div>
        </div>

        <div className="stat-card">
          <div className="stat-icon">📱</div>
          <div className="stat-content">
            <h3>Active Devices</h3>
            <div className="stat-value">{status.activeDevices.toLocaleString()}</div>
            <div className="stat-label">{status.inactiveDevices} inactive</div>
          </div>
        </div>

        <div className="stat-card">
          <div className="stat-icon">🚫</div>
          <div className="stat-content">
            <h3>Auto-Deactivated</h3>
            <div className="stat-value">{stats.deactivated.toLocaleString()}</div>
            <div className="stat-label">Tokens APNs rejected</div>
          </div>
        </div>
      </div>

      <div className="admin-section">
        <h2>APNs Configuration</h2>
        <div className="push-config-card">
          <dl className="push-config-list">
            <div className="push-config-row">
              <dt>Bundle ID</dt>
              <dd>{config.bundleId || <span className="push-unset">not set</span>}</dd>
            </div>
            <div className="push-config-row">
              <dt>Environment</dt>
              <dd>
                {config.environment ? (
                  <span className={`push-env push-env-${config.environment}`}>
                    {config.environment}
                  </span>
                ) : (
                  <span className="push-unset">not set</span>
                )}
              </dd>
            </div>
            <div className="push-config-row">
              <dt>Team ID</dt>
              <dd>{config.teamId || <span className="push-unset">not set</span>}</dd>
            </div>
            <div className="push-config-row">
              <dt>Key ID</dt>
              <dd>{config.keyId || <span className="push-unset">not set</span>}</dd>
            </div>
            <div className="push-config-row">
              <dt>Key Path</dt>
              <dd className="push-mono">
                {config.keyPath || <span className="push-unset">not set</span>}
              </dd>
            </div>
            <div className="push-config-row">
              <dt>Signing Key</dt>
              <dd>
                {config.keyLoaded ? (
                  <span className="push-ok">Loaded and parsed</span>
                ) : (
                  <span className="push-bad">{config.loadError || "Not loaded"}</span>
                )}
              </dd>
            </div>
            <div className="push-config-row">
              <dt>Last Delivery</dt>
              <dd>{formatTimestamp(stats.lastSentAt)}</dd>
            </div>
            <div className="push-config-row">
              <dt>Last Error</dt>
              <dd>
                {stats.lastError ? (
                  <span className="push-bad">
                    {stats.lastError} ({formatTimestamp(stats.lastErrorAt)})
                  </span>
                ) : (
                  "None"
                )}
              </dd>
            </div>
          </dl>
        </div>
      </div>

      <div className="admin-card">
        <div className="card-header">
          <div className="card-icon">🧪</div>
          <h3>Send Test Notification</h3>
        </div>
        <div className="card-content">
          <p>
            Sends a push to every active device registered to your own account. Chat notifications
            only go to family members who are offline, so this is the only way to trigger one
            without a second person and a second device.
          </p>

          <div className="push-test-form">
            <input
              type="text"
              className="push-test-input"
              aria-label="Test notification message"
              placeholder="Test notification from the Family Portal admin panel."
              value={state.testMessage}
              onInput={(e: any) => {
                state.testMessage = e.currentTarget.value;
              }}
              disabled={state.isSending}
            />
            <button
              className="admin-btn admin-btn-primary"
              onClick={sendTest}
              disabled={state.isSending || !stats.enabled}
            >
              {state.isSending ? "Sending..." : "Send Test"}
            </button>
          </div>

          {!stats.enabled && (
            <div className="push-hint">
              The push worker is not running, so a test cannot be sent. Fix the APNs configuration
              above and restart the server.
            </div>
          )}
          {state.sendResult && <div className="push-result push-result-ok">{state.sendResult}</div>}
          {state.sendError && <div className="push-result push-result-bad">{state.sendError}</div>}
        </div>
      </div>

      <div className="admin-section">
        <h2>Registered Devices ({devices.length})</h2>
        <div className="users-table-container">
          {devices.length === 0 ? (
            <div className="empty-state">
              <p>No devices have registered for push notifications.</p>
            </div>
          ) : (
            <div className="table-wrapper">
              <table className="users-table">
                <thead>
                  <tr>
                    <th>ID</th>
                    <th>User</th>
                    <th>Token</th>
                    <th>Platform</th>
                    <th>Environment</th>
                    <th>Bundle ID</th>
                    <th>Registered</th>
                    <th>Updated</th>
                    <th>Status</th>
                  </tr>
                </thead>
                <tbody>
                  {devices.map(device => (
                    <tr key={device.id} className={device.isActive ? "" : "push-row-inactive"}>
                      <td className="user-id">{device.id}</td>
                      <td>
                        <div className="user-table-name">{device.userName || "Unknown"}</div>
                        <div className="user-email">{device.userEmail}</div>
                      </td>
                      <td className="push-mono">{device.tokenHint}</td>
                      <td>{device.platform}</td>
                      <td>
                        <span className={`push-env push-env-${device.environment}`}>
                          {device.environment}
                        </span>
                        {device.environmentMismatch && (
                          <span
                            className="push-mismatch"
                            title="This device registered against a different APNs environment than the server is configured for. Notifications to it will be rejected."
                          >
                            ⚠ mismatch
                          </span>
                        )}
                      </td>
                      <td className="push-mono">{device.bundleId}</td>
                      <td className="user-created">{formatTimestamp(device.createdAt)}</td>
                      <td className="user-login">{formatTimestamp(device.updatedAt)}</td>
                      <td>
                        {device.isActive ? (
                          <span className="push-ok">Active</span>
                        ) : (
                          <span className="push-bad">Inactive</span>
                        )}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </div>
      </div>

      <div className="admin-section">
        <h2>Recent Delivery Attempts</h2>
        <p className="push-section-note">
          In-memory only — the last {RECENT_ATTEMPT_LIMIT} attempts since the server started.
          Restarting clears this list. For anything older, search the system logs for{" "}
          <code>[PUSH_NOTIFICATION]</code> under the Background Jobs category.
        </p>
        <div className="users-table-container">
          {stats.recentAttempts.length === 0 ? (
            <div className="empty-state">
              <p>No delivery attempts since the server started.</p>
            </div>
          ) : (
            <div className="table-wrapper">
              <table className="users-table">
                <thead>
                  <tr>
                    <th>Time</th>
                    <th>Result</th>
                    <th>Kind</th>
                    <th>User</th>
                    <th>Token</th>
                    <th>HTTP</th>
                    <th>Reason</th>
                    <th>APNs ID</th>
                  </tr>
                </thead>
                <tbody>
                  {stats.recentAttempts.map((attempt, index) => (
                    <tr key={index}>
                      <td className="user-created">{formatTimestamp(attempt.time)}</td>
                      <td>
                        {attempt.success ? (
                          <span className="push-ok">Delivered</span>
                        ) : (
                          <span className="push-bad">Failed</span>
                        )}
                      </td>
                      <td>{attempt.kind}</td>
                      <td className="user-id">{attempt.userId}</td>
                      <td className="push-mono">{attempt.tokenHint}</td>
                      <td className="user-id">{attempt.statusCode || "—"}</td>
                      <td>{attempt.reason || "—"}</td>
                      <td className="push-mono push-apns-id">{attempt.apnsId || "—"}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </div>
      </div>
    </div>
  );
};
