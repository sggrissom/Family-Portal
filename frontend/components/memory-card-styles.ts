import { block } from "vlens/css";

block(`
.memory-featured {
  display: grid;
  grid-template-columns: minmax(0, 280px) minmax(0, 1fr);
  align-items: start;
  overflow: hidden;
  border: 1px solid var(--border);
  border-radius: 16px;
  background: var(--surface);
}
`);

block(`
.memory-featured.no-photo {
  grid-template-columns: minmax(0, 1fr);
}
`);

block(`
.memory-featured-photo {
  display: block;
  aspect-ratio: 1;
  background: var(--bg);
}
`);

block(`
.memory-featured-photo img {
  display: block;
  width: 100%;
  height: 100%;
  object-fit: cover;
}
`);

block(`
.memory-featured-body {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 18px 20px;
  min-width: 0;
}
`);

block(`
.memory-eyebrow {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  color: var(--muted);
  font-size: 0.8rem;
  font-weight: 700;
  letter-spacing: 0.04em;
  text-transform: uppercase;
}
`);

block(`
.memory-question {
  color: var(--muted);
  font-weight: 600;
}
`);

block(`
.memory-featured-text {
  color: var(--text);
  font-size: 1.25rem;
  font-weight: 600;
  line-height: 1.35;
  text-decoration: none;
  overflow-wrap: anywhere;
}
`);

block(`
.memory-featured-text:hover {
  text-decoration: underline;
}
`);

block(`
.memory-featured-text q,
.memory-tile-text q {
  font-style: italic;
}
`);

block(`
.memory-context {
  margin: 0;
  color: var(--muted);
  display: -webkit-box;
  -webkit-line-clamp: 3;
  -webkit-box-orient: vertical;
  overflow: hidden;
}
`);

block(`
.memory-when {
  color: var(--muted);
  font-size: 0.9rem;
}
`);

block(`
.memory-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-top: auto;
  padding-top: 6px;
}
`);

block(`
.memory-actions .btn {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}
`);

block(`
@media (max-width: 599px) {
  .memory-featured {
    grid-template-columns: minmax(0, 1fr);
  }

  .memory-featured-photo {
    aspect-ratio: 4 / 3;
  }
}
`);

block(`
.memory-tiles {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(240px, 1fr));
  gap: 8px;
}
`);

block(`
.memory-tile {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 8px;
  border: 1px solid var(--border);
  border-radius: 12px;
  background: var(--surface);
  color: var(--text);
  text-decoration: none;
  min-width: 0;
}
`);

block(`
.memory-tile:hover {
  border-color: var(--accent);
}
`);

block(`
.memory-tile-photo,
.memory-tile-icon {
  flex: none;
  width: 56px;
  height: 56px;
  border-radius: 8px;
  object-fit: cover;
}
`);

block(`
.memory-tile-icon {
  display: grid;
  place-items: center;
  background: var(--accent-soft);
  font-size: 1.2rem;
}
`);

block(`
.memory-tile-body {
  display: grid;
  gap: 2px;
  min-width: 0;
}
`);

block(`
.memory-tile-text {
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
  font-weight: 600;
}
`);

block(`
.memory-tile-meta {
  overflow: hidden;
  color: var(--muted);
  font-size: 0.82rem;
  text-overflow: ellipsis;
  white-space: nowrap;
}
`);
