import { block } from "vlens/css";

block(`
.share-dialog-backdrop {
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
.share-dialog {
  display: grid;
  gap: 12px;
  width: min(560px, 100%);
  max-height: calc(100vh - 32px);
  overflow-y: auto;
  padding: 16px;
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 16px;
}
`);

block(`
.share-dialog-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}
`);

block(`
.share-dialog-head h2 {
  margin: 0;
  font-size: 1.1rem;
}
`);

block(`
.share-dialog-close {
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
.share-dialog-preview {
  display: grid;
  gap: 8px;
  justify-items: center;
  max-height: 55vh;
  overflow-y: auto;
  padding: 8px;
  border: 1px solid var(--border);
  border-radius: 12px;
  background: var(--bg);
}
`);

block(`
.share-dialog-preview img {
  display: block;
  width: 100%;
  max-width: 420px;
  height: auto;
  border-radius: 8px;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.25);
}
`);

block(`
.share-dialog-note {
  margin: 0;
  color: var(--muted);
  font-size: 0.9rem;
}
`);

block(`
.share-dialog-options {
  display: grid;
  gap: 6px;
}
`);

block(`
.share-dialog-options h3 {
  margin: 8px 0 2px;
  font-size: 0.95rem;
}
`);

block(`
.share-toggle {
  display: flex;
  align-items: center;
  gap: 10px;
  min-height: 36px;
  cursor: pointer;
}
`);

block(`
.share-dialog-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}
`);
