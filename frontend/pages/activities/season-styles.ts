import { block } from "vlens/css";

block(`
.activities-container .back-link {
  display: inline-block;
  margin-bottom: 1rem;
  color: var(--muted);
  text-decoration: none;
  font-size: 0.9rem;
}
`);

block(`
.activities-container .back-link:hover {
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

block(`
.results-grid-scroll {
  overflow-x: auto;
  border: 1px solid var(--border);
  border-radius: 14px;
  background: var(--surface);
}
`);

block(`
.results-grid {
  border-collapse: collapse;
  width: 100%;
  font-size: 0.9rem;
}
`);

block(`
.results-grid th,
.results-grid td {
  padding: 0.6rem 0.75rem;
  border-bottom: 1px solid var(--border);
  text-align: left;
  vertical-align: top;
}
`);

block(`
.results-grid tbody tr:last-child th,
.results-grid tbody tr:last-child td {
  border-bottom: none;
}
`);

block(`
.results-grid thead th {
  min-width: 7.5rem;
  font-weight: 600;
}
`);

block(`
.results-grid thead th a,
.results-grid tbody th a {
  color: var(--text);
  text-decoration: none;
}
`);

block(`
.results-grid thead th a:hover,
.results-grid tbody th a:hover {
  text-decoration: underline;
}
`);

block(`
.results-grid thead small {
  display: block;
  color: var(--muted);
  font-weight: 400;
  font-size: 0.75rem;
}
`);

block(`
.results-grid-upcoming {
  opacity: 0.6;
}
`);

block(`
.results-grid tbody th {
  position: sticky;
  left: 0;
  min-width: 9rem;
  max-width: 14rem;
  background: var(--surface);
  font-weight: 600;
}
`);

block(`
.results-grid-corner {
  position: sticky;
  left: 0;
  background: var(--surface);
  color: var(--muted);
  font-size: 0.75rem;
  text-transform: uppercase;
  letter-spacing: 0.05em;
}
`);

block(`
.results-grid td {
  color: var(--muted);
}
`);

block(`
.results-grid-hit a {
  display: inline-block;
  padding: 0.15rem 0.5rem;
  border-radius: 8px;
  background: var(--accent-soft);
  border: 1px solid var(--accent-soft-border);
  color: var(--text);
  font-weight: 600;
  text-decoration: none;
  white-space: nowrap;
}
`);

block(`
.schedule {
  list-style: none;
  margin: 0;
  padding: 0;
  display: grid;
  gap: 0.5rem;
}
`);

block(`
.schedule-row {
  display: flex;
  align-items: center;
  gap: 0.85rem;
  padding: 0.65rem 0.85rem;
  border: 1px solid var(--border);
  border-radius: 12px;
  background: var(--surface);
}
`);

block(`
.schedule-upcoming {
  background: transparent;
  border-style: dashed;
}
`);

block(`
.schedule-main {
  display: grid;
  gap: 0.1rem;
  flex: 1;
  min-width: 0;
  color: var(--text);
  text-decoration: none;
}
`);

block(`
.schedule-main:hover strong {
  text-decoration: underline;
}
`);

block(`
.schedule-status {
  flex: 0 0 auto;
}
`);

block(`
.schedule-status .btn {
  width: auto;
}
`);

block(`
.schedule-tag {
  padding: 0.15rem 0.55rem;
  border-radius: 999px;
  border: 1px solid var(--border);
  color: var(--muted);
  font-size: 0.78rem;
}
`);

block(`
.schedule-tag-missing {
  border-color: var(--warning);
  color: var(--text);
}
`);

block(`
.entry-cards {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(16rem, 1fr));
  gap: 0.75rem;
}
`);

block(`
.entry-card {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 0.5rem;
  padding: 0.85rem 1rem;
  border: 1px solid var(--border);
  border-radius: 12px;
  background: var(--surface);
  min-width: 0;
}
`);

block(`
.entry-card-main {
  display: grid;
  gap: 0.2rem;
  min-width: 0;
}
`);

block(`
.entry-card-name {
  color: var(--text);
  font-weight: 700;
  text-decoration: none;
}
`);

block(`
.entry-card-name:hover {
  text-decoration: underline;
}
`);

block(`
.entry-card-roster {
  font-size: 0.85rem;
}
`);

block(`
.entry-cards .activities-form {
  grid-column: 1 / -1;
}
`);

block(`
@media (max-width: 640px) {
  .schedule-row {
    flex-wrap: wrap;
    row-gap: 0.4rem;
  }

  .schedule-main {
    flex-basis: calc(100% - 3.6rem);
  }

  .schedule-status {
    margin-left: 3.6rem;
  }

  .schedule-row .event-item-actions {
    margin-left: auto;
  }
}
`);

block(`
.routine-roster {
  display: flex;
  flex-wrap: wrap;
  gap: 0.4rem;
  margin: 0.25rem 0 0;
}
`);

block(`
.routine-roster a {
  padding: 0.15rem 0.6rem;
  border: 1px solid var(--border);
  border-radius: 999px;
  color: var(--text);
  font-size: 0.85rem;
  text-decoration: none;
}
`);

block(`
.routine-roster a:hover {
  border-color: var(--accent);
}
`);
