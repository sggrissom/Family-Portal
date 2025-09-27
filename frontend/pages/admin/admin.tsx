import * as preact from "preact";
import * as rpc from "vlens/rpc";
import * as auth from "../../lib/authCache";
import * as core from "vlens/core";
import * as server from "../../server";
import { Header, Footer } from "../../layout";
import { ensureAuthInFetch, requireAuthInView } from "../../lib/authHelpers";
import "./admin-styles";

export async function fetch(route: string, prefix: string) {
  return rpc.ok<{}>({});
}

export function view(
  route: string,
  prefix: string,
  data: {},
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
            <a href="/dashboard" className="btn btn-primary">Return to Dashboard</a>
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
        <AdminPage user={currentAuth} />
      </main>
      <Footer />
    </div>
  );
}

interface AdminPageProps {
  user: auth.AuthCache;
}

const AdminPage = ({ user }: AdminPageProps) => {
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

      <div className="admin-grid">
        <a href="/admin/analytics" className="admin-card admin-card-link">
          <div className="card-header">
            <div className="card-icon">📊</div>
            <h3>Site Analytics</h3>
          </div>
          <div className="card-content">
            <p>View site usage statistics, user activity, and performance metrics.</p>
            <div className="card-action">
              View Analytics Dashboard →
            </div>
          </div>
        </a>

        <a href="/admin/users" className="admin-card admin-card-link">
          <div className="card-header">
            <div className="card-icon">👥</div>
            <h3>User Management</h3>
          </div>
          <div className="card-content">
            <p>Manage user accounts, family groups, and permissions.</p>
            <div className="card-action">
              View All Users →
            </div>
          </div>
        </a>

        <a href="/admin/photos" className="admin-card admin-card-link">
          <div className="card-header">
            <div className="card-icon">🖼️</div>
            <h3>Photo Management</h3>
          </div>
          <div className="card-content">
            <p>Reprocess photos with modern formats and optimized sizes.</p>
            <div className="card-action">
              Manage Photos →
            </div>
          </div>
        </a>

        <div className="admin-card">
          <div className="card-header">
            <div className="card-icon">🛠️</div>
            <h3>System Settings</h3>
          </div>
          <div className="card-content">
            <p>Configure system-wide settings and maintenance options.</p>
            <div className="card-placeholder">
              Coming Soon
            </div>
          </div>
        </div>

        <a href="/admin/logs" className="admin-card admin-card-link">
          <div className="card-header">
            <div className="card-icon">📋</div>
            <h3>System Logs</h3>
          </div>
          <div className="card-content">
            <p>Review application logs and system events.</p>
            <div className="card-action">
              View System Logs →
            </div>
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
