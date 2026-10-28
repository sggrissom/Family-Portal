import { block } from "vlens/css";

block(`
.matchup-line {
  color: var(--text);
  font-size: 0.9rem;
}
`);

block(`
.result-badges {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
}
`);

block(`
.result-badge {
  display: flex;
  flex-direction: column;
  gap: 0.1rem;
  padding: 0.4rem 0.7rem;
  border: 1px solid var(--border);
  border-radius: 10px;
  background: var(--bg);
  line-height: 1.25;
}
`);

block(`
.result-badge strong {
  color: var(--text);
  font-size: 1rem;
}
`);

block(`
.result-badge small {
  color: var(--muted);
  font-size: 0.75rem;
}
`);

block(`
.result-badge-placement,
.result-badge-adjudication {
  background: var(--accent-soft);
  border-color: var(--accent-soft-border);
}
`);

block(`
.result-badge-award {
  background: var(--surface-alt);
  border-color: var(--surface-alt-border);
}
`);

block(`
.result-badge-p1 {
  border-color: var(--warning);
  box-shadow: inset 0 0 0 1px var(--warning);
}
`);

block(`
.media-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(112px, 1fr));
  gap: 0.5rem;
}
`);

block(`
.media-grid-small {
  grid-template-columns: repeat(auto-fill, minmax(72px, 1fr));
}
`);

block(`
.media-grid-item {
  display: block;
  aspect-ratio: 1;
  border-radius: 10px;
  overflow: hidden;
  background: var(--border);
}
`);

block(`
.media-grid-item img {
  width: 100%;
  height: 100%;
  object-fit: cover;
  display: block;
}
`);
