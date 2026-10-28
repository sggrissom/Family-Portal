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
.person-season-title {
  display: grid;
  gap: 0.1rem;
}
`);

block(`
.person-season-title strong {
  font-size: 1.2rem;
}
`);

block(`
.person-past-heading {
  margin-top: 1rem;
}
`);

block(`
.person-season-past {
  margin-bottom: 0.75rem;
  padding: 0.85rem 1rem;
  border: 1px solid var(--border);
  border-radius: 14px;
  background: var(--surface);
}
`);

block(`
.person-season-past summary {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  cursor: pointer;
  list-style: none;
}
`);

block(`
.person-season-past summary::-webkit-details-marker {
  display: none;
}
`);

block(`
.person-season-past[open] summary {
  margin-bottom: 1rem;
}
`);

block(`
.person-season-past[open] summary svg {
  transform: rotate(90deg);
}
`);

block(`
.person-season-past > .link-action {
  display: inline-block;
  margin-bottom: 0.75rem;
}
`);
