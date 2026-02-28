import { block } from "vlens/css";

block(`
.manage-tags-container {
  max-width: 600px;
  margin: 0 auto;
  padding: 2rem 1rem;
}
`);

block(`
.tag-create-row {
  display: flex;
  gap: 0.75rem;
  align-items: center;
  margin-bottom: 2rem;
}
`);
block(`
.tag-create-row input[type="text"] {
  flex: 1;
}
`);

block(`
.tag-list-item {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  padding: 0.75rem 0;
  border-bottom: 1px solid var(--border);
}
`);

block(`
.tag-color-swatch {
  width: 18px;
  height: 18px;
  border-radius: 50%;
  flex-shrink: 0;
}
`);

block(`
.tag-name {
  flex: 1;
  font-size: 0.95rem;
  color: var(--text);
}
`);

block(`
.tag-color-input {
  width: 36px;
  height: 28px;
  border: 1px solid var(--border);
  border-radius: 4px;
  padding: 1px;
  cursor: pointer;
}
`);

block(`
.tag-edit-row {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  flex: 1;
}
`);
block(`
.tag-edit-row input[type="text"] {
  flex: 1;
}
`);

block(`
.tag-action-btn {
  background: none;
  border: none;
  cursor: pointer;
  font-size: 1rem;
  padding: 0.25rem;
  line-height: 1;
  opacity: 0.7;
}
`);
block(`
.tag-action-btn:hover {
  opacity: 1;
}
`);

block(`
.manage-tags-error {
  color: var(--error, red);
  font-size: 0.9rem;
  margin-bottom: 1rem;
}
`);

block(`
.manage-tags-empty {
  color: var(--muted);
  font-size: 0.9rem;
  padding: 1rem 0;
}
`);
