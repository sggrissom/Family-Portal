import { block } from "vlens/css";

block(`
.person-chips {
  display: flex;
  gap: 8px;
  overflow-x: auto;
  padding: 2px 2px 12px;
  margin-bottom: 8px;
  scrollbar-width: none;
}
`);

block(`
.person-chip {
  display: inline-flex;
  flex-shrink: 0;
  align-items: center;
  gap: 8px;
  min-height: 40px;
  padding: 4px 14px 4px 4px;
  border: 1px solid var(--border);
  border-radius: 999px;
  background: var(--bg);
  color: var(--text);
  font: inherit;
  font-weight: 600;
  cursor: pointer;
}
`);

block(`
.person-chip.selected {
  border-color: var(--accent);
  background: var(--accent-soft);
}
`);

block(`
.person-chip-avatar {
  display: grid;
  place-items: center;
  width: 32px;
  height: 32px;
  overflow: hidden;
  border-radius: 50%;
  background: var(--hover-bg);
  font-size: 0.85rem;
}
`);

block(`
.person-chip-avatar img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}
`);
