import { block } from "vlens/css";

block(`
.day-summaries {
  display: grid;
  gap: 18px;
}
`);

block(`
.day-summary {
  display: grid;
  gap: 8px;
}
`);

block(`
.day-summary-label {
  margin: 0;
  color: var(--muted);
  font-size: 0.8rem;
  font-weight: 700;
  letter-spacing: 0.06em;
  text-transform: uppercase;
}
`);

block(`
.day-birthday {
  padding: 6px 0;
  color: var(--muted);
  font-weight: 600;
}
`);

block(`
.day-milestone,
.day-checkup {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 12px;
  border: 1px solid var(--border);
  border-radius: 12px;
  background: var(--surface);
  color: var(--text);
  text-decoration: none;
}
`);

block(`
.day-milestone {
  border-color: var(--accent-soft-border);
  background: var(--accent-soft);
}
`);

block(`
.day-milestone:hover,
.day-event:hover,
.day-checkup:hover,
.day-mosaic:hover {
  border-color: var(--accent);
}
`);

block(`
.day-milestone-icon {
  flex-shrink: 0;
  font-size: 1.2rem;
}
`);

block(`
.day-mosaic {
  display: grid;
  gap: 6px;
  padding: 8px;
  border: 1px solid var(--border);
  border-radius: 12px;
  background: var(--surface);
  color: var(--text);
  text-decoration: none;
}
`);

block(`
.day-mosaic-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 120px));
  gap: 4px;
}
`);

block(`
.day-mosaic-grid.count-1 {
  grid-template-columns: minmax(0, 180px);
}
`);

block(`
.day-mosaic-grid img {
  width: 100%;
  aspect-ratio: 1;
  object-fit: cover;
  border-radius: 8px;
}
`);

block(`
.day-mosaic-caption {
  color: var(--muted);
  font-size: 0.9rem;
}
`);

block(`
.day-event {
  display: grid;
  gap: 6px;
  padding: 10px 12px;
  border: 1px solid var(--surface-alt-border);
  border-radius: 12px;
  background: var(--surface-alt);
  color: var(--text);
  text-decoration: none;
}
`);

block(`
.day-event-head {
  display: flex;
  align-items: center;
  gap: 10px;
}
`);

block(`
.day-event-line {
  padding-left: 32px;
  color: var(--muted);
}
`);

block(`
.day-event-line strong {
  color: var(--text);
}
`);

block(`
.day-event-photos {
  display: flex;
  gap: 4px;
  padding-left: 32px;
}
`);

block(`
.day-event-photos img {
  width: 64px;
  height: 64px;
  object-fit: cover;
  border-radius: 8px;
}
`);
