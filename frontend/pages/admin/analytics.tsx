import * as preact from "preact";
import * as rpc from "vlens/rpc";
import * as vlens from "vlens";
import * as server from "../../server";
import { Header, Footer } from "../../layout";
import { ensureAuthInFetch, requireAuthInView } from "../../lib/authHelpers";
import "./analytics-styles";

export async function fetch(route: string, prefix: string) {
  if (!await ensureAuthInFetch()) {
    return rpc.ok<server.AnalyticsOverviewResponse>({
      totalUsers: 0,
      totalFamilies: 0,
      totalPhotos: 0,
      totalMilestones: 0,
      activeUsers7d: 0,
      activeUsers30d: 0,
      newUsers7d: 0,
      newUsers30d: 0,
      recentActivity: [],
      systemHealth: { photosProcessing: 0, photosFailed: 0 }
    });
  }

  // For now, just return the overview data to fix the loading issue
  // Other sections will be loaded on-demand when their tabs are selected
  return server.GetAnalyticsOverview({});
}

export function view(
  route: string,
  prefix: string,
  data: server.AnalyticsOverviewResponse,
): preact.ComponentChild {
  const currentAuth = requireAuthInView();
  if (!currentAuth) {
    return;
  }

  // Check if user is admin (ID == 1)
  if (!currentAuth.isAdmin) {
    return (
      <div>
        <Header isHome={false} />
        <main id="app" className="page-container">
          <div className="error-page">
            <h1>Access Denied</h1>
            <p>You do not have permission to access this page.</p>
            <a href="/dashboard" className="btn btn-primary">Return to Dashboard</a>
          </div>
        </main>
        <Footer />
      </div>
    );
  }

  return (
    <div>
      <Header isHome={false} />
      <main id="app" className="analytics-container">
        <AnalyticsPage overviewData={data} />
      </main>
      <Footer />
    </div>
  );
}

interface AnalyticsPageProps {
  overviewData: server.AnalyticsOverviewResponse;
}

type AnalyticsState = {
  selectedTimeRange: string;
  selectedView: string;
  userData?: server.UserAnalyticsResponse;
  contentData?: server.ContentAnalyticsResponse;
  systemData?: server.SystemAnalyticsResponse;
  loading: { [key: string]: boolean };
};

const useAnalyticsState = vlens.declareHook((): AnalyticsState => {
  return {
    selectedTimeRange: "30d",
    selectedView: "overview",
    loading: {}
  };
});

const AnalyticsPage = ({ overviewData }: AnalyticsPageProps) => {
  const state = useAnalyticsState();
  const selectedTimeRange = vlens.ref(state, "selectedTimeRange");
  const selectedView = vlens.ref(state, "selectedView");

  return (
    <div className="analytics-page">
      <div className="analytics-header">
        <div className="analytics-badge">
          <span className="analytics-icon">📊</span>
          <span>Analytics Dashboard</span>
        </div>
        <h1>Site Analytics</h1>
        <p>Comprehensive insights into user engagement and system performance</p>
      </div>

      <div className="analytics-controls">
        <div className="view-selector">
          <button
            className={`view-btn ${vlens.refGet(selectedView) === "overview" ? "active" : ""}`}
            onClick={() => { vlens.refSet(selectedView, "overview"); vlens.scheduleRedraw(); }}
          >
            Overview
          </button>
          <button
            className={`view-btn ${vlens.refGet(selectedView) === "users" ? "active" : ""}`}
            onClick={() => { vlens.refSet(selectedView, "users"); vlens.scheduleRedraw(); }}
          >
            Users
          </button>
          <button
            className={`view-btn ${vlens.refGet(selectedView) === "content" ? "active" : ""}`}
            onClick={() => { vlens.refSet(selectedView, "content"); vlens.scheduleRedraw(); }}
          >
            Content
          </button>
          <button
            className={`view-btn ${vlens.refGet(selectedView) === "system" ? "active" : ""}`}
            onClick={() => { vlens.refSet(selectedView, "system"); vlens.scheduleRedraw(); }}
          >
            System
          </button>
        </div>

        <div className="time-selector">
          <select {...vlens.attrsBindInput(selectedTimeRange)}>
            <option value="7d">Last 7 days</option>
            <option value="30d">Last 30 days</option>
            <option value="90d">Last 90 days</option>
          </select>
        </div>
      </div>

      {vlens.refGet(selectedView) === "overview" && <OverviewView data={overviewData} />}
      {vlens.refGet(selectedView) === "users" && <UsersViewPlaceholder />}
      {vlens.refGet(selectedView) === "content" && <ContentViewPlaceholder />}
      {vlens.refGet(selectedView) === "system" && <SystemViewPlaceholder />}
    </div>
  );
};

