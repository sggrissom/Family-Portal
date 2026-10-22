import { block } from "vlens/css";

block(`
.video-badge {
  position: absolute;
  left: 8px;
  top: 8px;
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 2px 8px 2px 6px;
  border-radius: 12px;
  background: rgba(0, 0, 0, 0.65);
  color: #fff;
  font-size: 12px;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
  pointer-events: none;
  z-index: 1;
}
`);

block(`
.has-video-badge {
  position: relative;
  display: block;
}
`);

block(`
.video-badge .ui-icon {
  fill: currentColor;
}
`);

block(`
.video-player {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0.75rem;
  width: 100%;
}
`);

block(`
.video-player video {
  max-width: 100%;
  max-height: 70vh;
  border-radius: 8px;
  background: #000;
  box-shadow: 0 4px 16px rgba(0, 0, 0, 0.15);
}
`);

block(`
.video-player-poster {
  position: relative;
  display: inline-block;
  max-width: 100%;
}
`);

block(`
.video-player-poster img {
  max-width: 100%;
  max-height: 70vh;
  border-radius: 8px;
  opacity: 0.85;
}
`);

block(`
.video-player-note {
  max-width: 36rem;
  text-align: center;
  color: var(--muted);
  font-size: 0.95rem;
}
`);

block(`
.video-player-note strong {
  display: block;
  color: var(--text);
  margin-bottom: 0.25rem;
}
`);

block(`
.video-player-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
  justify-content: center;
}
`);

block(`
@media (max-width: 768px) {
  .video-player video,
  .video-player-poster img {
    max-height: 50vh;
  }
}
`);

block(`
.video-transcript {
  width: 100%;
  max-width: 44rem;
  border: 1px solid var(--border);
  border-radius: 8px;
  padding: 0.5rem 0.75rem;
  background: var(--surface);
}
`);

block(`
.video-transcript summary {
  cursor: pointer;
  font-weight: 600;
  display: flex;
  flex-wrap: wrap;
  align-items: baseline;
  gap: 0.5rem;
}
`);

block(`
.video-transcript-hint,
.video-transcript-note {
  color: var(--muted);
  font-size: 0.85rem;
  font-weight: normal;
}
`);

block(`
.video-transcript-lines {
  list-style: none;
  margin: 0.5rem 0 0;
  padding: 0;
  max-height: 18rem;
  overflow-y: auto;
}
`);

block(`
.video-transcript-lines li {
  display: flex;
  gap: 0.75rem;
  align-items: flex-start;
  padding: 0.25rem 0.25rem;
  border-radius: 6px;
}
`);

block(`
.video-transcript-lines li.active {
  background: var(--accent-soft);
}
`);

block(`
.video-transcript-lines li.unsure span {
  color: var(--muted);
}
`);

block(`
.video-transcript-lines textarea {
  flex: 1;
  font: inherit;
  resize: vertical;
}
`);

block(`
.video-transcript-time {
  flex-shrink: 0;
  min-width: 3.5rem;
  padding: 0.1rem 0.4rem;
  border: none;
  border-radius: 4px;
  background: var(--hover-bg);
  color: var(--text);
  font-variant-numeric: tabular-nums;
  cursor: pointer;
}
`);

block(`
.video-transcript-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
  margin-top: 0.5rem;
}
`);

block(`
.video-transcript-error {
  color: var(--danger);
  font-size: 0.9rem;
}
`);
