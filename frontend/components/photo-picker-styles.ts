import { block } from "vlens/css";

block(`
.photo-picker {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  padding: 8px;
  border: 1px solid var(--border);
  border-radius: 12px;
  background: var(--surface);
  max-height: 240px;
  overflow-y: auto;
}
`);

block(`
.photo-picker-empty {
  color: var(--muted);
  font-size: 14px;
  padding: 4px;
  margin: 0;
}
`);

block(`
.photo-picker-item {
  position: relative;
  cursor: pointer;
  border-radius: 6px;
  overflow: hidden;
  border: 2px solid transparent;
  width: 72px;
  height: 72px;
  flex-shrink: 0;
  padding: 0;
  background: none;
}
`);

block(`
.photo-picker-item.selected {
  border-color: var(--accent);
}
`);

block(`
.photo-picker-item:disabled {
  cursor: default;
  opacity: 0.6;
}
`);

block(`
.photo-picker-img {
  width: 72px;
  height: 72px;
  object-fit: cover;
  display: block;
}
`);

block(`
.photo-picker-check {
  position: absolute;
  top: 4px;
  right: 4px;
  width: 18px;
  height: 18px;
  background: var(--accent);
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  color: white;
  font-size: 11px;
  font-weight: bold;
  line-height: 1;
}
`);

block(`
.photo-strip {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  margin: 0.5rem 0 0;
}
`);

block(`
.photo-strip-item {
  display: block;
  width: 64px;
  height: 64px;
  border-radius: 6px;
  overflow: hidden;
}
`);

block(`
.photo-strip-img {
  width: 64px;
  height: 64px;
  object-fit: cover;
  display: block;
}
`);
