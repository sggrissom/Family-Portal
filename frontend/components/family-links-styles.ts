import { block } from "vlens/css";

block(`
.family-links {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}
`);

block(`
.family-link-card {
  border: 1px solid var(--border);
  border-radius: 8px;
  padding: 1rem;
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
  background: var(--surface);
}
`);

block(`
.family-link-heading {
  display: flex;
  flex-wrap: wrap;
  align-items: baseline;
  gap: 0.5rem;
  justify-content: space-between;
}
`);

block(`
.family-link-heading strong {
  font-size: 1.05rem;
  color: var(--text);
}
`);

block(`
.family-link-direction {
  font-size: 0.85rem;
  color: var(--muted);
}
`);

block(`
.family-link-status {
  font-size: 0.75rem;
  text-transform: uppercase;
  letter-spacing: 0.04em;
  padding: 0.15rem 0.5rem;
  border-radius: 999px;
  border: 1px solid var(--border);
  color: var(--muted);
}
`);

block(`
.family-link-status.is-pending {
  color: var(--warning, #b7791f);
  border-color: currentColor;
}
`);

block(`
.family-link-status.is-accepted {
  color: var(--success, #2f855a);
  border-color: currentColor;
}
`);

block(`
.family-link-scopes {
  display: flex;
  flex-wrap: wrap;
  gap: 0.75rem 1.25rem;
}
`);

block(`
.family-link-scopes label {
  display: flex;
  align-items: center;
  gap: 0.4rem;
  font-size: 0.9rem;
  color: var(--text);
}
`);

block(`
.family-link-note {
  font-size: 0.85rem;
  color: var(--muted);
  margin: 0;
}
`);

block(`
.family-link-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
}
`);

block(`
.person-sharing-list {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  margin-bottom: 1rem;
}
`);

block(`
.person-sharing-row {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 0.5rem;
  padding: 0.6rem 0.75rem;
  border: 1px solid var(--border);
  border-radius: 8px;
}
`);

block(`
.person-sharing-row .person-sharing-role {
  font-size: 0.85rem;
  color: var(--muted);
}
`);

block(`
.person-sharing-form {
  display: flex;
  flex-wrap: wrap;
  align-items: flex-end;
  gap: 0.75rem;
}
`);

block(`
.person-sharing-form > .form-group {
  flex: 1 1 100%;
}
`);

block(`
.relation-joiner {
  padding: 0 0.25rem;
}
`);
