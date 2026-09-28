import { block } from "vlens/css";

block(`
.home-container {
  max-width: 760px;
  margin: 0 auto;
  padding: 20px 16px 40px;
}
`);

block(`
.home-page {
  display: grid;
  gap: 24px;
}
`);

block(`
.home-greeting {
  margin: 0;
  font-size: 1.3rem;
}
`);

block(`
.family-strip {
  display: flex;
  gap: 14px;
  overflow-x: auto;
  padding: 2px 2px 6px;
  scrollbar-width: none;
}
`);

block(`
.family-strip-person {
  display: grid;
  flex-shrink: 0;
  justify-items: center;
  gap: 4px;
  width: 76px;
  color: var(--text);
  text-align: center;
  text-decoration: none;
}
`);

block(`
.family-strip-avatar {
  display: grid;
  place-items: center;
  width: 64px;
  height: 64px;
  overflow: hidden;
  border: 2px solid var(--border);
  border-radius: 50%;
  background: var(--surface);
  font-size: 1.4rem;
  font-weight: 700;
}
`);

block(`
.family-strip-person:hover .family-strip-avatar {
  border-color: var(--accent);
}
`);

block(`
.family-strip-avatar img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}
`);

block(`
.family-strip-person.pregnancy .family-strip-avatar {
  border-style: dashed;
  border-color: var(--accent);
}
`);

block(`
.family-strip-name {
  max-width: 100%;
  overflow: hidden;
  font-size: 0.85rem;
  font-weight: 600;
  text-overflow: ellipsis;
  white-space: nowrap;
}
`);

block(`
.family-strip-age {
  color: var(--muted);
  font-size: 0.75rem;
}
`);

block(`
.family-strip-add .family-strip-avatar {
  border-style: dashed;
  color: var(--muted);
}
`);

block(`
.family-strip-add .family-strip-name {
  color: var(--muted);
  font-weight: 500;
  white-space: normal;
}
`);

block(`
.home-nudges {
  display: grid;
  gap: 8px;
}
`);

block(`
.home-nudge {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 4px 4px 4px 14px;
  border: 1px solid var(--surface-alt-border);
  border-radius: 12px;
  background: var(--surface-alt);
}
`);

block(`
.home-nudge a {
  flex: 1;
  padding: 8px 0;
  color: var(--text);
  font-weight: 600;
  text-decoration: none;
}
`);

block(`
.home-nudge-dismiss {
  width: 36px;
  height: 36px;
  border: none;
  border-radius: 50%;
  background: transparent;
  color: var(--muted);
  font-size: 1.2rem;
  cursor: pointer;
}
`);

block(`
.home-nudge-dismiss:hover {
  background: var(--hover-bg);
}
`);

block(`
.home-section {
  display: grid;
  gap: 12px;
}
`);

block(`
.home-section h2 {
  margin: 0;
  font-size: 1.1rem;
}
`);

block(`
.home-section-head {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
}
`);

block(`
.home-section-head a {
  color: var(--accent);
  font-weight: 600;
  text-decoration: none;
}
`);

block(`
.home-seasons {
  display: grid;
  gap: 10px;
}
`);

block(`
.home-season {
  display: grid;
  gap: 8px;
  padding: 14px;
  border: 1px solid var(--border);
  border-radius: 14px;
  background: var(--surface);
}
`);

block(`
.home-season-name {
  color: var(--text);
  font-weight: 700;
  text-decoration: none;
}
`);

block(`
.home-season-event {
  color: var(--muted);
  text-decoration: none;
}
`);

block(`
.home-season-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}
`);

block(`
.home-on-this-day {
  display: grid;
  gap: 14px;
}
`);

block(`
.home-year {
  display: grid;
  gap: 8px;
}
`);

block(`
.home-year h3 {
  margin: 0;
  color: var(--muted);
  font-size: 0.8rem;
  font-weight: 700;
  letter-spacing: 0.06em;
  text-transform: uppercase;
}
`);

block(`
.home-year-photos {
  display: flex;
  gap: 6px;
  overflow-x: auto;
  scrollbar-width: none;
}
`);

block(`
.home-year-photos img {
  width: 96px;
  height: 96px;
  flex-shrink: 0;
  object-fit: cover;
  border-radius: 10px;
}
`);

block(`
.home-empty {
  margin: 0;
  color: var(--muted);
}
`);