const OverviewView = ({ data }: { data: server.AnalyticsOverviewResponse }) => {
  return (
    <div className="analytics-content">
      <div className="metrics-grid">
        <div className="metric-card">
          <div className="metric-value">{data.totalUsers}</div>
          <div className="metric-label">Total Users</div>
          <div className="metric-change positive">+{data.newUsers30d} this month</div>
        </div>

        <div className="metric-card">
          <div className="metric-value">{data.totalFamilies}</div>
          <div className="metric-label">Total Families</div>
        </div>

        <div className="metric-card">
          <div className="metric-value">{data.totalPhotos.toLocaleString()}</div>
          <div className="metric-label">Total Photos</div>
        </div>

        <div className="metric-card">
          <div className="metric-value">{data.totalMilestones.toLocaleString()}</div>
          <div className="metric-label">Total Milestones</div>
        </div>

        <div className="metric-card">
          <div className="metric-value">{data.activeUsers7d}</div>
          <div className="metric-label">Active Users (7d)</div>
          <div className="metric-change">
            {data.activeUsers30d} active this month
          </div>
        </div>

        <div className="metric-card">
          <div className="metric-value">
            {data.systemHealth.photosProcessing + data.systemHealth.photosFailed}
          </div>
          <div className="metric-label">Queue Status</div>
          <div className="metric-change">
            {data.systemHealth.photosProcessing} processing, {data.systemHealth.photosFailed} failed
          </div>
        </div>
      </div>

      <div className="charts-grid">
        <div className="chart-card">
          <h3>Recent Activity (Last 7 Days)</h3>
          <StackedBarChart data={data.recentActivity} />
        </div>

        <div className="chart-card">
          <h3>System Health</h3>
          <div className="health-indicators">
            <div className="health-item">
              <span className="health-label">Photos Processing</span>
              <span className="health-value">{data.systemHealth.photosProcessing}</span>
            </div>
            <div className="health-item">
              <span className="health-label">Failed Photos</span>
              <span className="health-value error">{data.systemHealth.photosFailed}</span>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
};

const UsersView = ({ data }: { data: server.UserAnalyticsResponse }) => {
  return (
    <div className="analytics-content">
      <div className="charts-grid">
        <div className="chart-card">
          <h3>Registration Trends</h3>
          <SimpleLineChart data={data.registrationTrends} />
        </div>

        <div className="chart-card">
          <h3>Login Activity</h3>
          <SimpleLineChart data={data.loginActivityTrends} />
        </div>

        <div className="chart-card">
          <h3>Family Size Distribution</h3>
          <SimplePieChart data={data.familySizeDistribution} />
        </div>

        <div className="chart-card">
          <h3>User Retention</h3>
          <div className="retention-metrics">
            <div className="retention-item">
              <span className="retention-label">Day 1</span>
              <span className="retention-value">{data.userRetention.day1.toFixed(1)}%</span>
            </div>
            <div className="retention-item">
              <span className="retention-label">Day 7</span>
              <span className="retention-value">{data.userRetention.day7.toFixed(1)}%</span>
            </div>
            <div className="retention-item">
              <span className="retention-label">Day 30</span>
              <span className="retention-value">{data.userRetention.day30.toFixed(1)}%</span>
            </div>
            <div className="retention-item">
              <span className="retention-label">Day 90</span>
              <span className="retention-value">{data.userRetention.day90.toFixed(1)}%</span>
            </div>
          </div>
        </div>
      </div>

      <div className="table-card">
        <h3>Top Active Families</h3>
        <div className="families-table">
          {data.topActiveFamilies.slice(0, 10).map((family, index) => (
            <div key={index} className="family-row">
              <span className="family-name">{family.familyName}</span>
              <span className="family-stats">
                {family.totalPhotos} photos, {family.totalMilestones} milestones
              </span>
              <span className="family-score">Score: {family.score}</span>
              <span className="family-active">Last: {family.lastActive || "Never"}</span>
            </div>
          ))}
        </div>
      </div>
    </div>
  );
};

const ContentView = ({ data }: { data: server.ContentAnalyticsResponse }) => {
  return (
    <div className="analytics-content">
      <div className="metrics-grid">
        <div className="metric-card">
          <div className="metric-value">{data.averagePhotosPerChild.toFixed(1)}</div>
          <div className="metric-label">Avg Photos per Child</div>
        </div>

        <div className="metric-card">
          <div className="metric-value">{data.averageMilestonesPerChild.toFixed(1)}</div>
          <div className="metric-label">Avg Milestones per Child</div>
        </div>
      </div>

      <div className="charts-grid">
        <div className="chart-card">
          <h3>Photo Upload Trends</h3>
          <SimpleLineChart data={data.photoUploadTrends} />
        </div>

        <div className="chart-card">
          <h3>Milestones by Category</h3>
          <SimplePieChart data={data.milestonesByCategory} />
        </div>

        <div className="chart-card">
          <h3>Photo Formats</h3>
          <SimplePieChart data={data.photoFormats} />
        </div>

        <div className="chart-card">
          <h3>Content per Family</h3>
          <div className="family-content-list">
            {data.contentPerFamily.slice(0, 8).map((family, index) => (
              <div key={index} className="family-content-item">
                <span className="family-name">{family.familyName}</span>
                <div className="family-content-stats">
                  <span>{family.photos} photos</span>
                  <span>{family.milestones} milestones</span>
                  <span>{family.children} children</span>
                </div>
              </div>
            ))}
          </div>
        </div>
      </div>
    </div>
  );
};

const SystemView = ({ data }: { data: server.SystemAnalyticsResponse }) => {
  const formatFileSize = (bytes: number) => {
    const units = ['B', 'KB', 'MB', 'GB'];
    let size = bytes;
    let unitIndex = 0;

    while (size >= 1024 && unitIndex < units.length - 1) {
      size /= 1024;
      unitIndex++;
    }

    return `${size.toFixed(1)} ${units[unitIndex]}`;
  };

  return (
    <div className="analytics-content">
      <div className="metrics-grid">
        <div className="metric-card">
          <div className="metric-value">{formatFileSize(data.storageUsage.totalSize)}</div>
          <div className="metric-label">Total Storage Used</div>
        </div>

        <div className="metric-card">
          <div className="metric-value">{formatFileSize(data.storageUsage.averageFileSize)}</div>
          <div className="metric-label">Average File Size</div>
        </div>

        <div className="metric-card">
          <div className="metric-value">{data.processingMetrics.successRate.toFixed(1)}%</div>
          <div className="metric-label">Processing Success Rate</div>
        </div>

        <div className="metric-card">
          <div className="metric-value">{data.processingMetrics.queueLength}</div>
          <div className="metric-label">Queue Length</div>
        </div>
      </div>

      <div className="charts-grid">
        <div className="chart-card">
          <h3>Error Analysis</h3>
          <div className="error-summary">
            <div className="error-stat">
              <span className="error-label">Total Errors</span>
              <span className="error-value">{data.errorAnalysis.totalErrors}</span>
            </div>
            <div className="error-categories">
              {data.errorAnalysis.errorsByCategory.map((category, index) => (
                <div key={index} className="error-category">
                  <span>{category.label}</span>
                  <span>{category.value}</span>
                </div>
              ))}
            </div>
          </div>
        </div>

        <div className="chart-card">
          <h3>Recent Errors</h3>
          <div className="recent-errors">
            {data.errorAnalysis.recentErrors.length > 0 ? (
              data.errorAnalysis.recentErrors.slice(0, 5).map((error, index) => (
                <div key={index} className="error-item">
                  {error}
                </div>
              ))
            ) : (
              <div className="no-errors">No recent errors</div>
            )}
          </div>
        </div>
      </div>
    </div>
  );
};

// Simple chart components (using CSS for visualization)
const SimpleLineChart = ({ data }: { data: server.DataPoint[] }) => {
  if (!data || data.length === 0) {
    return <div className="chart-placeholder">No data available</div>;
  }

  const maxValue = Math.max(...data.map(d => d.value));
  const minValue = Math.min(...data.map(d => d.value));
  const range = maxValue - minValue;

  return (
    <div className="simple-line-chart">
      <div className="chart-area">
        {data.map((point, index) => {
          const height = range > 0 ? ((point.value - minValue) / range) * 100 : 50;
          const left = (index / (data.length - 1)) * 100;

          return (
            <div
              key={index}
              className="chart-point"
              style={{
                left: `${left}%`,
                bottom: `${height}%`,
              }}
              title={`${point.date}: ${point.value}`}
            />
          );
        })}
      </div>
      <div className="chart-labels">
        <span>{data[0]?.date}</span>
        <span>{data[data.length - 1]?.date}</span>
      </div>
    </div>
  );
};

const StackedBarChart = ({ data }: { data: server.ActivitySummary[] }) => {
  if (!data || data.length === 0) {
    return <div className="chart-placeholder">No data available</div>;
  }

  const maxTotal = Math.max(...data.map(d => d.photos + d.milestones + d.logins));

  return (
    <div className="stacked-bar-chart">
      <div className="chart-area">
        {data.map((item, index) => {
          const total = item.photos + item.milestones + item.logins;
          const photosPercent = maxTotal > 0 ? (item.photos / maxTotal) * 100 : 0;
          const milestonesPercent = maxTotal > 0 ? (item.milestones / maxTotal) * 100 : 0;
          const loginsPercent = maxTotal > 0 ? (item.logins / maxTotal) * 100 : 0;

          return (
            <div key={index} className="stacked-bar-item">
              <div className="stacked-bar">
                {item.photos > 0 && (
                  <div
                    className="bar-segment photos"
                    style={{ height: `${photosPercent}%` }}
                    title={`${item.photos} photos`}
                  />
                )}
                {item.milestones > 0 && (
                  <div
                    className="bar-segment milestones"
                    style={{ height: `${milestonesPercent}%` }}
                    title={`${item.milestones} milestones`}
                  />
                )}
                {item.logins > 0 && (
                  <div
                    className="bar-segment logins"
                    style={{ height: `${loginsPercent}%` }}
                    title={`${item.logins} logins`}
                  />
                )}
              </div>
              <div className="bar-label">{item.date.slice(-2)}</div>
              <div className="bar-total">{total}</div>
            </div>
          );
        })}
      </div>
      <div className="chart-legend">
        <div className="legend-item">
          <div className="legend-color photos"></div>
          <span>Photos</span>
        </div>
        <div className="legend-item">
          <div className="legend-color milestones"></div>
          <span>Milestones</span>
        </div>
        <div className="legend-item">
          <div className="legend-color logins"></div>
          <span>Logins</span>
        </div>
      </div>
    </div>
  );
};

const SimpleBarChart = ({ data }: { data: server.ActivitySummary[] }) => {
  if (!data || data.length === 0) {
    return <div className="chart-placeholder">No data available</div>;
  }

  const maxTotal = Math.max(...data.map(d => d.photos + d.milestones + d.logins));

  return (
    <div className="simple-bar-chart">
      {data.map((item, index) => {
        const total = item.photos + item.milestones + item.logins;
        const height = maxTotal > 0 ? (total / maxTotal) * 100 : 0;

        return (
          <div key={index} className="bar-item">
            <div
              className="bar"
              style={{ height: `${height}%` }}
              title={`${item.date}: ${item.photos} photos, ${item.milestones} milestones, ${item.logins} logins`}
            />
            <div className="bar-label">{item.date.slice(-2)}</div>
          </div>
        );
      })}
    </div>
  );
};

const SimplePieChart = ({ data }: { data: server.DistributionPoint[] }) => {
  if (!data || data.length === 0) {
    return <div className="chart-placeholder">No data available</div>;
  }

  const total = data.reduce((sum, item) => sum + item.value, 0);

  return (
    <div className="simple-pie-chart">
      {data.slice(0, 6).map((item, index) => {
        const percentage = total > 0 ? (item.value / total) * 100 : 0;
        return (
          <div key={index} className="pie-item">
            <div className="pie-color" style={{ backgroundColor: `hsl(${index * 60}, 60%, 60%)` }} />
            <span className="pie-label">{item.label}</span>
            <span className="pie-value">{item.value} ({percentage.toFixed(1)}%)</span>
          </div>
        );
      })}
    </div>
  );
};

// Placeholder components for other views
const UsersViewPlaceholder = () => {
  return (
    <div className="analytics-content">
      <div className="chart-placeholder" style={{ minHeight: "400px", fontSize: "1.2rem" }}>
        <div>
          <h3>User Analytics</h3>
          <p>Loading user analytics data...</p>
          <p>This section will show registration trends, user retention, and family engagement metrics.</p>
        </div>
      </div>
    </div>
  );
};

const ContentViewPlaceholder = () => {
  return (
    <div className="analytics-content">
      <div className="chart-placeholder" style={{ minHeight: "400px", fontSize: "1.2rem" }}>
        <div>
          <h3>Content Analytics</h3>
          <p>Loading content analytics data...</p>
          <p>This section will show photo upload trends, milestone tracking, and content patterns.</p>
        </div>
      </div>
    </div>
  );
};

const SystemViewPlaceholder = () => {
  return (
    <div className="analytics-content">
      <div className="chart-placeholder" style={{ minHeight: "400px", fontSize: "1.2rem" }}>
        <div>
          <h3>System Analytics</h3>
          <p>Loading system analytics data...</p>
          <p>This section will show storage usage, processing metrics, and error analysis.</p>
        </div>
      </div>
    </div>
  );
};