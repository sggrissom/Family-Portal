import { block } from "vlens/css";

block(`
.results-editor {
  margin-top: 0.75rem;
}
`);

block(`
.result-edit-row {
  border-top: 1px solid var(--border);
  padding-top: 0.75rem;
  margin-bottom: 0.5rem;
}
`);

block(`
.result-edit-row:first-child {
  border-top: none;
  padding-top: 0;
}
`);

block(`
.result-edit-row .form-row {
  align-items: flex-end;
}
`);

block(`
.result-remove {
  margin-bottom: 0.75rem;
}
`);

block(`
.result-row-error {
  margin: 0.1rem 0 0.5rem;
  color: var(--weight-color);
  font-size: 0.85rem;
}
`);
