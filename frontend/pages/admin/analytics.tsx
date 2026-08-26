import * as preact from "preact";
import * as rpc from "vlens/rpc";
import * as vlens from "vlens";
import * as server from "../../server";
import { Header, Footer } from "../../layout";
import { ensureAuthInFetch, requireAuthInView } from "../../lib/authHelpers";
import "./analytics-styles";

export async function fetch(route: string, prefix: string) {
  if (!(await ensureAuthInFetch())) {
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
      systemHealth: { photosProcessing: 0, photosFailed: 0 },
    });
  }

  return server.GetAnalyticsOverview({});
}

export function view(
  route: string,
  prefix: string,
  data: server.AnalyticsOverviewResponse
): preact.ComponentChild {
  const currentAuth = requireAuthInView();
  if (!currentAuth) {
    return;
  }

  if (!currentAuth.isAdmin) {
    return (
      <div>
        <Header isHome={false} />
        <main id="app" className="page-container">
          <div className="error-page">
            <h1>Access Denied</h1>
            <p>You do not have permission to access this page.</p>
            <a href="/dashboard" className="btn btn-primary">
              Return to Dashboard
            </a>
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

type AnalyticsView = "overview" | "users" | "content" | "system";

type AnalyticsState = {
  selectedView: AnalyticsView;
  userData?: server.UserAnalyticsResponse;
  contentData?: server.ContentAnalyticsResponse;
  systemData?: server.SystemAnalyticsResponse;
  loading: { [key: string]: boolean };
  errors: { [key: string]: string };
};

const useAnalyticsState = vlens.declareHook((): AnalyticsState => {
  return {
    selectedView: "overview",
    loading: {},
    errors: {},
  };
});

const AnalyticsPage = ({ overviewData }: AnalyticsPageProps) => {
  const state = useAnalyticsState();

  const loadTab = async (view: AnalyticsView) => {
    if (state.loading[view]) return;
    state.loading[view] = true;
    state.errors[view] = "";

    const finish = (message: string) => {
      state.loading[view] = false;
      state.errors[view] = message;
      vlens.scheduleRedraw();
    };

    if (view === "users") {
      const [result, error] = await server.GetUserAnalytics({});
      if (result) state.userData = result;
      finish(error ?? "");
    } else if (view === "content") {
      const [result, error] = await server.GetContentAnalytics({});
      if (result) state.contentData = result;
      finish(error ?? "");
    } else if (view === "system") {
      const [result, error] = await server.GetSystemAnalytics({});
      if (result) state.systemData = result;
      finish(error ?? "");
    }
  };

  const selectView = (view: AnalyticsView) => {
    state.selectedView = view;
    vlens.scheduleRedraw();
    if (view !== "overview" && !tabData(state, view)) {
      loadTab(view);
    }
  };

  const view = state.selectedView;

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
          {(["overview", "users", "content", "system"] as AnalyticsView[]).map(name => (
            <button
              key={name}
              className={`view-btn ${view === name ? "active" : ""}`}
              onClick={() => selectView(name)}
            >
              {viewLabels[name]}
            </button>
          ))}
        </div>
      </div>

      {view === "overview" && <OverviewView data={overviewData} />}
      {view !== "overview" && (
        <TabContent state={state} view={view} onRetry={() => loadTab(view)} />
      )}
    </div>
  );
};

const viewLabels: { [key in AnalyticsView]: string } = {
  overview: "Overview",
  users: "Users",
  content: "Content",
  system: "System",
};

function tabData(state: AnalyticsState, view: AnalyticsView) {
  if (view === "users") return state.userData;
  if (view === "content") return state.contentData;
  if (view === "system") return state.systemData;
  return undefined;
}

const TabContent = ({
  state,
  view,
  onRetry,
}: {
  state: AnalyticsState;
  view: AnalyticsView;
  onRetry: () => void;
}) => {
  const error = state.errors[view];
  if (error) {
    return (
      <div className="analytics-content">
        <div className="chart-placeholder large">
          <div>
            <h3>{viewLabels[view]} Analytics</h3>
            <p>{error}</p>
            <button className="admin-btn admin-btn-secondary" onClick={onRetry}>
              Try again
            </button>
          </div>
        </div>
      </div>
    );
  }

  if (view === "users" && state.userData) return <UsersView data={state.userData} />;
  if (view === "content" && state.contentData) return <ContentView data={state.contentData} />;
  if (view === "system" && state.systemData) return <SystemView data={state.systemData} />;

  return (
    <div className="analytics-content">
      <div className="chart-placeholder large">
        <div>
          <h3>{viewLabels[view]} Analytics</h3>
          <p>Loading…</p>
        </div>
      </div>
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
          <div className="metric-change">{data.activeUsers30d} active this month</div>
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
          <h3>Account Engagement</h3>
          <div className="retention-metrics">
            <div className="retention-item">
              <span className="retention-label">Accounts</span>
              <span className="retention-value">{data.userEngagement.total}</span>
            </div>
            <div className="retention-item">
              <span className="retention-label">Never signed in</span>
              <span className="retention-value">{data.userEngagement.neverLoggedIn}</span>
            </div>
            <div className="retention-item">
              <span className="retention-label">Active (7d)</span>
              <span className="retention-value">{data.userEngagement.active7d}</span>
            </div>
            <div className="retention-item">
              <span className="retention-label">Active (30d)</span>
              <span className="retention-value">{data.userEngagement.active30d}</span>
            </div>
            <div className="retention-item">
              <span className="retention-label">Dormant 90d+</span>
              <span className="retention-value">{data.userEngagement.dormant90d}</span>
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
    const units = ["B", "KB", "MB", "GB"];
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
          <h3>Photo Processing Failures</h3>
          <div className="error-summary">
            <div className="error-stat">
              <span className="error-label">Failed</span>
              <span className="error-value">{data.photoFailures.failed}</span>
            </div>
            <div className="error-stat">
              <span className="error-label">Stuck in processing over an hour</span>
              <span className="error-value">{data.photoFailures.stuck}</span>
            </div>
          </div>
        </div>

        <div className="chart-card">
          <h3>Recent Failures</h3>
          <div className="recent-errors">
            {data.photoFailures.recentFailures.length > 0 ? (
              data.photoFailures.recentFailures.map(photo => (
                <div key={photo.id} className="error-item">
                  #{photo.id} · {photo.filePath} · {photo.createdAt}
                </div>
              ))
            ) : (
              <div className="no-errors">No failed photos</div>
            )}
          </div>
        </div>
      </div>
    </div>
  );
};

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
            <div
              className="pie-color"
              style={{ backgroundColor: `hsl(${index * 60}, 60%, 60%)` }}
            />
            <span className="pie-label">{item.label}</span>
            <span className="pie-value">
              {item.value} ({percentage.toFixed(1)}%)
            </span>
          </div>
        );
      })}
    </div>
  );
};
