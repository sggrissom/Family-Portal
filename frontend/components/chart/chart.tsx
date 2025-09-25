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

export const GrowthChart = ({ growthData, width = 600, height = 400 }: GrowthChartProps) => {
  const selectedPoint = useSelectedPoint();
  const hoveredPoint = useHoveredPoint();

  // Make chart responsive
  const isMobile = typeof window !== 'undefined' && window.innerWidth < 768;
  const responsiveWidth = isMobile ? Math.min(window.innerWidth - 32, 400) : width;
  const responsiveHeight = isMobile ? Math.min(300, responsiveWidth * 0.75) : height;

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

  // Chart margins (responsive)
  const margin = isMobile
    ? { top: 15, right: 40, bottom: 45, left: 45 }
    : { top: 20, right: 80, bottom: 60, left: 60 };
  const chartWidth = responsiveWidth - margin.left - margin.right;
  const chartHeight = responsiveHeight - margin.top - margin.bottom;

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
    ((date.getTime() - paddedMinDate.getTime()) / (paddedMaxDate.getTime() - paddedMinDate.getTime())) * chartWidth;

  const heightToY = (value: number) =>
    chartHeight - ((value - (heightMin - heightPadding)) / ((heightMax + heightPadding) - (heightMin - heightPadding))) * chartHeight;

  const weightToY = (value: number) =>
    chartHeight - ((value - (weightMin - weightPadding)) / ((weightMax + weightPadding) - (weightMin - weightPadding))) * chartHeight;

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
    <div className="growth-chart" style={{ position: 'relative' }}>
      <svg
        width={responsiveWidth}
        height={responsiveHeight}
        className="growth-chart-svg"
        viewBox={`0 0 ${responsiveWidth} ${responsiveHeight}`}
        preserveAspectRatio="xMidYMid meet"
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

        <g transform={`translate(${margin.left}, ${margin.top})`}>
          {/* Grid lines */}
          <g className="grid">
            {/* Vertical grid lines (dates) */}
            {[0, 0.25, 0.5, 0.75, 1].map(ratio => (
              <line
                key={`vgrid-${ratio}`}
                x1={ratio * chartWidth}
                y1={0}
                x2={ratio * chartWidth}
                y2={chartHeight}
                className="grid-line"
              />
            ))}

            {/* Horizontal grid lines */}
            {[0, 0.25, 0.5, 0.75, 1].map(ratio => (
              <line
                key={`hgrid-${ratio}`}
                x1={0}
                y1={ratio * chartHeight}
                x2={chartWidth}
                y2={ratio * chartHeight}
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
            const baseRadius = isMobile ? 6 : 5;
            const selectedRadius = isMobile ? 10 : 8;

            return (
              <g key={`height-group-${d.id}`}>
                {/* Invisible larger touch target for mobile */}
                {isMobile && (
                  <circle
                    cx={dateToX(new Date(d.measurementDate))}
                    cy={heightToY(d.value)}
                    r={22} // 44px diameter touch target
                    fill="transparent"
                    onClick={() => handleDataPointClick(d, 'Height')}
                    style={{ cursor: 'pointer' }}
                  />
                )}

                {/* Visible data point */}
                <circle
                  cx={dateToX(new Date(d.measurementDate))}
                  cy={heightToY(d.value)}
                  r={isSelected ? selectedRadius : baseRadius}
                  fill="#3b82f6"
                  stroke="white"
                  strokeWidth={isSelected ? 3 : 2}
                  className={`data-point height-point ${isSelected ? 'selected' : ''} ${isHovered ? 'hovered' : ''}`}
                  onClick={() => handleDataPointClick(d, 'Height')}
                  onMouseEnter={() => handleDataPointHover(d)}
                  onMouseLeave={handleDataPointLeave}
                  style={{ cursor: 'pointer', pointerEvents: isMobile ? 'none' : 'auto' }}
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
              </g>
            );
          })}

          {/* Data points for weight */}
          {weightData.map((d, i) => {
            const isSelected = selectedPoint.id === d.id;
            const isHovered = hoveredPoint.id === d.id;
            const baseRadius = isMobile ? 6 : 5;
            const selectedRadius = isMobile ? 10 : 8;

            return (
              <g key={`weight-group-${d.id}`}>
                {/* Invisible larger touch target for mobile */}
                {isMobile && (
                  <circle
                    cx={dateToX(new Date(d.measurementDate))}
                    cy={weightToY(d.value)}
                    r={22} // 44px diameter touch target
                    fill="transparent"
                    onClick={() => handleDataPointClick(d, 'Weight')}
                    style={{ cursor: 'pointer' }}
                  />
                )}

                {/* Visible data point */}
                <circle
                  cx={dateToX(new Date(d.measurementDate))}
                  cy={weightToY(d.value)}
                  r={isSelected ? selectedRadius : baseRadius}
                  fill="#ef4444"
                  stroke="white"
                  strokeWidth={isSelected ? 3 : 2}
                  className={`data-point weight-point ${isSelected ? 'selected' : ''} ${isHovered ? 'hovered' : ''}`}
                  onClick={() => handleDataPointClick(d, 'Weight')}
                  onMouseEnter={() => handleDataPointHover(d)}
                  onMouseLeave={handleDataPointLeave}
                  style={{ cursor: 'pointer', pointerEvents: isMobile ? 'none' : 'auto' }}
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
              </g>
            );
          })}

          {/* Axes */}
          <g className="axes">
            {/* X-axis */}
            <line
              x1={0}
              y1={chartHeight}
              x2={chartWidth}
              y2={chartHeight}
              className="axis-line"
            />

            {/* Y-axis */}
            <line
              x1={0}
              y1={0}
              x2={0}
              y2={chartHeight}
              className="axis-line"
            />

            {/* X-axis labels (dates) */}
            {(isMobile ? [0, 0.5, 1] : [0, 0.25, 0.5, 0.75, 1]).map((ratio, i) => {
              const date = new Date(paddedMinDate.getTime() + ratio * (paddedMaxDate.getTime() - paddedMinDate.getTime()));
              return (
                <text
                  key={`x-label-${i}`}
                  x={ratio * chartWidth}
                  y={chartHeight + (isMobile ? 15 : 20)}
                  textAnchor="middle"
                  className="axis-label"
                  fontSize={isMobile ? "8" : "10"}
                >
                  {isMobile
                    ? date.toLocaleDateString(undefined, { month: 'short' })
                    : date.toLocaleDateString(undefined, { month: 'short', year: '2-digit' })
                  }
                </text>
              );
            })}

            {/* Y-axis labels for height (left side, blue) */}
            {heightData.length > 0 && (isMobile ? [0, 0.5, 1] : [0, 0.25, 0.5, 0.75, 1]).map((ratio, i) => {
              const value = (heightMin - heightPadding) + ratio * ((heightMax + heightPadding) - (heightMin - heightPadding));
              const displayValue = Math.round(value);
              if (displayValue < heightMin || displayValue > heightMax) return null;

              return (
                <text
                  key={`height-y-label-${i}`}
                  x={isMobile ? -8 : -10}
                  y={chartHeight - ratio * chartHeight}
                  textAnchor="end"
                  className="axis-label height-axis-label"
                  fontSize={isMobile ? "8" : "10"}
                  dy="0.35em"
                >
                  {displayValue}
                </text>
              );
            })}

            {/* Y-axis labels for weight (right side, red) */}
            {weightData.length > 0 && (isMobile ? [0, 0.5, 1] : [0, 0.25, 0.5, 0.75, 1]).map((ratio, i) => {
              const value = (weightMin - weightPadding) + ratio * ((weightMax + weightPadding) - (weightMin - weightPadding));
              const displayValue = Math.round(value);
              if (displayValue < weightMin || displayValue > weightMax) return null;

              return (
                <text
                  key={`weight-y-label-${i}`}
                  x={chartWidth + (isMobile ? 8 : 10)}
                  y={chartHeight - ratio * chartHeight}
                  textAnchor="start"
                  className="axis-label weight-axis-label"
                  fontSize={isMobile ? "8" : "10"}
                  dy="0.35em"
                >
                  {displayValue}
                </text>
              );
            })}
          </g>
        </g>

        {/* Legend */}
        {!isMobile && (
          <g className="legend" transform={`translate(${responsiveWidth - margin.right + 10}, ${margin.top + 20})`}>
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
        )}

        {/* Mobile legend */}
        {isMobile && (
          <g className="legend-mobile" transform={`translate(${responsiveWidth / 2}, ${responsiveHeight - 10})`}>
            <g className="legend-item" transform="translate(-40, 0)">
              <circle cx="0" cy="0" r="4" fill="#3b82f6" stroke="white" strokeWidth="1.5" />
              <text x="8" y="0" className="legend-text" dy="0.35em" fontSize="8">Height</text>
            </g>
            <g className="legend-item" transform="translate(10, 0)">
              <circle cx="0" cy="0" r="4" fill="#ef4444" stroke="white" strokeWidth="1.5" />
              <text x="8" y="0" className="legend-text" dy="0.35em" fontSize="8">Weight</text>
            </g>
          </g>
        )}
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
          <div className="info-hint">Click point again to deselect</div>
        </div>
      ) : (
        <div className="data-point-info placeholder">
          <div className="info-hint">Click on a data point to see details</div>
        </div>
      )}
    </div>
  );
};