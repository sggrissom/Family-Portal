import { block } from "vlens/css";

block(`
:root {
  --admin-accent: #6366f1;
  --admin-accent-hover: #4f46e5;
  --admin-danger: #dc2626;
  --admin-danger-hover: #b91c1c;
  --admin-success: #059669;
  --admin-border: #d1d5db;
  --admin-surface: #f9fafb;
  --admin-surface-elevated: #ffffff;
  --admin-text-on-accent: #ffffff;
}
`);

block(`
[data-theme="dark"] {
  --admin-accent: #818cf8;
  --admin-accent-hover: #6366f1;
  --admin-danger: #f87171;
  --admin-danger-hover: #ef4444;
  --admin-success: #34d399;
  --admin-border: #4b5563;
  --admin-surface: #111827;
  --admin-surface-elevated: #1f2937;
  --admin-text-on-accent: #ffffff;
}
`);

block(`
.admin-container {
  max-width: 1200px;
  margin: 0 auto;
  padding: 2rem;
  min-height: calc(100vh - 200px);
}
`);

block(`
.admin-page {
  background: var(--bg);
  border-radius: 8px;
  overflow: hidden;
}
`);

block(`
.admin-header {
  background: linear-gradient(135deg, var(--admin-accent) 0%, var(--admin-accent-hover) 100%);
  color: var(--admin-text-on-accent);
  padding: 2rem;
  text-align: center;
  margin-bottom: 2rem;
  border-radius: 8px;
  box-shadow: 0 4px 6px rgba(0, 0, 0, 0.1);
}
`);

block(`
.admin-badge {
  display: inline-flex;
  align-items: center;
  gap: 0.5rem;
  background: rgba(255, 255, 255, 0.2);
  padding: 0.5rem 1rem;
  border-radius: 20px;
  font-weight: 600;
  font-size: 0.875rem;
  margin-bottom: 1rem;
  backdrop-filter: blur(10px);
}
`);

block(`
.admin-icon {
  font-size: 1rem;
}
`);

block(`
.admin-header h1 {
  margin: 0 0 0.5rem 0;
  font-size: 2.5rem;
  font-weight: 700;
  text-shadow: 0 2px 4px rgba(0, 0, 0, 0.1);
}
`);

block(`
.admin-header p {
  margin: 0;
  font-size: 1.1rem;
  opacity: 0.9;
}
`);

block(`
.admin-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(300px, 1fr));
  gap: 1.5rem;
  margin-bottom: 2rem;
}
`);

block(`
.admin-card {
  background: var(--surface);
  border: 1px solid var(--admin-border);
  border-radius: 8px;
  padding: 1.5rem;
  transition: all 0.3s ease;
  box-shadow: 0 2px 4px rgba(0, 0, 0, 0.1);
}
`);

block(`
.admin-card:hover {
  border-color: var(--admin-accent);
  box-shadow: 0 4px 8px rgba(0, 0, 0, 0.15);
  transform: translateY(-2px);
}
`);

block(`
.admin-card-link {
  text-decoration: none;
  color: inherit;
  display: block;
  cursor: pointer;
}
`);

block(`
.admin-card-link:hover .card-action {
  color: var(--admin-accent);
  text-decoration: underline;
}
`);

block(`
.card-action {
  font-weight: 600;
  color: var(--admin-accent);
  margin-top: 1rem;
  font-size: 0.875rem;
  transition: all 0.2s ease;
}
`);

block(`
.card-header {
  display: flex;
  align-items: center;
  gap: 1rem;
  margin-bottom: 1rem;
  padding-bottom: 1rem;
  border-bottom: 1px solid var(--admin-border);
}
`);

block(`
.card-icon {
  font-size: 2rem;
  width: 3rem;
  height: 3rem;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--admin-accent);
  color: var(--admin-text-on-accent);
  border-radius: 8px;
  font-weight: bold;
}
`);

block(`
.card-header h3 {
  margin: 0;
  font-size: 1.25rem;
  font-weight: 600;
  color: var(--text);
}
`);

block(`
.card-content p {
  margin: 0 0 1rem 0;
  color: var(--muted);
  line-height: 1.6;
}
`);

block(`
.card-placeholder {
  background: var(--admin-surface);
  border: 2px dashed var(--admin-border);
  border-radius: 4px;
  padding: 2rem;
  text-align: center;
  color: var(--muted);
  font-style: italic;
  font-size: 1rem;
}
`);

block(`
.admin-section {
  background: var(--surface);
  border: 1px solid var(--admin-border);
  border-radius: 8px;
  padding: 2rem;
  margin-top: 2rem;
}
`);

block(`
.admin-section h2 {
  margin: 0 0 1.5rem 0;
  font-size: 1.5rem;
  font-weight: 600;
  color: var(--text);
  border-bottom: 2px solid var(--admin-accent);
  padding-bottom: 0.5rem;
}
`);

