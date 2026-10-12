import { block } from "vlens/css";

block(`
.interview-card {
  display: flex;
  align-items: flex-start;
  gap: 0.75rem;
  padding: 0.7rem 0.85rem;
  margin: 0.35rem 0;
  border: 1px solid var(--border);
  border-radius: 10px;
  background: var(--surface);
  color: var(--text);
  text-decoration: none;
}
`);

block(`
.interview-card:hover {
  border-color: var(--accent-soft-border);
}
`);

block(`
.interview-card-photo {
  width: 44px;
  height: 44px;
  border-radius: 50%;
  object-fit: cover;
  flex: none;
}
`);

block(`
.interview-card-icon {
  width: 44px;
  height: 44px;
  border-radius: 50%;
  display: grid;
  place-items: center;
  background: var(--accent-soft);
  flex: none;
  font-size: 1.2rem;
}
`);

block(`
.interview-card-body {
  display: flex;
  flex-direction: column;
  gap: 0.15rem;
  min-width: 0;
}
`);

block(`
.interview-card-lines {
  display: flex;
  flex-direction: column;
  font-size: 0.92rem;
}
`);

block(`
.interview-card-line {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
`);

block(`
.interview-card-q,
.interview-card-count {
  color: var(--muted);
}
`);

block(`
.interview-card-count {
  font-size: 0.82rem;
}
`);
