import { block } from "vlens/css";

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
