import { block } from "vlens/css";

block(`
.family-members {
  list-style: none;
  margin: 0 0 1rem 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}
`);

block(`
.family-member {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  padding: 0.75rem 1rem;
  border: 1px solid var(--border);
  border-radius: 8px;
  background: var(--surface);
}
`);

block(`
.family-member-identity {
  display: flex;
  flex-direction: column;
  gap: 0.15rem;
  min-width: 0;
}
`);

block(`
.family-member-identity strong {
  color: var(--text);
}
`);

block(`
.family-member-email {
  color: var(--muted);
  font-size: 0.875rem;
  overflow-wrap: anywhere;
}
`);

block(`
.family-member-actions {
  display: flex;
  align-items: center;
  gap: 0.5rem;
}
`);

block(`
.family-member-badge {
  font-size: 0.75rem;
  text-transform: uppercase;
  letter-spacing: 0.04em;
  color: var(--muted);
  border: 1px solid var(--border);
  border-radius: 999px;
  padding: 0.15rem 0.6rem;
}
`);

block(`
.btn-small {
  padding: 0.35rem 0.75rem;
  font-size: 0.875rem;
}
`);

block(`
.family-member-leave {
  display: flex;
  justify-content: flex-start;
}
`);
