import * as preact from "preact";
import * as rpc from "vlens/rpc";
import * as vlens from "vlens";
import * as auth from "../../lib/authCache";
import * as server from "../../server";
import { Header, Footer } from "../../layout";
import { ensureAuthInFetch } from "../../lib/authHelpers";
import { adminView } from "../../components/AdminGuard";
import { formatDateTime, formatRelativeTime } from "../../lib/dateUtils";
import { logWarn } from "../../lib/logger";
import "./admin-styles";

export async function fetch(route: string, prefix: string) {
  if (!(await ensureAuthInFetch())) {
    return rpc.ok<server.ListAllUsersResponse>({ users: [] });
  }

  return server.ListAllUsers({});
}

export function view(
  route: string,
  prefix: string,
  data: server.ListAllUsersResponse
): preact.ComponentChild {
  return adminView(currentAuth => {
    return (
      <div>
        <Header isHome={false} />
        <main id="app" className="admin-container">
          <UserManagementPage user={currentAuth} data={data} />
        </main>
        <Footer />
      </div>
    );
  });
}

interface UserManagementPageProps {
  user: auth.AuthCache;
  data: server.ListAllUsersResponse;
}

type UsersPageState = {
  busy: { [userId: number]: string };
  results: { [userId: number]: string };
  mail: server.MailWorkerStats | null;
  mailError: string;
  mailLoading: boolean;
};

const useUsersPageState = vlens.declareHook(
  (): UsersPageState => ({ busy: {}, results: {}, mail: null, mailError: "", mailLoading: false })
);

async function loadMailStats(state: UsersPageState) {
  if (state.mailLoading) return;
  state.mailLoading = true;

  const [result, error] = await server.GetMailStats({});
  state.mailLoading = false;
  if (error) {
    logWarn("admin", "Failed to load mail stats", error);
    state.mailError = error;
  } else if (result) {
    state.mail = result;
    state.mailError = "";
  }
  vlens.scheduleRedraw();
}

async function resendPasswordReset(state: UsersPageState, u: server.AdminUserInfo) {
  const confirmed = confirm(
    `Email a new password reset link to ${u.email}?\n\n` +
      "Any earlier link for this account stops working the moment this one is created, so if they " +
      "are already holding one, they have to use the new email instead."
  );
  if (!confirmed) return;

  state.busy[u.id] = "resend";
  state.results[u.id] = "";
  vlens.scheduleRedraw();

  const [result, error] = await server.ResendPasswordReset({ userId: u.id });
  state.busy[u.id] = "";
  if (error) {
    state.results[u.id] = error;
  } else if (result) {
    const previous = result.invalidatedPrevious ? " Their earlier link is now dead." : "";
    state.results[u.id] = result.queued
      ? `Sent to ${result.email}.${previous}`
      : `Not sent — ${result.detail}`;
  }
  vlens.scheduleRedraw();
  loadMailStats(state);
}

async function revokeSessions(state: UsersPageState, u: server.AdminUserInfo) {
  const confirmed = confirm(
    `Sign ${u.name} out of every device?\n\n` +
      "Their refresh tokens are deleted immediately. The access token they already hold keeps " +
      "working until it expires, within the hour."
  );
  if (!confirmed) return;

  state.busy[u.id] = "revoke";
  state.results[u.id] = "";
  vlens.scheduleRedraw();

  const [result, error] = await server.RevokeUserSessions({ userId: u.id });
  state.busy[u.id] = "";
  state.results[u.id] = error
    ? error
    : result && result.revoked > 0
      ? `Revoked ${result.revoked}`
      : "No active sessions";
  vlens.scheduleRedraw();
}

