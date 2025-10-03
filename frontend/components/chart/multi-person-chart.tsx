import * as preact from "preact";
import { JSX } from "preact";
import * as vlens from "vlens";
import * as server from "../../server";
import "./chart.styles";

export interface PersonGrowthData {
  person: server.Person;
  growthData: server.GrowthData[];
  color: string;
}

export interface MultiPersonChartProps {
  peopleData: PersonGrowthData[];
  width?: number;
  height?: number;
  showHeight?: boolean;
  showWeight?: boolean;
}

type Kind = "Height" | "Weight";

interface SelectedDataPoint {
  key: { id: number; kind: Kind; personId: number } | null;
  value: number;
  unit: string;
  type: Kind | "";
  date: string;
  personName: string;
  color: string;
}

const formatDate = (s: string) => new Date(s).toLocaleDateString();

const useSelectedPoint = vlens.declareHook(
  (): SelectedDataPoint => ({
    key: null,
    value: 0,
    unit: "",
    type: "",
    date: "",
    personName: "",
    color: "",
  })
);

const useHoveredPoint = vlens.declareHook(
  (): { key: { id: number; kind: Kind; personId: number } | null } => ({
    key: null,
  })
);

interface ZoomState {
  scale: number;
  translateX: number;
  translateY: number;
  isDragging: boolean;
}

