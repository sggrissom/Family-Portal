import { block } from "vlens/css";
import "./admin-tokens";

block(`
:root {
  --analytics-secondary: #8b5cf6;
  --analytics-success: #10b981;
  --analytics-warning: #f59e0b;
  --analytics-error: #ef4444;
  --analytics-chart-1: #3b82f6;
  --analytics-chart-2: #8b5cf6;
  --analytics-chart-3: #10b981;
  --analytics-chart-4: #f59e0b;
  --analytics-chart-5: #ef4444;
  --analytics-chart-6: #06b6d4;
}
`);

block(`
[data-theme="dark"] {
  --analytics-secondary: #a78bfa;
  --analytics-success: #34d399;
  --analytics-warning: #fbbf24;
  --analytics-error: #f87171;
  --analytics-chart-1: #60a5fa;
  --analytics-chart-2: #a78bfa;
  --analytics-chart-3: #34d399;
  --analytics-chart-4: #fbbf24;
  --analytics-chart-5: #f87171;
  --analytics-chart-6: #22d3ee;
}
`);

block(`
.admin-container.analytics-container {
  max-width: 1400px;
}
`);

block(`
.admin-header.analytics-header {
  background: linear-gradient(135deg, var(--admin-accent) 0%, var(--analytics-secondary) 100%);
}
`);

block(`
.analytics-header h1 {
  margin: 0 0 0.5rem 0;
  font-size: 2.5rem;
  font-weight: 700;
  text-shadow: 0 2px 4px rgba(0, 0, 0, 0.1);
}
`);

block(`
.analytics-header p {
  margin: 0;
  font-size: 1.1rem;
  opacity: 0.9;
}
`);

block(`
.analytics-controls {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 2rem;
  background: var(--surface);
  padding: 1rem;
  border-radius: 8px;
  border: 1px solid var(--border);
}
`);

block(`
.view-selector {
  display: flex;
  gap: 0.25rem;
  background: var(--bg);
  padding: 0.25rem;
  border-radius: 6px;
  border: 1px solid var(--border);
}
`);

block(`
.view-btn {
  padding: 0.5rem 1rem;
  border: none;
  background: transparent;
  color: var(--text);
  border-radius: 4px;
  cursor: pointer;
  font-size: 0.875rem;
  font-weight: 500;
  transition: all 0.2s ease;
}
`);

block(`
.view-btn:hover {
  background: var(--hover-bg);
}
`);

block(`
.view-btn.active {
  background: var(--admin-accent);
  color: white;
}
`);

block(`
.analytics-content {
  display: flex;
  flex-direction: column;
  gap: 2rem;
}
`);

block(`
.metrics-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
  gap: 1.5rem;
}
`);

block(`
.metric-card {
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 8px;
  padding: 1.5rem;
  text-align: center;
  transition: all 0.3s ease;
  box-shadow: 0 2px 4px rgba(0, 0, 0, 0.05);
}
`);

block(`
.metric-card:hover {
  transform: translateY(-2px);
  box-shadow: 0 4px 8px rgba(0, 0, 0, 0.1);
  border-color: var(--admin-accent);
}
`);

block(`
.metric-value {
  font-size: 2.5rem;
  font-weight: 700;
  color: var(--admin-accent);
  margin-bottom: 0.5rem;
  line-height: 1;
}
`);

block(`
.metric-label {
  font-size: 0.875rem;
  color: var(--muted);
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.05em;
}
`);

block(`
.metric-change {
  font-size: 0.75rem;
  margin-top: 0.5rem;
  padding: 0.25rem 0.5rem;
  border-radius: 4px;
  background: var(--bg);
}
`);

block(`
.metric-change.positive {
  color: var(--analytics-success);
  background: rgba(16, 185, 129, 0.1);
}
`);

block(`
.metric-change.negative {
  color: var(--analytics-error);
  background: rgba(239, 68, 68, 0.1);
}
`);

block(`
.charts-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(400px, 1fr));
  gap: 1.5rem;
}
`);

block(`
.chart-card {
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 8px;
  padding: 1.5rem;
  box-shadow: 0 2px 4px rgba(0, 0, 0, 0.05);
}
`);

block(`
.chart-card h3 {
  margin: 0 0 1rem 0;
  font-size: 1.125rem;
  font-weight: 600;
  color: var(--text);
  border-bottom: 2px solid var(--admin-accent);
  padding-bottom: 0.5rem;
}
`);

block(`
.analytics-chart-placeholder {
  display: flex;
  align-items: center;
  justify-content: center;
  height: 200px;
  background: var(--bg);
  border: 2px dashed var(--border);
  border-radius: 6px;
  color: var(--muted);
  font-style: italic;
}
`);

block(`
.analytics-chart-placeholder.large {
  min-height: 400px;
  font-size: 1.2rem;
}
`);

block(`
.simple-line-chart {
  position: relative;
  height: 200px;
  background: var(--bg);
  border-radius: 6px;
  overflow: hidden;
}
`);

block(`
.chart-area {
  position: relative;
  height: 180px;
  margin: 10px;
}
`);

