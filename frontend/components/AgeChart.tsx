import * as preact from "preact";
import { BandRow, ChartPoint, METRIC_UNIT, Metric, ageTicks, niceTicks } from "../lib/ageChart";
import "./age-chart-styles";

export interface AgeSeries {
  key: number;
  label: string;
  color: string;
  points: ChartPoint[];
  faint?: boolean;
}

interface AgeChartProps {
  series: AgeSeries[];
  metric: Metric;
  band?: BandRow[];
  label: string;
}

const W = 640;
const H = 320;
const M = { top: 16, right: 16, bottom: 34, left: 44 };

export const AgeChart = ({ series, metric, band = [], label }: AgeChartProps) => {
  const points = series.flatMap(s => s.points);
  if (points.length === 0) return null;

  const ages = points.map(p => p.ageMonths);
  const minAge = Math.max(0, Math.min(...ages) - 1);
  const maxAge = Math.max(...ages, minAge + 6) + 1;
  const inRange = band.filter(r => r.ageMonths >= minAge && r.ageMonths <= maxAge);
  const values = [...points.map(p => p.value), ...inRange.flatMap(r => [r.p3, r.p97])];
  const pad = Math.max((Math.max(...values) - Math.min(...values)) * 0.06, 0.5);
  const minValue = Math.max(0, Math.min(...values) - pad);
  const maxValue = Math.max(...values) + pad;

  const x = (age: number) => M.left + ((age - minAge) / (maxAge - minAge)) * (W - M.left - M.right);
  const y = (value: number) =>
    H - M.bottom - ((value - minValue) / (maxValue - minValue)) * (H - M.top - M.bottom);

  const area = (lo: keyof BandRow, hi: keyof BandRow) =>
    [
      ...inRange.map(r => `${x(r.ageMonths)},${y(r[hi])}`),
      ...[...inRange].reverse().map(r => `${x(r.ageMonths)},${y(r[lo])}`),
    ].join(" ");

  const ordered = [...series].sort((a, b) => Number(!a.faint) - Number(!b.faint));

  return (
    <figure className="age-chart">
      <svg viewBox={`0 0 ${W} ${H}`} role="img" aria-label={label}>
        {niceTicks(minValue, maxValue, 5).map(v => (
          <g key={`y${v}`}>
            <line className="age-chart-grid" x1={M.left} x2={W - M.right} y1={y(v)} y2={y(v)} />
            <text className="age-chart-tick" x={M.left - 6} y={y(v) + 4} text-anchor="end">
              {v}
            </text>
          </g>
        ))}
        {ageTicks(minAge, maxAge).map(t => (
          <text
            key={`x${t.months}`}
            className="age-chart-tick"
            x={x(t.months)}
            y={H - M.bottom + 18}
            text-anchor="middle"
          >
            {t.label}
          </text>
        ))}
        <text className="age-chart-unit" x={M.left - 6} y={M.top - 4} text-anchor="end">
          {METRIC_UNIT[metric]}
        </text>

        {inRange.length > 1 && (
          <g className="age-chart-band">
            <polygon className="band-outer" points={area("p3", "p97")} />
            <polygon className="band-inner" points={area("p15", "p85")} />
            <polyline
              className="band-median"
              points={inRange.map(r => `${x(r.ageMonths)},${y(r.p50)}`).join(" ")}
            />
          </g>
        )}

        {ordered.map(s => (
          <g key={s.key} className={s.faint ? "age-chart-series faint" : "age-chart-series"}>
            <polyline
              points={s.points.map(p => `${x(p.ageMonths)},${y(p.value)}`).join(" ")}
              stroke={s.color}
            />
            {s.points.map(p =>
              s.faint ? (
                <circle key={p.id} cx={x(p.ageMonths)} cy={y(p.value)} r={3} fill={s.color} />
              ) : (
                <a
                  key={p.id}
                  href={`/view-growth/${p.id}`}
                  aria-label={`${p.value.toFixed(1)} ${METRIC_UNIT[metric]}`}
                >
                  <circle className="age-chart-hit" cx={x(p.ageMonths)} cy={y(p.value)} r={12} />
                  <circle cx={x(p.ageMonths)} cy={y(p.value)} r={5} fill={s.color} />
                </a>
              )
            )}
          </g>
        ))}
      </svg>
      {series.length > 1 && (
        <figcaption className="age-chart-legend">
          {series.map(s => (
            <span key={s.key} className={s.faint ? "faint" : ""}>
              <i style={{ background: s.color }} /> {s.label}
            </span>
          ))}
        </figcaption>
      )}
    </figure>
  );
};

export const SERIES_COLORS = [
  "#22a06b",
  "#3b82f6",
  "#e5484d",
  "#f59e0b",
  "#8b5cf6",
  "#06b6d4",
  "#ec4899",
  "#84cc16",
];