interface TouchState {
  touches: Touch[];
  initialDistance: number;
  initialScale: number;
  initialTranslate: { x: number; y: number };
  focalPoint: { x: number; y: number };
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

const clamp = (v: number, lo: number, hi: number) => Math.max(lo, Math.min(hi, v));

function niceTicks(min: number, max: number, targetCount = 5): number[] {
  if (!isFinite(min) || !isFinite(max)) return [];
  if (min === max) return [min];

  const span = max - min;
  const step0 = span / Math.max(1, targetCount);
  const mag = Math.pow(10, Math.floor(Math.log10(step0)));
  const norm = step0 / mag;
  const step = norm >= 7.5 ? 10 * mag : norm >= 3.5 ? 5 * mag : norm >= 1.5 ? 2 * mag : 1 * mag;

  const start = Math.ceil(min / step) * step;
  const ticks: number[] = [];
  for (let v = start; v <= max + 1e-9; v += step) ticks.push(Math.round(v * 1e6) / 1e6);
  return ticks;
}

export const MultiPersonChart = ({
  peopleData,
  width = 600,
  height = 400,
  showHeight = true,
  showWeight = true,
}: MultiPersonChartProps) => {
  const selected = useSelectedPoint();
  const hovered = useHoveredPoint();
  const zoom = useZoomState();
  const touch = useTouchState();

  const chartWidth = width;
  const chartHeight = height;
  const margin = { top: 20, right: 100, bottom: 60, left: 60 };
  const innerW = chartWidth - margin.left - margin.right;
  const innerH = chartHeight - margin.top - margin.bottom;

  if (!peopleData || peopleData.length === 0) {
    return (
      <div className="chart-placeholder">
        <p>📈 Select people to compare their growth</p>
      </div>
    );
  }

  // Collect all growth data across all people
  const allGrowthData = peopleData.flatMap(pd => pd.growthData);

  if (allGrowthData.length === 0) {
    return (
      <div className="chart-placeholder">
        <p>📈 No growth data available for selected people</p>
      </div>
    );
  }

  const sortedData = allGrowthData
    .slice()
    .sort((a, b) => new Date(a.measurementDate).getTime() - new Date(b.measurementDate).getTime());

  const heightData = sortedData.filter(d => d.measurementType === server.Height && showHeight);
  const weightData = sortedData.filter(d => d.measurementType === server.Weight && showWeight);

  // Dates
  const dateTimes = sortedData.map(d => new Date(d.measurementDate).getTime());
  const minTs = Math.min(...dateTimes);
  const maxTs = Math.max(...dateTimes);
  const rawDR = Math.max(1, maxTs - minTs);
  const MIN_DATE_PAD_MS = 24 * 60 * 60 * 1000;
  const paddedMinTs = minTs - Math.max(rawDR * 0.05, MIN_DATE_PAD_MS);
  const paddedMaxTs = maxTs + Math.max(rawDR * 0.05, MIN_DATE_PAD_MS);
  const dateDen = Math.max(1, paddedMaxTs - paddedMinTs);

  // Values
  const hv = heightData.map(d => d.value);
  const wv = weightData.map(d => d.value);
  const hMin = hv.length ? Math.min(...hv) : 0;
  const hMax = hv.length ? Math.max(...hv) : 100;
  const wMin = wv.length ? Math.min(...wv) : 0;
  const wMax = wv.length ? Math.max(...wv) : 50;

  const hSpan = Math.max(1, hMax - hMin);
  const wSpan = Math.max(1, wMax - wMin);
  const hPad = Math.max(hSpan * 0.1, 1);
  const wPad = Math.max(wSpan * 0.1, 1);
  const hLo = hMin - hPad,
    hHi = hMax + hPad;
  const wLo = wMin - wPad,
    wHi = wMax + wPad;
  const hDen = Math.max(1e-9, hHi - hLo);
  const wDen = Math.max(1e-9, wHi - wLo);

  // Scales
  const dateToX = (t: number) => ((t - paddedMinTs) / dateDen) * innerW;
  const heightToY = (v: number) => innerH - ((v - hLo) / hDen) * innerH;
  const weightToY = (v: number) => innerH - ((v - wLo) / wDen) * innerH;

  // Create paths for each person
  const createPath = (
    data: server.GrowthData[],
    yScale: (value: number) => number,
    personId: number
  ) => {
    const personData = data.filter(d => d.personId === personId);
    if (personData.length === 0) return "";
    const sorted = personData
      .slice()
      .sort(
        (a, b) => new Date(a.measurementDate).getTime() - new Date(b.measurementDate).getTime()
      );
    let s = "";
    for (let i = 0; i < sorted.length; i++) {
      const d = sorted[i];
      const x = dateToX(new Date(d.measurementDate).getTime());
      const y = yScale(d.value);
      s += (i === 0 ? "M" : " L") + ` ${x} ${y}`;
    }
    return s;
  };

  // Zoom helpers
  const constrainZoom = (scale: number) => clamp(scale, 1, 8);

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
    const sx = ((clientX - rect.left) / rect.width) * chartWidth;
    const sy = ((clientY - rect.top) / rect.height) * chartHeight;
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

  // Touch handlers
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

      const delta = newScale - touch.initialScale;
      const relFx = touch.initialFocalPoint.x - innerW / 2;
      const relFy = touch.initialFocalPoint.y - innerH / 2;

      zoom.scale = newScale;
      zoom.translateX = touch.initialTranslate.x - relFx * delta;
      zoom.translateY = touch.initialTranslate.y - relFy * delta;

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

  const handleWheel = (e: JSX.TargetedWheelEvent<SVGSVGElement>) => {
    if (!e.ctrlKey && !e.metaKey) {
      if (zoom.scale > 1) {
        e.preventDefault();
        const ty = zoom.translateY - e.deltaY;
        const c = constrainTranslate(zoom.translateX, ty, zoom.scale);
        zoom.translateX = c.x;
        zoom.translateY = c.y;
        vlens.scheduleRedraw();
      }
      return;
    }

    e.preventDefault();
    const svg = e.currentTarget;
    const innerPt = screenToInnerSVG(e.clientX, e.clientY, svg);

    const zoomFactor = Math.exp(-e.deltaY * 0.0015);
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

  // Point interactions
  const selectPoint = (d: server.GrowthData, kind: Kind, personData: PersonGrowthData) => {
    const key = { id: d.id as number, kind, personId: d.personId };
    if (
      selected.key &&
      selected.key.id === key.id &&
      selected.key.kind === key.kind &&
      selected.key.personId === key.personId
    ) {
      selected.key = null;
      selected.value = 0;
      selected.unit = "";
      selected.type = "";
      selected.date = "";
      selected.personName = "";
      selected.color = "";
    } else {
      selected.key = key;
      selected.value = d.value;
      selected.unit = d.unit;
      selected.type = kind;
      selected.date = formatDate(d.measurementDate);
      selected.personName = personData.person.name;
      selected.color = personData.color;
    }
    vlens.scheduleRedraw();
  };

  const hoverPoint = (d: server.GrowthData, kind: Kind) => {
    hovered.key = { id: d.id as number, kind, personId: d.personId };
    vlens.scheduleRedraw();
  };

  const clearHover = () => {
    hovered.key = null;
    vlens.scheduleRedraw();
  };

  const xRatios = [0, 0.25, 0.5, 0.75, 1];
  const hTicks =
    heightData.length && showHeight
      ? niceTicks(hLo, hHi, 5).filter(v => v >= hMin && v <= hMax)
      : [];
  const wTicks =
    weightData.length && showWeight
      ? niceTicks(wLo, wHi, 5).filter(v => v >= wMin && v <= wMax)
      : [];

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
        role="img"
        aria-label="Multi-person growth chart comparing height and weight over time"
      >
        <defs>
          <clipPath id="plotArea">
            <rect x="0" y="0" width={innerW} height={innerH} />
          </clipPath>
        </defs>

        <g transform={`translate(${margin.left}, ${margin.top})`} clipPath="url(#plotArea)">
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
              {xRatios.map(r => (
                <line
                  key={`vgrid-${r}`}
                  x1={r * innerW}
                  y1={0}
                  x2={r * innerW}
                  y2={innerH}
                  className="grid-line"
                />
              ))}
              {[0, 0.25, 0.5, 0.75, 1].map(r => (
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

            {/* Lines for each person */}
            {peopleData.map(personData => {
              const personHeightData = heightData.filter(d => d.personId === personData.person.id);
              const personWeightData = weightData.filter(d => d.personId === personData.person.id);

              return (
                <g key={`person-${personData.person.id}`}>
                  {/* Height line */}
                  {personHeightData.length > 0 && showHeight && (
                    <path
                      d={createPath(heightData, heightToY, personData.person.id)}
                      fill="none"
                      stroke={personData.color}
                      strokeWidth={3}
                      className="chart-line"
                      opacity={0.7}
                    />
                  )}

                  {/* Weight line */}
                  {personWeightData.length > 0 && showWeight && (
                    <path
                      d={createPath(weightData, weightToY, personData.person.id)}
                      fill="none"
                      stroke={personData.color}
                      strokeWidth={3}
                      strokeDasharray="5,5"
                      className="chart-line"
                      opacity={0.7}
                    />
                  )}
                </g>
              );
            })}

            {/* Data points for all people */}
            {peopleData.map(personData => {
              const personHeightData = heightData.filter(d => d.personId === personData.person.id);
              const personWeightData = weightData.filter(d => d.personId === personData.person.id);

              return (
                <g key={`points-${personData.person.id}`}>
                  {/* Height points */}
                  {personHeightData.map(d => {
                    const key = {
                      id: d.id as number,
                      kind: "Height" as const,
                      personId: d.personId,
                    };
                    const isSelected =
                      !!selected.key &&
                      selected.key.id === key.id &&
                      selected.key.kind === key.kind &&
                      selected.key.personId === key.personId;
                    const isHovered =
                      !!hovered.key &&
                      hovered.key.id === key.id &&
                      hovered.key.kind === key.kind &&
                      hovered.key.personId === key.personId;
                    return (
                      <circle
                        key={`h-${d.id}`}
                        cx={dateToX(new Date(d.measurementDate).getTime())}
                        cy={heightToY(d.value)}
                        r={getPointRadius(isSelected)}
                        fill={personData.color}
                        stroke="white"
                        strokeWidth={getStrokeWidth(isSelected)}
                        className={`data-point height-point ${isSelected ? "selected" : ""} ${
                          isHovered ? "hovered" : ""
                        }`}
                        onClick={() => selectPoint(d, "Height", personData)}
                        onMouseEnter={() => hoverPoint(d, "Height")}
                        onMouseLeave={clearHover}
                        tabIndex={0}
                        role="button"
                        aria-label={`${personData.person.name} height: ${d.value} ${d.unit} on ${formatDate(
                          d.measurementDate
                        )}`}
                        onKeyDown={e => {
                          if (e.key === "Enter" || e.key === " ") {
                            e.preventDefault();
                            selectPoint(d, "Height", personData);
                          }
                        }}
                        onFocus={() => hoverPoint(d, "Height")}
                        onBlur={clearHover}
                      />
                    );
                  })}

                  {/* Weight points */}
                  {personWeightData.map(d => {
                    const key = {
                      id: d.id as number,
                      kind: "Weight" as const,
                      personId: d.personId,
                    };
                    const isSelected =
                      !!selected.key &&
                      selected.key.id === key.id &&
                      selected.key.kind === key.kind &&
                      selected.key.personId === key.personId;
                    const isHovered =
                      !!hovered.key &&
                      hovered.key.id === key.id &&
                      hovered.key.kind === key.kind &&
                      hovered.key.personId === key.personId;
                    return (
                      <circle
                        key={`w-${d.id}`}
                        cx={dateToX(new Date(d.measurementDate).getTime())}
                        cy={weightToY(d.value)}
                        r={getPointRadius(isSelected)}
                        fill={personData.color}
                        stroke="white"
                        strokeWidth={getStrokeWidth(isSelected)}
                        className={`data-point weight-point ${isSelected ? "selected" : ""} ${
                          isHovered ? "hovered" : ""
                        }`}
                        onClick={() => selectPoint(d, "Weight", personData)}
                        onMouseEnter={() => hoverPoint(d, "Weight")}
                        onMouseLeave={clearHover}
                        tabIndex={0}
                        role="button"
                        aria-label={`${personData.person.name} weight: ${d.value} ${d.unit} on ${formatDate(
                          d.measurementDate
                        )}`}
                        onKeyDown={e => {
                          if (e.key === "Enter" || e.key === " ") {
                            e.preventDefault();
                            selectPoint(d, "Weight", personData);
                          }
                        }}
                        onFocus={() => hoverPoint(d, "Weight")}
                        onBlur={clearHover}
                      />
                    );
                  })}
                </g>
              );
            })}

            {/* Axes */}
            <g className="axes">
              <line x1={0} y1={innerH} x2={innerW} y2={innerH} className="axis-line" />
              <line x1={0} y1={0} x2={0} y2={innerH} className="axis-line" />

              {/* X-axis labels */}
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

              {/* Y-axis labels for height */}
              {showHeight &&
                heightData.length > 0 &&
                hTicks.map((v, i) => (
                  <text
                    key={`hlab-${i}`}
                    x={-10}
                    y={heightToY(v)}
                    textAnchor="end"
                    className="axis-label"
                    fontSize="10"
                    dy="0.35em"
                    fill="var(--text)"
                  >
                    {Math.round(v)}
                  </text>
                ))}

              {/* Y-axis labels for weight */}
              {showWeight &&
                weightData.length > 0 &&
                wTicks.map((v, i) => (
                  <text
                    key={`wlab-${i}`}
                    x={innerW + 10}
                    y={weightToY(v)}
                    textAnchor="start"
                    className="axis-label"
                    fontSize="10"
                    dy="0.35em"
                    fill="var(--text)"
                  >
                    {Math.round(v)}
                  </text>
                ))}
            </g>
          </g>
        </g>

        {/* Legend */}
        <g
          className="legend"
          transform={`translate(${chartWidth - margin.right + 10}, ${margin.top + 20})`}
        >
          {peopleData.map((personData, idx) => (
            <g key={`legend-${personData.person.id}`} transform={`translate(0, ${idx * 25})`}>
              <circle cx="0" cy="0" r="5" fill={personData.color} stroke="white" strokeWidth={2} />
              <text x="15" y="0" className="legend-text" dy="0.35em" fontSize="11">
                {personData.person.name}
              </text>
            </g>
          ))}
          {showHeight && heightData.length > 0 && (
            <g transform={`translate(0, ${(peopleData.length + 0.5) * 25})`}>
              <line x1="-5" y1="0" x2="5" y2="0" stroke="var(--text)" strokeWidth="2" />
              <text x="15" y="0" className="legend-text" dy="0.35em" fontSize="10">
                Height (solid)
              </text>
            </g>
          )}
          {showWeight && weightData.length > 0 && (
            <g transform={`translate(0, ${(peopleData.length + 1.5) * 25})`}>
              <line
                x1="-5"
                y1="0"
                x2="5"
                y2="0"
                stroke="var(--text)"
                strokeWidth="2"
                strokeDasharray="3,3"
              />
              <text x="15" y="0" className="legend-text" dy="0.35em" fontSize="10">
                Weight (dashed)
              </text>
            </g>
          )}
        </g>
      </svg>

      {/* Data Point Info Panel */}
      {selected.key ? (
        <div className="data-point-info">
          <div className="info-header">
            <span className="info-type" style={{ color: selected.color }}>
              {selected.personName} - {selected.type}
            </span>
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

function distance(a: Touch, b: Touch) {
  const dx = a.clientX - b.clientX;
  const dy = a.clientY - b.clientY;
  return Math.hypot(dx, dy);
}

function touchCenter(a: Touch, b: Touch) {
  return { x: (a.clientX + b.clientX) / 2, y: (a.clientY + b.clientY) / 2 };
}
