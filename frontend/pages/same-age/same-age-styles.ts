import { block } from "vlens/css";

block(`
.same-age-container {
  max-width: 960px;
  margin: 0 auto;
  padding: 20px 16px 40px;
}
`);

block(`
.same-age-page {
  display: grid;
  gap: 14px;
}
`);

block(`
.same-age-page h1 {
  margin: 0;
  font-size: 1.4rem;
}
`);

block(`
.same-age-control {
  display: grid;
  grid-template-columns: 44px minmax(0, 1fr) 44px;
  align-items: center;
  gap: 10px;
  max-width: 420px;
}
`);

block(`
.same-age-step {
  width: 44px;
  height: 44px;
  border: 1px solid var(--border);
  border-radius: 50%;
  background: var(--surface);
  color: var(--text);
  cursor: pointer;
}
`);

block(`
.same-age-step:disabled {
  opacity: 0.4;
  cursor: default;
}
`);

block(`
.same-age-empty {
  margin: 0;
  padding: 20px;
  border: 1px solid var(--border);
  border-radius: 14px;
  background: var(--surface);
  color: var(--muted);
}
`);

block(`
.same-age-results {
  display: grid;
  gap: 14px;
}
`);

block(`
.same-age-results[aria-busy="true"] {
  opacity: 0.5;
  pointer-events: none;
}
`);

block(`
.same-age-load-status {
  min-height: 1.5em;
  color: var(--muted);
  font-size: 0.9rem;
}
`);

block(`
.same-age-picker {
  width: 100%;
  min-height: 44px;
  padding: 8px 12px;
  border: 1px solid var(--border);
  border-radius: 10px;
  background: var(--surface);
  color: var(--text);
  font: inherit;
}
`);

block(`
.same-age-views,
.same-age-shortcuts {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}
`);

block(`
.same-age-views {
  justify-self: start;
  gap: 0;
  padding: 3px;
  border: 1px solid var(--border);
  border-radius: 12px;
  background: var(--surface);
}
`);

block(`
.same-age-views button,
.same-age-shortcuts button {
  min-height: 40px;
  padding: 6px 14px;
  border: 1px solid transparent;
  border-radius: 9px;
  background: none;
  color: var(--text);
  font: inherit;
  cursor: pointer;
}
`);

block(`
.same-age-shortcuts button {
  padding: 6px 11px;
  border-color: var(--border);
  border-radius: 999px;
  background: var(--surface);
  font-size: 0.9rem;
}
`);

block(`
.same-age-views button[aria-pressed="true"],
.same-age-shortcuts button[aria-pressed="true"] {
  border-color: var(--primary-accent);
  background: var(--primary-accent);
  color: var(--button-text);
  font-weight: 600;
}
`);

block(`
.same-age-views button:focus-visible,
.same-age-shortcuts button:focus-visible,
.same-age-step:focus-visible,
.same-age-picker:focus-visible {
  outline: 3px solid var(--primary-accent);
  outline-offset: 2px;
}
`);

block(`
.same-age-help {
  font-size: 0.85rem;
}
`);
