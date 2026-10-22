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
