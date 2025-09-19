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

export const GrowthChart = ({ growthData, width = 600, height = 400 }: GrowthChartProps) => {
  const selectedPoint = vlens.declareHook((): SelectedDataPoint => ({
    id: null,
    value: 0,
    unit: '',
    type: '',
    date: ''
  }));

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
  const chartWidth = width - margin.left - margin.right;
  const chartHeight = height - margin.top - margin.bottom;

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
    const selected = selectedPoint();
    if (selected.id === dataPoint.id) {
      // Deselect if clicking the same point
      selected.id = null;
    } else {
      // Select the new point
      selected.id = dataPoint.id;
      selected.value = dataPoint.value;
      selected.unit = dataPoint.unit;
      selected.type = type;
      selected.date = formatDate(dataPoint.measurementDate);
    }
    vlens.scheduleRedraw();
  };

  return (
    <div className="growth-chart">
      <svg width={width} height={height} className="growth-chart-svg">
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
            const isSelected = selectedPoint().id === d.id;
            return (
              <circle
                key={`height-${d.id}`}
                cx={dateToX(new Date(d.measurementDate))}
                cy={heightToY(d.value)}
                r={isSelected ? 8 : 5}
                fill="#3b82f6"
                stroke="white"
                strokeWidth={isSelected ? 3 : 2}
                className={`data-point height-point ${isSelected ? 'selected' : ''}`}
                onClick={() => handleDataPointClick(d, 'Height')}
                style={{ cursor: 'pointer' }}
              />
            );
          })}

          {/* Data points for weight */}
          {weightData.map((d, i) => {
            const isSelected = selectedPoint().id === d.id;
            return (
              <circle
                key={`weight-${d.id}`}
                cx={dateToX(new Date(d.measurementDate))}
                cy={weightToY(d.value)}
                r={isSelected ? 8 : 5}
                fill="#ef4444"
                stroke="white"
                strokeWidth={isSelected ? 3 : 2}
                className={`data-point weight-point ${isSelected ? 'selected' : ''}`}
                onClick={() => handleDataPointClick(d, 'Weight')}
                style={{ cursor: 'pointer' }}
              />
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
          </g>
        </g>

        {/* Legend */}
        <g className="legend" transform={`translate(${width - margin.right + 10}, ${margin.top + 20})`}>
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
      {selectedPoint().id !== null && (
        <div className="data-point-info">
          <div className="info-header">
            <span className="info-type">{selectedPoint().type}</span>
            <span className="info-date">{selectedPoint().date}</span>
          </div>
          <div className="info-value">
            {selectedPoint().value} {selectedPoint().unit}
          </div>
          <div className="info-hint">Click point again to deselect</div>
        </div>
      )}

      {selectedPoint().id === null && (
        <div className="data-point-info placeholder">
          <div className="info-hint">Click on a data point to see details</div>
        </div>
      )}
    </div>
  );
};