import { block } from "vlens/css";

block(`
.activities-container {
  max-width: 780px;
  margin: 0 auto;
  padding: 2rem 1rem;
}
`);

block(`
.activities-intro {
  color: var(--muted);
  margin-bottom: 1.5rem;
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
.activities-section-head h2 {
  margin: 0;
  font-size: 1.15rem;
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
.activity-chips {
  display: flex;
  flex-wrap: wrap;
  gap: 0.75rem;
}
`);

block(`
.activity-chip {
  display: flex;
  align-items: center;
  gap: 0.25rem;
  border: 1px solid var(--border);
  border-radius: 8px;
  padding: 0.35rem 0.5rem 0.35rem 0.25rem;
  background: var(--surface);
}
`);

block(`
.activity-chip.selected {
  border-color: var(--accent);
  box-shadow: 0 0 0 1px var(--accent);
}
`);

block(`
.activity-chip-name {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 0.1rem;
  background: none;
  border: none;
  padding: 0.25rem 0.5rem;
  cursor: pointer;
  color: var(--text);
  font: inherit;
  text-align: left;
}
`);

block(`
.activity-chip-name small {
  color: var(--muted);
  font-size: 0.75rem;
}
`);

block(`
.activity-chip-actions {
  display: flex;
  gap: 0.15rem;
}
`);

block(`
.activity-chip-edit {
  flex: 1 1 100%;
  margin-bottom: 0;
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
.season-list {
  list-style: none;
  margin: 0;
  padding: 0;
}
`);

block(`
.season-item {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 1rem;
  padding: 0.85rem 0;
  border-bottom: 1px solid var(--border);
}
`);

block(`
.season-item-main {
  display: flex;
  flex-direction: column;
  gap: 0.2rem;
  min-width: 0;
}
`);

block(`
.season-name {
  color: var(--text);
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
.season-item-actions {
  display: flex;
  gap: 0.15rem;
  flex-shrink: 0;
}
`);
