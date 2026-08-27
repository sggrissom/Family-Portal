import { block } from "vlens/css";
import "./admin-tokens";

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
.admin-card-icon {
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
.admin-family-badge {
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
.admin-empty-state {
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

block(`
.photo-stats-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
  gap: 1.5rem;
  margin-bottom: 2rem;
}
`);

block(`
.admin-stat-card {
  background: var(--admin-surface-elevated);
  border: 1px solid var(--admin-border);
  border-radius: 0.75rem;
  padding: 1.5rem;
  text-align: center;
  transition: transform 0.2s;
}
`);

block(`
.admin-stat-card:hover {
  transform: translateY(-2px);
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.1);
}
`);

block(`
.admin-stat-icon {
  font-size: 2rem;
  margin-bottom: 0.5rem;
  opacity: 0.8;
}
`);

block(`
.admin-stat-value {
  font-size: 2rem;
  font-weight: 700;
  color: var(--admin-accent);
  margin-bottom: 0.25rem;
}
`);

block(`
.admin-stat-label {
  font-size: 0.875rem;
  color: var(--muted);
  margin-top: 0.25rem;
}
`);

block(`
.reprocess-card {
  border-left: 4px solid var(--admin-accent);
  background: linear-gradient(135deg, var(--admin-surface-elevated) 0%, var(--admin-surface) 100%);
}
`);

block(`
.reprocess-progress {
  margin-top: 1rem;
}
`);

block(`
.progress-text {
  font-size: 0.875rem;
  color: var(--muted);
  text-align: center;
}
`);

block(`
.reprocess-actions {
  margin-top: 1rem;
  display: flex;
  flex-direction: column;
  gap: 1rem;
}
`);

block(`
.last-reprocess {
  font-size: 0.875rem;
  color: var(--muted);
  text-align: center;
}
`);

block(`
.error-card {
  border-left: 4px solid var(--admin-danger);
  background: rgba(220, 38, 38, 0.05);
}
`);

block(`
.error-list {
  list-style: none;
  padding: 0;
  margin: 0;
}
`);

block(`
.error-list li {
  background: rgba(220, 38, 38, 0.1);
  border: 1px solid rgba(220, 38, 38, 0.2);
  border-radius: 0.375rem;
  padding: 0.75rem;
  margin-bottom: 0.5rem;
  font-size: 0.875rem;
  font-family: monospace;
}
`);

block(`
.info-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(300px, 1fr));
  gap: 1.5rem;
}
`);

block(`
.info-card {
  background: var(--admin-surface-elevated);
  border: 1px solid var(--admin-border);
  border-radius: 0.5rem;
  padding: 1.5rem;
}
`);

block(`
.info-card h4 {
  margin: 0 0 1rem 0;
  color: var(--admin-accent);
  font-size: 1.125rem;
}
`);

block(`
.info-card ul {
  margin: 0;
  padding-left: 1.25rem;
}
`);

block(`
.info-card li {
  margin-bottom: 0.5rem;
  font-size: 0.9rem;
  line-height: 1.4;
}
`);

block(`
.admin-btn-primary {
  background: var(--admin-accent);
  color: var(--admin-text-on-accent);
  border: 1px solid var(--admin-accent);
}
`);

block(`
.admin-btn-primary:hover:not(:disabled) {
  background: var(--admin-accent-hover);
  border-color: var(--admin-accent-hover);
}
`);

block(`
@media (max-width: 768px) {
  .photo-stats-grid {
    grid-template-columns: repeat(2, 1fr);
    gap: 1rem;
  }

  .admin-stat-card {
    padding: 1rem;
  }

  .admin-stat-value {
    font-size: 1.5rem;
  }

  .info-grid {
    grid-template-columns: 1fr;
  }
}
`);

block(`
.admin-notice {
  background: var(--surface);
  border: 1px solid var(--border);
  border-left: 4px solid var(--weight-color);
  border-radius: 8px;
  padding: 14px 18px;
  margin-bottom: 20px;
  line-height: 1.6;
  color: var(--text);
}
`);

block(`
.admin-notice-ok {
  border-left-color: var(--accent);
}
`);

block(`
.diagnostics {
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 10px;
  padding: 18px 22px;
  margin-bottom: 24px;
}
`);

block(`
.diagnostics-primary {
  display: flex;
  align-items: baseline;
  flex-wrap: wrap;
  gap: 12px;
  margin-bottom: 16px;
}
`);

block(`
.diagnostics-version {
  font-size: 1.35rem;
  font-weight: 700;
  color: var(--hero);
}
`);

block(`
.diagnostics-commit {
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 0.95rem;
  color: var(--muted);
  user-select: all;
}
`);

block(`
.diagnostics-tag {
  font-size: 0.7rem;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  border: 1px solid var(--border);
  border-radius: 999px;
  padding: 3px 10px;
  color: var(--muted);
}
`);

block(`
.diagnostics-tag-warn {
  border-color: var(--weight-color);
  color: var(--weight-color);
}
`);

block(`
.diagnostics-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(170px, 1fr));
  gap: 14px 24px;
  margin: 0;
}
`);

block(`
.diagnostics-item dt {
  font-size: 0.72rem;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  color: var(--muted);
  margin-bottom: 2px;
}
`);

block(`
.diagnostics-item dd {
  margin: 0;
  color: var(--text);
  font-size: 0.95rem;
  overflow-wrap: anywhere;
}
`);

block(`
.problems {
  background: var(--admin-surface-elevated);
  border: 1px solid var(--admin-border);
  border-left: 3px solid var(--admin-danger);
  border-radius: 8px;
  padding: 1.25rem 1.5rem;
  margin-bottom: 2rem;
}
`);

block(`
.problems-clear {
  display: flex;
  align-items: center;
  gap: 0.6rem;
  border-left-color: var(--admin-success);
  padding: 0.9rem 1.25rem;
  font-size: 0.9rem;
  color: var(--muted);
}
`);

block(`
.problems-icon {
  font-size: 1rem;
}
`);

block(`
.problems-title {
  margin: 0 0 0.75rem;
  font-size: 1.05rem;
  color: var(--text);
}
`);

block(`
.problem-group {
  padding-top: 0.85rem;
  border-top: 1px solid var(--admin-border);
}
`);

block(`
.problem-group:first-of-type {
  padding-top: 0;
  border-top: none;
}
`);

block(`
.problem-group h3 {
  margin: 0 0 0.4rem;
  font-size: 0.85rem;
  text-transform: uppercase;
  letter-spacing: 0.04em;
  color: var(--muted);
}
`);

block(`
.problem-note {
  margin: 0 0 0.5rem;
  font-size: 0.85rem;
  color: var(--muted);
}
`);

block(`
.problem-list {
  margin: 0 0 0.6rem;
  padding-left: 1.1rem;
  color: var(--text);
  font-size: 0.92rem;
}
`);

block(`
.problem-list li {
  margin-bottom: 0.2rem;
}
`);

block(`
.problem-list code {
  font-family: monospace;
  font-size: 0.85rem;
}
`);

block(`
.problem-errors {
  margin-bottom: 0.6rem;
}
`);

block(`
.problem-error {
  display: flex;
  align-items: baseline;
  gap: 0.6rem;
  flex-wrap: wrap;
  padding: 0.3rem 0;
  font-size: 0.88rem;
  border-top: 1px solid var(--admin-border);
}
`);

block(`
.problem-error-time {
  color: var(--muted);
  font-size: 0.8rem;
  white-space: nowrap;
}
`);

block(`
.problem-error-message {
  flex: 1 1 12rem;
  color: var(--text);
}
`);

block(`
.problem-error-ref {
  font-family: monospace;
  font-size: 0.8rem;
  color: var(--admin-accent);
  text-decoration: none;
  border-bottom: 1px dashed var(--admin-accent);
}
`);

block(`
.problem-error-ref:hover {
  border-bottom-style: solid;
}
`);

block(`
.problem-action {
  display: inline-block;
  font-size: 0.85rem;
  color: var(--admin-accent);
  text-decoration: none;
}
`);

block(`
.problem-action:hover {
  text-decoration: underline;
}
`);

block(`
.admin-btn-small {
  padding: 0.3rem 0.7rem;
  font-size: 0.78rem;
}
`);

block(`
.session-result {
  margin-top: 0.25rem;
  font-size: 0.75rem;
  color: var(--muted);
}
`);

block(`
.user-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 0.375rem;
}
`);

block(`
.releases {
  display: flex;
  align-items: baseline;
  flex-wrap: wrap;
  gap: 0.5rem 1.25rem;
  margin-bottom: 24px;
  padding: 14px 22px;
  border: 1px solid var(--border);
  border-radius: 10px;
  background: var(--surface);
}
`);

block(`
.releases-title {
  margin: 0;
  font-size: 0.72rem;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  color: var(--muted);
}
`);

block(`
.releases-list {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem 1.25rem;
  margin: 0;
  padding: 0;
  list-style: none;
}
`);

block(`
.release {
  display: flex;
  align-items: baseline;
  gap: 0.4rem;
  font-size: 0.85rem;
  color: var(--muted);
}
`);

block(`
.release-current {
  color: var(--text);
}
`);

block(`
.release-sha {
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  user-select: all;
}
`);

block(`
.release-tag {
  font-size: 0.65rem;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  border: 1px solid var(--admin-success);
  border-radius: 999px;
  padding: 1px 8px;
  color: var(--admin-success);
}
`);

block(`
.digest-people {
  margin: 1rem 0 0.5rem;
  padding: 0;
  list-style: none;
}
`);

block(`
.digest-person {
  display: flex;
  align-items: baseline;
  flex-wrap: wrap;
  gap: 0.25rem 0.75rem;
  padding: 0.4rem 0;
  border-top: 1px solid var(--admin-border);
}
`);

block(`
.digest-person:first-child {
  border-top: none;
}
`);

block(`
.digest-person-name {
  font-weight: 600;
  color: var(--text);
  min-width: 8rem;
}
`);

block(`
.digest-person-what {
  color: var(--text);
  font-size: 0.9rem;
}
`);

block(`
.digest-person-when {
  margin-left: auto;
  color: var(--muted);
  font-size: 0.82rem;
}
`);
