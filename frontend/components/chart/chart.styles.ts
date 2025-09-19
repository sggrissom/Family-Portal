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
  justify-content: center;
  align-items: center;
  min-height: 200px;
}
`);

block(`
.growth-chart-svg {
  max-width: 100%;
  height: auto;
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
  transition: all 0.2s ease;
  filter: drop-shadow(0 2px 4px rgba(0, 0, 0, 0.2));
}
`);

block(`
.data-point:hover {
  filter: drop-shadow(0 4px 8px rgba(0, 0, 0, 0.4));
}
`);

block(`
.data-point.selected {
  filter: drop-shadow(0 4px 12px rgba(0, 0, 0, 0.5));
  animation: pulse 2s infinite;
}
`);

block(`
@keyframes pulse {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.8; }
}
`);

block(`
.data-point-info {
  margin-top: 16px;
  padding: 16px;
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 8px;
  min-height: 80px;
  display: flex;
  flex-direction: column;
  justify-content: center;
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
@media (max-width: 768px) {
  .growth-chart-container {
    padding: 16px;
  }

  .growth-chart-svg {
    width: 100%;
    height: 300px;
  }

  .legend-text {
    font-size: 10px;
  }

  .chart-placeholder {
    height: 150px;
    font-size: 1rem;
  }
}
`);