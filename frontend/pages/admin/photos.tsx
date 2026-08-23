import * as preact from "preact";
import * as vlens from "vlens";
import * as rpc from "vlens/rpc";
import * as auth from "../../lib/authCache";
import * as server from "../../server";
import { Header, Footer } from "../../layout";
import { ensureAuthInFetch, requireAuthInView } from "../../lib/authHelpers";
import { logWarn } from "../../lib/logger";
import "./admin-styles";

type PhotoManagementState = {
  isReprocessing: boolean;
  reprocessQueued: number;
  reprocessError: string;
  lastReprocessTime: string | null;
  processingStats: server.ProcessingStats | null;
  analysisStats: server.AnalysisWorkerStats | null;
  isReanalyzing: boolean;
  lastReanalysisTime: string | null;
  reanalysisError: string;
  isRequeueing: boolean;
  requeueResult: string;
  requeueError: string;
};

const usePhotoManagementState = vlens.declareHook(
  (): PhotoManagementState => ({
    isReprocessing: false,
    reprocessQueued: 0,
    reprocessError: "",
    lastReprocessTime: null,
    processingStats: null,
    analysisStats: null,
    isReanalyzing: false,
    lastReanalysisTime: null,
    reanalysisError: "",
    isRequeueing: false,
    requeueResult: "",
    requeueError: "",
  })
);

export async function fetch(route: string, prefix: string) {
  if (!(await ensureAuthInFetch())) {
    return rpc.ok<server.GetPhotoStatsResponse>({
      totalPhotos: 0,
      processedPhotos: 0,
      pendingPhotos: 0,
      analysisPending: 0,
      analysisAnalyzing: 0,
      analysisDone: 0,
      analysisFailed: 0,
      autoTaggedCount: 0,
      personsWithFace: 0,
    });
  }

  return server.GetPhotoStats({});
}

