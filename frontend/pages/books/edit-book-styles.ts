import { block } from "vlens/css";

block(`
.book-editor {
  max-width: 760px;
  margin: 0 auto;
  padding: 0 16px 64px;
}
`);

block(`
.book-editor-bar {
  position: sticky;
  top: 0;
  z-index: 10;
  display: flex;
  align-items: center;
  gap: 12px;
  margin: 0 -16px;
  padding: 10px 16px;
  background: var(--bg);
  border-bottom: 1px solid var(--border);
}
`);

block(`
.book-editor-status {
  flex: 1;
  color: var(--muted);
  font-size: 0.9rem;
  text-align: right;
}
`);

block(`
.book-editor-section {
  padding: 20px 0;
  border-bottom: 1px solid var(--border);
}
`);

block(`
.book-editor-section h1,
.book-editor-section h2 {
  margin: 0 0 12px;
}
`);

block(`
.book-editor-section h1 {
  font-size: 1.4rem;
}
`);

block(`
.book-editor-section h2 {
  font-size: 1.1rem;
}
`);

block(`
.book-editor-field {
  display: grid;
  gap: 4px;
  margin-bottom: 12px;
}
`);

block(`
.book-editor-field span {
  font-weight: 600;
  font-size: 0.9rem;
}
`);

block(`
.book-editor-field input,
.book-editor-field textarea,
.book-editor-caption {
  width: 100%;
  padding: 8px 10px;
  border: 1px solid var(--control-border);
  border-radius: 8px;
  background: var(--surface);
  color: var(--text);
  font: inherit;
}
`);

block(`
.book-editor-check {
  display: flex;
  align-items: center;
  gap: 8px;
}
`);

block(`
.book-editor-hint {
  margin: 4px 0;
  color: var(--muted);
  font-size: 0.85rem;
}
`);

block(`
.book-editor-row {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  align-items: center;
}
`);

block(`
.book-editor-warning {
  margin: 16px 0;
  padding: 10px 14px;
  border: 1px solid var(--surface-alt-border, var(--border));
  border-radius: 8px;
  background: var(--surface-alt, var(--surface));
}
`);

block(`
.book-editor-cover {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 12px;
}
`);

block(`
.book-editor-cover-current {
  width: 96px;
  height: 96px;
  overflow: hidden;
  border-radius: 8px;
  background: var(--border);
  display: grid;
  place-items: center;
  color: var(--muted);
  font-size: 0.8rem;
}
`);

block(`
.book-editor-cover-current img,
.book-editor-item-thumb img,
.book-editor-thumbs img,
.book-editor-attached-thumb img {
  width: 100%;
  height: 100%;
  object-fit: cover;
  display: block;
}
`);

block(`
.book-editor-thumbs {
  list-style: none;
  margin: 8px 0 0;
  padding: 0;
  width: 100%;
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(84px, 1fr));
  gap: 6px;
}
`);

block(`
.book-editor-thumbs button {
  position: relative;
  display: block;
  width: 100%;
  aspect-ratio: 1;
  padding: 0;
  border: 2px solid transparent;
  border-radius: 6px;
  overflow: hidden;
  background: var(--border);
  cursor: pointer;
}
`);

block(`
.book-editor-thumbs button[aria-pressed="true"] {
  border-color: var(--accent);
}
`);

block(`
.book-editor-thumbs picture {
  display: block;
  width: 100%;
  height: 100%;
}
`);

block(`
.book-editor-add {
  position: absolute;
  right: 4px;
  bottom: 4px;
  width: 24px;
  height: 24px;
  border-radius: 50%;
  background: var(--accent);
  color: var(--button-text);
  font-weight: 700;
  line-height: 24px;
}
`);

block(`
.book-editor-chapter h3 {
  display: flex;
  flex-wrap: wrap;
  align-items: baseline;
  gap: 4px 10px;
  margin: 18px 0 8px;
  font-size: 1rem;
}
`);

block(`
.book-editor-chapter h3 span {
  color: var(--muted);
  font-weight: 400;
  font-size: 0.85rem;
}
`);

