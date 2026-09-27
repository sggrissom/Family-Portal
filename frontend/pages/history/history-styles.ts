import { block } from "vlens/css";

block(`
.history-container {
  max-width: 760px;
  margin: 0 auto;
  padding: 20px 16px 40px;
}
`);

block(`
.history-page {
  display: grid;
  gap: 16px;
}
`);

block(`
.history-page h1 {
  margin: 0;
  font-size: 1.4rem;
}
`);

block(`
.history-filters {
  display: grid;
  gap: 8px;
}
`);

block(`
.history-filter-row {
  display: flex;
  flex-wrap: wrap;
  align-items: flex-start;
  gap: 10px;
}
`);

block(`
.history-more {
  position: relative;
}
`);

block(`
.history-more > summary {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  min-height: 36px;
  padding: 6px 14px;
  border: 1px solid var(--border);
  border-radius: 999px;
  background: var(--surface);
  font-weight: 600;
  list-style: none;
  cursor: pointer;
}
`);

block(`
.history-more > summary::-webkit-details-marker {
  display: none;
}
`);

block(`
.history-more-panel {
  position: absolute;
  top: calc(100% + 6px);
  left: 0;
  z-index: 20;
  display: grid;
  gap: 8px;
  width: min(340px, calc(100vw - 32px));
  padding: 12px;
  border: 1px solid var(--border);
  border-radius: 14px;
  background: var(--surface);
  box-shadow: 0 18px 50px rgba(15, 23, 42, 0.18);
}
`);

block(`
.history-more-label {
  color: var(--muted);
  font-size: 0.75rem;
  font-weight: 700;
  letter-spacing: 0.06em;
  text-transform: uppercase;
}
`);

block(`
.history-pills {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}
`);

block(`
.history-pill {
  padding: 5px 12px;
  border: 1px solid var(--border);
  border-radius: 999px;
  background: var(--bg);
  color: var(--muted);
  font: inherit;
  font-size: 0.85rem;
  cursor: pointer;
}
`);

block(`
.history-pill.selected {
  border-color: var(--accent);
  background: var(--accent-soft);
  color: var(--text);
  font-weight: 600;
}
`);

block(`
.history-manage-tags {
  color: var(--accent);
  font-size: 0.85rem;
  text-decoration: none;
}
`);

block(`
.history-search {
  flex: 1;
  min-width: 180px;
}
`);

block(`
.history-search input {
  width: 100%;
  min-height: 36px;
  padding: 6px 14px;
  border: 1px solid var(--control-border);
  border-radius: 999px;
  background: var(--bg);
  color: var(--text);
  font: inherit;
}
`);

block(`
.history-page .link-button {
  min-height: 36px;
  padding: 0 4px;
  border: none;
  background: none;
  color: var(--accent);
  font: inherit;
  cursor: pointer;
}
`);

block(`
.history-years {
  display: flex;
  gap: 6px;
  overflow-x: auto;
  scrollbar-width: none;
}
`);

block(`
.history-years button {
  flex-shrink: 0;
  padding: 4px 12px;
  border: 1px solid var(--border);
  border-radius: 999px;
  background: var(--surface);
  color: var(--text);
  font: inherit;
  font-size: 0.85rem;
  cursor: pointer;
}
`);

block(`
.history-month {
  display: grid;
  gap: 12px;
}
`);

block(`
.history-month > span[id] {
  scroll-margin-top: 80px;
}
`);

block(`
.history-month-label {
  position: sticky;
  top: 64px;
  z-index: 5;
  margin: 0;
  padding: 8px 0;
  background: var(--bg);
  font-size: 1.1rem;
}
`);

block(`
.history-empty {
  margin: 0;
  color: var(--muted);
}
`);

block(`
.history-results {
  display: grid;
  gap: 8px;
}
`);

block(`
.history-results-head {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 10px;
}
`);

block(`
.history-results-head h2 {
  margin: 0;
  font-size: 1.05rem;
}
`);

block(`
.history-result-date {
  color: var(--muted);
}
`);
