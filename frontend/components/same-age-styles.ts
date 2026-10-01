import { block } from "vlens/css";

block(`
.same-age-rows {
  display: grid;
  gap: 10px;
}
`);

block(`
.same-age-row {
  display: grid;
  gap: 8px;
  padding: 12px;
  border: 1px solid var(--border);
  border-radius: 14px;
  background: var(--surface);
}
`);

block(`
.same-age-row.empty {
  display: flex;
  align-items: baseline;
  gap: 10px;
  padding: 8px 12px;
  border-style: dashed;
  background: transparent;
}
`);

block(`
.same-age-row-head {
  display: flex;
  flex-wrap: wrap;
  align-items: baseline;
  gap: 4px 10px;
}
`);

block(`
.same-age-name {
  color: var(--text);
  font-weight: 700;
  text-decoration: none;
}
`);

block(`
.same-age-when {
  color: var(--muted);
  font-size: 0.9rem;
}
`);

block(`
.same-age-measure {
  margin-left: auto;
  font-weight: 600;
}
`);

block(`
.same-age-photos {
  display: flex;
  gap: 6px;
  overflow-x: auto;
  scrollbar-width: none;
}
`);

block(`
.same-age-photos img {
  width: 88px;
  height: 88px;
  flex-shrink: 0;
  object-fit: cover;
  border-radius: 10px;
}
`);

block(`
.same-age-milestone {
  color: var(--text);
  text-decoration: none;
}
`);

block(`
.same-age-milestone:hover,
.same-age-name:hover {
  color: var(--accent);
}
`);

block(`
.same-age-strip {
  display: grid;
  gap: 12px;
}
`);

block(`
.same-age-strip-head {
  display: flex;
  flex-wrap: wrap;
  align-items: baseline;
  justify-content: space-between;
  gap: 8px;
}
`);

block(`
.same-age-strip-head h2 {
  margin: 0;
  font-size: 1.1rem;
}
`);

block(`
.same-age-strip-head a {
  color: var(--accent);
  font-weight: 600;
  text-decoration: none;
}
`);

block(`
.same-age-none {
  margin: 0;
  color: var(--muted);
}
`);

block(`
.same-age-montage {
  display: grid;
  gap: 8px;
}
`);

block(`
.same-age-tiles {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(124px, 1fr));
  gap: 12px;
}
`);

block(`
.same-age-tile {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 4px;
  margin: 0;
  text-align: center;
}
`);

block(`
.same-age-tile figcaption {
  display: grid;
  gap: 1px;
  font-size: 0.85rem;
  color: var(--muted);
}
`);

block(`
.same-age-tile figcaption strong {
  color: var(--text);
}
`);

block(`
.same-age-tile-date {
  font-size: 0.75rem;
}
`);

block(`
.same-age-gap {
  margin: 0;
  color: var(--muted);
  font-size: 0.85rem;
}
`);

block(`
.same-age-another {
  border: none;
  background: none;
  color: var(--accent);
  font-size: 0.8rem;
  cursor: pointer;
  padding: 2px 4px;
}
`);
