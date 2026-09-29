import { block } from "vlens/css";

block(`
.suggestions-container {
  max-width: 960px;
  margin: 0 auto;
  padding: 2rem 1rem;
}
`);

block(`
.suggestions-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 1rem;
  margin-bottom: 1.5rem;
}
`);

block(`
.suggestions-subtitle,
.suggestions-hint {
  color: var(--muted);
  font-size: 0.95rem;
}
`);

block(`
.suggestions-error {
  color: var(--danger);
  margin-bottom: 1rem;
}
`);

block(`
.suggestions-notice {
  padding: 0.75rem 1rem;
  border-radius: 8px;
  background: var(--accent-soft);
  border: 1px solid var(--accent-soft-border);
  margin-bottom: 1rem;
}
`);

block(`
.suggestions-empty {
  padding: 2rem;
  text-align: center;
  color: var(--muted);
  border: 1px dashed var(--border);
  border-radius: 12px;
}
`);

block(`
.suggestion-group {
  padding: 1rem;
  margin-bottom: 1.25rem;
  border: 1px solid var(--border);
  border-radius: 12px;
  background: var(--surface);
}
`);

block(`
.suggestion-group-header {
  display: flex;
  flex-wrap: wrap;
  justify-content: space-between;
  align-items: center;
  gap: 0.75rem;
}
`);

block(`
.suggestion-group-header h2 {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  margin: 0;
  font-size: 1.2rem;
}
`);

block(`
.suggestion-group-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
}
`);

block(`
.suggestion-swatch {
  width: 14px;
  height: 14px;
  border-radius: 50%;
  display: inline-block;
}
`);

block(`
.suggestions-count {
  font-size: 0.9rem;
  font-weight: 500;
  color: var(--muted);
}
`);

block(`
.suggestion-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(110px, 1fr));
  gap: 0.5rem;
}
`);

block(`
.suggestion-photo {
  padding: 0;
  border: 3px solid var(--primary-accent);
  border-radius: 8px;
  overflow: hidden;
  aspect-ratio: 1;
  cursor: pointer;
  background: var(--bg);
}
`);

block(`
.suggestion-photo.excluded {
  border-color: transparent;
  opacity: 0.35;
}
`);

block(`
.suggestion-thumb {
  width: 100%;
  height: 100%;
  object-fit: cover;
  display: block;
}
`);

block(`
.suggestions-header .btn {
  white-space: nowrap;
  flex-shrink: 0;
}
`);
