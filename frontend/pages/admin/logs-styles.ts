import { block } from "vlens/css"

// Logs page container
block(`
.logs-page {
  max-width: 1200px;
  margin: 0 auto;
  padding: 2rem 1rem;
}
`)

// Header section
block(`
.logs-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 2rem;
  padding-bottom: 1rem;
  border-bottom: 1px solid var(--border);
}
`)

block(`
.logs-header h1 {
  margin: 0;
  color: var(--admin-accent);
  font-size: 2rem;
}
`)

block(`
.back-link {
  color: var(--admin-accent);
  text-decoration: none;
  font-weight: 500;
  transition: color 0.2s ease;
}
`)

block(`
.back-link:hover {
  color: var(--primary-accent);
  text-decoration: underline;
}
`)

// Statistics section
block(`
.logs-stats {
  margin-bottom: 2rem;
}
`)

block(`
.stats-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(150px, 1fr));
  gap: 1rem;
}
`)

block(`
.stat-card {
  background: var(--admin-surface);
  border: 1px solid var(--admin-border);
  border-radius: 8px;
  padding: 1rem;
  text-align: center;
}
`)

block(`
.stat-number {
  font-size: 1.5rem;
  font-weight: bold;
  color: var(--admin-accent);
  margin-bottom: 0.25rem;
}
`)

block(`
.stat-label {
  font-size: 0.875rem;
  color: var(--muted);
  text-transform: uppercase;
  letter-spacing: 0.5px;
}
`)

// Controls section
block(`
.logs-controls {
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 8px;
  padding: 1.5rem;
  margin-bottom: 1.5rem;
}
`)

block(`
.logs-filters {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
  gap: 1rem;
  margin-bottom: 1rem;
}
`)

block(`
.filter-group {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}
`)

block(`
.filter-group label {
  font-weight: 500;
  color: var(--text);
  font-size: 0.875rem;
}
`)

block(`
.filter-group select {
  padding: 0.5rem;
  border: 1px solid var(--border);
  border-radius: 4px;
  background: var(--bg);
  color: var(--text);
  font-size: 0.875rem;
}
`)

block(`
.filter-group select:focus {
  outline: none;
  border-color: var(--admin-accent);
  box-shadow: 0 0 0 2px var(--admin-accent)20;
}
`)

// Pagination
block(`
.logs-pagination {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-top: 1rem;
  padding-top: 1rem;
  border-top: 1px solid var(--border);
}
`)

block(`
.pagination-info {
  font-size: 0.875rem;
  color: var(--muted);
}
`)

block(`
.pagination-controls {
  display: flex;
  gap: 0.5rem;
}
`)

block(`
.pagination-btn {
  padding: 0.5rem 1rem;
  border: 1px solid var(--border);
  border-radius: 4px;
  background: var(--bg);
  color: var(--text);
  cursor: pointer;
  font-size: 0.875rem;
  transition: all 0.2s ease;
}
`)

block(`
.pagination-btn:hover:not(:disabled) {
  background: var(--hover-bg);
  border-color: var(--admin-accent);
}
`)

block(`
.pagination-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}
`)

// Log content table
block(`
.logs-content {
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 8px;
  overflow: hidden;
}
`)

block(`
.logs-table {
  width: 100%;
}
`)

block(`
.table-header {
  display: grid;
  grid-template-columns: 180px 80px 100px 1fr 80px;
  gap: 1rem;
  padding: 1rem;
  background: var(--admin-surface);
  border-bottom: 1px solid var(--admin-border);
  font-weight: 600;
  font-size: 0.875rem;
  color: var(--admin-accent);
  text-transform: uppercase;
  letter-spacing: 0.5px;
}
`)

block(`
.table-row {
  display: grid;
  grid-template-columns: 180px 80px 100px 1fr 80px;
  gap: 1rem;
  padding: 1rem;
  border-bottom: 1px solid var(--border);
  font-size: 0.875rem;
  align-items: start;
}
`)

block(`
.table-row:last-child {
  border-bottom: none;
}
`)

block(`
.table-row:hover {
  background: var(--hover-bg);
}
`)

// Column specific styles
block(`
.col-timestamp {
  font-family: 'Monaco', 'Menlo', 'Ubuntu Mono', monospace;
  font-size: 0.8rem;
  color: var(--muted);
}
`)

block(`
.col-message {
  line-height: 1.4;
  word-break: break-word;
}
`)

block(`
.col-user {
  font-size: 0.8rem;
  color: var(--muted);
}
`)

// Log badges
block(`
.log-badge {
  display: inline-block;
  padding: 0.25rem 0.5rem;
  border-radius: 12px;
  font-size: 0.75rem;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.5px;
}
`)

block(`
.log-badge-error {
  background: #fee2e2;
  color: #dc2626;
}
`)

block(`
.log-badge-warn {
  background: #fef3c7;
  color: #d97706;
}
`)

block(`
.log-badge-info {
  background: #dbeafe;
  color: #2563eb;
}
`)

block(`
.log-badge-debug {
  background: #f3e8ff;
  color: #7c3aed;
}
`)

