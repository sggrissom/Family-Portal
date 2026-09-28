import { block } from "vlens/css";

block(`
.profile-container {
  max-width: 820px;
  margin: 0 auto;
  padding: 20px 16px 40px;
}
`);

block(`
.profile-page {
  display: grid;
  gap: 20px;
}
`);

block(`
.profile-header {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 16px;
}
`);

block(`
.profile-avatar {
  display: grid;
  place-items: center;
  width: 84px;
  height: 84px;
  flex-shrink: 0;
  overflow: hidden;
  border: 2px solid var(--border);
  border-radius: 50%;
  background: var(--surface);
}
`);

block(`
.profile-avatar img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}
`);

block(`
.profile-initial {
  font-size: 2rem;
  font-weight: 700;
}
`);

block(`
.profile-info {
  flex: 1;
  min-width: 180px;
}
`);

block(`
.profile-info h1 {
  margin: 0;
  font-size: 1.6rem;
}
`);

block(`
.profile-age {
  margin: 4px 0 0;
  color: var(--muted);
}
`);

block(`
.profile-relationship {
  text-transform: capitalize;
}
`);

block(`
.profile-actions {
  display: flex;
  gap: 8px;
}
`);

block(`
.profile-snapshot {
  display: grid;
  gap: 8px;
  padding: 14px;
  border: 1px solid var(--border);
  border-radius: 14px;
  background: var(--surface);
}
`);

block(`
.snapshot-row {
  display: flex;
  align-items: baseline;
  gap: 10px;
  color: var(--text);
  text-decoration: none;
}
`);

block(`
.snapshot-label {
  width: 60px;
  color: var(--muted);
}
`);

block(`
.snapshot-pct {
  color: var(--muted);
  font-size: 0.85rem;
}
`);

block(`
.sparkline {
  width: 80px;
  height: 20px;
  margin-left: auto;
  align-self: center;
}
`);

block(`
.sparkline polyline {
  fill: none;
  stroke: var(--accent);
  stroke-width: 1.5;
}
`);

block(`
.snapshot-note {
  margin: 0;
  color: var(--muted);
  font-size: 0.9rem;
}
`);

block(`
.snapshot-season {
  padding: 8px 0 0;
  border: none;
  border-top: 1px solid var(--border);
  background: none;
  color: var(--text);
  font: inherit;
  text-align: left;
  cursor: pointer;
}
`);

block(`
.profile-tabs {
  display: flex;
  gap: 4px;
  overflow-x: auto;
  border-bottom: 1px solid var(--border);
  scrollbar-width: none;
}
`);

block(`
.profile-tab {
  padding: 10px 14px;
  border: none;
  border-bottom: 2px solid transparent;
  background: none;
  color: var(--muted);
  font: inherit;
  font-weight: 600;
  white-space: nowrap;
  cursor: pointer;
}
`);

block(`
.profile-tab.active {
  border-bottom-color: var(--accent);
  color: var(--text);
}
`);

block(`
.profile-empty {
  margin: 0;
  color: var(--muted);
}
`);

block(`
.story {
  display: grid;
  gap: 28px;
}
`);

block(`
.story-chapter {
  display: grid;
  gap: 12px;
}
`);

block(`
.story-chapter-head {
  display: flex;
  flex-wrap: wrap;
  align-items: baseline;
  gap: 4px 12px;
}
`);

block(`
.story-chapter-head h2 {
  margin: 0;
  font-size: 1.25rem;
}
`);

block(`
.story-grew {
  color: var(--accent);
  font-weight: 600;
}
`);

block(`
.profile-photos {
  display: grid;
  gap: 10px;
}
`);

block(`
.profile-photos-open {
  justify-self: end;
  color: var(--accent);
  font-weight: 600;
  text-decoration: none;
}
`);

block(`
.profile-photo-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(110px, 1fr));
  gap: 6px;
}
`);

block(`
.profile-photo-grid img {
  width: 100%;
  aspect-ratio: 1;
  object-fit: cover;
  border-radius: 8px;
}
`);

block(`
.profile-growth {
  display: grid;
  gap: 14px;
}
`);

block(`
.profile-growth-controls {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 12px;
}
`);

block(`
.profile-growth-controls .btn {
  margin-left: auto;
}
`);

block(`
.profile-growth-toggle {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  color: var(--muted);
}
`);

block(`
.profile-measurements {
  display: grid;
  gap: 0;
  margin: 0;
  padding: 0;
  list-style: none;
}
`);

block(`
.profile-measurements a {
  display: flex;
  align-items: baseline;
  gap: 12px;
  padding: 10px 4px;
  border-bottom: 1px solid var(--border);
  color: var(--text);
  text-decoration: none;
}
`);

block(`
.measurement-date {
  width: 90px;
  color: var(--muted);
}
`);

block(`
.measurement-kind {
  width: 60px;
  color: var(--muted);
}
`);

block(`
@media (max-width: 560px) {
  .profile-actions {
    width: 100%;
  }

  .profile-actions > * {
    flex: 1;
  }
}
`);
