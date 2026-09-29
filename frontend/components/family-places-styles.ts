import { block } from "vlens/css";

block(`
.family-places {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}
`);

block(`
.family-place-row {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.5rem 0.75rem;
}
`);

block(`
.family-place-row input {
  flex: 1 1 12rem;
  min-width: 0;
  padding: 0.375rem 0.5rem;
  border: 1px solid var(--border);
  border-radius: 6px;
  background: var(--bg);
  color: var(--text);
}
`);

block(`
.family-place-meta,
.family-places-empty {
  color: var(--muted);
  font-size: 0.875rem;
}
`);
