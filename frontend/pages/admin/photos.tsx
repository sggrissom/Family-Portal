import * as preact from "preact";
import * as vlens from "vlens";
import * as rpc from "vlens/rpc";
import * as auth from "../../lib/authCache";
import * as server from "../../server";
import { Header, Footer } from "../../layout";
import { ensureAuthInFetch, requireAuthInView } from "../../lib/authHelpers";
import "./admin-styles";

type PhotoManagementState = {
  isReprocessing: boolean;
  reprocessProgress: number;
  reprocessTotal: number;
  reprocessErrors: string[];
  lastReprocessTime: string | null;
  processingStats: server.ProcessingStats | null;
};

const usePhotoManagementState = vlens.declareHook((): PhotoManagementState => ({
  isReprocessing: false,
  reprocessProgress: 0,
  reprocessTotal: 0,
  reprocessErrors: [],
  lastReprocessTime: null,
  processingStats: null,
}));

export async function fetch(route: string, prefix: string) {
  if (!await ensureAuthInFetch()) {
    return rpc.ok<server.GetPhotoStatsResponse>({
      totalPhotos: 0,
      processedPhotos: 0,
      pendingPhotos: 0
    });
  }

  return server.GetPhotoStats({});
}

export function view(
  route: string,
  prefix: string,
  data: server.GetPhotoStatsResponse,
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
            <a href="/admin" className="btn btn-primary">Return to Admin Dashboard</a>
          </div>
        </main>
        <Footer />
      </div>
    );
  }

  return (
    <div>
      <Header isHome={false} />
      <main id="app" className="admin-container">
        <PhotoManagementPage data={data} />
      </main>
      <Footer />
    </div>
  );
}

interface PhotoManagementPageProps {
  data: server.GetPhotoStatsResponse;
}

