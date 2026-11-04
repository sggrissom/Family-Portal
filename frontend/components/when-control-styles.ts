import { block } from "vlens/css";

block(`
.when-control {
  display: inline-flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: flex-end;
  gap: 8px;
}
`);

block(`
.when-select {
  min-height: 36px;
  padding: 6px 10px;
  border: 1px solid var(--border);
  border-radius: 999px;
  background: var(--bg);
  color: var(--text);
  font: inherit;
  font-weight: 600;
  cursor: pointer;
}
`);

block(`
.when-date {
  min-height: 36px;
  padding: 6px 10px;
  border: 1px solid var(--control-border);
  border-radius: 8px;
  background: var(--bg);
  color: var(--text);
  font: inherit;
}
`);

block(`
.when-age {
  display: inline-flex;
  gap: 6px;
}
`);

block(`
.when-age input {
  width: 64px;
  min-height: 36px;
  padding: 6px 8px;
  border: 1px solid var(--control-border);
  border-radius: 8px;
  background: var(--bg);
  color: var(--text);
  font: inherit;
}
`);

block(`
.when-age-date {
  font-size: 0.85rem;
  color: var(--muted);
}
`);

block(`
.when-age-future {
  color: var(--danger);
}
`);

block(`
.record-when {
  display: grid;
  justify-items: end;
  gap: 8px;
}
`);

block(`
.record-when-row {
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  align-items: center;
  gap: 12px;
}
`);

block(`
.when-year,
.when-range {
  width: 96px;
}
`);

block(`
.record-when-preview {
  margin: 0;
  color: var(--muted);
  text-align: right;
}
`);
