import * as preact from "preact";
import * as vlens from "vlens";
import * as server from "../../server";
import { formatRelativeTime } from "../../lib/dateUtils";

type VideoPanelState = {
  loaded: boolean;
  stats: server.VideoProcessingStats | null;
  error: string;
  retrying: boolean;
  result: string;
};

const useVideoPanelState = vlens.declareHook(
  (): VideoPanelState => ({ loaded: false, stats: null, error: "", retrying: false, result: "" })
);

async function load(state: VideoPanelState) {
  const [stats, error] = await server.GetVideoProcessingStats({});
  state.stats = stats;
  state.error = error;
  vlens.scheduleRedraw();
}

async function retryFailed(state: VideoPanelState) {
  if (
    !confirm(
      "Retry failed videos and convert existing HEVC videos for browser playback? Their originals must still be stored."
    )
  ) {
    return;
  }
  state.retrying = true;
  state.result = "";
  vlens.scheduleRedraw();
  const [result, error] = await server.RetryFailedVideos({});
  state.retrying = false;
  state.result = error
    ? error
    : result && result.queued > 0
      ? `Requeued ${result.queued} video${result.queued === 1 ? "" : "s"}.`
      : "No videos need reprocessing.";
  await load(state);
}

export const VideoPanel = () => {
  const state = useVideoPanelState();
  if (!state.loaded) {
    state.loaded = true;
    load(state);
  }
  const stats = state.stats;

  return (
    <div className="admin-section">
      <h2>Video Processing</h2>
      {state.error && <p className="error-message">{state.error}</p>}
      {stats && (
        <>
          <p>
            {stats.available
              ? stats.isRunning
                ? `Worker running · ${stats.queueLength} queued · ${stats.processed} ready, ${stats.rejected} rejected, ${stats.failed} failed since start`
                : "Worker stopped"
              : `Unavailable: ${stats.reason}`}
          </p>
          {stats.lastError && (
            <p className="last-reprocess">
              Last failure {formatRelativeTime(stats.lastErrorAt)}: {stats.lastError}
            </p>
          )}
          <button
            className="admin-btn admin-btn-secondary"
            onClick={() => retryFailed(state)}
            disabled={state.retrying || !stats.isRunning}
          >
            {state.retrying ? "Retrying…" : "Reprocess failed / HEVC videos"}
          </button>
          {state.result && <p className="last-reprocess">{state.result}</p>}
          {stats.recentAttempts.length > 0 && (
            <ul className="video-attempts">
              {stats.recentAttempts.map(a => (
                <li key={`${a.imageId}-${a.time}`}>
                  <a href={`/view-photo/${a.imageId}`}>#{a.imageId}</a> {a.outcome} in{" "}
                  {(a.durationMs / 1000).toFixed(1)}s{a.reason && ` — ${a.reason}`}
                </li>
              ))}
            </ul>
          )}
        </>
      )}
    </div>
  );
};
