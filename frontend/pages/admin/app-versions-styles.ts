import { block } from "vlens/css";

block(`
.version-explainer {
  background: var(--surface);
  border: 1px solid var(--admin-border);
  border-radius: 8px;
  padding: 1.25rem 1.5rem;
  margin-bottom: 1.5rem;
}
`);

block(`
.version-explainer dl {
  margin: 0;
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(240px, 1fr));
  gap: 1rem 2rem;
}
`);

block(`
.version-explainer dt {
  font-size: 0.75rem;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: var(--muted);
  margin-bottom: 0.25rem;
}
`);

block(`
.version-explainer dd {
  margin: 0;
  color: var(--text);
  font-size: 0.875rem;
}
`);

block(`
.version-state-set,
.version-state-unset {
  margin-left: auto;
  font-size: 0.75rem;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.03em;
}
`);

block(`
.version-state-set {
  color: var(--admin-success);
}
`);

block(`
.version-state-unset {
  color: var(--muted);
}
`);

block(`
.version-warnings {
  margin: 0 0 1.25rem;
  padding: 0.75rem 1rem 0.75rem 2rem;
  border: 1px solid var(--admin-danger);
  border-radius: 6px;
  background: rgba(220, 38, 38, 0.12);
  color: var(--text);
  font-size: 0.875rem;
}
`);

block(`
.version-warnings li + li {
  margin-top: 0.5rem;
}
`);

block(`
.version-form {
  display: flex;
  flex-direction: column;
  gap: 1.25rem;
}
`);

block(`
.version-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
  gap: 1.25rem;
}
`);

block(`
.version-hint {
  color: var(--muted);
  font-size: 0.8125rem;
}
`);

block(`
.version-actions {
  display: flex;
  align-items: center;
  gap: 1rem;
  flex-wrap: wrap;
}
`);

block(`
.version-result-ok {
  color: var(--admin-success);
  font-weight: 600;
  font-size: 0.875rem;
}
`);

block(`
.version-result-bad {
  color: var(--admin-danger);
  font-weight: 600;
  font-size: 0.875rem;
}
`);