block(`
.book-editor-items {
  list-style: none;
  margin: 0;
  padding: 0;
  display: grid;
  gap: 8px;
}
`);

block(`
.book-editor-item {
  display: grid;
  grid-template-columns: 64px 1fr;
  gap: 4px 12px;
  padding: 10px;
  border: 1px solid var(--border);
  border-radius: 10px;
  background: var(--surface);
}
`);

block(`
.book-editor-item.is-pinned {
  border-color: var(--accent-soft-border, var(--accent));
}
`);

block(`
.book-editor-item-thumb {
  width: 64px;
  height: 64px;
  overflow: hidden;
  border-radius: 6px;
  background: var(--border);
  display: grid;
  place-items: center;
  font-size: 1.6rem;
}
`);

block(`
.book-editor-item-thumb picture {
  display: block;
  width: 100%;
  height: 100%;
}
`);

block(`
.book-editor-item-body {
  min-width: 0;
}
`);

block(`
.book-editor-item-text {
  margin: 0;
  overflow-wrap: anywhere;
}
`);

block(`
.book-editor-badge {
  display: inline-block;
  margin-bottom: 4px;
  padding: 1px 8px;
  border-radius: 999px;
  background: var(--accent-soft, var(--hover-bg));
  font-size: 0.75rem;
}
`);

block(`
.book-editor-attached {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  margin-top: 6px;
}
`);

block(`
.book-editor-attached button {
  height: 40px;
  padding: 0 10px;
  border: 2px solid var(--border);
  border-radius: 6px;
  background: var(--surface);
  color: var(--text);
  font-size: 0.8rem;
  cursor: pointer;
}
`);

block(`
.book-editor-attached .book-editor-attached-thumb {
  width: 40px;
  padding: 0;
  overflow: hidden;
}
`);

block(`
.book-editor-attached button[aria-pressed="true"] {
  border-color: var(--accent);
}
`);

block(`
.book-editor-original {
  display: inline-block;
  margin-top: 6px;
  font-size: 0.85rem;
}
`);

block(`
.book-editor-item-actions {
  grid-column: 1 / -1;
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  justify-content: flex-end;
}
`);

block(`
.book-editor-item-actions button,
.book-editor-leftout button {
  min-width: 40px;
  min-height: 36px;
  padding: 4px 10px;
  border: 1px solid var(--control-border);
  border-radius: 8px;
  background: var(--surface);
  color: var(--text);
  cursor: pointer;
}
`);

block(`
.book-editor-item-actions button:disabled {
  opacity: 0.4;
  cursor: default;
}
`);

block(`
.book-editor-item-actions button[aria-pressed="true"] {
  background: var(--accent-soft, var(--hover-bg));
  border-color: var(--accent);
}
`);

block(`
.book-editor-month {
  border-bottom: 1px dashed var(--border);
}
`);

block(`
.book-editor-month-toggle {
  display: flex;
  justify-content: space-between;
  align-items: baseline;
  gap: 12px;
  width: 100%;
  padding: 10px 0;
  border: 0;
  background: none;
  color: var(--text);
  font: inherit;
  text-align: left;
  cursor: pointer;
}
`);

block(`
.book-editor-month-body {
  padding-bottom: 12px;
}
`);

block(`
.book-editor-leftout {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
  padding: 6px 0;
}
`);

block(`
.book-editor-danger {
  border-bottom: 0;
}
`);

block(`
.book-editor .book-editor-notes {
  margin: 20px 0;
  color: var(--muted);
  border-color: var(--border);
}
`);

block(`
.book-new-people {
  display: flex;
  flex-wrap: wrap;
  gap: 8px 18px;
  margin-bottom: 12px;
}
`);

block(`
.book-new-dates {
  margin: 16px 0;
  font-weight: 600;
}
`);

block(`
.book-editor-field select {
  padding: 8px 10px;
  border: 1px solid var(--control-border);
  border-radius: 8px;
  background: var(--surface);
  color: var(--text);
  font: inherit;
}
`);

block(`
.book-editor-addition {
  display: flex;
  align-items: center;
  gap: 10px;
  min-width: 0;
}
`);
