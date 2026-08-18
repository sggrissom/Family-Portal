import { block } from "vlens/css";

block(`
.routine-tally {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem 1rem;
  padding: 0.75rem 1rem;
  margin-bottom: 2rem;
  border: 1px solid var(--border);
  border-radius: 8px;
  background: var(--surface);
}
`);

block(`
.routine-tally-item {
  font-size: 0.9rem;
  color: var(--muted);
}
`);

block(`
.routine-tally-item strong {
  color: var(--text);
}
`);
