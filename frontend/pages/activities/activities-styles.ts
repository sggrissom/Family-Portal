import { block } from "vlens/css";

block(`
.activities-container {
  max-width: 780px;
  margin: 0 auto;
  padding: 2rem 1rem;
}
`);

block(`
.activities-title {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  margin-bottom: 1rem;
}
`);

block(`
.activities-title h1 {
  margin: 0;
}
`);

block(`
.activities-title > .btn,
.activities-section-head > .btn {
  flex: 0 0 auto;
  width: auto;
}
`);

block(`
.activities-section {
  margin-bottom: 2.5rem;
}
`);

block(`
.activities-section-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  margin-bottom: 1rem;
}
`);

block(`
.activities-section-head h2,
.activities-heading {
  margin: 0;
  font-size: 1.15rem;
}
`);

block(`
.activities-heading {
  margin-bottom: 0.85rem;
}
`);

block(`
.activities-form {
  border: 1px solid var(--border);
  border-radius: 8px;
  padding: 1rem;
  margin-bottom: 1.25rem;
  background: var(--surface);
}
`);

block(`
.activities-form textarea {
  width: 100%;
  padding: 0.6rem 0.75rem;
  border: 1px solid var(--border);
  border-radius: 6px;
  background: var(--bg);
  color: var(--text);
  font: inherit;
  resize: vertical;
}
`);

block(`
.icon-btn {
  background: none;
  border: none;
  cursor: pointer;
  font-size: 0.9rem;
  padding: 0.25rem;
  border-radius: 4px;
  line-height: 1;
}
`);

block(`
.icon-btn:hover:not(:disabled) {
  background: var(--hover-bg);
}
`);

block(`
.home-season-cards {
  display: grid;
  gap: 0.85rem;
}
`);

block(`
.home-season-card {
  display: grid;
  gap: 0.75rem;
  padding: 1rem;
  border: 1px solid var(--border);
  border-radius: 14px;
  background: var(--surface);
  min-width: 0;
}
`);

block(`
.home-season-card-head {
  display: grid;
  gap: 0.15rem;
  color: inherit;
  text-decoration: none;
}
`);

block(`
.home-season-program {
  display: inline-flex;
  align-items: center;
  gap: 0.35rem;
  color: var(--muted);
  font-size: 0.8rem;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.05em;
}
`);

block(`
.home-season-title {
  display: inline-flex;
  align-items: center;
  gap: 0.25rem;
  font-size: 1.2rem;
}
`);

block(`
.home-season-card-head:hover .home-season-title {
  color: var(--accent);
}
`);

block(`
.home-season-dates {
  color: var(--muted);
  font-size: 0.85rem;
}
`);

block(`
.home-season-entries {
  display: flex;
  flex-wrap: wrap;
  gap: 0.4rem;
  margin: 0;
}
`);

block(`
.home-season-entries a {
  padding: 0.2rem 0.6rem;
  border: 1px solid var(--border);
  border-radius: 999px;
  color: var(--text);
  font-size: 0.85rem;
  text-decoration: none;
}
`);

block(`
.home-season-entries a:hover {
  background: var(--hover-bg);
}
`);

block(`
.home-season-events {
  list-style: none;
  margin: 0;
  padding: 0;
  display: grid;
  gap: 0.4rem;
}
`);

block(`
.home-event {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  padding: 0.55rem 0.75rem;
  border-radius: 10px;
  background: var(--bg);
  border-left: 3px solid var(--border);
}
`);

block(`
.home-event-needs-results,
.home-event-now {
  border-left-color: var(--warning);
}
`);

block(`
.home-event-latest {
  border-left-color: var(--accent);
}
`);

block(`
.home-event-main {
  display: flex;
  flex-wrap: wrap;
  align-items: baseline;
  gap: 0.15rem 0.6rem;
  min-width: 0;
  color: var(--text);
  text-decoration: none;
}
`);

block(`
.home-event-main:hover strong {
  text-decoration: underline;
}
`);

block(`
.home-event-focus {
  flex-basis: 100%;
  color: var(--muted);
  font-size: 0.72rem;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.05em;
}
`);

block(`
.home-event-when,
.home-event-count {
  color: var(--muted);
  font-size: 0.85rem;
  white-space: nowrap;
}
`);

block(`
.home-event > .btn {
  flex: 0 0 auto;
  width: auto;
}
`);

block(`
.home-season-foot {
  margin: 0;
  color: var(--muted);
  font-size: 0.8rem;
}
`);

block(`
.idle-programs,
.past-seasons,
.manage-seasons {
  list-style: none;
  margin: 0;
  padding: 0;
}
`);

block(`
.idle-program {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  padding: 0.75rem 0;
  border-bottom: 1px solid var(--border);
}
`);

block(`
.idle-program > .btn {
  flex: 0 0 auto;
  width: auto;
}
`);

block(`
.idle-program .season-form {
  flex-basis: 100%;
  margin-bottom: 0;
}
`);

block(`
.idle-program-main {
  display: grid;
  gap: 0.15rem;
}
`);

block(`
.past-season {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  padding: 0.75rem 0;
  border-bottom: 1px solid var(--border);
  color: var(--text);
  text-decoration: none;
}
`);

block(`
.past-season:hover strong {
  color: var(--accent);
}
`);

block(`
.past-season-main {
  display: grid;
  gap: 0.1rem;
  flex: 1;
  min-width: 0;
}
`);

block(`
.past-season-counts {
  color: var(--muted);
  font-size: 0.85rem;
  text-align: right;
}
`);

block(`
.manage-toggle {
  display: inline-flex;
  align-items: center;
  gap: 0.35rem;
  background: none;
  border: none;
  padding: 0;
  color: var(--muted);
  font: inherit;
  font-size: 0.9rem;
  cursor: pointer;
}
`);

block(`
.manage-toggle[aria-expanded="true"] svg {
  transform: rotate(90deg);
}
`);

block(`
.manage-programs {
  display: grid;
  gap: 1.25rem;
  margin-top: 1rem;
}
`);

block(`
.manage-program {
  display: grid;
  gap: 0.35rem;
  padding: 1rem;
  border: 1px solid var(--border);
  border-radius: 12px;
}
`);

block(`
.manage-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
}
`);

block(`
.manage-seasons .manage-row {
  padding: 0.35rem 0 0.35rem 0.75rem;
  border-left: 2px solid var(--border);
}
`);

block(`
.manage-row-main {
  display: flex;
  flex-wrap: wrap;
  align-items: baseline;
  gap: 0.25rem 0.6rem;
}
`);

block(`
.manage-row-main small {
  color: var(--muted);
}
`);

block(`
.season-item-actions {
  display: flex;
  gap: 0.15rem;
  flex-shrink: 0;
}
`);

block(`
.season-link {
  color: inherit;
  text-decoration: none;
}
`);

block(`
.season-link:hover {
  color: var(--accent);
}
`);

block(`
.season-dates {
  color: var(--muted);
  font-size: 0.85rem;
}
`);

block(`
.season-notes {
  margin: 0.25rem 0 0;
  color: var(--muted);
  font-size: 0.9rem;
  white-space: pre-wrap;
}
`);

block(`
.link-action {
  justify-self: start;
  background: none;
  border: none;
  padding: 0;
  color: var(--accent);
  font: inherit;
  font-size: 0.85rem;
  font-weight: 600;
  cursor: pointer;
}
`);
