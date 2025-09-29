import { block } from "vlens/css";

block(`
.settings-container {
  max-width: 800px;
  margin: 0 auto;
  padding: 2rem 1rem;
}
`);

block(`
.settings-page {
  display: flex;
  flex-direction: column;
  gap: 2rem;
}
`);

block(`
.settings-header {
  text-align: center;
  margin-bottom: 2rem;
}
`);

block(`
.settings-header h1 {
  font-size: 2.5rem;
  font-weight: 700;
  color: var(--text);
  margin-bottom: 0.5rem;
}
`);

block(`
.settings-header p {
  font-size: 1.125rem;
  color: var(--muted);
}
`);

block(`
.settings-sections {
  display: flex;
  flex-direction: column;
  gap: 2rem;
}
`);

block(`
.settings-section h2 {
  font-size: 1.5rem;
  font-weight: 600;
  color: var(--text);
  margin-bottom: 1rem;
}
`);

block(`
.settings-card {
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 8px;
  padding: 1.5rem;
}
`);

block(`
.section-description {
  color: var(--muted);
  margin-bottom: 1.5rem;
  line-height: 1.5;
}
`);

block(`
.readonly-field {
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 4px;
  padding: 0.75rem;
  color: var(--text);
  font-size: 1rem;
}
`);

block(`
.invite-code-display {
  display: flex;
  align-items: center;
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 4px;
  padding: 0.75rem;
}
`);

block(`
.invite-code {
  font-family: 'Monaco', 'Menlo', 'Ubuntu Mono', monospace;
  font-size: 1.125rem;
  font-weight: 600;
  color: var(--primary-accent);
  letter-spacing: 0.05em;
}
`);

block(`
.invite-link-display {
  display: flex;
  gap: 0.5rem;
}
`);

block(`
.invite-link-input {
  flex: 1;
  padding: 0.75rem;
  border: 1px solid var(--border);
  border-radius: 4px;
  background: var(--surface);
  color: var(--text);
  font-size: 0.875rem;
  font-family: 'Monaco', 'Menlo', 'Ubuntu Mono', monospace;
}
`);

block(`
.copy-button {
  min-width: 100px;
  transition: all 0.2s ease;
}
`);

block(`
.copy-button.copied {
  background: var(--accent);
  border-color: var(--accent);
  color: var(--button-text);
}
`);

block(`
.invite-instructions {
  margin-top: 1.5rem;
  padding-top: 1.5rem;
  border-top: 1px solid var(--border);
}
`);

block(`
.invite-instructions h4 {
  color: var(--text);
  margin-bottom: 0.75rem;
  font-size: 1rem;
  font-weight: 600;
}
`);

block(`
.invite-instructions ol {
  color: var(--muted);
  padding-left: 1.25rem;
  line-height: 1.6;
}
`);

block(`
.invite-instructions li {
  margin-bottom: 0.5rem;
}
`);

block(`
.data-management-actions {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 2rem;
}
`);

block(`
.data-action {
  padding: 1.5rem;
  border: 1px solid var(--border);
  border-radius: 6px;
  background: var(--bg);
}
`);

block(`
.data-action h4 {
  color: var(--text);
  font-size: 1.125rem;
  font-weight: 600;
  margin-bottom: 0.5rem;
}
`);

block(`
.data-action p {
  color: var(--muted);
  margin-bottom: 1rem;
  line-height: 1.5;
  font-size: 0.875rem;
}
`);

block(`
.merge-card {
  border-color: var(--warning, #ff8c00);
  background: var(--surface);
}
`);

block(`
.warning-banner {
  background: rgba(255, 140, 0, 0.1);
  border: 1px solid rgba(255, 140, 0, 0.3);
  border-radius: 6px;
  padding: 1rem;
  margin-bottom: 1.5rem;
  color: var(--warning, #ff8c00);
  font-size: 0.95rem;
}
`);

block(`
.merge-selectors {
  display: grid;
  grid-template-columns: 1fr auto 1fr;
  gap: 1rem;
  align-items: end;
  margin-bottom: 1.5rem;
}
`);

block(`
.merge-arrow {
  font-size: 2rem;
  color: var(--primary-accent);
  padding-bottom: 0.5rem;
  font-weight: bold;
}
`);

block(`
.merge-confirmation {
  border-top: 2px solid var(--border);
  padding-top: 1.5rem;
}
`);

block(`
.merge-confirmation h4 {
  color: var(--text);
  font-size: 1.125rem;
  font-weight: 600;
  margin-bottom: 1rem;
}
`);

block(`
.merge-preview {
  background: var(--bg);
  border: 1px solid var(--border);
  border-radius: 6px;
  padding: 1.5rem;
  margin-bottom: 1.5rem;
}
`);

block(`
.merge-preview p {
  color: var(--text);
  margin-bottom: 0.75rem;
}
`);

block(`
.merge-stats {
  margin-top: 1rem;
  padding-top: 1rem;
  border-top: 1px solid var(--border);
}
`);

block(`
.merge-stats p {
  color: var(--muted);
  font-weight: 600;
  margin-bottom: 0.5rem;
}
`);

block(`
.merge-stats ul {
  list-style-type: disc;
  padding-left: 1.5rem;
  color: var(--text);
}
`);

block(`
.merge-stats li {
  margin-bottom: 0.25rem;
}
`);

block(`
.confirmation-actions {
  display: flex;
  gap: 1rem;
}
`);

block(`
.btn-warning {
  background: var(--warning, #ff8c00);
  color: white;
  border: 1px solid var(--warning, #ff8c00);
}
`);

block(`
.btn-warning:hover:not(:disabled) {
  background: rgba(255, 140, 0, 0.8);
  border-color: rgba(255, 140, 0, 0.8);
}
`);

block(`
.btn-danger {
  background: var(--error, #dc3545);
  color: white;
  border: 1px solid var(--error, #dc3545);
}
`);

block(`
.btn-danger:hover:not(:disabled) {
  background: rgba(220, 53, 69, 0.8);
  border-color: rgba(220, 53, 69, 0.8);
}
`);

block(`
@media (max-width: 600px) {
  .settings-container {
    padding: 1rem 0.75rem;
  }

  .settings-header h1 {
    font-size: 2rem;
  }

  .invite-link-display {
    flex-direction: column;
  }

  .copy-button {
    min-width: auto;
  }

  .data-management-actions {
    grid-template-columns: 1fr;
    gap: 1rem;
  }

  .data-action {
    padding: 1rem;
  }

  .merge-selectors {
    grid-template-columns: 1fr;
    gap: 0.5rem;
  }

  .merge-arrow {
    display: none;
  }

  .confirmation-actions {
    flex-direction: column;
  }
}
`);
