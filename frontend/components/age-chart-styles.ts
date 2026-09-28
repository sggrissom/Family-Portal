import { block } from "vlens/css";

block(`
.age-chart {
  margin: 0;
}
`);

block(`
.age-chart svg {
  display: block;
  width: 100%;
  height: auto;
}
`);

block(`
.age-chart-grid {
  stroke: var(--border);
  stroke-width: 1;
}
`);

block(`
.age-chart-tick,
.age-chart-unit {
  fill: var(--muted);
  font-size: 12px;
}
`);

block(`
.age-chart-band .band-outer {
  fill: var(--accent);
  opacity: 0.07;
}
`);

block(`
.age-chart-band .band-inner {
  fill: var(--accent);
  opacity: 0.1;
}
`);

block(`
.age-chart-band .band-median {
  fill: none;
  stroke: var(--muted);
  stroke-dasharray: 4 4;
  stroke-width: 1;
}
`);

block(`
.age-chart-series polyline {
  fill: none;
  stroke-width: 2.5;
  stroke-linejoin: round;
}
`);

block(`
.age-chart-series.faint {
  opacity: 0.35;
}
`);

block(`
.age-chart-series.faint polyline {
  stroke-width: 1.5;
}
`);

block(`
.age-chart-hit {
  fill: transparent;
}
`);

block(`
.age-chart-legend {
  display: flex;
  flex-wrap: wrap;
  gap: 6px 14px;
  margin-top: 6px;
  color: var(--muted);
  font-size: 0.85rem;
}
`);

block(`
.age-chart-legend span {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}
`);

block(`
.age-chart-legend span.faint {
  opacity: 0.6;
}
`);

block(`
.age-chart-legend i {
  width: 10px;
  height: 10px;
  border-radius: 50%;
}
`);
