import { block } from "vlens/css";
import "./admin-tokens";

block(`
.seed-form {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(240px, 1fr));
  gap: 1.25rem;
  margin-bottom: 1.25rem;
}
`);

block(`
.seed-field {
  display: flex;
  flex-direction: column;
  gap: 0.35rem;
  min-width: 0;
}
`);

block(`
.seed-field-narrow {
  max-width: 200px;
}
`);

block(`
.seed-label {
  font-size: 0.75rem;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: var(--muted);
}
`);

block(`
.seed-input {
  padding: 0.6rem 0.75rem;
  border: 1px solid var(--admin-border);
  border-radius: 6px;
  background: var(--surface);
  color: var(--text);
  font-size: 0.95rem;
  width: 100%;
}
`);

block(`
.seed-input:focus {
  outline: 2px solid var(--admin-accent);
  outline-offset: 1px;
}
`);

block(`
.seed-input-small {
  padding: 0.35rem 0.5rem;
  font-size: 0.85rem;
  max-width: 220px;
}
`);

block(`
.seed-hint {
  font-size: 0.8rem;
  color: var(--muted);
  overflow-wrap: anywhere;
}
`);

block(`
.seed-actions {
  display: flex;
  gap: 0.75rem;
  margin: 1rem 0;
  flex-wrap: wrap;
}
`);

block(`
.seed-note {
  font-size: 0.875rem;
  color: var(--muted);
}
`);

block(`
.seed-mono {
  font-family: monospace;
  font-size: 0.875rem;
  overflow-wrap: anywhere;
}
`);

block(`
.seed-result {
  margin-top: 1rem;
  padding: 0.75rem 1rem;
  border-radius: 6px;
  font-size: 0.9rem;
  border: 1px solid var(--admin-border);
}
`);

block(`
.seed-result-ok {
  background: rgba(5, 150, 105, 0.12);
  border-color: var(--admin-success);
  color: var(--text);
}
`);

block(`
.seed-result-bad {
  background: rgba(220, 38, 38, 0.12);
  border-color: var(--admin-danger);
  color: var(--text);
}
`);

block(`
.seed-remove {
  display: flex;
  gap: 0.5rem;
  align-items: center;
  flex-wrap: wrap;
}
`);

block(`
.seed-btn-danger {
  background: var(--admin-danger);
  border-color: var(--admin-danger);
  color: var(--admin-text-on-accent);
}
`);

block(`
.seed-btn-danger:hover:not(:disabled) {
  background: var(--admin-danger-hover);
  border-color: var(--admin-danger-hover);
}
`);
