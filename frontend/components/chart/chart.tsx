import * as preact from "preact";
import * as vlens from "vlens";
import * as server from "../../server";
import "./chart.styles";

export interface GrowthChartProps {
  growthData: server.GrowthData[];
  width?: number;
  height?: number;
}

interface SelectedDataPoint {
  id: number | null;
  value: number;
  unit: string;
  type: string;
  date: string;
}


const formatDate = (dateString: string) => {
  if (!dateString) return '';
  if (dateString.includes('T') && dateString.endsWith('Z')) {
    const dateParts = dateString.split('T')[0].split('-');
    const year = parseInt(dateParts[0]);
    const month = parseInt(dateParts[1]) - 1; // Month is 0-indexed
    const day = parseInt(dateParts[2]);
    return new Date(year, month, day).toLocaleDateString();
  }
  return new Date(dateString).toLocaleDateString();
};

const useSelectedPoint = vlens.declareHook((): SelectedDataPoint => ({
  id: null,
  value: 0,
  unit: '',
  type: '',
  date: ''
}));

const useHoveredPoint = vlens.declareHook((): { id: number | null } => ({ id: null }));

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

const useZoomState = vlens.declareHook((): ZoomState => ({
  scale: 1,
  translateX: 0,
  translateY: 0,
  isDragging: false
}));

const useTouchState = vlens.declareHook((): TouchState => ({
  touches: [],
  initialDistance: 0,
  initialScale: 1,
  initialTranslate: { x: 0, y: 0 },
  focalPoint: { x: 0, y: 0 },
  initialFocalPoint: { x: 0, y: 0 },
  touchStartTime: 0,
  touchStartPosition: { x: 0, y: 0 },
  hasMoved: false
}));