block(`
.log-badge-default {
  background: var(--hover-bg);
  color: var(--text);
}
`)

// Category badges
block(`
.log-category {
  display: inline-block;
  padding: 0.25rem 0.5rem;
  border-radius: 4px;
  font-size: 0.75rem;
  font-weight: 500;
  text-transform: uppercase;
  letter-spacing: 0.5px;
}
`)

block(`
.log-category-auth {
  background: #fef3c7;
  color: #92400e;
}
`)

block(`
.log-category-photo {
  background: #ecfdf5;
  color: #065f46;
}
`)

block(`
.log-category-admin {
  background: #f3e8ff;
  color: #6b21a8;
}
`)

block(`
.log-category-api {
  background: #dbeafe;
  color: #1e40af;
}
`)

block(`
.log-category-worker {
  background: #fff7ed;
  color: #9a3412;
}
`)

block(`
.log-category-system {
  background: #f1f5f9;
  color: #475569;
}
`)

// Log data display
block(`
.log-data {
  margin-top: 0.5rem;
  padding: 0.5rem;
  background: var(--hover-bg);
  border-radius: 4px;
  border-left: 3px solid var(--admin-accent);
}
`)

block(`
.log-data pre {
  margin: 0;
  font-family: 'Monaco', 'Menlo', 'Ubuntu Mono', monospace;
  font-size: 0.75rem;
  color: var(--muted);
  white-space: pre-wrap;
  word-break: break-all;
}
`)

// State messages
block(`
.loading-message,
.error-message,
.empty-logs {
  text-align: center;
  padding: 2rem;
  color: var(--muted);
}
`)

block(`
.error-message {
  background: #fee2e2;
  color: #dc2626;
  border: 1px solid #fecaca;
  border-radius: 8px;
  margin-bottom: 1rem;
}
`)

block(`
.error-icon {
  margin-right: 0.5rem;
}
`)

block(`
.loading-spinner {
  margin-right: 0.5rem;
  animation: spin 1s linear infinite;
}
`)

block(`
@keyframes spin {
  from { transform: rotate(0deg); }
  to { transform: rotate(360deg); }
}
`)

// Dark mode support
block(`
[data-theme="dark"] .stat-card {
  background: var(--admin-surface);
  border-color: var(--admin-border);
}
`)

block(`
[data-theme="dark"] .log-badge-error {
  background: #7f1d1d;
  color: #fca5a5;
}
`)

block(`
[data-theme="dark"] .log-badge-warn {
  background: #78350f;
  color: #fbbf24;
}
`)

block(`
[data-theme="dark"] .log-badge-info {
  background: #1e3a8a;
  color: #93c5fd;
}
`)

block(`
[data-theme="dark"] .log-badge-debug {
  background: #581c87;
  color: #c4b5fd;
}
`)

block(`
[data-theme="dark"] .log-category-auth {
  background: #78350f;
  color: #fbbf24;
}
`)

block(`
[data-theme="dark"] .log-category-photo {
  background: #064e3b;
  color: #6ee7b7;
}
`)

block(`
[data-theme="dark"] .log-category-admin {
  background: #581c87;
  color: #c4b5fd;
}
`)

block(`
[data-theme="dark"] .log-category-api {
  background: #1e3a8a;
  color: #93c5fd;
}
`)

block(`
[data-theme="dark"] .log-category-worker {
  background: #7c2d12;
  color: #fdba74;
}
`)

block(`
[data-theme="dark"] .log-category-system {
  background: #334155;
  color: #cbd5e1;
}
`)

// Mobile responsive design
block(`
@media (max-width: 768px) {
  .logs-header {
    flex-direction: column;
    align-items: flex-start;
    gap: 1rem;
  }

  .stats-grid {
    grid-template-columns: repeat(2, 1fr);
  }

  .logs-filters {
    grid-template-columns: 1fr;
  }

  .table-header,
  .table-row {
    grid-template-columns: 1fr;
    gap: 0.5rem;
  }

  .table-header {
    display: none;
  }

  .table-row {
    padding: 1rem;
    border-radius: 8px;
    margin-bottom: 0.5rem;
    border: 1px solid var(--border);
    background: var(--surface);
  }

  .col-timestamp::before {
    content: "Time: ";
    font-weight: 600;
    color: var(--text);
  }

  .col-level::before {
    content: "Level: ";
    font-weight: 600;
    color: var(--text);
  }

  .col-category::before {
    content: "Category: ";
    font-weight: 600;
    color: var(--text);
  }

  .col-message::before {
    content: "Message: ";
    font-weight: 600;
    color: var(--text);
  }

  .col-user::before {
    content: "User: ";
    font-weight: 600;
    color: var(--text);
  }

  .pagination-info {
    font-size: 0.8rem;
  }
}
`)

block(`
@media (max-width: 480px) {
  .logs-page {
    padding: 1rem 0.5rem;
  }

  .stats-grid {
    grid-template-columns: 1fr;
  }

  .logs-pagination {
    flex-direction: column;
    gap: 1rem;
    align-items: stretch;
  }

  .pagination-controls {
    justify-content: center;
  }
}`)