block(`
.chart-point {
  position: absolute;
  width: 8px;
  height: 8px;
  background: var(--analytics-chart-1);
  border-radius: 50%;
  transform: translate(-50%, 50%);
  border: 2px solid white;
  box-shadow: 0 2px 4px rgba(0, 0, 0, 0.2);
  cursor: pointer;
  transition: all 0.2s ease;
}
`);

block(`
.chart-point:hover {
  width: 12px;
  height: 12px;
  background: var(--admin-accent);
}
`);

block(`
.chart-labels {
  display: flex;
  justify-content: space-between;
  padding: 0 10px;
  font-size: 0.75rem;
  color: var(--muted);
}
`);

block(`
.stacked-bar-chart {
  background: var(--bg);
  border-radius: 6px;
  padding: 1rem;
}
`);

block(`
.stacked-bar-chart .chart-area {
  display: flex;
  align-items: end;
  justify-content: space-between;
  height: 200px;
  gap: 8px;
  margin-bottom: 1rem;
}
`);

block(`
.stacked-bar-item {
  display: flex;
  flex-direction: column;
  align-items: center;
  flex: 1;
  height: 100%;
}
`);

block(`
.stacked-bar {
  display: flex;
  flex-direction: column;
  align-items: center;
  width: 100%;
  height: 100%;
  max-width: 40px;
  position: relative;
}
`);

block(`
.bar-segment {
  width: 100%;
  cursor: pointer;
  transition: all 0.2s ease;
  border-radius: 2px 2px 0 0;
}
`);

block(`
.bar-segment:hover {
  transform: scaleX(1.1);
}
`);

block(`
.bar-segment.photos {
  background: var(--analytics-chart-1);
}
`);

block(`
.bar-segment.milestones {
  background: var(--analytics-chart-2);
}
`);

block(`
.bar-segment.logins {
  background: var(--analytics-chart-3);
}
`);

block(`
.stacked-bar-item .bar-label {
  font-size: 0.7rem;
  color: var(--muted);
  margin-top: 0.5rem;
}
`);

block(`
.bar-total {
  font-size: 0.75rem;
  color: var(--text);
  font-weight: 600;
  margin-top: 0.25rem;
}
`);

block(`
.chart-legend {
  display: flex;
  justify-content: center;
  gap: 1rem;
  margin-top: 0.5rem;
}
`);

block(`
.legend-item {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  font-size: 0.875rem;
}
`);

block(`
.legend-color {
  width: 16px;
  height: 16px;
  border-radius: 4px;
}
`);

block(`
.legend-color.photos {
  background: var(--analytics-chart-1);
}
`);

block(`
.legend-color.milestones {
  background: var(--analytics-chart-2);
}
`);

block(`
.legend-color.logins {
  background: var(--analytics-chart-3);
}
`);

block(`
.simple-bar-chart {
  display: flex;
  align-items: end;
  justify-content: space-between;
  height: 200px;
  padding: 10px;
  background: var(--bg);
  border-radius: 6px;
  gap: 4px;
}
`);

block(`
.bar-item {
  display: flex;
  flex-direction: column;
  align-items: center;
  flex: 1;
  height: 100%;
}
`);

block(`
.bar {
  background: linear-gradient(0deg, var(--analytics-chart-1), var(--analytics-chart-2));
  border-radius: 4px 4px 0 0;
  width: 100%;
  min-height: 4px;
  transition: all 0.3s ease;
  cursor: pointer;
}
`);

block(`
.bar:hover {
  background: linear-gradient(0deg, var(--admin-accent), var(--analytics-secondary));
  transform: scaleY(1.1);
}
`);

block(`
.bar-label {
  font-size: 0.7rem;
  color: var(--muted);
  margin-top: 0.5rem;
  writing-mode: horizontal-tb;
}
`);

block(`
.simple-pie-chart {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
  padding: 1rem;
  background: var(--bg);
  border-radius: 6px;
  max-height: 200px;
  overflow-y: auto;
}
`);

block(`
.pie-item {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  font-size: 0.875rem;
}
`);

block(`
.pie-color {
  width: 16px;
  height: 16px;
  border-radius: 50%;
  flex-shrink: 0;
}
`);

block(`
.pie-label {
  flex: 1;
  color: var(--text);
  font-weight: 500;
}
`);

block(`
.pie-value {
  color: var(--muted);
  font-weight: 400;
  font-size: 0.8rem;
}
`);

block(`
.health-indicators {
  display: flex;
  flex-direction: column;
  gap: 1rem;
  background: var(--bg);
  padding: 1rem;
  border-radius: 6px;
}
`);

block(`
.health-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 0.75rem;
  background: var(--surface);
  border-radius: 6px;
  border: 1px solid var(--border);
}
`);

block(`
.health-label {
  font-size: 0.875rem;
  color: var(--text);
  font-weight: 500;
}
`);

block(`
.health-value {
  font-size: 1.25rem;
  font-weight: 600;
  color: var(--analytics-success);
}
`);

