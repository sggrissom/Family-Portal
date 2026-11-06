import { block } from "vlens/css";

block(`
.faces-container {
  max-width: 960px;
  margin: 0 auto;
  padding: 2rem 1rem;
}
`);

block(`
.faces-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 1rem;
  margin-bottom: 1.5rem;
}
`);

block(`
.faces-header-actions {
  display: flex;
  gap: 0.5rem;
  flex-shrink: 0;
}
`);

block(`
.faces-subtitle,
.faces-hint {
  color: var(--muted);
  font-size: 0.95rem;
}
`);

block(`
.faces-error {
  color: var(--danger);
  margin-bottom: 1rem;
}
`);

block(`
.faces-notice {
  padding: 0.75rem 1rem;
  border-radius: 8px;
  background: var(--accent-soft);
  border: 1px solid var(--accent-soft-border);
  margin-bottom: 1rem;
}
`);

block(`
.faces-empty {
  padding: 2rem;
  text-align: center;
  color: var(--muted);
  border: 1px dashed var(--border);
  border-radius: 12px;
}
`);

block(`
.faces-section {
  margin-bottom: 2.5rem;
}
`);

block(`
.faces-count {
  font-size: 0.9rem;
  font-weight: 500;
  color: var(--muted);
  margin-left: 0.25rem;
}
`);

block(`
.face-group {
  display: flex;
  gap: 1.25rem;
  align-items: center;
  padding: 1rem;
  margin-bottom: 0.75rem;
  border: 1px solid var(--border);
  border-radius: 12px;
  background: var(--surface);
}
`);

block(`
.face-group-crops {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
  flex: 1;
  min-width: 0;
}
`);

block(`
.face-tile {
  position: relative;
}
`);

block(`
.face-open {
  position: absolute;
  top: 4px;
  right: 4px;
  width: 22px;
  height: 22px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 6px;
  background: rgba(0, 0, 0, 0.55);
  color: #fff;
  font-size: 0.75rem;
  text-decoration: none;
  opacity: 0;
  transition: opacity 0.15s;
}
`);

block(`
.face-tile:hover .face-open,
.face-open:focus-visible {
  opacity: 1;
}
`);

block(`
@media (hover: none) {
  .face-open {
    opacity: 1;
  }
}
`);

block(`
.face-toggle {
  padding: 0;
  border: none;
  background: none;
  cursor: pointer;
  border-radius: 10px;
}
`);

block(`
.face-toggle.excluded,
.face-toggle.rejected {
  opacity: 0.3;
  filter: grayscale(1);
}
`);

block(`
.face-toggle.rejected {
  opacity: 0.45;
  outline: 3px solid var(--danger);
  outline-offset: -3px;
}
`);

block(`
.face-group-more {
  width: 88px;
  height: 88px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 10px;
  border: 1px dashed var(--border);
  background: none;
  color: var(--muted);
  font: inherit;
  cursor: pointer;
}
`);

block(`
.face-group-controls {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  width: 240px;
  flex-shrink: 0;
}
`);

block(`
.face-group-family,
.face-group-meta {
  font-size: 0.85rem;
  color: var(--muted);
}
`);

block(`
.face-group-assign {
  display: flex;
  gap: 0.5rem;
}
`);

block(`
.face-group-assign select {
  flex: 1;
  min-width: 8rem;
}
`);

block(`
.face-dismiss {
  background: none;
  border: none;
  padding: 0.25rem 0;
  color: var(--muted);
  text-decoration: underline;
  cursor: pointer;
  font-size: 0.85rem;
  text-align: left;
}
`);

block(`
.faces-more {
  width: 100%;
}
`);

block(`
.auto-tag-name {
  font-weight: 600;
}
`);

block(`
@media (max-width: 640px) {
  .face-group {
    flex-direction: column;
    align-items: stretch;
  }
  .face-group-controls {
    width: auto;
  }
  .face-group-assign .btn {
    width: auto;
    flex: 0 0 auto;
  }
  .faces-header {
    flex-direction: column;
  }
}
`);

block(`
.faces-hint a {
  color: var(--accent);
}
`);
