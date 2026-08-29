import { block } from "vlens/css";
import "./admin-tokens";

block(`
.push-config-card {
  background: var(--surface);
  border: 1px solid var(--admin-border);
  border-radius: 8px;
  padding: 1.5rem;
}
`);

block(`
.push-config-list {
  margin: 0;
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
  gap: 1rem 2rem;
}
`);

block(`
.push-config-row {
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
  min-width: 0;
}
`);

block(`
.push-config-row dt {
  font-size: 0.75rem;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: var(--muted);
}
`);

block(`
.push-config-row dd {
  margin: 0;
  color: var(--text);
  overflow-wrap: anywhere;
}
`);

block(`
.push-mono {
  font-family: monospace;
  font-size: 0.875rem;
}
`);

block(`
.push-apns-id {
  font-size: 0.75rem;
  color: var(--muted);
}
`);

block(`
.push-unset {
  color: var(--muted);
  font-style: italic;
}
`);

block(`
.push-ok {
  color: var(--admin-success);
  font-weight: 600;
}
`);

block(`
.push-bad {
  color: var(--admin-danger);
  font-weight: 600;
}
`);

block(`
.push-env {
  display: inline-block;
  padding: 0.125rem 0.5rem;
  border-radius: 4px;
  font-size: 0.75rem;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.03em;
  border: 1px solid var(--admin-border);
  color: var(--text);
}
`);

block(`
.push-env-production {
  background: rgba(5, 150, 105, 0.15);
  border-color: var(--admin-success);
}
`);

block(`
.push-env-sandbox {
  background: rgba(99, 102, 241, 0.15);
  border-color: var(--admin-accent);
}
`);

block(`
.push-mismatch {
  display: inline-block;
  margin-left: 0.5rem;
  font-size: 0.75rem;
  font-weight: 600;
  color: var(--admin-danger);
  cursor: help;
}
`);

block(`
.push-row-inactive {
  opacity: 0.55;
}
`);

block(`
.push-test-form {
  display: flex;
  gap: 0.75rem;
  flex-wrap: wrap;
  margin-top: 1rem;
}
`);

block(`
.push-test-input {
  flex: 1 1 320px;
  min-width: 0;
  padding: 0.6rem 0.75rem;
  border: 1px solid var(--admin-border);
  border-radius: 6px;
  background: var(--admin-surface-elevated);
  color: var(--text);
  font-size: 0.9rem;
}
`);

block(`
.push-result {
  margin-top: 1rem;
  padding: 0.75rem 1rem;
  border-radius: 6px;
  font-size: 0.9rem;
  border: 1px solid var(--admin-border);
}
`);

block(`
.push-result-ok {
  background: rgba(5, 150, 105, 0.12);
  border-color: var(--admin-success);
  color: var(--text);
}
`);

block(`
.push-result-bad {
  background: rgba(220, 38, 38, 0.12);
  border-color: var(--admin-danger);
  color: var(--text);
}
`);

block(`
.push-hint {
  margin-top: 1rem;
  color: var(--muted);
  font-size: 0.875rem;
}
`);

block(`
.push-section-note {
  color: var(--muted);
  font-size: 0.875rem;
  margin: 0 0 1rem;
}
`);
