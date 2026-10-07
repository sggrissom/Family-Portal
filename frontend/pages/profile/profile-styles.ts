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

block(`
.profile-avatar .profile-face {
  border: none;
  border-radius: 50%;
}
`);

block(`
.profile-growing-up,
.profile-often-with {
  margin-bottom: 1.25rem;
}
`);

block(`
.profile-growing-up h3,
.profile-often-with h3 {
  font-size: 1rem;
  margin: 0 0 0.5rem;
}
`);

block(`
.growing-up-strip {
  display: flex;
  gap: 0.75rem;
  overflow-x: auto;
  padding-bottom: 0.5rem;
}
`);

block(`
.growing-up-item {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0.25rem;
  color: var(--muted);
  font-size: 0.8rem;
  text-decoration: none;
}
`);

block(`
.often-with-chips {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
}
`);

block(`
.often-with-chip {
  display: inline-flex;
  align-items: center;
  gap: 0.5rem;
  padding: 0.375rem 0.75rem;
  border: 1px solid var(--border);
  border-radius: 999px;
  color: var(--text);
  text-decoration: none;
  font-size: 0.9rem;
}
`);

block(`
.often-with-count {
  color: var(--muted);
  font-size: 0.8rem;
}
`);

block(`
.profile-quotes {
  display: grid;
  gap: 12px;
}
`);

block(`
.profile-quote {
  display: grid;
  gap: 6px;
  padding: 14px 16px;
  border: 1px solid var(--border);
  border-left: 4px solid var(--accent);
  border-radius: 12px;
  background: var(--surface);
  color: var(--text);
  text-decoration: none;
}
`);

block(`
.profile-quote blockquote {
  margin: 0;
  font-size: 1.15rem;
  line-height: 1.4;
}
`);

block(`
.profile-quote-context {
  margin: 0;
  color: var(--muted);
}
`);

block(`
.profile-quote-when {
  color: var(--muted);
}
`);

block(`
.profile-artwork {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(150px, 1fr));
  gap: 16px;
}
`);

block(`
.profile-artwork-piece {
  display: grid;
  gap: 4px;
  align-content: start;
  color: var(--text);
  text-decoration: none;
}
`);

block(`
.profile-artwork-frame {
  position: relative;
  display: flex;
  align-items: center;
  justify-content: center;
  aspect-ratio: 1;
  overflow: hidden;
  border: 1px solid var(--border);
  border-radius: 12px;
  background: var(--surface);
  font-size: 2.5rem;
}
`);

block(`
.profile-artwork-frame picture,
.profile-artwork-frame img {
  width: 100%;
  height: 100%;
  object-fit: cover;
  display: block;
}
`);

block(`
.profile-artwork-more {
  position: absolute;
  right: 8px;
  bottom: 8px;
  padding: 2px 8px;
  border-radius: 999px;
  background: rgba(0, 0, 0, 0.6);
  color: #fff;
  font-size: 0.8rem;
}
`);

block(`
.profile-artwork-title {
  font-weight: 600;
  line-height: 1.3;
}
`);

block(`
.profile-artwork-when {
  color: var(--muted);
}
`);

block(`
.growing-up-preview {
  padding: 12px 0;
  border-top: 1px solid var(--border);
  border-bottom: 1px solid var(--border);
}
`);

block(`
.growing-up-preview h3 {
  margin: 0 0 8px;
  font-size: 1rem;
}
`);

block(`
.growing-up-preview-link {
  display: block;
  color: var(--accent);
  text-decoration: none;
}
`);

block(`
.growing-up-preview-more {
  display: inline-block;
  margin-top: 4px;
  font-size: 0.85rem;
}
`);

block(`
.growing-up-preview-link:hover .growing-up-preview-more {
  text-decoration: underline;
}
`);

block(`
#growing-up-heading {
  scroll-margin-top: 80px;
}
`);