const PhotoManagementPage = ({ data }: PhotoManagementPageProps) => {
  const state = usePhotoManagementState();


  const loadProcessingStats = async () => {
    try {
      const [result, error] = await server.GetPhotoProcessingStats({});
      if (result && !error) {
        state.processingStats = result;
        vlens.scheduleRedraw();
      }
    } catch (err) {
      console.warn("Failed to load processing stats:", err);
    }
  };


  // Load stats initially and set up periodic refresh
  if (!state.processingStats) {
    loadProcessingStats();
    setInterval(loadProcessingStats, 3000); // Poll every 3 seconds
  }

  const startReprocessing = async () => {
    const confirmed = confirm(
      `This will reprocess all ${data.totalPhotos} photos with modern formats and optimized sizes. ` +
      "This may take several minutes and cannot be undone. Continue?"
    );

    if (!confirmed) return;

    state.isReprocessing = true;
    state.reprocessProgress = 0;
    state.reprocessTotal = data.totalPhotos;
    state.reprocessErrors = [];
    vlens.scheduleRedraw();

    try {
      const [result, error] = await server.ReprocessAllPhotos({});

      if (error) {
        state.reprocessErrors.push(error);
      } else if (result) {
        state.reprocessProgress = result.processed;
        state.lastReprocessTime = new Date().toLocaleString();
        // Refresh the page data
        setTimeout(() => {
          window.location.reload();
        }, 1000);
      }
    } catch (err) {
      state.reprocessErrors.push("Failed to start reprocessing: " + String(err));
    }

    state.isReprocessing = false;
    vlens.scheduleRedraw();
  };

  const needsReprocessing = data.totalPhotos > data.processedPhotos;

  return (
    <div className="admin-page">
      <div className="admin-header">
        <div className="admin-breadcrumb">
          <a href="/admin">Admin</a> → Photo Management
        </div>
        <h1>Photo Management</h1>
        <p>Manage photo processing and optimization across the system.</p>
      </div>

      <div className="photo-stats-grid">
        <div className="stat-card">
          <div className="stat-icon">🖼️</div>
          <div className="stat-content">
            <h3>Total Photos</h3>
            <div className="stat-value">{data.totalPhotos.toLocaleString()}</div>
            <div className="stat-label">Across all families</div>
          </div>
        </div>

        <div className="stat-card">
          <div className="stat-icon">✅</div>
          <div className="stat-content">
            <h3>Processed</h3>
            <div className="stat-value">{data.processedPhotos.toLocaleString()}</div>
            <div className="stat-label">With modern formats</div>
          </div>
        </div>

        <div className="stat-card">
          <div className="stat-icon">⏳</div>
          <div className="stat-content">
            <h3>Needs Processing</h3>
            <div className="stat-value">{data.pendingPhotos.toLocaleString()}</div>
            <div className="stat-label">Awaiting optimization</div>
          </div>
        </div>

        <div className="stat-card">
          <div className="stat-icon">📊</div>
          <div className="stat-content">
            <h3>Progress</h3>
            <div className="stat-value">
              {data.totalPhotos > 0 ? Math.round((data.processedPhotos / data.totalPhotos) * 100) : 0}%
            </div>
            <div className="stat-label">Optimization complete</div>
          </div>
        </div>

        {state.processingStats && (
          <div className="stat-card">
            <div className="stat-icon">
              {state.processingStats.isRunning ? '🔄' : '⏸️'}
            </div>
            <div className="stat-content">
              <h3>Processing Queue</h3>
              <div className="stat-value">{state.processingStats.queueLength}</div>
              <div className="stat-label">
                {state.processingStats.isRunning ? 'Photos in queue' : 'Worker stopped'}
              </div>
            </div>
          </div>
        )}
      </div>

      {needsReprocessing && (
        <div className="admin-card reprocess-card">
          <div className="card-header">
            <div className="card-icon">🔄</div>
            <h3>Photo Reprocessing</h3>
          </div>
          <div className="card-content">
            <p>
              Some photos need to be reprocessed with modern formats and optimized sizes.
              This will generate WebP, AVIF, and responsive variants for better performance.
            </p>

            {state.isReprocessing ? (
              <div className="reprocess-progress">
                <div className="progress-bar">
                  <div
                    className="progress-fill"
                    style={{ width: `${(state.reprocessProgress / state.reprocessTotal) * 100}%` }}
                  ></div>
                </div>
                <div className="progress-text">
                  Processing... {state.reprocessProgress} / {state.reprocessTotal} photos
                </div>
              </div>
            ) : (
              <div className="reprocess-actions">
                <button
                  className="admin-btn admin-btn-primary"
                  onClick={startReprocessing}
                  disabled={data.pendingPhotos === 0}
                >
                  {data.pendingPhotos > 0
                    ? `Reprocess ${data.pendingPhotos} Photos`
                    : 'All Photos Processed'
                  }
                </button>
                {state.lastReprocessTime && (
                  <div className="last-reprocess">
                    Last processed: {state.lastReprocessTime}
                  </div>
                )}
              </div>
            )}
          </div>
        </div>
      )}

      {state.reprocessErrors.length > 0 && (
        <div className="admin-card error-card">
          <div className="card-header">
            <div className="card-icon">⚠️</div>
            <h3>Processing Errors</h3>
          </div>
          <div className="card-content">
            <ul className="error-list">
              {state.reprocessErrors.map((error, index) => (
                <li key={index}>{error}</li>
              ))}
            </ul>
          </div>
        </div>
      )}


      <div className="admin-section">
        <h2>Photo Processing Information</h2>
        <div className="info-grid">
          <div className="info-card">
            <h4>Modern Formats</h4>
            <ul>
              <li><strong>AVIF:</strong> Next-generation format, up to 50% smaller</li>
              <li><strong>WebP:</strong> Widely supported, 25-35% smaller</li>
              <li><strong>JPEG:</strong> Universal fallback compatibility</li>
            </ul>
          </div>
          <div className="info-card">
            <h4>Responsive Sizes</h4>
            <ul>
              <li><strong>Small:</strong> 150px for mobile thumbnails</li>
              <li><strong>Medium:</strong> 600px for tablet displays</li>
              <li><strong>Large:</strong> 1200px for desktop viewing</li>
              <li><strong>Plus:</strong> Additional sizes for optimal loading</li>
            </ul>
          </div>
        </div>
      </div>
    </div>
  );
};
