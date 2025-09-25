import * as preact from "preact";
import { JSX } from "preact";
import * as vlens from "vlens";
import * as server from "../../server";
import "./chart.styles";

export interface GrowthChartProps {
  growthData: server.GrowthData[];
  width?: number;
  height?: number;
}

type Kind = "Height" | "Weight";

interface SelectedDataPoint {
  key: { id: number; kind: Kind } | null;
  value: number;
  unit: string;
  type: Kind | "";
  date: string;
}

const formatDate = (s: string) => new Date(s).toLocaleDateString();

const useSelectedPoint = vlens.declareHook(
  (): SelectedDataPoint => ({
    key: null,
    value: 0,
    unit: "",
    type: "",
    date: "",
  })
);

const useHoveredPoint = vlens.declareHook(
  (): { key: { id: number; kind: Kind } | null } => ({ key: null })
);

interface ZoomState {
  scale: number;           // content units (centered scaling)
  translateX: number;      // content units
  translateY: number;      // content units
  isDragging: boolean;
}

interface TouchState {
  touches: Touch[];
  initialDistance: number;
  initialScale: number;
  initialTranslate: { x: number; y: number };
  focalPoint: { x: number; y: number };           // in inner-plot SVG coords
  initialFocalPoint: { x: number; y: number };
  touchStartTime: number;
  touchStartPosition: { x: number; y: number };
  hasMoved: boolean;
}

const useZoomState = vlens.declareHook(
  (): ZoomState => ({
    scale: 1,
    translateX: 0,
    translateY: 0,
    isDragging: false,
  })
);

const useTouchState = vlens.declareHook(
  (): TouchState => ({
    touches: [],
    initialDistance: 0,
    initialScale: 1,
    initialTranslate: { x: 0, y: 0 },
    focalPoint: { x: 0, y: 0 },
    initialFocalPoint: { x: 0, y: 0 },
    touchStartTime: 0,
    touchStartPosition: { x: 0, y: 0 },
    hasMoved: false,
  })
);

// -------------------- Helpers --------------------

const clamp = (v: number, lo: number, hi: number) => Math.max(lo, Math.min(hi, v));

/** Generate ~5 nice ticks between [min, max] (inclusive-ish). */
function niceTicks(min: number, max: number, targetCount = 5): number[] {
  if (!isFinite(min) || !isFinite(max)) return [];
  if (min === max) return [min];

  const span = max - min;
  const step0 = span / Math.max(1, targetCount);
  const mag = Math.pow(10, Math.floor(Math.log10(step0)));
  const norm = step0 / mag;
  const step =
    norm >= 7.5 ? 10 * mag :
    norm >= 3.5 ? 5 * mag :
    norm >= 1.5 ? 2 * mag : 1 * mag;

  const start = Math.ceil(min / step) * step;
  const ticks: number[] = [];
  for (let v = start; v <= max + 1e-9; v += step) ticks.push(Math.round(v * 1e6) / 1e6);
  return ticks;
}