export function view(
  route: string,
  prefix: string,
  data: server.GetPhotoStatsResponse
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
            <a href="/admin" className="btn btn-primary">
              Return to Admin Dashboard
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

// The photo worker, in the shape the push page already uses: counters, the last
// error, and a short history with measured durations. Photo processing failure
// is the most common real problem this site has, and until now the worker
// reported a queue length and a boolean.
const WorkerPanel = ({
  stats,
  onRequeue,
  state,
}: {
  stats: server.ProcessingStats;
  onRequeue: () => void;
  state: PhotoManagementState;
}) => {
  return (
    <div className="admin-section">
      <h2>Processing Worker</h2>

      {!stats.isRunning && (
        <div className="admin-notice">
          <strong>The photo worker is not running.</strong> Nothing in the queue will be processed
          and new uploads will sit unprocessed until the app is restarted.
        </div>
      )}

      <div className="photo-stats-grid">
        <div className="stat-card">
          <div className="stat-icon">✅</div>
          <div className="stat-content">
            <h3>Processed</h3>
            <div className="stat-value">{stats.processed.toLocaleString()}</div>
            <div className="stat-label">Since this process started</div>
          </div>
        </div>

        <div className="stat-card">
          <div className="stat-icon">❌</div>
          <div className="stat-content">
            <h3>Failed</h3>
            <div className="stat-value">{stats.failed.toLocaleString()}</div>
            <div className="stat-label">Since this process started</div>
          </div>
        </div>

        <div className="stat-card">
          <div className="stat-icon">🕒</div>
          <div className="stat-content">
            <h3>Last Processed</h3>
            <div className="stat-value">{shortWhen(stats.lastProcessedAt)}</div>
            {/* A quiet worker and a stalled one look identical without this. */}
            <div className="stat-label">Most recent success</div>
          </div>
        </div>
      </div>

      {stats.lastError && (
        <div className="admin-notice">
          <strong>Last error</strong> ({shortWhen(stats.lastErrorAt)}): {stats.lastError}
        </div>
      )}

      {stats.recentAttempts.length > 0 && (
        <div className="admin-card">
          <div className="card-header">
            <div className="card-icon">📜</div>
            <h3>Recent Attempts</h3>
          </div>
          <div className="card-content">
            <ul className="error-list">
              {stats.recentAttempts.map(attempt => (
                <li key={`${attempt.imageId}-${attempt.time}`}>
                  {attempt.success ? "✅" : "❌"} Photo #{attempt.imageId}
                  {attempt.reprocess ? " (reprocess)" : ""} · {attempt.durationMs}ms ·{" "}
                  {shortWhen(attempt.time)}
                  {attempt.reason && ` · ${attempt.reason}`}
                </li>
              ))}
            </ul>
          </div>
        </div>
      )}

      <div className="admin-actions">
        <button
          className="admin-btn admin-btn-secondary"
          onClick={onRequeue}
          disabled={state.isRequeueing}
        >
          {state.isRequeueing ? "Requeuing…" : "Requeue stuck photos"}
        </button>
        {state.requeueResult && <span className="last-reprocess">{state.requeueResult}</span>}
        {state.requeueError && <span className="last-reprocess">{state.requeueError}</span>}
      </div>
    </div>
  );
};

// Short relative time. These are read at a glance next to a counter.
function shortWhen(timestamp: string): string {
  const then = new Date(timestamp).getTime();
  if (!Number.isFinite(then) || then <= 0) return "never";

  const seconds = Math.max(0, Math.round((Date.now() - then) / 1000));
  if (seconds < 60) return "just now";
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return `${minutes}m ago`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours}h ago`;
  return `${Math.floor(hours / 24)}d ago`;
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
      logWarn("admin", "Failed to load processing stats", err);
    }
  };

  const loadAnalysisStats = async () => {
    try {
      const [result, error] = await server.GetAnalysisStats({});
      if (result && !error) {
        state.analysisStats = result;
        vlens.scheduleRedraw();
      }
    } catch (err) {
      logWarn("admin", "Failed to load analysis stats", err);
    }
  };

  // Load stats initially and set up periodic refresh
  if (!state.processingStats) {
    loadProcessingStats();
    loadAnalysisStats();
    setInterval(() => {
      loadProcessingStats();
      loadAnalysisStats();
    }, 3000); // Poll every 3 seconds
  }

  const startReprocessing = async () => {
    const confirmed = confirm(
      `Queue ${data.pendingPhotos} photos for reprocessing into modern formats and optimized sizes? ` +
        "They are handled in the background; the queue count below shows progress."
    );

    if (!confirmed) return;

    state.isReprocessing = true;
    state.reprocessError = "";
    vlens.scheduleRedraw();

    try {
      const [result, error] = await server.ReprocessAllPhotos({});

      if (error) {
        logWarn("admin", "Reprocessing failed", error);
        // The server refuses outright when the worker is not running, rather
        // than reporting a queue nothing will read. Show what it said.
        state.reprocessError = error;
      } else if (result) {
        state.reprocessQueued = result.queued;
        state.lastReprocessTime = new Date().toLocaleString();
      }
    } catch (err) {
      state.reprocessError = "Failed to start reprocessing: " + String(err);
    }

    state.isReprocessing = false;
    vlens.scheduleRedraw();
  };

  const startReanalysis = async () => {
    const pendingCount = data.analysisPending + data.analysisFailed;
    const confirmed = confirm(
      `Queue ${pendingCount} photos for face analysis? This will analyze pending/failed photos. Continue?`
    );

    if (!confirmed) return;

    state.isReanalyzing = true;
    state.reanalysisError = "";
    vlens.scheduleRedraw();

    try {
      const [result, error] = await server.ReanalyzeAllPhotos({});

      if (error) {
        logWarn("admin", "Reanalysis failed", error);
        // The server refuses outright when the daemon is absent, rather than
        // reporting a queue that nothing will ever read. Show what it said.
        state.reanalysisError = error;
      } else if (result) {
        state.reanalysisError = "";
        state.lastReanalysisTime = new Date().toLocaleString();
        setTimeout(() => {
          window.location.reload();
        }, 1000);
      }
    } catch (err) {
      logWarn("admin", "Failed to start reanalysis", err);
    }

    state.isReanalyzing = false;
    vlens.scheduleRedraw();
  };

  // Rows stranded in Processing are the failure mode nothing retries: the queue
  // is in-memory, so the job that owned the row died with the process.
  const requeueStuck = async () => {
    state.isRequeueing = true;
    state.requeueError = "";
    state.requeueResult = "";
    vlens.scheduleRedraw();

    const [result, error] = await server.RequeueStuckPhotos({});
    state.isRequeueing = false;
    if (error) {
      logWarn("admin", "Requeue failed", error);
      state.requeueError = error;
    } else if (result) {
      state.requeueResult =
        result.queued > 0
          ? `Requeued ${result.queued} photo${result.queued === 1 ? "" : "s"}.`
          : "Nothing was stuck.";
    }
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
              {data.totalPhotos > 0
                ? Math.round((data.processedPhotos / data.totalPhotos) * 100)
                : 0}
              %
            </div>
            <div className="stat-label">Optimization complete</div>
          </div>
        </div>

        {state.processingStats && (
          <div className="stat-card">
            <div className="stat-icon">{state.processingStats.isRunning ? "🔄" : "⏸️"}</div>
            <div className="stat-content">
              <h3>Processing Queue</h3>
              <div className="stat-value">{state.processingStats.queueLength}</div>
              <div className="stat-label">
                {state.processingStats.isRunning ? "Photos in queue" : "Worker stopped"}
              </div>
            </div>
          </div>
        )}
      </div>

      <div className="admin-section">
        <h2>Face Analysis</h2>
        {state.analysisStats && !state.analysisStats.isRunning && (
          <div className="admin-notice">
            <strong>Face analysis is not running.</strong> The daemon was unreachable at startup, or
            this is a local build. The counts below are historical; nothing new is being analyzed,
            and photos keep every tag applied by hand.
          </div>
        )}
        {state.reanalysisError && <div className="admin-notice">{state.reanalysisError}</div>}
        <div className="photo-stats-grid">
          <div className="stat-card">
            <div className="stat-icon">⏳</div>
            <div className="stat-content">
              <h3>Pending</h3>
              <div className="stat-value">{data.analysisPending.toLocaleString()}</div>
              <div className="stat-label">Not yet analyzed</div>
            </div>
          </div>

          <div className="stat-card">
            <div className="stat-icon">🔍</div>
            <div className="stat-content">
              <h3>Analyzing</h3>
              <div className="stat-value">{data.analysisAnalyzing.toLocaleString()}</div>
              <div className="stat-label">In progress</div>
            </div>
          </div>

          <div className="stat-card">
            <div className="stat-icon">✅</div>
            <div className="stat-content">
              <h3>Done</h3>
              <div className="stat-value">{data.analysisDone.toLocaleString()}</div>
              <div className="stat-label">
                {data.totalPhotos > 0
                  ? Math.round((data.analysisDone / data.totalPhotos) * 100) + "% complete"
                  : "0% complete"}
              </div>
            </div>
          </div>

          <div className="stat-card">
            <div className="stat-icon">❌</div>
            <div className="stat-content">
              <h3>Failed</h3>
              <div className="stat-value">{data.analysisFailed.toLocaleString()}</div>
              <div className="stat-label">Analysis errors</div>
            </div>
          </div>

          <div className="stat-card">
            <div className="stat-icon">🏷️</div>
            <div className="stat-content">
              <h3>Auto-tags</h3>
              <div className="stat-value">{data.autoTaggedCount.toLocaleString()}</div>
              <div className="stat-label">Person appearances tagged</div>
            </div>
          </div>

          <div className="stat-card">
            <div className="stat-icon">👤</div>
            <div className="stat-content">
              <h3>Face Models</h3>
              <div className="stat-value">{data.personsWithFace.toLocaleString()}</div>
              <div className="stat-label">People with face model</div>
            </div>
          </div>

          {state.analysisStats && (
            <div className="stat-card">
              <div className="stat-icon">{state.analysisStats.isRunning ? "🟢" : "🔴"}</div>
              <div className="stat-content">
                <h3>Analysis Queue</h3>
                <div className="stat-value">{state.analysisStats.queueLength}</div>
                <div className="stat-label">
                  {state.analysisStats.isRunning ? "Worker running" : "Worker stopped"}
                </div>
              </div>
            </div>
          )}
        </div>
      </div>

      {data.analysisPending + data.analysisFailed > 0 && (
        <div className="admin-card reprocess-card">
          <div className="card-header">
            <div className="card-icon">🔍</div>
            <h3>Face Reanalysis</h3>
          </div>
          <div className="card-content">
            <p>
              {data.analysisPending} pending and {data.analysisFailed} failed photos have not been
              analyzed. Queue them for face recognition.
            </p>

            {state.isReanalyzing ? (
              <div className="reprocess-progress">
                <div className="progress-text">Queuing photos for analysis...</div>
              </div>
            ) : (
              <div className="reprocess-actions">
                <button
                  className="admin-btn admin-btn-primary"
                  onClick={startReanalysis}
                  disabled={state.isReanalyzing}
                >
                  Reanalyze {data.analysisPending + data.analysisFailed} Photos
                </button>
                {state.lastReanalysisTime && (
                  <div className="last-reprocess">Last queued: {state.lastReanalysisTime}</div>
                )}
              </div>
            )}
          </div>
        </div>
      )}

      {state.processingStats && (
        <WorkerPanel stats={state.processingStats} onRequeue={requeueStuck} state={state} />
      )}

      {needsReprocessing && (
        <div className="admin-card reprocess-card">
          <div className="card-header">
            <div className="card-icon">🔄</div>
            <h3>Photo Reprocessing</h3>
          </div>
          <div className="card-content">
            <p>
              Some photos need to be reprocessed with modern formats and optimized sizes. This will
              generate WebP, AVIF, and responsive variants for better performance.
            </p>

            {state.isReprocessing ? (
              <div className="reprocess-progress">
                <div className="progress-text">Queuing photos for reprocessing...</div>
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
                    : "All Photos Processed"}
                </button>
                {state.lastReprocessTime && (
                  <div className="last-reprocess">
                    Queued {state.reprocessQueued} photos at {state.lastReprocessTime}
                  </div>
                )}
              </div>
            )}
          </div>
        </div>
      )}

      {state.reprocessError && (
        <div className="admin-card error-card">
          <div className="card-header">
            <div className="card-icon">⚠️</div>
            <h3>Reprocessing Error</h3>
          </div>
          <div className="card-content">{state.reprocessError}</div>
        </div>
      )}

      <div className="admin-section">
        <h2>Photo Processing Information</h2>
        <div className="info-grid">
          <div className="info-card">
            <h4>Modern Formats</h4>
            <ul>
              <li>
                <strong>AVIF:</strong> Next-generation format, up to 50% smaller
              </li>
              <li>
                <strong>WebP:</strong> Widely supported, 25-35% smaller
              </li>
              <li>
                <strong>JPEG:</strong> Universal fallback compatibility
              </li>
            </ul>
          </div>
          <div className="info-card">
            <h4>Responsive Sizes</h4>
            <ul>
              <li>
                <strong>Small:</strong> 150px for mobile thumbnails
              </li>
              <li>
                <strong>Medium:</strong> 600px for tablet displays
              </li>
              <li>
                <strong>Large:</strong> 1200px for desktop viewing
              </li>
              <li>
                <strong>Plus:</strong> Additional sizes for optimal loading
              </li>
            </ul>
          </div>
        </div>
      </div>
    </div>
  );
};