block(`
.health-value.error {
  color: var(--analytics-error);
}
`);

block(`
.health-value.warning {
  color: var(--analytics-warning);
}
`);

block(`
.retention-metrics {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 1rem;
  background: var(--bg);
  padding: 1rem;
  border-radius: 6px;
}
`);

block(`
.retention-item {
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 1rem;
  background: var(--surface);
  border-radius: 6px;
  border: 1px solid var(--border);
}
`);

block(`
.retention-label {
  font-size: 0.75rem;
  color: var(--muted);
  text-transform: uppercase;
  letter-spacing: 0.05em;
  margin-bottom: 0.5rem;
}
`);

block(`
.retention-value {
  font-size: 1.5rem;
  font-weight: 700;
  color: var(--admin-accent);
}
`);

block(`
.table-card {
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 8px;
  padding: 1.5rem;
  box-shadow: 0 2px 4px rgba(0, 0, 0, 0.05);
}
`);

block(`
.table-card h3 {
  margin: 0 0 1rem 0;
  font-size: 1.125rem;
  font-weight: 600;
  color: var(--text);
  border-bottom: 2px solid var(--admin-accent);
  padding-bottom: 0.5rem;
}
`);

block(`
.families-table {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
  max-height: 300px;
  overflow-y: auto;
}
`);

block(`
.family-row {
  display: grid;
  grid-template-columns: 1fr auto auto auto;
  gap: 1rem;
  padding: 1rem;
  background: var(--bg);
  border: 1px solid var(--border);
  border-radius: 6px;
  transition: all 0.2s ease;
}
`);

block(`
.family-row:hover {
  background: var(--hover-bg);
  border-color: var(--admin-accent);
}
`);

block(`
.analytics-family-name {
  font-weight: 600;
  color: var(--text);
}
`);

block(`
.family-stats {
  font-size: 0.875rem;
  color: var(--muted);
}
`);

block(`
.family-score {
  font-size: 0.875rem;
  color: var(--admin-accent);
  font-weight: 600;
}
`);

block(`
.family-active {
  font-size: 0.875rem;
  color: var(--muted);
}
`);

block(`
.family-content-list {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
  background: var(--bg);
  padding: 1rem;
  border-radius: 6px;
  max-height: 200px;
  overflow-y: auto;
}
`);

block(`
.family-content-item {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  padding: 0.75rem;
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 6px;
}
`);

block(`
.family-content-stats {
  display: flex;
  gap: 1rem;
  font-size: 0.875rem;
  color: var(--muted);
}
`);

block(`
.error-summary {
  background: var(--bg);
  padding: 1rem;
  border-radius: 6px;
}
`);

block(`
.error-stat {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 0.75rem;
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 6px;
  margin-bottom: 1rem;
}
`);

block(`
.error-label {
  font-size: 0.875rem;
  color: var(--text);
  font-weight: 500;
}
`);

block(`
.error-value {
  font-size: 1.25rem;
  font-weight: 600;
  color: var(--analytics-error);
}
`);

block(`
.recent-errors {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  background: var(--bg);
  padding: 1rem;
  border-radius: 6px;
  max-height: 200px;
  overflow-y: auto;
}
`);

block(`
.error-item {
  padding: 0.75rem;
  background: rgba(239, 68, 68, 0.1);
  border: 1px solid rgba(239, 68, 68, 0.2);
  border-radius: 4px;
  font-size: 0.875rem;
  font-family: monospace;
  color: var(--analytics-error);
}
`);

block(`
.no-errors {
  padding: 2rem;
  text-align: center;
  color: var(--muted);
  font-style: italic;
}
`);

block(`
@media (max-width: 1200px) {
  .charts-grid {
    grid-template-columns: 1fr;
  }
}
`);

block(`
@media (max-width: 768px) {
  .analytics-container {
    padding: 1rem;
  }

  .analytics-header {
    padding: 1.5rem;
  }

  .analytics-header h1 {
    font-size: 2rem;
  }

  .analytics-controls {
    flex-direction: column;
    gap: 1rem;
    align-items: stretch;
  }

  .view-selector {
    width: 100%;
    justify-content: space-between;
  }

  .metrics-grid {
    grid-template-columns: repeat(2, 1fr);
    gap: 1rem;
  }

  .metric-card {
    padding: 1rem;
  }

  .metric-value {
    font-size: 1.75rem;
  }

  .charts-grid {
    grid-template-columns: 1fr;
    gap: 1rem;
  }

  .chart-card {
    padding: 1rem;
  }

  .family-row {
    grid-template-columns: 1fr;
    gap: 0.5rem;
  }

  .retention-metrics {
    grid-template-columns: 1fr;
  }

  .simple-bar-chart {
    height: 150px;
  }

  .simple-line-chart {
    height: 150px;
  }
}
`);

block(`
@media (max-width: 480px) {
  .metrics-grid {
    grid-template-columns: 1fr;
  }

  .view-selector {
    flex-direction: column;
    gap: 0.25rem;
  }

  .view-btn {
    text-align: center;
  }
}
`);