export const GrowthChart = ({ growthData, width = 600, height = 400 }: GrowthChartProps) => {
  const selected = useSelectedPoint();
  const hovered = useHoveredPoint();
  const zoom = useZoomState();
  const touch = useTouchState();

  // ---- Layout ----
  const chartWidth = width;
  const chartHeight = height;
  const margin = { top: 20, right: 80, bottom: 60, left: 60 };
  const innerW = chartWidth - margin.left - margin.right;
  const innerH = chartHeight - margin.top - margin.bottom;

  if (!growthData || growthData.length === 0) {
    return (
      <div className="chart-placeholder">
        <p>📈 No data to display</p>
      </div>
    );
  }

  // ---- Data prep (once per props change) ----
  const sortedData = growthData
    .slice()
    .sort((a, b) => new Date(a.measurementDate).getTime() - new Date(b.measurementDate).getTime());

  const heightData = sortedData.filter((d) => d.measurementType === server.Height);
  const weightData = sortedData.filter((d) => d.measurementType === server.Weight);

  // Dates
  const dateTimes = sortedData.map((d) => new Date(d.measurementDate).getTime());
  const minTs = Math.min(...dateTimes);
  const maxTs = Math.max(...dateTimes);
  const rawDR = Math.max(1, maxTs - minTs); // avoid 0
  const MIN_DATE_PAD_MS = 24 * 60 * 60 * 1000; // 1 day minimum pad
  const paddedMinTs = minTs - Math.max(rawDR * 0.05, MIN_DATE_PAD_MS);
  const paddedMaxTs = maxTs + Math.max(rawDR * 0.05, MIN_DATE_PAD_MS);
  const dateDen = Math.max(1, paddedMaxTs - paddedMinTs);

  // Values
  const hv = heightData.map((d) => d.value);
  const wv = weightData.map((d) => d.value);
  const hMin = hv.length ? Math.min(...hv) : 0;
  const hMax = hv.length ? Math.max(...hv) : 100;
  const wMin = wv.length ? Math.min(...wv) : 0;
  const wMax = wv.length ? Math.max(...wv) : 50;

  const hSpan = Math.max(1, hMax - hMin);
  const wSpan = Math.max(1, wMax - wMin);
  const hPad = Math.max(hSpan * 0.1, 1);
  const wPad = Math.max(wSpan * 0.1, 1);
  const hLo = hMin - hPad, hHi = hMax + hPad;
  const wLo = wMin - wPad, wHi = wMax + wPad;
  const hDen = Math.max(1e-9, hHi - hLo);
  const wDen = Math.max(1e-9, wHi - wLo);

  // Scales (inner plot coords)
  const dateToX = (t: number) => ((t - paddedMinTs) / dateDen) * innerW;
  const heightToY = (v: number) => innerH - ((v - hLo) / hDen) * innerH;
  const weightToY = (v: number) => innerH - ((v - wLo) / wDen) * innerH;

  // Paths
  const createPath = (data: server.GrowthData[], yScale: (value: number) => number) => {
    if (data.length === 0) return "";
    let s = "";
    for (let i = 0; i < data.length; i++) {
      const d = data[i];
      const x = dateToX(new Date(d.measurementDate).getTime());
      const y = yScale(d.value);
      s += (i === 0 ? "M" : " L") + ` ${x} ${y}`;
    }
    return s;
  };
  const heightPath = createPath(heightData, heightToY);
  const weightPath = createPath(weightData, weightToY);

  // ---- Zoom / Pan helpers ----

  const constrainZoom = (scale: number) => clamp(scale, 1, 8);

  // Note: translateX/Y are in content (inner plot) units, with centered scaling:
  // translate(center) -> scale -> translate(-center) -> translate(pan)
  const constrainTranslate = (tx: number, ty: number, scale: number) => {
    const maxX = (innerW * (scale - 1)) / 2;
    const maxY = (innerH * (scale - 1)) / 2;
    return {
      x: clamp(tx, -maxX, maxX),
      y: clamp(ty, -maxY, maxY),
    };
  };

  const screenToInnerSVG = (clientX: number, clientY: number, svgEl: SVGSVGElement) => {
    const rect = svgEl.getBoundingClientRect();
    // Map to viewBox space (0..chartWidth/Height)
    const sx = ((clientX - rect.left) / rect.width) * chartWidth;
    const sy = ((clientY - rect.top) / rect.height) * chartHeight;
    // Then to inner plot (account for margins)
    return { x: clamp(sx - margin.left, 0, innerW), y: clamp(sy - margin.top, 0, innerH) };
  };

  const getPointRadius = (isSelected: boolean) => {
    const base = isSelected ? 8 : 6;
    const scaleFactor = clamp(1 / Math.sqrt(zoom.scale), 0.5, 1);
    return Math.max(3, base * scaleFactor);
  };

  const getStrokeWidth = (isSelected: boolean) => {
    const base = isSelected ? 3 : 2;
    const scaleFactor = clamp(1 / Math.sqrt(zoom.scale), 0.5, 1);
    return Math.max(1, base * scaleFactor);
  };

  // ---- Touch handlers (mobile) ----

  const handleTouchStart = (e: JSX.TargetedTouchEvent<SVGSVGElement>) => {
    const touches = Array.from(e.touches);

    if (touches.length === 2) {
      e.preventDefault();
      touch.touches = touches;
      touch.initialDistance = distance(touches[0], touches[1]);
      touch.initialScale = zoom.scale;
      touch.initialTranslate = { x: zoom.translateX, y: zoom.translateY };

      const center = touchCenter(touches[0], touches[1]);
      const svgCenter = screenToInnerSVG(center.x, center.y, e.currentTarget);
      touch.focalPoint = svgCenter;
      touch.initialFocalPoint = svgCenter;
    } else if (touches.length === 1 && zoom.scale > 1) {
      touch.touches = touches;
      touch.initialTranslate = { x: zoom.translateX, y: zoom.translateY };
      touch.touchStartTime = Date.now();
      touch.touchStartPosition = { x: touches[0].clientX, y: touches[0].clientY };
      touch.hasMoved = false;
    } else if (touches.length === 1) {
      touch.touchStartTime = Date.now();
      touch.touchStartPosition = { x: touches[0].clientX, y: touches[0].clientY };
      touch.hasMoved = false;
    }
    vlens.scheduleRedraw();
  };

  const handleTouchMove = (e: JSX.TargetedTouchEvent<SVGSVGElement>) => {
    const touches = Array.from(e.touches);
    const DRAG_THRESHOLD = 8;

    if (touches.length === 2 && touch.touches.length === 2) {
      e.preventDefault();
      const currDist = distance(touches[0], touches[1]);
      const scaleChange = currDist / touch.initialDistance;
      const newScale = constrainZoom(touch.initialScale * scaleChange);

      // Centered scale focal anchoring
      const delta = newScale - touch.initialScale;
      const relFx = touch.initialFocalPoint.x - innerW / 2;
      const relFy = touch.initialFocalPoint.y - innerH / 2;

      zoom.scale = newScale;
      zoom.translateX = touch.initialTranslate.x - relFx * delta;
      zoom.translateY = touch.initialTranslate.y - relFy * delta;

      // Clamp pan
      const c = constrainTranslate(zoom.translateX, zoom.translateY, zoom.scale);
      zoom.translateX = c.x;
      zoom.translateY = c.y;

      vlens.scheduleRedraw();
    } else if (touches.length === 1 && touch.touches.length === 1 && zoom.scale > 1) {
      const dx = touches[0].clientX - touch.touchStartPosition.x;
      const dy = touches[0].clientY - touch.touchStartPosition.y;
      const dist = Math.hypot(dx, dy);

      if (!touch.hasMoved && dist > DRAG_THRESHOLD) {
        touch.hasMoved = true;
        zoom.isDragging = true;
        e.preventDefault();
      }

      if (zoom.isDragging) {
        e.preventDefault();
        const panDX = touches[0].clientX - touch.touches[0].clientX;
        const panDY = touches[0].clientY - touch.touches[0].clientY;

        const tx = touch.initialTranslate.x + panDX;
        const ty = touch.initialTranslate.y + panDY;
        const c = constrainTranslate(tx, ty, zoom.scale);
        zoom.translateX = c.x;
        zoom.translateY = c.y;

        vlens.scheduleRedraw();
      }
    }
    // taps fall through (no preventDefault) so clicks still work
  };

  const handleTouchEnd = (e: JSX.TargetedTouchEvent<SVGSVGElement>) => {
    const wasDragging = zoom.isDragging;
    const hadMulti = touch.touches.length > 1;
    const dur = Date.now() - touch.touchStartTime;
    const wasTap = !touch.hasMoved && dur < 300;

    if ((wasDragging || hadMulti) && !wasTap) e.preventDefault();

    zoom.isDragging = false;
    touch.touches = [];
    touch.initialDistance = 0;
    touch.hasMoved = false;
    touch.touchStartTime = 0;
    touch.touchStartPosition = { x: 0, y: 0 };

    vlens.scheduleRedraw();
  };

  const handleTouchCancel = (e: JSX.TargetedTouchEvent<SVGSVGElement>) => {
    zoom.isDragging = false;
    touch.touches = [];
    touch.initialDistance = 0;
    touch.hasMoved = false;
    touch.touchStartTime = 0;
    touch.touchStartPosition = { x: 0, y: 0 };
    vlens.scheduleRedraw();
  };

  // ---- Wheel zoom (desktop) ----
  const handleWheel = (e: JSX.TargetedWheelEvent<SVGSVGElement>) => {
    if (!e.ctrlKey && !e.metaKey) {
      // Treat regular wheel as page scroll unless already zoomed; if zoomed, pan vertically a bit
      if (zoom.scale > 1) {
        e.preventDefault();
        const ty = zoom.translateY - e.deltaY; // natural pan
        const c = constrainTranslate(zoom.translateX, ty, zoom.scale);
        zoom.translateX = c.x;
        zoom.translateY = c.y;
        vlens.scheduleRedraw();
      }
      return;
    }

    // Ctrl/Cmd + wheel = zoom (common UX on web maps)
    e.preventDefault();
    const svg = e.currentTarget;
    const innerPt = screenToInnerSVG(e.clientX, e.clientY, svg);

    const zoomFactor = Math.exp(-e.deltaY * 0.0015); // smooth
    const newScale = constrainZoom(zoom.scale * zoomFactor);
    const delta = newScale - zoom.scale;

    const relFx = innerPt.x - innerW / 2;
    const relFy = innerPt.y - innerH / 2;

    zoom.scale = newScale;
    zoom.translateX = zoom.translateX - relFx * delta;
    zoom.translateY = zoom.translateY - relFy * delta;

    const c = constrainTranslate(zoom.translateX, zoom.translateY, zoom.scale);
    zoom.translateX = c.x;
    zoom.translateY = c.y;

    vlens.scheduleRedraw();
  };

  const resetZoom = () => {
    zoom.scale = 1;
    zoom.translateX = 0;
    zoom.translateY = 0;
    zoom.isDragging = false;
    vlens.scheduleRedraw();
  };

  // ---- Interactions on points ----

  const selectPoint = (d: server.GrowthData, kind: Kind) => {
    const key = { id: d.id as number, kind };
    if (selected.key && selected.key.id === key.id && selected.key.kind === key.kind) {
      selected.key = null;
      selected.value = 0;
      selected.unit = "";
      selected.type = "";
      selected.date = "";
    } else {
      selected.key = key;
      selected.value = d.value;
      selected.unit = d.unit;
      selected.type = kind;
      selected.date = formatDate(d.measurementDate);
    }
    vlens.scheduleRedraw();
  };

  const hoverPoint = (d: server.GrowthData, kind: Kind) => {
    hovered.key = { id: d.id as number, kind };
    vlens.scheduleRedraw();
  };

  const clearHover = () => {
    hovered.key = null;
    vlens.scheduleRedraw();
  };

  // ---- Colors via CSS vars with sensible fallbacks ----
  const heightColor = "var(--height-color, #3b82f6)";
  const weightColor = "var(--weight-color, #ef4444)";

  // ---- Ticks ----
  const xRatios = [0, 0.25, 0.5, 0.75, 1];
  const hTicks = heightData.length ? niceTicks(hLo, hHi, 5).filter((v) => v >= hMin && v <= hMax) : [];
  const wTicks = weightData.length ? niceTicks(wLo, wHi, 5).filter((v) => v >= wMin && v <= wMax) : [];

  return (
    <div className="growth-chart">
      {zoom.scale > 1 && (
        <div className="zoom-controls">
          <button className="btn-zoom-reset" onClick={resetZoom} aria-label="Reset zoom">
            Reset Zoom (1×)
          </button>
          <span className="zoom-level">{zoom.scale.toFixed(1)}×</span>
        </div>
      )}

      <svg
        className="growth-chart-svg"
        viewBox={`0 0 ${chartWidth} ${chartHeight}`}
        preserveAspectRatio="xMidYMid meet"
        onTouchStart={handleTouchStart}
        onTouchMove={handleTouchMove}
        onTouchEnd={handleTouchEnd}
        onTouchCancel={handleTouchCancel}
        onWheel={handleWheel}
        // Disable default pan/zoom gestures inside the SVG; we handle them.
        style={{ touchAction: "none" }}
        role="img"
        aria-label="Growth chart of height and weight over time"
      >
        <defs>
          <clipPath id="plotArea">
            <rect x="0" y="0" width={innerW} height={innerH} />
          </clipPath>

          <linearGradient id="heightGradient" x1="0%" y1="0%" x2="0%" y2="100%">
            <stop offset="0%" stopColor={heightColor} stopOpacity="0.3" />
            <stop offset="100%" stopColor={heightColor} stopOpacity="0.1" />
          </linearGradient>
          <linearGradient id="weightGradient" x1="0%" y1="0%" x2="0%" y2="100%">
            <stop offset="0%" stopColor={weightColor} stopOpacity="0.3" />
            <stop offset="100%" stopColor={weightColor} stopOpacity="0.1" />
          </linearGradient>
        </defs>

        {/* Outer margin group */}
        <g transform={`translate(${margin.left}, ${margin.top})`} clipPath="url(#plotArea)">
          {/* Centered scale then pan in content units */}
          <g
            transform={`
              translate(${innerW / 2}, ${innerH / 2})
              scale(${zoom.scale})
              translate(${-innerW / 2}, ${-innerH / 2})
              translate(${zoom.translateX}, ${zoom.translateY})
            `}
          >
            {/* Grid */}
            <g className="grid">
              {/* Vertical grid lines */}
              {xRatios.map((r) => (
                <line
                  key={`vgrid-${r}`}
                  x1={r * innerW}
                  y1={0}
                  x2={r * innerW}
                  y2={innerH}
                  className="grid-line"
                />
              ))}
              {/* Horizontal grid lines (use 5 steps) */}
              {[0, 0.25, 0.5, 0.75, 1].map((r) => (
                <line
                  key={`hgrid-${r}`}
                  x1={0}
                  y1={r * innerH}
                  x2={innerW}
                  y2={r * innerH}
                  className="grid-line"
                />
              ))}
            </g>

            {/* Height line */}
            {heightData.length > 0 && (
              <g className="height-line">
                <path d={heightPath} fill="none" stroke={heightColor} strokeWidth={3} className="chart-line" />
              </g>
            )}

            {/* Weight line */}
            {weightData.length > 0 && (
              <g className="weight-line">
                <path d={weightPath} fill="none" stroke={weightColor} strokeWidth={3} className="chart-line" />
              </g>
            )}

            {/* Data points: Height */}
            {heightData.map((d) => {
              const key = { id: d.id as number, kind: "Height" as const };
              const isSelected = !!selected.key && selected.key.id === key.id && selected.key.kind === key.kind;
              const isHovered = !!hovered.key && hovered.key.id === key.id && hovered.key.kind === key.kind;
              return (
                <circle
                  key={`h-${d.id}`}
                  cx={dateToX(new Date(d.measurementDate).getTime())}
                  cy={heightToY(d.value)}
                  r={getPointRadius(isSelected)}
                  fill={heightColor}
                  stroke="white"
                  strokeWidth={getStrokeWidth(isSelected)}
                  className={`data-point height-point ${isSelected ? "selected" : ""} ${isHovered ? "hovered" : ""}`}
                  onClick={() => selectPoint(d, "Height")}
                  onMouseEnter={() => hoverPoint(d, "Height")}
                  onMouseLeave={clearHover}
                  style={{ cursor: "pointer" }}
                  tabIndex={0}
                  role="button"
                  aria-label={`Height measurement: ${d.value} ${d.unit} on ${formatDate(d.measurementDate)}`}
                  onKeyDown={(e) => {
                    if (e.key === "Enter" || e.key === " ") {
                      e.preventDefault();
                      selectPoint(d, "Height");
                    }
                  }}
                  onFocus={() => hoverPoint(d, "Height")}
                  onBlur={clearHover}
                />
              );
            })}

            {/* Data points: Weight */}
            {weightData.map((d) => {
              const key = { id: d.id as number, kind: "Weight" as const };
              const isSelected = !!selected.key && selected.key.id === key.id && selected.key.kind === key.kind;
              const isHovered = !!hovered.key && hovered.key.id === key.id && hovered.key.kind === key.kind;
              return (
                <circle
                  key={`w-${d.id}`}
                  cx={dateToX(new Date(d.measurementDate).getTime())}
                  cy={weightToY(d.value)}
                  r={getPointRadius(isSelected)}
                  fill={weightColor}
                  stroke="white"
                  strokeWidth={getStrokeWidth(isSelected)}
                  className={`data-point weight-point ${isSelected ? "selected" : ""} ${isHovered ? "hovered" : ""}`}
                  onClick={() => selectPoint(d, "Weight")}
                  onMouseEnter={() => hoverPoint(d, "Weight")}
                  onMouseLeave={clearHover}
                  style={{ cursor: "pointer" }}
                  tabIndex={0}
                  role="button"
                  aria-label={`Weight measurement: ${d.value} ${d.unit} on ${formatDate(d.measurementDate)}`}
                  onKeyDown={(e) => {
                    if (e.key === "Enter" || e.key === " ") {
                      e.preventDefault();
                      selectPoint(d, "Weight");
                    }
                  }}
                  onFocus={() => hoverPoint(d, "Weight")}
                  onBlur={clearHover}
                />
              );
            })}

            {/* Axes */}
            <g className="axes">
              {/* X-axis */}
              <line x1={0} y1={innerH} x2={innerW} y2={innerH} className="axis-line" />
              {/* Y-axis */}
              <line x1={0} y1={0} x2={0} y2={innerH} className="axis-line" />

              {/* X-axis labels (dates at fixed ratios) */}
              {xRatios.map((ratio, i) => {
                const t = paddedMinTs + ratio * (paddedMaxTs - paddedMinTs);
                const date = new Date(t);
                return (
                  <text
                    key={`xlab-${i}`}
                    x={ratio * innerW}
                    y={innerH + 20}
                    textAnchor="middle"
                    className="axis-label"
                    fontSize="10"
                  >
                    {date.toLocaleDateString(undefined, { month: "short", year: "2-digit" })}
                  </text>
                );
              })}

              {/* Y-axis labels for height (left, blue) */}
              {heightData.length > 0 &&
                hTicks.map((v, i) => (
                  <text
                    key={`hlab-${i}`}
                    x={-10}
                    y={heightToY(v)}
                    textAnchor="end"
                    className="axis-label height-axis-label"
                    fontSize="10"
                    dy="0.35em"
                  >
                    {Math.round(v)}
                  </text>
                ))}

              {/* Y-axis labels for weight (right, red) */}
              {weightData.length > 0 &&
                wTicks.map((v, i) => (
                  <text
                    key={`wlab-${i}`}
                    x={innerW + 10}
                    y={weightToY(v)}
                    textAnchor="start"
                    className="axis-label weight-axis-label"
                    fontSize="10"
                    dy="0.35em"
                  >
                    {Math.round(v)}
                  </text>
                ))}
            </g>
          </g>
        </g>

        {/* Legend (outside clip & zoom) */}
        <g className="legend" transform={`translate(${chartWidth - margin.right + 10}, ${margin.top + 20})`}>
          {heightData.length > 0 && (
            <g className="legend-item">
              <circle cx="0" cy="0" r="5" fill={heightColor} stroke="white" strokeWidth={2} />
              <text x="15" y="0" className="legend-text" dy="0.35em">
                Height
              </text>
            </g>
          )}
          {weightData.length > 0 && (
            <g className="legend-item" transform="translate(0, 25)">
              <circle cx="0" cy="0" r="5" fill={weightColor} stroke="white" strokeWidth={2} />
              <text x="15" y="0" className="legend-text" dy="0.35em">
                Weight
              </text>
            </g>
          )}
        </g>
      </svg>

      {/* Data Point Info Panel */}
      {selected.key ? (
        <div className="data-point-info">
          <div className="info-header">
            <span className="info-type">{selected.type}</span>
            <span className="info-date">{selected.date}</span>
          </div>
          <div className="info-value">
            {selected.value} {selected.unit}
          </div>
        </div>
      ) : (
        <div className="data-point-info placeholder">
          <div className="info-hint">Tap or click a data point to see details</div>
        </div>
      )}
    </div>
  );
};

// -------------------- local fns --------------------

function distance(a: Touch, b: Touch) {
  const dx = a.clientX - b.clientX;
  const dy = a.clientY - b.clientY;
  return Math.hypot(dx, dy);
}

function touchCenter(a: Touch, b: Touch) {
  return { x: (a.clientX + b.clientX) / 2, y: (a.clientY + b.clientY) / 2 };
}