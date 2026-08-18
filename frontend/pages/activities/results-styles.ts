import { block } from "vlens/css";

block(`
.result-list {
  list-style: none;
  margin: 0.4rem 0 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
}
`);

block(`
.result-row {
  display: flex;
  flex-wrap: wrap;
  align-items: baseline;
  gap: 0.5rem;
  font-size: 0.9rem;
  padding-left: 0.6rem;
  border-left: 3px solid var(--border);
}
`);

block(`
.result-row.result-adjudication {
  border-left-color: var(--accent);
}
`);

block(`
.result-row.result-placement {
  border-left-color: var(--primary-accent);
}
`);

block(`
.result-row.result-award {
  border-left-color: var(--height-color);
}
`);

block(`
.result-text {
  color: var(--text);
  font-weight: 600;
}
`);

block(`
.result-detail {
  color: var(--muted);
  font-size: 0.85rem;
}
`);
