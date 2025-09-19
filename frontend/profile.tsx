import * as preact from "preact";
import * as vlens from "vlens";
import * as rpc from "vlens/rpc";
import * as auth from "./authCache";
import * as core from "vlens/core";
import * as server from "./server";
import { Header, Footer } from "./layout";

type ProfileState = {
  activeTab: 'timeline' | 'growth' | 'photos';
}

const useProfileState = vlens.declareHook((): ProfileState => ({
  activeTab: 'timeline'
}));

export async function fetch(route: string, prefix: string) {
  const personId = parseInt(route.split('/')[2]);
  return server.GetPerson({ id: personId });
}

type ProfileData = server.GetPersonResponse | { person: null; growthData: server.GrowthData[] };

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

export function view(
  route: string,
  prefix: string,
  data: ProfileData,
): preact.ComponentChild {
  const currentAuth = auth.getAuth();
  if (!currentAuth || currentAuth.id <= 0) {
    auth.clearAuth();
    core.setRoute('/login');
    return;
  }

  if (!data.person) {
    return (
      <div>
        <Header isHome={false} />
        <main id="app" className="profile-container">
          <div className="error-page">
            <h1>Error</h1>
            <p>Failed to load person data</p>
            <a href="/dashboard" className="btn btn-primary">Back to Dashboard</a>
          </div>
        </main>
        <Footer />
      </div>
    );
  }

  return (
    <div>
      <Header isHome={false} />
      <main id="app" className="profile-container">
        <ProfilePage person={data.person} growthData={data.growthData} />
      </main>
      <Footer />
    </div>
  );
}

interface ProfilePageProps {
  person: server.Person;
  growthData: server.GrowthData[];
}

function setActiveTab(state: ProfileState, tab: 'timeline' | 'growth' | 'photos') {
  state.activeTab = tab;
  vlens.scheduleRedraw();
}

const ProfilePage = ({ person, growthData }: ProfilePageProps) => {
  const state = useProfileState();

  const getGenderIcon = (gender: number) => {
    switch (gender) {
      case 0: return "👨";
      case 1: return "👩";
      default: return "👤";
    }
  };

  const getTypeLabel = (type: number) => {
    return type === 0 ? "Parent" : "Child";
  };

  const calculateAge = (birthday: string) => {
    if (!birthday) return person.age || 0;
    // Parse birthday as local date to avoid timezone issues
    const dateParts = birthday.split('T')[0].split('-');
    const birthYear = parseInt(dateParts[0]);
    const birthMonth = parseInt(dateParts[1]) - 1; // Month is 0-indexed
    const birthDay = parseInt(dateParts[2]);
    const birth = new Date(birthYear, birthMonth, birthDay);

    const today = new Date();
    let age = today.getFullYear() - birth.getFullYear();
    const monthDiff = today.getMonth() - birth.getMonth();
    if (monthDiff < 0 || (monthDiff === 0 && today.getDate() < birth.getDate())) {
      age--;
    }
    return age;
  };

  return (
    <div className="profile-page">
      {/* Profile Header */}
      <div className="profile-header">
        <div className="profile-header-main">
          <div className="profile-avatar">
            {getGenderIcon(person.gender)}
          </div>
          <div className="profile-info">
            <h1>{person.name}</h1>
            <p className="profile-details">
              {getTypeLabel(person.type)} • Age {calculateAge(person.birthday)}
            </p>
            <p className="profile-birthday">
              Birthday: {formatDate(person.birthday)}
            </p>
          </div>
        </div>

        <div className="profile-actions">
          <button className="btn btn-primary">
            📝 Add Milestone
          </button>
          <a href={`/add-growth/${person.id}`} className="btn btn-primary">
            📏 Add Growth
          </a>
          <button className="btn btn-primary">
            📸 Add Photo
          </button>
        </div>
      </div>

      {/* Navigation Tabs */}
      <div className="profile-tabs">
        <button
          className={`tab ${state.activeTab === 'timeline' ? 'active' : ''}`}
          onClick={() => setActiveTab(state, 'timeline')}
        >
          📰 Timeline
        </button>
        <button
          className={`tab ${state.activeTab === 'growth' ? 'active' : ''}`}
          onClick={() => setActiveTab(state, 'growth')}
        >
          📊 Growth
        </button>
        <button
          className={`tab ${state.activeTab === 'photos' ? 'active' : ''}`}
          onClick={() => setActiveTab(state, 'photos')}
        >
          🖼️ Photos
        </button>
      </div>

      {/* Tab Content */}
      <div className="profile-content">
        {state.activeTab === 'timeline' && <TimelineTab person={person} />}
        {state.activeTab === 'growth' && <GrowthTab person={person} growthData={growthData} />}
        {state.activeTab === 'photos' && <PhotosTab person={person} />}
      </div>
    </div>
  );
};

const TimelineTab = ({ person }: { person: server.Person }) => {
  return (
    <div className="timeline-tab">
      <h2>Timeline for {person.name}</h2>
      <div className="timeline-content">
        <div className="empty-state">
          <p>No timeline entries yet.</p>
          <button className="btn btn-primary">Add First Milestone</button>
        </div>
      </div>
    </div>
  );
};

interface GrowthChartProps {
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

const GrowthChart = ({ growthData, width = 600, height = 400 }: GrowthChartProps) => {
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

const GrowthTab = ({ person, growthData }: { person: server.Person; growthData: server.GrowthData[] }) => {
  const getMeasurementTypeLabel = (type: server.MeasurementType) => {
    return type === server.Height ? 'Height' : 'Weight';
  };

  // Sort growth data by measurement date (newest first) for table
  const sortedGrowthData = (growthData || []).slice().sort((a, b) =>
    new Date(b.measurementDate).getTime() - new Date(a.measurementDate).getTime()
  );

  return (
    <div className="growth-tab">
      <h2>Growth Tracking for {person.name}</h2>
      <div className="growth-content">
        <div className="growth-chart-container">
          <h3>Growth Chart</h3>
          <GrowthChart growthData={growthData} />
        </div>

        <div className="growth-table">
          <h3>Growth Records</h3>
          {sortedGrowthData.length === 0 ? (
            <div className="empty-state">
              <p>No growth records yet.</p>
              <a href={`/add-growth/${person.id}`} className="btn btn-primary">Add First Measurement</a>
            </div>
          ) : (
            <div className="growth-records">
              <table className="data-table">
                <thead>
                  <tr>
                    <th>Type</th>
                    <th>Value</th>
                    <th>Date</th>
                    <th>Added</th>
                  </tr>
                </thead>
                <tbody>
                  {sortedGrowthData.map(record => (
                    <tr key={record.id}>
                      <td>{getMeasurementTypeLabel(record.measurementType)}</td>
                      <td>{record.value} {record.unit}</td>
                      <td>{formatDate(record.measurementDate)}</td>
                      <td>{formatDate(record.createdAt)}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
              <div className="table-actions">
                <a href={`/add-growth/${person.id}`} className="btn btn-primary">Add New Measurement</a>
              </div>
            </div>
          )}
        </div>
      </div>
    </div>
  );
};

const PhotosTab = ({ person }: { person: server.Person }) => {
  return (
    <div className="photos-tab">
      <h2>Photos of {person.name}</h2>
      <div className="photos-content">
        <div className="photos-gallery">
          <div className="empty-state">
            <p>No photos yet.</p>
            <button className="btn btn-primary">Add First Photo</button>
          </div>
        </div>
      </div>
    </div>
  );
};
