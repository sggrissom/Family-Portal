import * as preact from "preact";
import * as vlens from "vlens";
import {
  AgeRange,
  BandRow,
  ChartDomain,
  ChartPoint,
  METRIC_UNIT,
  Metric,
  ageTicks,
  chartDomain,
  niceTicks,
} from "../lib/ageChart";
import { copy } from "../lib/copy";
import "./age-chart-styles";

export interface AgeSeries {
  key: number;
  label: string;
  color: string;
  points: ChartPoint[];
  faint?: boolean;
}

export interface ChartZoom {
  range: AgeRange | null;
  domain: ChartDomain | null;
  pointerId: number | null;
  dragStart: number;
  dragEnd: number;
  dragging: boolean;
}

export function newChartZoom(): ChartZoom {
  return { range: null, domain: null, pointerId: null, dragStart: 0, dragEnd: 0, dragging: false };
}

interface AgeChartProps {
  series: AgeSeries[];
  metric: Metric;
  band?: BandRow[];
  label: string;
  zoom: ChartZoom;
}

const W = 640;
const H = 320;
const M = { top: 16, right: 16, bottom: 34, left: 44 };
const DRAG_THRESHOLD = 8;
const CLIP_ID = "age-chart-plot";

function svgX(event: PointerEvent): number {
  const rect = (event.currentTarget as SVGSVGElement).getBoundingClientRect();
  const px = ((event.clientX - rect.left) / rect.width) * W;
  return Math.max(M.left, Math.min(W - M.right, px));
}

function ageAt(domain: ChartDomain, px: number): number {
  return domain.minAge + ((px - M.left) / (W - M.left - M.right)) * (domain.maxAge - domain.minAge);
}

function onPointerDown(zoom: ChartZoom, event: PointerEvent) {
  if (!event.isPrimary || event.button !== 0) return;
  zoom.pointerId = event.pointerId;
  zoom.dragStart = zoom.dragEnd = svgX(event);
  zoom.dragging = false;
}

function onPointerMove(zoom: ChartZoom, event: PointerEvent) {
  if (event.pointerId !== zoom.pointerId) return;
  zoom.dragEnd = svgX(event);
  if (!zoom.dragging && Math.abs(zoom.dragEnd - zoom.dragStart) >= DRAG_THRESHOLD) {
    zoom.dragging = true;
    (event.currentTarget as SVGSVGElement).setPointerCapture(event.pointerId);
  }
  if (zoom.dragging) {
    event.preventDefault();
    vlens.scheduleRedraw();
  }
}

function onPointerUp(zoom: ChartZoom, event: PointerEvent) {
  if (event.pointerId !== zoom.pointerId) return;
  if (zoom.dragging && zoom.domain) {
    zoom.range = {
      from: ageAt(zoom.domain, Math.min(zoom.dragStart, zoom.dragEnd)),
      to: ageAt(zoom.domain, Math.max(zoom.dragStart, zoom.dragEnd)),
    };
  }
  endDrag(zoom);
}

function endDrag(zoom: ChartZoom) {
  zoom.pointerId = null;
  if (zoom.dragging) {
    zoom.dragging = false;
    vlens.scheduleRedraw();
  }
}

function preventDrag(event: DragEvent) {
  event.preventDefault();
}

function resetZoom(zoom: ChartZoom) {
  zoom.range = null;
  vlens.scheduleRedraw();
}

export const AgeChart = ({ series, metric, band = [], label, zoom }: AgeChartProps) => {
  const domain = chartDomain(
    series.map(s => s.points),
    band,
    zoom.range
  );
  zoom.domain = domain;
  if (!domain) return null;
  const { minAge, maxAge, minValue, maxValue, zoomed } = domain;

  const x = (age: number) => M.left + ((age - minAge) / (maxAge - minAge)) * (W - M.left - M.right);
  const y = (value: number) =>
    H - M.bottom - ((value - minValue) / (maxValue - minValue)) * (H - M.top - M.bottom);

  const area = (lo: keyof BandRow, hi: keyof BandRow) =>
    [
      ...domain.band.map(r => `${x(r.ageMonths)},${y(r[hi])}`),
      ...[...domain.band].reverse().map(r => `${x(r.ageMonths)},${y(r[lo])}`),
    ].join(" ");

  const ordered = [...series].sort((a, b) => Number(!a.faint) - Number(!b.faint));

  return (
    <figure className="age-chart">
      <div className="age-chart-zoom">
        {zoomed ? (
          <button
            type="button"
            className="age-chart-reset"
            onClick={vlens.cachePartial(resetZoom, zoom)}
          >
            {copy.ageChart.resetZoom}
          </button>
        ) : (
          <span>{copy.ageChart.zoomHint}</span>
        )}
      </div>
      <svg
        viewBox={`0 0 ${W} ${H}`}
        role="img"
        aria-label={label}
        className={zoom.dragging ? "dragging" : ""}
        onPointerDown={vlens.cachePartial(onPointerDown, zoom)}
        onPointerMove={vlens.cachePartial(onPointerMove, zoom)}
        onPointerUp={vlens.cachePartial(onPointerUp, zoom)}
        onPointerCancel={vlens.cachePartial(endDrag, zoom)}
        onDragStart={preventDrag}
      >
        <defs>
          <clipPath id={CLIP_ID}>
            <rect x={M.left} y={M.top} width={W - M.left - M.right} height={H - M.top - M.bottom} />
          </clipPath>
        </defs>
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

        <g clip-path={`url(#${CLIP_ID})`}>
          {domain.band.length > 1 && (
            <g className="age-chart-band">
              <polygon className="band-outer" points={area("p3", "p97")} />
              <polygon className="band-inner" points={area("p15", "p85")} />
              <polyline
                className="band-median"
                points={domain.band.map(r => `${x(r.ageMonths)},${y(r.p50)}`).join(" ")}
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
        </g>

        {zoom.dragging && (
          <rect
            className="age-chart-brush"
            x={Math.min(zoom.dragStart, zoom.dragEnd)}
            y={M.top}
            width={Math.abs(zoom.dragEnd - zoom.dragStart)}
            height={H - M.top - M.bottom}
          />
        )}
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
