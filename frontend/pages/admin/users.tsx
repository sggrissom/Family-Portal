import * as preact from "preact";
import * as rpc from "vlens/rpc";
import * as vlens from "vlens";
import * as auth from "../../lib/authCache";
import * as server from "../../server";
import { Header, Footer } from "../../layout";
import { ensureAuthInFetch, requireAuthInView } from "../../lib/authHelpers";
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
        <UserManagementPage user={currentAuth} data={data} />
      </main>
      <Footer />
    </div>
  );
}

interface UserManagementPageProps {
  user: auth.AuthCache;
  data: server.ListAllUsersResponse;
}

// Per-user outcomes, keyed by id, so one revoked account's message does not
// appear on another's row.
type UsersPageState = {
  busy: { [userId: number]: boolean };
  results: { [userId: number]: string };
};

const useUsersPageState = vlens.declareHook((): UsersPageState => ({ busy: {}, results: {} }));

// Signing someone out everywhere: for a lost phone, or a session that outlived
// a JWT secret change. The access token is a short-lived JWT this cannot
// recall, so the effect lands when it expires — deleting the refresh tokens is
// what stops it being renewed, and the confirm says so.
async function revokeSessions(state: UsersPageState, u: server.AdminUserInfo) {
  const confirmed = confirm(
    `Sign ${u.name} out of every device?\n\n` +
      "Their refresh tokens are deleted immediately. The access token they already hold keeps " +
      "working until it expires, within the hour."
  );
  if (!confirmed) return;

  state.busy[u.id] = true;
  state.results[u.id] = "";
  vlens.scheduleRedraw();

  const [result, error] = await server.RevokeUserSessions({ userId: u.id });
  state.busy[u.id] = false;
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

  const formatDate = (dateString: string) => {
    const date = new Date(dateString);
    return (
      date.toLocaleDateString() +
      " " +
      date.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" })
    );
  };

  return (
    <div className="admin-page">
      {/* Breadcrumb Navigation */}
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
                  <th>Sessions</th>
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
                    <td className="user-created">{formatDate(u.creation)}</td>
                    <td className="user-login">
                      {u.lastLogin ? formatDate(u.lastLogin) : "Never"}
                    </td>
                    <td className="user-role">
                      {u.isAdmin ? (
                        <span className="admin-badge-small">⚡ Admin</span>
                      ) : (
                        <span className="user-badge">User</span>
                      )}
                    </td>
                    <td className="user-sessions">
                      <button
                        className="admin-btn admin-btn-secondary admin-btn-small"
                        onClick={() => revokeSessions(state, u)}
                        disabled={state.busy[u.id]}
                      >
                        {state.busy[u.id] ? "Revoking…" : "Revoke"}
                      </button>
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
    </div>
  );
};
