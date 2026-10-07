import { block } from "vlens/css";

block(`
.same-age-container {
  max-width: 760px;
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
  height: 72px;
}
`);

block(`
.same-age-age {
  min-width: 0;
  line-height: 1.3;
  text-align: center;
  font-size: 1.15rem;
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
.same-age-slider {
  width: 100%;
  accent-color: var(--primary-accent);
}
`);

block(`
.same-age-browse {
  display: grid;
  gap: 6px;
  font-size: 0.9rem;
  color: var(--muted);
}
`);

block(`
.same-age-browse select {
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
.same-age-slider-help {
  font-size: 0.85rem;
}
`);
