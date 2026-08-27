import { block } from "vlens/css";

block(`
.crop-selector-modal {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: rgba(0, 0, 0, 0.8);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
  padding: 1rem;
  overscroll-behavior: contain;
}
`);

block(`
.crop-selector-content {
  background: var(--surface);
  border-radius: 12px;
  max-width: 600px;
  width: 100%;
  max-height: 90vh;
  overflow-y: auto;
  box-shadow: 0 8px 32px rgba(0, 0, 0, 0.3);
}
`);

block(`
.crop-selector-header {
  padding: 1.5rem;
  border-bottom: 1px solid var(--border);
  text-align: center;
}
`);

block(`
.crop-selector-header h2 {
  margin: 0 0 0.5rem 0;
  font-size: 1.5rem;
  color: var(--text);
}
`);

block(`
.crop-selector-header p {
  margin: 0;
  color: var(--muted);
  font-size: 0.9rem;
}
`);

block(`
.crop-selector-body {
  padding: 1.5rem;
  display: flex;
  flex-direction: column;
  gap: 1.5rem;
}
`);

block(`
.crop-container {
  position: relative;
  width: 100%;
  aspect-ratio: 1 / 1;
  overflow: hidden;
  border-radius: 8px;
  background: #000;
  cursor: grab;
  touch-action: none;
  user-select: none;
  -webkit-user-select: none;
}
`);

block(`
.crop-container.dragging {
  cursor: grabbing;
}
`);

block(`
.crop-image-wrapper {
  position: absolute;
  top: 0;
  left: 0;
  width: 100%;
  height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: none;
}
`);

block(`
.crop-image {
  width: 100%;
  height: 100%;
  object-fit: cover;
  pointer-events: none;
}
`);

block(`
.crop-overlay {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  pointer-events: none;
}
`);

block(`
.crop-circle {
  position: absolute;
  top: 10%;
  left: 10%;
  right: 10%;
  bottom: 10%;
  border-radius: 50%;
  box-shadow: 0 0 0 9999px rgba(0, 0, 0, 0.5);
  border: 2px solid rgba(255, 255, 255, 0.8);
}
`);

block(`
.crop-preview-section {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0.75rem;
}
`);

block(`
.crop-preview-section h4 {
  margin: 0;
  font-size: 0.9rem;
  color: var(--muted);
}
`);

block(`
.crop-preview-container {
  width: 100px;
  height: 100px;
  border-radius: 50%;
  overflow: hidden;
  border: 3px solid var(--border);
  background: #000;
}
`);

block(`
.crop-controls {
  padding: 0 1.5rem;
  display: flex;
  align-items: center;
  gap: 1rem;
}
`);

block(`
.zoom-label {
  display: flex;
  align-items: center;
  gap: 1rem;
  flex: 1;
  font-size: 0.9rem;
  color: var(--text);
}
`);

block(`
.zoom-label span {
  min-width: 80px;
}
`);

block(`
.zoom-slider {
  flex: 1;
  height: 8px;
  border-radius: 4px;
  background: var(--border);
  cursor: pointer;
  accent-color: var(--accent);
  touch-action: none;
}
`);

block(`
.crop-selector-actions {
  padding: 1.5rem;
  border-top: 1px solid var(--border);
  display: flex;
  justify-content: flex-end;
  gap: 1rem;
}
`);

block(`
.crop-selector-actions .btn {
  padding: 0.75rem 1.5rem;
  font-size: 1rem;
  border-radius: 6px;
  cursor: pointer;
  transition: all 0.2s ease;
}
`);

block(`
.crop-selector-actions .btn-outline {
  background: transparent;
  color: var(--text);
  border: 1px solid var(--border);
}
`);

block(`
.crop-selector-actions .btn-outline:hover {
  background: var(--hover-bg);
  border-color: var(--muted);
}
`);

block(`
@media (max-width: 480px) {
  .crop-selector-content {
    max-height: 95vh;
  }

  .crop-selector-header,
  .crop-selector-body {
    padding: 1rem;
  }

  .crop-controls {
    padding: 0 1rem;
    flex-direction: column;
    align-items: stretch;
  }

  .zoom-label {
    flex-direction: column;
    align-items: stretch;
    gap: 0.5rem;
  }

  .zoom-label span {
    text-align: center;
  }

  .crop-selector-actions {
    padding: 1rem;
    flex-direction: column;
  }

  .crop-selector-actions .btn {
    width: 100%;
  }
}
`);
