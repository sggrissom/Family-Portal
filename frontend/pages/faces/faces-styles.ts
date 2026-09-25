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
.face-toggle {
  padding: 0;
  border: none;
  background: none;
  cursor: pointer;
  border-radius: 10px;
}
`);

block(`
.face-toggle.excluded {
  opacity: 0.3;
  filter: grayscale(1);
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
  color: var(--muted);
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
.auto-tag-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(150px, 1fr));
  gap: 0.75rem;
}
`);

block(`
.auto-tag-card {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0.5rem;
  padding: 0.75rem;
  border: 1px solid var(--border);
  border-radius: 12px;
  background: var(--surface);
}
`);

block(`
.auto-tag-name {
  font-weight: 600;
}
`);

block(`
.auto-tag-actions {
  display: flex;
  gap: 0.35rem;
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
