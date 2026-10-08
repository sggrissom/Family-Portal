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
.same-age-portraits {
  display: grid;
  gap: 12px;
}
`);

block(`
.same-age-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 14px 12px;
  margin: 0;
  padding: 0;
  list-style: none;
}
`);

block(`
@media (min-width: 600px) {
  .same-age-grid {
    grid-template-columns: repeat(auto-fill, minmax(180px, 1fr));
  }
}
`);

block(`
@media (max-width: 299px) {
  .same-age-grid {
    grid-template-columns: minmax(0, 1fr);
  }
}
`);

block(`
.same-age-portrait button {
  display: grid;
  gap: 2px;
  width: 100%;
  padding: 0;
  border: none;
  background: none;
  color: var(--text);
  font: inherit;
  text-align: center;
  cursor: pointer;
}
`);

block(`
.same-age-portrait .face-crop {
  margin-bottom: 6px;
  border-radius: 16px;
}
`);

block(`
.same-age-portrait button:focus-visible {
  outline: 3px solid var(--primary-accent);
  outline-offset: 3px;
  border-radius: 16px;
}
`);

block(`
.same-age-portrait-name {
  font-weight: 700;
  overflow-wrap: anywhere;
}
`);

block(`
.same-age-portrait-age {
  color: var(--muted);
  font-size: 0.9rem;
}
`);

block(`
.same-age-missing {
  color: var(--muted);
  font-size: 0.9rem;
}
`);

block(`
.same-age-missing summary {
  cursor: pointer;
}
`);

block(`
.same-age-missing p {
  margin: 6px 0 0;
}
`);

block(`
.same-age-viewer-backdrop {
  position: fixed;
  inset: 0;
  z-index: 50;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 16px;
  background: rgba(15, 23, 42, 0.7);
}
`);

block(`
.same-age-viewer {
  display: grid;
  gap: 12px;
  width: min(720px, 100%);
  max-height: calc(100vh - 32px);
  overflow-y: auto;
  padding: 16px;
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 16px;
}
`);

block(`
.same-age-viewer-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}
`);

block(`
.same-age-viewer-head h2 {
  margin: 0;
  font-size: 1.1rem;
}
`);

block(`
.same-age-viewer-head h2 span {
  color: var(--muted);
  font-weight: 400;
}
`);

block(`
.same-age-viewer-close {
  flex-shrink: 0;
  width: 44px;
  height: 44px;
  border: none;
  border-radius: 50%;
  background: var(--hover-bg);
  color: var(--text);
  font-size: 1.3rem;
  cursor: pointer;
}
`);

block(`
.same-age-viewer-image {
  display: block;
  width: 100%;
  max-height: 70vh;
  object-fit: contain;
  border-radius: 10px;
}
`);

block(`
.same-age-viewer-actions {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 8px 16px;
}
`);

block(`
.same-age-viewer-actions a {
  color: var(--accent);
  font-weight: 600;
}
`);

block(`
.same-age-another {
  min-height: 44px;
  padding: 8px 14px;
  border: 1px solid var(--border);
  border-radius: 10px;
  background: var(--surface);
  color: var(--text);
  font: inherit;
  cursor: pointer;
}
`);
