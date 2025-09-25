import { block } from "vlens/css";

// Chart-specific styles
block(`
.growth-chart-container {
  background: var(--bg);
  border: 1px solid var(--border);
  border-radius: 12px;
  padding: 24px;
  text-align: center;
}
`);

block(`
.growth-chart-container h3 {
  margin: 0 0 16px;
  color: var(--text);
}
`);

block(`
.growth-chart {
  display: flex;
  flex-direction: column;
  align-items: center;
  min-height: 200px;
  gap: 16px;
}
`);


block(`
.grid-line {
  stroke: var(--border);
  stroke-width: 1;
  opacity: 0.5;
}
`);

block(`
.axis-line {
  stroke: var(--text);
  stroke-width: 2;
}
`);

block(`
.chart-line {
  stroke-width: 3;
  filter: drop-shadow(0 2px 4px rgba(0, 0, 0, 0.1));
}
`);

block(`
.data-point {
  cursor: pointer;
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
  filter: drop-shadow(0 2px 4px rgba(0, 0, 0, 0.2));
  transform-origin: center;
  /* Larger touch targets on mobile */
  stroke-width: 8;
  stroke: transparent;
}
`);

block(`
@media (hover: hover) {
  .data-point {
    stroke: transparent;
    stroke-width: 0;
  }
}
`);

block(`
.data-point:hover,
.data-point.hovered {
  filter: drop-shadow(0 4px 8px rgba(0, 0, 0, 0.3));
}
`);

block(`
.data-point.selected {
  filter: drop-shadow(0 4px 12px rgba(0, 0, 0, 0.5));
  animation: pulse 2s infinite;
}
`);

block(`
.data-point:focus {
  outline: 3px solid var(--primary-accent);
  outline-offset: 2px;
  filter: drop-shadow(0 4px 8px rgba(0, 0, 0, 0.3));
}
`);

block(`
@keyframes pulse {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.8; }
}
`);

block(`
@keyframes ripple {
  0% {
    transform: scale(1);
    opacity: 1;
  }
  100% {
    transform: scale(1.4);
    opacity: 0;
  }
}
`);

block(`
.data-label {
  fill: var(--text);
  font-size: 11px;
  font-weight: 600;
  text-shadow: 0 0 3px var(--bg), 0 0 3px var(--bg);
  opacity: 0.9;
}
`);

block(`
.height-label {
  fill: #3b82f6;
}
`);

block(`
.weight-label {
  fill: #ef4444;
}
`);

block(`
.axis-label {
  fill: var(--muted);
  font-size: 10px;
  font-weight: 500;
}
`);

block(`
.height-axis-label {
  fill: #3b82f6;
}
`);

block(`
.weight-axis-label {
  fill: #ef4444;
}
`);

block(`
.data-point-info {
  width: 100%;
  max-width: 600px;
  margin: 0 auto;
  padding: 16px;
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 8px;
  min-height: 80px;
  display: flex;
  flex-direction: column;
  justify-content: center;
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
}
`);

block(`
.data-point-info:not(.placeholder) {
  animation: panelSlideIn 0.3s ease-out;
}
`);

block(`
@keyframes panelSlideIn {
  0% {
    transform: translateY(-10px);
  }
  100% {
    transform: translateY(0);
  }
}
`);

block(`
.data-point-info.placeholder {
  text-align: center;
  color: var(--muted);
  font-style: italic;
}
`);

block(`
.info-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 8px;
}
`);

block(`
.info-type {
  font-weight: 600;
  color: var(--text);
  font-size: 14px;
}
`);

block(`
.info-date {
  color: var(--muted);
  font-size: 12px;
}
`);

block(`
.info-value {
  font-size: 24px;
  font-weight: 700;
  color: var(--text);
  margin-bottom: 4px;
}
`);

block(`
.info-hint {
  color: var(--muted);
  font-size: 11px;
  font-style: italic;
}
`);

block(`
.chart-tooltip {
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 8px;
  padding: 8px 12px;
  font-size: 12px;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
  transform: translate(-50%, -100%);
  opacity: 0;
  animation: tooltipFadeIn 0.2s ease-out forwards;
  backdrop-filter: blur(8px);
  max-width: 200px;
  white-space: nowrap;
}
`);

block(`
@keyframes tooltipFadeIn {
  0% {
    opacity: 0;
    transform: translate(-50%, -90%);
  }
  100% {
    opacity: 1;
    transform: translate(-50%, -100%);
  }
}
`);

block(`
.tooltip-content {
  display: flex;
  flex-direction: column;
  gap: 2px;
}
`);

block(`
.tooltip-type {
  font-weight: 600;
  color: var(--muted);
  font-size: 10px;
  text-transform: uppercase;
  letter-spacing: 0.05em;
}
`);

block(`
.tooltip-value {
  font-weight: 600;
  color: var(--text);
  font-size: 14px;
}
`);

block(`
.tooltip-date {
  font-size: 10px;
  color: var(--muted);
  margin-top: 2px;
}
`);

block(`
.legend-text {
  fill: var(--text);
  font-size: 12px;
  font-weight: 500;
}
`);

block(`
.chart-placeholder {
  height: 200px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--surface);
  border-radius: 8px;
  color: var(--muted);
  font-size: 1.1rem;
}
`);

block(`
.zoom-controls {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 8px 12px;
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 6px;
  margin-bottom: 8px;
  font-size: 12px;
}
`);

block(`
.btn-zoom-reset {
  padding: 4px 8px;
  background: var(--accent);
  color: white;
  border: none;
  border-radius: 4px;
  font-size: 11px;
  font-weight: 600;
  cursor: pointer;
  transition: background-color 0.2s ease;
}
`);

block(`
.btn-zoom-reset:hover {
  background: var(--primary-accent);
}
`);

block(`
.zoom-level {
  color: var(--muted);
  font-weight: 600;
  font-size: 11px;
}
`);

block(`
.growth-chart-svg {
  width: 100%;
  height: auto;
  max-width: 100%;
  touch-action: none;
  user-select: none;
}
`);

block(`
@media (max-width: 768px) {
  .growth-chart-container {
    padding: 12px;
  }

  .growth-chart {
    gap: 12px;
  }

  .growth-chart-svg {
    width: 100%;
    height: auto;
    max-width: 100%;
  }

  .legend-text {
    font-size: 8px;
  }

  .chart-placeholder {
    height: 150px;
    font-size: 0.9rem;
  }

  .data-point-info {
    padding: 12px;
    font-size: 14px;
  }

  .info-value {
    font-size: 20px;
  }

  .info-type {
    font-size: 12px;
  }

  .info-date {
    font-size: 10px;
  }

  .info-hint {
    font-size: 10px;
  }
}
`);