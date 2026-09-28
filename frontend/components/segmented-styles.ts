import { block } from "vlens/css";

block(`
.segmented {
  display: inline-flex;
  padding: 2px;
  border: 1px solid var(--border);
  border-radius: 999px;
  background: var(--bg);
}
`);

block(`
.segmented-option {
  min-height: 30px;
  padding: 4px 12px;
  border: none;
  border-radius: 999px;
  background: transparent;
  color: var(--muted);
  font: inherit;
  font-size: 0.85rem;
  font-weight: 600;
  cursor: pointer;
}
`);

block(`
.segmented-option.selected {
  background: var(--accent-soft);
  color: var(--text);
}
`);
