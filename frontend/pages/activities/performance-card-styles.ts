import { block } from "vlens/css";

block(`
.date-block {
  display: flex;
  flex-direction: column;
  align-items: center;
  flex: 0 0 2.75rem;
  line-height: 1.1;
}
`);

block(`
.date-block small {
  color: var(--muted);
  font-size: 0.7rem;
  text-transform: uppercase;
  letter-spacing: 0.05em;
}
`);

block(`
.date-block strong {
  font-size: 1.2rem;
}
`);

block(`
.perf-run-summary {
  margin: 0;
  font-weight: 600;
}
`);

block(`
.perf-date {
  color: var(--muted);
  font-size: 0.85rem;
}
`);

block(`
.perf-notes {
  margin: 0;
  color: var(--muted);
  font-size: 0.9rem;
  white-space: pre-wrap;
}
`);

block(`
.story-cards {
  display: grid;
  gap: 0.75rem;
}
`);

block(`
.story-card {
  display: flex;
  gap: 0.85rem;
  padding: 1rem;
  border: 1px solid var(--border);
  border-radius: 14px;
  background: var(--surface);
  min-width: 0;
}
`);

block(`
.story-card-body {
  display: grid;
  gap: 0.6rem;
  flex: 1;
  min-width: 0;
}
`);

block(`
.story-card-head {
  display: grid;
  gap: 0.1rem;
}
`);

block(`
.story-card-event {
  color: var(--text);
  font-weight: 700;
  font-size: 1.05rem;
  text-decoration: none;
}
`);

block(`
a.story-card-event:hover {
  text-decoration: underline;
}
`);

block(`
.story-card-where {
  color: var(--muted);
  font-size: 0.85rem;
}
`);

block(`
.story-card-entry {
  color: var(--accent);
  font-size: 0.85rem;
  font-weight: 600;
  text-decoration: none;
}
`);

block(`
.story-card-performance {
  display: grid;
  gap: 0.4rem;
}
`);

block(`
.stat-tiles {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(9.5rem, 1fr));
  gap: 0.6rem;
  margin-bottom: 2rem;
}
`);

block(`
.stat-tile {
  display: grid;
  align-content: start;
  gap: 0.1rem;
  padding: 0.75rem 0.9rem;
  border: 1px solid var(--border);
  border-radius: 12px;
  background: var(--surface);
  color: var(--text);
  text-decoration: none;
}
`);

block(`
a.stat-tile:hover {
  border-color: var(--accent);
}
`);

block(`
.stat-tile small {
  color: var(--muted);
  font-size: 0.72rem;
  text-transform: uppercase;
  letter-spacing: 0.05em;
}
`);

block(`
.stat-tile strong {
  font-size: 1.3rem;
}
`);

block(`
.stat-tile span {
  color: var(--muted);
  font-size: 0.8rem;
}
`);