block(`
.admin-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 1rem;
}
`);

block(`
.admin-btn {
  padding: 0.75rem 1.5rem;
  border: none;
  border-radius: 6px;
  font-size: 0.875rem;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.2s ease;
  text-transform: uppercase;
  letter-spacing: 0.05em;
}
`);

block(`
.admin-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}
`);

block(`
.admin-btn-secondary {
  background: var(--admin-surface);
  color: var(--text);
  border: 1px solid var(--admin-border);
}
`);

block(`
.admin-btn-secondary:hover:not(:disabled) {
  background: var(--admin-surface-elevated);
  border-color: var(--admin-accent);
}
`);

block(`
.admin-btn-danger {
  background: var(--admin-danger);
  color: white;
  border: 1px solid var(--admin-danger);
}
`);

block(`
.admin-btn-danger:hover:not(:disabled) {
  background: var(--admin-danger-hover);
  border-color: var(--admin-danger-hover);
}
`);

block(`
.admin-breadcrumb {
  margin-bottom: 1rem;
  color: var(--muted);
  font-size: 0.875rem;
}
`);

block(`
.admin-breadcrumb a {
  color: var(--admin-accent);
  text-decoration: none;
}
`);

block(`
.admin-breadcrumb a:hover {
  text-decoration: underline;
}
`);

block(`
.breadcrumb-separator {
  margin: 0 0.5rem;
  color: var(--muted);
}
`);

block(`
.users-table-container {
  background: var(--surface);
  border: 1px solid var(--admin-border);
  border-radius: 8px;
  overflow: hidden;
}
`);

block(`
.table-wrapper {
  overflow-x: auto;
}
`);

block(`
.users-table {
  width: 100%;
  border-collapse: collapse;
}
`);

block(`
.users-table th {
  background: var(--admin-surface);
  color: var(--text);
  font-weight: 600;
  padding: 1rem;
  text-align: left;
  border-bottom: 2px solid var(--admin-border);
  font-size: 0.875rem;
  text-transform: uppercase;
  letter-spacing: 0.05em;
}
`);

block(`
.users-table td {
  padding: 1rem;
  border-bottom: 1px solid var(--admin-border);
  color: var(--text);
}
`);

block(`
.users-table tr:hover {
  background: var(--hover-bg);
}
`);

block(`
.admin-row {
  background: rgba(99, 102, 241, 0.1);
}
`);

block(`
[data-theme="dark"] .admin-row {
  background: rgba(129, 140, 248, 0.1);
}
`);

block(`
.user-id {
  font-weight: 600;
  font-family: monospace;
  color: var(--text);
}
`);

block(`
.user-table-name {
  font-weight: 500;
  color: var(--text);
}
`);

block(`
.user-email {
  color: var(--muted);
  font-size: 0.875rem;
}
`);

block(`
.family-name {
  background: var(--admin-accent);
  color: var(--admin-text-on-accent);
  padding: 0.25rem 0.5rem;
  border-radius: 4px;
  font-size: 0.75rem;
  font-weight: 600;
}
`);

block(`
.no-family {
  color: var(--muted);
  font-style: italic;
  font-size: 0.875rem;
}
`);

block(`
.user-created,
.user-login {
  font-size: 0.875rem;
  color: var(--muted);
}
`);

block(`
.admin-badge-small {
  background: var(--admin-accent);
  color: var(--admin-text-on-accent);
  padding: 0.25rem 0.5rem;
  border-radius: 4px;
  font-size: 0.75rem;
  font-weight: 600;
  display: inline-flex;
  align-items: center;
  gap: 0.25rem;
}
`);

block(`
.user-badge {
  background: var(--admin-surface-elevated);
  color: var(--text);
  padding: 0.25rem 0.5rem;
  border-radius: 4px;
  font-size: 0.75rem;
  font-weight: 500;
  border: 1px solid var(--admin-border);
}
`);

block(`
.empty-state {
  padding: 2rem;
  text-align: center;
  color: var(--muted);
}
`);

block(`
@media (max-width: 768px) {
  .admin-container {
    padding: 1rem;
  }

  .admin-header {
    padding: 1.5rem;
  }

  .admin-header h1 {
    font-size: 2rem;
  }

  .admin-grid {
    grid-template-columns: 1fr;
    gap: 1rem;
  }

  .admin-card {
    padding: 1rem;
  }

  .admin-actions {
    flex-direction: column;
  }

  .admin-btn {
    width: 100%;
  }

  .users-table th,
  .users-table td {
    padding: 0.75rem 0.5rem;
    font-size: 0.875rem;
  }

  .users-table th {
    font-size: 0.75rem;
  }
}
`);