export const GrowthChart = ({ growthData, width = 600, height = 400 }: GrowthChartProps) => {
  const selectedPoint = useSelectedPoint();
  const hoveredPoint = useHoveredPoint();
  const zoomState = useZoomState();
  const touchState = useTouchState();

  // Utility functions for touch/zoom handling
  const getDistance = (touch1: Touch, touch2: Touch): number => {
    const dx = touch1.clientX - touch2.clientX;
    const dy = touch1.clientY - touch2.clientY;
    return Math.sqrt(dx * dx + dy * dy);
  };

  const getTouchCenter = (touch1: Touch, touch2: Touch) => ({
    x: (touch1.clientX + touch2.clientX) / 2,
    y: (touch1.clientY + touch2.clientY) / 2
  });

  const constrainZoom = (scale: number): number => {
    return Math.max(1, Math.min(8, scale));
  };

  const constrainTranslate = (translateX: number, translateY: number, scale: number) => {
    // With centered scaling, constraints are symmetric around content center
    // Calculate how far we can pan based on scaled content extending beyond viewport
    const maxX = (innerChartWidth * (scale - 1)) / 2;
    const maxY = (innerChartHeight * (scale - 1)) / 2;

    return {
      x: Math.max(-maxX, Math.min(maxX, translateX)),
      y: Math.max(-maxY, Math.min(maxY, translateY))
    };
  };

  const screenToSVG = (screenX: number, screenY: number, svgElement: SVGSVGElement) => {
    const rect = svgElement.getBoundingClientRect();
    const scaleX = chartWidth / rect.width;
    const scaleY = chartHeight / rect.height;
    return {
      x: (screenX - rect.left) * scaleX,
      y: (screenY - rect.top) * scaleY
    };
  };

  // Calculate dynamic point radius based on zoom level
  const getPointRadius = (isSelected: boolean) => {
    const baseRadius = isSelected ? 8 : 6;
    // Scale down points when zoomed in to reduce clustering
    const scaleFactor = Math.max(0.5, Math.min(1, 1 / Math.sqrt(zoomState.scale)));
    return Math.max(3, baseRadius * scaleFactor);
  };

  const getStrokeWidth = (isSelected: boolean) => {
    const baseStroke = isSelected ? 3 : 2;
    const scaleFactor = Math.max(0.5, Math.min(1, 1 / Math.sqrt(zoomState.scale)));
    return Math.max(1, baseStroke * scaleFactor);
  };

  // Touch event handlers
  const handleTouchStart = (e: TouchEvent) => {
    const touches = Array.from(e.touches);

    if (touches.length === 2) {
      // Pinch gesture starting - prevent default to stop browser zoom
      e.preventDefault();
      touchState.touches = touches;
      touchState.initialDistance = getDistance(touches[0], touches[1]);
      touchState.initialScale = zoomState.scale;
      touchState.initialTranslate = { x: zoomState.translateX, y: zoomState.translateY };

      // Calculate and store initial focal point
      const svgElement = e.currentTarget as SVGSVGElement;
      const screenCenter = getTouchCenter(touches[0], touches[1]);
      const svgCenter = screenToSVG(screenCenter.x, screenCenter.y, svgElement);
      touchState.focalPoint = svgCenter;
      touchState.initialFocalPoint = svgCenter;
    } else if (touches.length === 1 && zoomState.scale > 1) {
      // Single touch when zoomed - track for potential pan but don't prevent clicks yet
      touchState.touches = touches;
      touchState.initialTranslate = { x: zoomState.translateX, y: zoomState.translateY };
      touchState.touchStartTime = Date.now();
      touchState.touchStartPosition = { x: touches[0].clientX, y: touches[0].clientY };
      touchState.hasMoved = false;
      // Don't set isDragging = true yet, wait for movement
    } else if (touches.length === 1) {
      // Single touch on non-zoomed chart - still track for consistency
      touchState.touchStartTime = Date.now();
      touchState.touchStartPosition = { x: touches[0].clientX, y: touches[0].clientY };
      touchState.hasMoved = false;
    }

    vlens.scheduleRedraw();
  };

  const handleTouchMove = (e: TouchEvent) => {
    const touches = Array.from(e.touches);
    const DRAG_THRESHOLD = 8; // pixels

    if (touches.length === 2 && touchState.touches.length === 2) {
      // Pinch zoom - prevent default to stop browser zoom
      e.preventDefault();

      const currentDistance = getDistance(touches[0], touches[1]);
      const scaleChange = currentDistance / touchState.initialDistance;
      const newScale = constrainZoom(touchState.initialScale * scaleChange);

      // Calculate focal point offset to anchor zoom
      const scaleDelta = newScale - touchState.initialScale;

      // Calculate focal point relative to scale center (where scaling actually happens)
      const relativeFocalX = touchState.initialFocalPoint.x - innerChartWidth / 2;
      const relativeFocalY = touchState.initialFocalPoint.y - innerChartHeight / 2;

      // Calculate how much the focal point moves due to centered scaling
      const focalOffsetX = relativeFocalX * scaleDelta;
      const focalOffsetY = relativeFocalY * scaleDelta;

      zoomState.scale = newScale;
      zoomState.translateX = touchState.initialTranslate.x - focalOffsetX;
      zoomState.translateY = touchState.initialTranslate.y - focalOffsetY;

      vlens.scheduleRedraw();
    } else if (touches.length === 1 && touchState.touches.length === 1 && zoomState.scale > 1) {
      // Single touch when zoomed - check for drag threshold
      const deltaX = touches[0].clientX - touchState.touchStartPosition.x;
      const deltaY = touches[0].clientY - touchState.touchStartPosition.y;
      const distance = Math.sqrt(deltaX * deltaX + deltaY * deltaY);

      if (!touchState.hasMoved && distance > DRAG_THRESHOLD) {
        // Movement exceeded threshold - start panning
        touchState.hasMoved = true;
        zoomState.isDragging = true;
        e.preventDefault(); // Now prevent default to stop scrolling
      }

      if (zoomState.isDragging) {
        // Continue panning
        e.preventDefault();

        const panDeltaX = touches[0].clientX - touchState.touches[0].clientX;
        const panDeltaY = touches[0].clientY - touchState.touches[0].clientY;

        const newTranslateX = touchState.initialTranslate.x + panDeltaX;
        const newTranslateY = touchState.initialTranslate.y + panDeltaY;

        const constrained = constrainTranslate(newTranslateX, newTranslateY, zoomState.scale);
        zoomState.translateX = constrained.x;
        zoomState.translateY = constrained.y;

        vlens.scheduleRedraw();
      }
    }
    // For taps or single touch on non-zoomed chart: don't prevent default to allow clicks
  };

  const handleTouchEnd = (e: TouchEvent) => {
    const wasDragging = zoomState.isDragging;
    const hadMultipleFingers = touchState.touches.length > 1;
    const touchDuration = Date.now() - touchState.touchStartTime;
    const wasTap = !touchState.hasMoved && touchDuration < 300; // Quick tap under 300ms

    // Only prevent default if we were actually zooming or panning (not for taps)
    if ((wasDragging || hadMultipleFingers) && !wasTap) {
      e.preventDefault();
    }

    // Reset all touch state
    zoomState.isDragging = false;
    touchState.touches = [];
    touchState.initialDistance = 0;
    touchState.hasMoved = false;
    touchState.touchStartTime = 0;
    touchState.touchStartPosition = { x: 0, y: 0 };

    vlens.scheduleRedraw();
  };

  const resetZoom = () => {
    zoomState.scale = 1;
    zoomState.translateX = 0;
    zoomState.translateY = 0;
    zoomState.isDragging = false;
    vlens.scheduleRedraw();
  };

  // Use fixed dimensions for stable zoom behavior
  const chartWidth = width;
  const chartHeight = height;

  if (!growthData || growthData.length === 0) {
    return (
      <div className="chart-placeholder">
        <p>📈 No data to display</p>
      </div>
    );
  }

  // Sort data by date for proper line connections
  const sortedData = growthData.slice().sort((a, b) =>
    new Date(a.measurementDate).getTime() - new Date(b.measurementDate).getTime()
  );

  // Separate height and weight data
  const heightData = sortedData.filter(d => d.measurementType === server.Height);
  const weightData = sortedData.filter(d => d.measurementType === server.Weight);

  // Chart margins
  const margin = { top: 20, right: 80, bottom: 60, left: 60 };
  const innerChartWidth = chartWidth - margin.left - margin.right;
  const innerChartHeight = chartHeight - margin.top - margin.bottom;

  // Date range
  const dates = sortedData.map(d => new Date(d.measurementDate));
  const minDate = new Date(Math.min(...dates.map(d => d.getTime())));
  const maxDate = new Date(Math.max(...dates.map(d => d.getTime())));

  // Add some padding to the date range
  const dateRange = maxDate.getTime() - minDate.getTime();
  const paddedMinDate = new Date(minDate.getTime() - dateRange * 0.05);
  const paddedMaxDate = new Date(maxDate.getTime() + dateRange * 0.05);

  // Value ranges for each measurement type
  const heightValues = heightData.map(d => d.value);
  const weightValues = weightData.map(d => d.value);

  const heightMin = heightValues.length > 0 ? Math.min(...heightValues) : 0;
  const heightMax = heightValues.length > 0 ? Math.max(...heightValues) : 100;
  const weightMin = weightValues.length > 0 ? Math.min(...weightValues) : 0;
  const weightMax = weightValues.length > 0 ? Math.max(...weightValues) : 50;

  // Add padding to value ranges
  const heightPadding = (heightMax - heightMin) * 0.1;
  const weightPadding = (weightMax - weightMin) * 0.1;

  // Scaling functions
  const dateToX = (date: Date) =>
    ((date.getTime() - paddedMinDate.getTime()) / (paddedMaxDate.getTime() - paddedMinDate.getTime())) * innerChartWidth;

  const heightToY = (value: number) =>
    innerChartHeight - ((value - (heightMin - heightPadding)) / ((heightMax + heightPadding) - (heightMin - heightPadding))) * innerChartHeight;

  const weightToY = (value: number) =>
    innerChartHeight - ((value - (weightMin - weightPadding)) / ((weightMax + weightPadding) - (weightMin - weightPadding))) * innerChartHeight;

  // Generate path strings
  const createPath = (data: server.GrowthData[], yScale: (value: number) => number) => {
    if (data.length === 0) return '';

    const pathCommands = data.map((d, i) => {
      const x = dateToX(new Date(d.measurementDate));
      const y = yScale(d.value);
      return `${i === 0 ? 'M' : 'L'} ${x} ${y}`;
    });

    return pathCommands.join(' ');
  };

  const heightPath = createPath(heightData, heightToY);
  const weightPath = createPath(weightData, weightToY);

  const handleDataPointClick = (dataPoint: server.GrowthData, type: string) => {
    if (selectedPoint.id === dataPoint.id) {
      // Deselect if clicking the same point - reset all values
      selectedPoint.id = null;
      selectedPoint.value = 0;
      selectedPoint.unit = '';
      selectedPoint.type = '';
      selectedPoint.date = '';
    } else {
      // Select the new point
      selectedPoint.id = dataPoint.id;
      selectedPoint.value = dataPoint.value;
      selectedPoint.unit = dataPoint.unit;
      selectedPoint.type = type;
      selectedPoint.date = formatDate(dataPoint.measurementDate);
    }

    vlens.scheduleRedraw();
  };

  const handleDataPointHover = (dataPoint: server.GrowthData) => {
    hoveredPoint.id = dataPoint.id;
    vlens.scheduleRedraw();
  };

  const handleDataPointLeave = () => {
    hoveredPoint.id = null;
    vlens.scheduleRedraw();
  };

  return (
    <div className="growth-chart">
      {/* Zoom Controls */}
      {zoomState.scale > 1 && (
        <div className="zoom-controls">
          <button
            className="btn-zoom-reset"
            onClick={resetZoom}
            aria-label="Reset zoom"
          >
            Reset Zoom (1x)
          </button>
          <span className="zoom-level">{zoomState.scale.toFixed(1)}x</span>
        </div>
      )}

      <svg
        className="growth-chart-svg"
        viewBox={`0 0 ${chartWidth} ${chartHeight}`}
        preserveAspectRatio="xMidYMid meet"
        onTouchStart={handleTouchStart}
        onTouchMove={handleTouchMove}
        onTouchEnd={handleTouchEnd}
        style={{ touchAction: 'none' }}
      >
        <defs>
          <linearGradient id="heightGradient" x1="0%" y1="0%" x2="0%" y2="100%">
            <stop offset="0%" stopColor="var(--primary-accent)" stopOpacity="0.3" />
            <stop offset="100%" stopColor="var(--primary-accent)" stopOpacity="0.1" />
          </linearGradient>
          <linearGradient id="weightGradient" x1="0%" y1="0%" x2="0%" y2="100%">
            <stop offset="0%" stopColor="var(--accent)" stopOpacity="0.3" />
            <stop offset="100%" stopColor="var(--accent)" stopOpacity="0.1" />
          </linearGradient>
        </defs>

        {/* Margin group - applied first, outside of scaling */}
        <g transform={`translate(${margin.left}, ${margin.top})`}>
          {/* Zoom/pan group - scales from center of content area */}
          <g
            transform={`
              translate(${innerChartWidth / 2}, ${innerChartHeight / 2})
              scale(${zoomState.scale})
              translate(${-innerChartWidth / 2}, ${-innerChartHeight / 2})
              translate(${zoomState.translateX}, ${zoomState.translateY})
            `}
          >
          {/* Grid lines */}
          <g className="grid">
            {/* Vertical grid lines (dates) */}
            {[0, 0.25, 0.5, 0.75, 1].map(ratio => (
              <line
                key={`vgrid-${ratio}`}
                x1={ratio * innerChartWidth}
                y1={0}
                x2={ratio * innerChartWidth}
                y2={innerChartHeight}
                className="grid-line"
              />
            ))}

            {/* Horizontal grid lines */}
            {[0, 0.25, 0.5, 0.75, 1].map(ratio => (
              <line
                key={`hgrid-${ratio}`}
                x1={0}
                y1={ratio * innerChartHeight}
                x2={innerChartWidth}
                y2={ratio * innerChartHeight}
                className="grid-line"
              />
            ))}
          </g>

          {/* Height line */}
          {heightData.length > 0 && (
            <g className="height-line">
              <path
                d={heightPath}
                fill="none"
                stroke="#3b82f6"
                strokeWidth="3"
                className="chart-line"
              />
            </g>
          )}

          {/* Weight line */}
          {weightData.length > 0 && (
            <g className="weight-line">
              <path
                d={weightPath}
                fill="none"
                stroke="#ef4444"
                strokeWidth="3"
                className="chart-line"
              />
            </g>
          )}

          {/* Data points for height */}
          {heightData.map((d, i) => {
            const isSelected = selectedPoint.id === d.id;
            const isHovered = hoveredPoint.id === d.id;
            return (
              <circle
                key={`height-${d.id}`}
                cx={dateToX(new Date(d.measurementDate))}
                cy={heightToY(d.value)}
                r={getPointRadius(isSelected)}
                fill="#3b82f6"
                stroke="white"
                strokeWidth={getStrokeWidth(isSelected)}
                className={`data-point height-point ${isSelected ? 'selected' : ''} ${isHovered ? 'hovered' : ''}`}
                onClick={() => handleDataPointClick(d, 'Height')}
                onMouseEnter={() => handleDataPointHover(d)}
                onMouseLeave={handleDataPointLeave}
                style={{ cursor: 'pointer' }}
                tabIndex={0}
                role="button"
                aria-label={`Height measurement: ${d.value} ${d.unit} on ${formatDate(d.measurementDate)}`}
                onKeyDown={(e) => {
                  if (e.key === 'Enter' || e.key === ' ') {
                    e.preventDefault();
                    handleDataPointClick(d, 'Height');
                  }
                }}
                onFocus={() => handleDataPointHover(d)}
                onBlur={handleDataPointLeave}
              />
            );
          })}

          {/* Data points for weight */}
          {weightData.map((d, i) => {
            const isSelected = selectedPoint.id === d.id;
            const isHovered = hoveredPoint.id === d.id;
            return (
              <circle
                key={`weight-${d.id}`}
                cx={dateToX(new Date(d.measurementDate))}
                cy={weightToY(d.value)}
                r={getPointRadius(isSelected)}
                fill="#ef4444"
                stroke="white"
                strokeWidth={getStrokeWidth(isSelected)}
                className={`data-point weight-point ${isSelected ? 'selected' : ''} ${isHovered ? 'hovered' : ''}`}
                onClick={() => handleDataPointClick(d, 'Weight')}
                onMouseEnter={() => handleDataPointHover(d)}
                onMouseLeave={handleDataPointLeave}
                style={{ cursor: 'pointer' }}
                tabIndex={0}
                role="button"
                aria-label={`Weight measurement: ${d.value} ${d.unit} on ${formatDate(d.measurementDate)}`}
                onKeyDown={(e) => {
                  if (e.key === 'Enter' || e.key === ' ') {
                    e.preventDefault();
                    handleDataPointClick(d, 'Weight');
                  }
                }}
                onFocus={() => handleDataPointHover(d)}
                onBlur={handleDataPointLeave}
              />
            );
          })}

          {/* Axes */}
          <g className="axes">
            {/* X-axis */}
            <line
              x1={0}
              y1={innerChartHeight}
              x2={innerChartWidth}
              y2={innerChartHeight}
              className="axis-line"
            />

            {/* Y-axis */}
            <line
              x1={0}
              y1={0}
              x2={0}
              y2={innerChartHeight}
              className="axis-line"
            />

            {/* X-axis labels (dates) */}
            {[0, 0.25, 0.5, 0.75, 1].map((ratio, i) => {
              const date = new Date(paddedMinDate.getTime() + ratio * (paddedMaxDate.getTime() - paddedMinDate.getTime()));
              return (
                <text
                  key={`x-label-${i}`}
                  x={ratio * innerChartWidth}
                  y={innerChartHeight + 20}
                  textAnchor="middle"
                  className="axis-label"
                  fontSize="10"
                >
                  {date.toLocaleDateString(undefined, { month: 'short', year: '2-digit' })}
                </text>
              );
            })}

            {/* Y-axis labels for height (left side, blue) */}
            {heightData.length > 0 && [0, 0.25, 0.5, 0.75, 1].map((ratio, i) => {
              const value = (heightMin - heightPadding) + ratio * ((heightMax + heightPadding) - (heightMin - heightPadding));
              const displayValue = Math.round(value);
              if (displayValue < heightMin || displayValue > heightMax) return null;

              return (
                <text
                  key={`height-y-label-${i}`}
                  x={-10}
                  y={innerChartHeight - ratio * innerChartHeight}
                  textAnchor="end"
                  className="axis-label height-axis-label"
                  fontSize="10"
                  dy="0.35em"
                >
                  {displayValue}
                </text>
              );
            })}

            {/* Y-axis labels for weight (right side, red) */}
            {weightData.length > 0 && [0, 0.25, 0.5, 0.75, 1].map((ratio, i) => {
              const value = (weightMin - weightPadding) + ratio * ((weightMax + weightPadding) - (weightMin - weightPadding));
              const displayValue = Math.round(value);
              if (displayValue < weightMin || displayValue > weightMax) return null;

              return (
                <text
                  key={`weight-y-label-${i}`}
                  x={innerChartWidth + 10}
                  y={innerChartHeight - ratio * innerChartHeight}
                  textAnchor="start"
                  className="axis-label weight-axis-label"
                  fontSize="10"
                  dy="0.35em"
                >
                  {displayValue}
                </text>
              );
            })}
          </g>
          </g>
        </g>

        {/* Legend - outside of scaling/margin groups */}
        <g className="legend" transform={`translate(${chartWidth - margin.right + 10}, ${margin.top + 20})`}>
          {heightData.length > 0 && (
            <g className="legend-item">
              <circle cx="0" cy="0" r="5" fill="#3b82f6" stroke="white" strokeWidth="2" />
              <text x="15" y="0" className="legend-text" dy="0.35em">Height</text>
            </g>
          )}
          {weightData.length > 0 && (
            <g className="legend-item" transform="translate(0, 25)">
              <circle cx="0" cy="0" r="5" fill="#ef4444" stroke="white" strokeWidth="2" />
              <text x="15" y="0" className="legend-text" dy="0.35em">Weight</text>
            </g>
          )}
        </g>
      </svg>

      {/* Data Point Info Panel */}
      {selectedPoint.id !== null ? (
        <div className="data-point-info">
          <div className="info-header">
            <span className="info-type">{selectedPoint.type}</span>
            <span className="info-date">{selectedPoint.date}</span>
          </div>
          <div className="info-value">
            {selectedPoint.value} {selectedPoint.unit}
          </div>
        </div>
      ) : (
        <div className="data-point-info placeholder">
          <div className="info-hint">Click on a data point to see details</div>
        </div>
      )}
    </div>
  );
};