const UserManagementPage = ({ user, data }: UserManagementPageProps) => {
  const users = data.users || [];
  const state = useUsersPageState();

  if (!state.mail && !state.mailError && !state.mailLoading) {
    loadMailStats(state);
  }

  return (
    <div className="admin-page">
      <div className="admin-breadcrumb">
        <a href="/admin">Admin Dashboard</a>
        <span className="breadcrumb-separator">›</span>
        <span>User Management</span>
      </div>

      <div className="admin-header">
        <div className="admin-badge">
          <span className="admin-icon">👥</span>
          <span>User Management</span>
        </div>
        <h1>Registered Users</h1>
        <p>Total users: {users.length}</p>
      </div>

      <div className="users-table-container">
        {users.length === 0 ? (
          <div className="empty-state">
            <p>No users found.</p>
          </div>
        ) : (
          <div className="table-wrapper">
            <table className="users-table">
              <thead>
                <tr>
                  <th>ID</th>
                  <th>Name</th>
                  <th>Email</th>
                  <th>Family</th>
                  <th>Account Created</th>
                  <th>Last Login</th>
                  <th>Role</th>
                  <th>Actions</th>
                </tr>
              </thead>
              <tbody>
                {users.map(u => (
                  <tr key={u.id} className={u.isAdmin ? "admin-row" : ""}>
                    <td className="user-id">{u.id}</td>
                    <td className="user-table-name">{u.name}</td>
                    <td className="user-email">{u.email}</td>
                    <td className="user-family">
                      {u.familyName ? (
                        <span className="family-name">{u.familyName}</span>
                      ) : (
                        <span className="no-family">No family</span>
                      )}
                    </td>
                    <td className="user-created">{formatDateTime(u.creation)}</td>
                    <td className="user-login">
                      {u.lastLogin ? formatDateTime(u.lastLogin) : "Never"}
                    </td>
                    <td className="user-role">
                      {u.isAdmin ? (
                        <span className="admin-badge-small">⚡ Admin</span>
                      ) : (
                        <span className="user-badge">User</span>
                      )}
                    </td>
                    <td className="user-sessions">
                      <div className="user-actions">
                        <button
                          className="admin-btn admin-btn-secondary admin-btn-small"
                          onClick={() => revokeSessions(state, u)}
                          disabled={!!state.busy[u.id]}
                        >
                          {state.busy[u.id] === "revoke" ? "Revoking…" : "Revoke sessions"}
                        </button>
                        <button
                          className="admin-btn admin-btn-secondary admin-btn-small"
                          onClick={() => resendPasswordReset(state, u)}
                          disabled={!!state.busy[u.id]}
                        >
                          {state.busy[u.id] === "resend" ? "Sending…" : "Send reset link"}
                        </button>
                      </div>
                      {state.results[u.id] && (
                        <div className="session-result">{state.results[u.id]}</div>
                      )}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>

      <MailPanel state={state} />
    </div>
  );
};

const MailPanel = ({ state }: { state: UsersPageState }) => {
  const stats = state.mail;

  return (
    <div className="admin-section">
      <h2>Mail Delivery</h2>
      <p>
        Every account email — reset links, password-changed notices — goes through one worker. A
        message that was handed over is not a message that arrived; these are the attempts it
        actually made.
      </p>

      {state.mailError && (
        <div className="admin-notice">
          <strong>Delivery stats are unavailable</strong> — {state.mailError}
        </div>
      )}

      {stats && !stats.isRunning && (
        <div className="admin-notice">
          <strong>The mail worker is not running.</strong> Nothing queued will be delivered, and
          anything sent now goes out on the calling request instead.
        </div>
      )}

      {stats && stats.lastError && (
        <div className="admin-notice">
          <strong>Last error</strong> ({formatRelativeTime(stats.lastErrorAt, "never")}):{" "}
          {stats.lastError}
        </div>
      )}

      {stats && (
        <div className="photo-stats-grid">
          <div className="stat-card">
            <div className="stat-icon">📤</div>
            <div className="stat-content">
              <h3>Sent</h3>
              <div className="stat-value">{stats.sent.toLocaleString()}</div>
              <div className="stat-label">Since this process started</div>
            </div>
          </div>

          <div className="stat-card">
            <div className="stat-icon">{stats.failed > 0 ? "❌" : "✅"}</div>
            <div className="stat-content">
              <h3>Failed</h3>
              <div className="stat-value">{stats.failed.toLocaleString()}</div>
              <div className="stat-label">Gave up or was dropped</div>
            </div>
          </div>

          <div className="stat-card">
            <div className="stat-icon">🕒</div>
            <div className="stat-content">
              <h3>Last Sent</h3>
              <div className="stat-value">{formatRelativeTime(stats.lastSentAt, "never")}</div>
              <div className="stat-label">{stats.queueLength} waiting in the queue</div>
            </div>
          </div>
        </div>
      )}

      {stats && stats.recentAttempts.length > 0 && (
        <div className="admin-card">
          <div className="card-header">
            <div className="card-icon">📜</div>
            <h3>Recent Attempts</h3>
          </div>
          <div className="card-content">
            <ul className="error-list">
              {stats.recentAttempts.map(attempt => (
                <li key={`${attempt.to}-${attempt.time}`}>
                  {attempt.success ? "✅" : "❌"} {attempt.kind} → {attempt.to} ·{" "}
                  {formatRelativeTime(attempt.time, "never")}
                  {attempt.attempts > 1 && ` · ${attempt.attempts} tries`}
                  {attempt.error &&
                    ` · ${attempt.permanent ? "permanent" : "gave up"}: ${attempt.error}`}
                </li>
              ))}
            </ul>
          </div>
        </div>
      )}

      {stats && stats.recentAttempts.length === 0 && (
        <p className="last-reprocess">No mail has been attempted since this process started.</p>
      )}
    </div>
  );
};
