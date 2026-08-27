import { block } from "vlens/css";

block(`
.person-season-group {
  margin-bottom: 2.5rem;
}
`);

block(`
.person-season-head {
  display: flex;
  flex-wrap: wrap;
  align-items: baseline;
  gap: 0.25rem 0.75rem;
  padding-bottom: 0.5rem;
  margin-bottom: 1rem;
  border-bottom: 1px solid var(--border);
}
`);

block(`
.person-season-head h2 {
  margin: 0;
  font-size: 1.25rem;
}
`);

block(`
.person-season-when {
  font-size: 0.9rem;
  color: var(--muted);
}
`);

block(`
.person-entry-chips {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
  margin-bottom: 1rem;
}
`);

block(`
.person-entry-chip {
  display: inline-flex;
  flex-direction: column;
  gap: 0.1rem;
  padding: 0.4rem 0.75rem;
  border: 1px solid var(--border);
  border-radius: 999px;
  background: var(--surface);
  font-size: 0.9rem;
  color: var(--text);
  text-decoration: none;
}
`);

block(`
a.person-entry-chip:hover {
  border-color: var(--accent);
}
`);

block(`
.person-entry-chip-traits {
  font-size: 0.75rem;
  color: var(--muted);
}
`);

block(`
.person-appearance-entry {
  font-size: 0.85rem;
  color: var(--muted);
}
`);

block(`
.person-appearance-entry a {
  color: inherit;
}
`);

block(`
.person-season-subhead {
  margin: 1.5rem 0 0.75rem;
  font-size: 0.8rem;
  font-weight: 600;
  letter-spacing: 0.06em;
  text-transform: uppercase;
  color: var(--muted);
}
`);
