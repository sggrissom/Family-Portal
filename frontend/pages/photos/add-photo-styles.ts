import { block } from "vlens/css";

block(`
.photo-drop {
  margin-bottom: 18px;
  border: 2px dashed var(--border);
  border-radius: 14px;
}
`);

block(`
.photo-drop.has-items {
  border: none;
}
`);

block(`
.photo-drop.drag-active {
  border-color: var(--accent);
  background: var(--accent-soft);
}
`);

block(`
.photo-drop-empty {
  display: grid;
  justify-items: center;
  gap: 10px;
  padding: 36px 16px;
  color: var(--muted);
}
`);

block(`
.upload-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(96px, 1fr));
  gap: 8px;
}
`);

block(`
.upload-tile {
  position: relative;
  aspect-ratio: 1;
  overflow: hidden;
  border-radius: 10px;
  background: var(--bg);
}
`);

block(`
.upload-tile img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}
`);

block(`
.upload-tile.upload-queued img,
.upload-tile.upload-uploading img {
  opacity: 0.55;
}
`);

block(`
.upload-tile-status {
  position: absolute;
  left: 6px;
  bottom: 6px;
  max-width: calc(100% - 12px);
  padding: 2px 6px;
  overflow: hidden;
  border-radius: 6px;
  background: rgba(15, 23, 42, 0.7);
  color: #fff;
  font-size: 0.72rem;
  text-overflow: ellipsis;
  white-space: nowrap;
}
`);

block(`
.upload-tile.upload-failed {
  outline: 2px solid var(--danger);
}
`);

block(`
.upload-tile-actions {
  position: absolute;
  top: 6px;
  left: 6px;
  right: 6px;
  display: flex;
  gap: 4px;
}
`);

block(`
.upload-tile-actions button {
  flex: 1;
  padding: 3px 0;
  border: none;
  border-radius: 6px;
  background: var(--surface);
  color: var(--text);
  font: inherit;
  font-size: 0.72rem;
  cursor: pointer;
}
`);

block(`
.upload-add {
  display: grid;
  place-content: center;
  justify-items: center;
  gap: 2px;
  border: 2px dashed var(--border);
  color: var(--muted);
  font-size: 0.8rem;
  cursor: pointer;
}
`);

block(`
.upload-add span {
  font-size: 1.6rem;
  line-height: 1;
}
`);

block(`
.photo-taken {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
}
`);

block(`
.link-button {
  padding: 0;
  border: none;
  background: none;
  color: var(--accent);
  font: inherit;
  text-decoration: underline;
  cursor: pointer;
}
`);

block(`
.tag-picker {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
}
`);

block(`
.tag-pill {
  display: inline-flex;
  font-family: inherit;
  color: inherit;
  align-items: center;
  gap: 0.4rem;
  padding: 0.3rem 0.75rem;
  border-radius: 999px;
  border: 2px solid transparent;
  background: var(--surface);
  cursor: pointer;
  font-size: 0.85rem;
  user-select: none;
  transition: background 0.15s;
}
`);

block(`
.tag-pill.selected {
  background: color-mix(in srgb, var(--surface) 60%, currentColor 40%);
}
`);

block(`
.tag-color-dot {
  width: 10px;
  height: 10px;
  border-radius: 50%;
  flex-shrink: 0;
}
`);
