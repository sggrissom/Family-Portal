import { block } from "vlens/css";

block(`
.search-container {
  max-width: 760px;
  margin: 0 auto;
  padding: 20px 16px 40px;
}
`);

block(`
.search-page {
  display: grid;
  gap: 16px;
}
`);

block(`
.search-title {
  position: absolute;
  width: 1px;
  height: 1px;
  overflow: hidden;
  clip: rect(0 0 0 0);
  white-space: nowrap;
}
`);

block(`
.search-field {
  position: relative;
  display: flex;
  align-items: center;
}
`);

block(`
.search-field-icon {
  position: absolute;
  left: 14px;
  pointer-events: none;
}
`);

block(`
.search-field input {
  width: 100%;
  min-height: 48px;
  padding: 10px 44px 10px 42px;
  border: 1px solid var(--control-border);
  border-radius: 12px;
  background: var(--surface);
  color: var(--text);
  font: inherit;
  font-size: 1.05rem;
}
`);

block(`
.search-field input:focus {
  outline: 2px solid var(--accent);
  outline-offset: 1px;
}
`);

block(`
.search-field input::-webkit-search-cancel-button {
  display: none;
}
`);

block(`
.search-clear {
  position: absolute;
  right: 6px;
  width: 36px;
  height: 36px;
  border: none;
  border-radius: 50%;
  background: transparent;
  color: var(--muted);
  font-size: 1.4rem;
  line-height: 1;
  cursor: pointer;
}
`);

block(`
.search-clear:hover {
  background: var(--hover-bg);
  color: var(--text);
}
`);

block(`
.search-chips {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}
`);

block(`
.search-chip {
  display: inline-flex;
  align-items: center;
  gap: 2px;
  padding: 2px 4px 2px 12px;
  border: 1px solid var(--accent-soft-border);
  border-radius: 999px;
  background: var(--accent-soft);
  font-weight: 600;
}
`);

block(`
.search-chip button {
  width: 28px;
  height: 28px;
  border: none;
  border-radius: 50%;
  background: transparent;
  color: var(--muted);
  font-size: 1.1rem;
  line-height: 1;
  cursor: pointer;
}
`);

block(`
.search-chip button:hover {
  background: var(--hover-bg);
  color: var(--text);
}
`);

block(`
.search-status,
.search-note {
  margin: 0;
  color: var(--muted);
}
`);

block(`
.search-results {
  display: grid;
  gap: 20px;
  transition: opacity var(--transition-speed);
}
`);

block(`
.search-results.stale {
  opacity: 0.6;
}
`);

block(`
.search-results mark {
  background: transparent;
  color: inherit;
  font-weight: 700;
  text-decoration: underline;
  text-decoration-color: var(--accent);
  text-decoration-thickness: 2px;
  text-underline-offset: 2px;
}
`);

block(`
.search-group,
.search-empty,
.search-empty section {
  display: grid;
  gap: 8px;
}
`);

block(`
.search-empty {
  gap: 20px;
}
`);

block(`
.search-group-head {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 12px;
}
`);

block(`
.search-group-title {
  margin: 0;
  color: var(--muted);
  font-size: 0.85rem;
  font-weight: 700;
  letter-spacing: 0.04em;
  text-transform: uppercase;
}
`);

block(`
.search-see-all {
  font-weight: 600;
}
`);

block(`
.search-list,
.search-recent {
  display: grid;
  gap: 6px;
  margin: 0;
  padding: 0;
  list-style: none;
}
`);

block(`
.search-row {
  display: flex;
  align-items: center;
  gap: 12px;
  min-height: 48px;
  padding: 8px 12px;
  border: 1px solid var(--border);
  border-radius: 12px;
  background: var(--surface);
  color: var(--text);
  text-decoration: none;
}
`);

block(`
.search-row:hover,
.search-row:focus-visible,
.search-results .day-milestone:focus-visible,
.search-photo:focus-visible {
  border-color: var(--accent);
}
`);

block(`
.search-row-icon {
  flex-shrink: 0;
  width: 32px;
  text-align: center;
  font-size: 1.1rem;
}
`);

block(`
.search-tag-dot {
  width: 14px;
  height: 14px;
  margin: 0 9px;
  border-radius: 50%;
}
`);

block(`
.search-row-main {
  display: grid;
  min-width: 0;
  font-weight: 600;
}
`);

block(`
.search-row-main small {
  overflow: hidden;
  color: var(--muted);
  font-weight: 400;
  text-overflow: ellipsis;
  white-space: nowrap;
}
`);

block(`
.search-person {
  display: grid;
  gap: 4px;
}
`);

block(`
.search-avatar {
  display: grid;
  flex-shrink: 0;
  place-items: center;
  width: 40px;
  height: 40px;
  overflow: hidden;
  border-radius: 50%;
  background: var(--hover-bg);
  font-weight: 700;
}
`);

block(`
.search-avatar img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}
`);

block(`
.search-quick {
  display: flex;
  flex-wrap: wrap;
  gap: 4px 14px;
  padding-left: 64px;
  font-size: 0.9rem;
}
`);

block(`
.search-quick a {
  color: var(--accent);
  font-weight: 600;
  text-decoration: none;
}
`);

block(`
.search-quick a:hover {
  text-decoration: underline;
}
`);

block(`
.search-moments {
  display: grid;
  gap: 6px;
}
`);

block(`
.search-moment-context,
.search-moment-date {
  color: var(--muted);
}
`);

block(`
.search-photos {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(96px, 1fr));
  gap: 6px;
}
`);

block(`
.search-photo {
  display: block;
  overflow: hidden;
  aspect-ratio: 1;
  border: 2px solid transparent;
  border-radius: 10px;
  background: var(--hover-bg);
}
`);

block(`
.search-photo img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}
`);

block(`
.search-recent button {
  width: 100%;
  min-height: 44px;
  padding: 8px 12px;
  border: 1px solid var(--border);
  border-radius: 12px;
  background: var(--surface);
  color: var(--text);
  font: inherit;
  text-align: left;
  cursor: pointer;
}
`);

block(`
.search-recent button:hover {
  border-color: var(--accent);
}
`);

block(`
.search-nothing {
  display: grid;
  justify-items: start;
  gap: 6px;
}
`);

block(`
.search-nothing p {
  margin: 0;
}
`);

block(`
.search-photo-snippet {
  position: absolute;
  left: 0;
  right: 0;
  bottom: 0;
  padding: 2px 4px;
  background: rgba(0, 0, 0, 0.65);
  color: #fff;
  font-size: 0.7rem;
  line-height: 1.2;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}
`);
