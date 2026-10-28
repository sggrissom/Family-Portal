import { block } from "vlens/css";

block(`
.results-editor {
  display: grid;
  gap: 0.5rem;
  margin-bottom: 0.75rem;
}
`);

block(`
.result-edit-row {
  display: grid;
  gap: 0.35rem;
}
`);

block(`
.result-edit-fields {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.4rem;
}
`);

block(`
.result-edit-fields input,
.result-edit-fields select,
.result-edit-notes {
  min-width: 0;
  padding: 0.45rem 0.6rem;
  border: 1px solid var(--control-border);
  border-radius: 8px;
  background: var(--bg);
  color: var(--text);
  font: inherit;
  font-size: 0.9rem;
}
`);

block(`
.result-edit-kind {
  flex: 0 0 8.5rem;
}
`);

block(`
.result-edit-num {
  flex: 0 0 4.5rem;
  width: 4.5rem;
}
`);

block(`
.result-edit-of {
  color: var(--muted);
  font-size: 0.85rem;
}
`);

block(`
.result-edit-wide {
  flex: 1 1 12rem;
}
`);

block(`
.result-edit-person {
  flex: 0 1 10rem;
}
`);

block(`
.result-remove {
  margin-left: auto;
}
`);

block(`
.result-row-error {
  margin: 0.1rem 0 0.5rem;
  color: var(--weight-color);
  font-size: 0.85rem;
}
`);

block(`
.result-quick-add {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.4rem;
  margin: 0.25rem 0 0.5rem;
}
`);

block(`
.result-quick-label {
  color: var(--muted);
  font-size: 0.85rem;
  margin-right: 0.25rem;
}
`);

block(`
.result-chip {
  border: 1px dashed var(--control-border);
  background: var(--surface);
  color: var(--text);
  border-radius: 999px;
  padding: 0.3rem 0.75rem;
  font: inherit;
  font-size: 0.85rem;
  cursor: pointer;
}
`);

block(`
.result-chip:hover:not(:disabled) {
  background: var(--hover-bg);
}
`);

block(`
.result-chip-adjudication {
  border-style: solid;
  border-color: var(--accent-soft-border);
  background: var(--accent-soft);
  font-weight: 600;
}
`);
