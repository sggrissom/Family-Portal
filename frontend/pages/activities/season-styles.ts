import { block } from "vlens/css";

block(`
.back-link {
  display: inline-block;
  margin-bottom: 1rem;
  color: var(--muted);
  text-decoration: none;
  font-size: 0.9rem;
}
`);

block(`
.back-link:hover {
  color: var(--text);
}
`);

block(`
.season-header {
  margin-bottom: 2rem;
}
`);

block(`
.season-header h1 {
  margin: 0.15rem 0 0.35rem;
}
`);

block(`
.season-eyebrow {
  text-transform: uppercase;
  letter-spacing: 0.08em;
  font-size: 0.75rem;
  color: var(--muted);
}
`);

block(`
.season-header .season-dates {
  margin: 0;
}
`);

block(`
.season-header .season-notes {
  margin-top: 0.5rem;
}
`);

block(`
.event-list {
  list-style: none;
  margin: 0;
  padding: 0;
}
`);

block(`
.event-item {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 1rem;
  padding: 0.85rem 0;
  border-bottom: 1px solid var(--border);
}
`);

block(`
.event-item-main {
  display: flex;
  flex-direction: column;
  gap: 0.2rem;
  min-width: 0;
}
`);

block(`
.event-name {
  color: var(--text);
  font-weight: 600;
  text-decoration: none;
}
`);

block(`
a.event-name:hover {
  text-decoration: underline;
}
`);

block(`
.event-meta {
  color: var(--muted);
  font-size: 0.85rem;
}
`);

block(`
.event-count {
  color: var(--muted);
  font-size: 0.8rem;
}
`);

block(`
.event-notes {
  margin: 0.25rem 0 0;
  color: var(--muted);
  font-size: 0.9rem;
  white-space: pre-wrap;
}
`);

block(`
.event-item-actions {
  display: flex;
  gap: 0.15rem;
  flex-shrink: 0;
}
`);

block(`
.roster-picker {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem 1rem;
}
`);

block(`
.roster-option {
  display: flex;
  align-items: center;
  gap: 0.4rem;
  font-size: 0.9rem;
  color: var(--text);
  cursor: pointer;
}
`);

block(`
.roster-option input[type="checkbox"] {
  cursor: pointer;
}
`);
