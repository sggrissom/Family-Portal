import * as vlens from "vlens";
import * as server from "../server";

/** Processing status contract:
 *  0 = Done/Ready, 1 = Processing, 2 = Failed/Error (kept from your code)
 *  -1 = Unknown (client default before the first successful fetch)
 */
export enum Status {
  Unknown = -1,
  Done = 0,
  Processing = 1,
  Failed = 2,
}

type PhotoId = number;

interface PhotoMeta {
  retries: number;
  lastError?: string;
  lastCheckedAt?: number; // epoch ms
}

interface PhotoStatusState {
  statuses: Record<PhotoId, Status>;
  pollingPhotos: Set<PhotoId>;
  meta: Map<PhotoId, PhotoMeta>;
}

const createInitialState = (): PhotoStatusState => ({
  statuses: Object.create(null),
  pollingPhotos: new Set<PhotoId>(),
  meta: new Map<PhotoId, PhotoMeta>(),
});

// Global store (singleton)
const photoStatusState = vlens.declareHook((): PhotoStatusState => createInitialState());

const POLL_INTERVAL_MS = 2000;
const MAX_RETRIES_PER_PHOTO = 8;
const BACKOFF_BASE_MS = 1500; // exponential backoff base

let pollInterval: number | null = null;
let isPageHidden = typeof document !== "undefined" ? document.hidden : false;

// Visibility-aware polling
if (typeof document !== "undefined" && typeof document.addEventListener === "function") {
  document.addEventListener("visibilitychange", () => {
    isPageHidden = document.hidden;
    if (isPageHidden) {
      stopInterval();
    } else {
      ensureInterval();
    }
  });
}

function stopInterval() {
  if (pollInterval !== null) {
    clearInterval(pollInterval);
    pollInterval = null;
  }
}

function ensureInterval() {
  const state = photoStatusState();
  if (pollInterval === null && state.pollingPhotos.size > 0 && !isPageHidden) {
    pollInterval = setInterval(pollPhotoStatuses, POLL_INTERVAL_MS) as any;
  }
}

function shouldPollNow(id: PhotoId, meta: PhotoMeta | undefined): boolean {
  if (!meta) return true;
  if (meta.retries === 0) return true;

  // Exponential backoff: BACKOFF_BASE_MS * 2^(retries-1), capped by 20s
  const delay = Math.min(20000, BACKOFF_BASE_MS * (1 << Math.max(0, meta.retries - 1)));
  const now = Date.now();
  const nextAllowed = (meta.lastCheckedAt ?? 0) + delay;
  return now >= nextAllowed;
}

async function pollPhotoStatuses() {
  const state = photoStatusState();

  if (state.pollingPhotos.size === 0) {
    stopInterval();
    return;
  }

  // Respect visibility
  if (isPageHidden) {
    stopInterval();
    return;
  }

  // Filter photos that are due to poll (backoff aware)
  const due: PhotoId[] = [];
  for (const id of state.pollingPhotos) {
    const meta = state.meta.get(id);
    if (shouldPollNow(id, meta)) {
      due.push(id);
    }
  }

  if (due.length === 0) return;

  try {
    // Batch parallel requests (still per-id API, but not serialized)
    const results = await Promise.all(
      due.map(async (photoId) => {
        const startedAt = Date.now();
        try {
          const [response, error] = await server.GetPhotoStatus({ id: photoId });
          const ok = !error && response;
          return { photoId, response, error, startedAt };
        } catch (e: any) {
          return { photoId, response: null as any, error: e, startedAt };
        }
      })
    );

    let changed = false;

    for (const { photoId, response, error, startedAt } of results) {
      const meta = state.meta.get(photoId) ?? { retries: 0 };
      meta.lastCheckedAt = startedAt;

      if (error || !response) {
        meta.retries = Math.min(MAX_RETRIES_PER_PHOTO, meta.retries + 1);
        meta.lastError = String(error ?? "Unknown error");
        state.meta.set(photoId, meta);

        // Give up after too many failures
        if (meta.retries >= MAX_RETRIES_PER_PHOTO) {
          state.statuses[photoId] = Status.Failed;
          state.pollingPhotos.delete(photoId);
          changed = true;
        }

        continue;
      }

      // Successful response resets retries
      meta.retries = 0;
      meta.lastError = undefined;
      state.meta.set(photoId, meta);

      const prev = state.statuses[photoId] ?? Status.Unknown;
      const next: Status = Number(response.status);

      if (next !== prev) {
        state.statuses[photoId] = next;
        changed = true;
      }

      if (next === Status.Done || next === Status.Failed) {
        state.pollingPhotos.delete(photoId);
      }
    }

    // No more photos? stop interval
    if (state.pollingPhotos.size === 0) {
      stopInterval();
    }

    if (changed) {
      vlens.scheduleRedraw();
    }
  } catch (e) {
    // Catastrophic failure—don’t spam logs; keep interval alive for next tick
    console.warn("pollPhotoStatuses: batch error", e);
  }
}

export const usePhotoStatus = () => {
  const state = photoStatusState();

  return {
    /** Returns the status for a photo, defaulting to Unknown (-1) */
    getStatus(photoId: PhotoId): Status {
      return state.statuses[photoId] ?? Status.Unknown;
    },

    /** Begin monitoring a photo (idempotent). Defaults to Processing if not provided. */
    startMonitoring(photoId: PhotoId, initialStatus: Status = Status.Processing) {
      // If already terminal, don’t re-add to polling
      state.statuses[photoId] = initialStatus;

      // Initialize meta bucket
      if (!state.meta.has(photoId)) {
        state.meta.set(photoId, { retries: 0 });
      }

      if (initialStatus === Status.Processing) {
        state.pollingPhotos.add(photoId);
        ensureInterval();
      } else {
        // Terminal state: ensure it’s not in the polling set
        state.pollingPhotos.delete(photoId);
      }

      vlens.scheduleRedraw();
    },

    /** Stop monitoring and forget the status/meta. */
    stopMonitoring(photoId: PhotoId) {
      delete state.statuses[photoId];
      state.pollingPhotos.delete(photoId);
      state.meta.delete(photoId);

      if (state.pollingPhotos.size === 0) stopInterval();
      vlens.scheduleRedraw();
    },

    /** Manually update a status and adjust polling accordingly. */
    updateStatus(photoId: PhotoId, status: Status) {
      const prev = state.statuses[photoId];
      state.statuses[photoId] = status;

      if (status === Status.Processing) {
        state.pollingPhotos.add(photoId);
        ensureInterval();
      } else {
        state.pollingPhotos.delete(photoId);
      }

      if (prev !== status) vlens.scheduleRedraw();
    },

    /** Returns whether any photos are currently processing. */
    hasProcessingPhotos(): boolean {
      return state.pollingPhotos.size > 0;
    },

    /** Returns the ids of processing photos. */
    getProcessingPhotos(): PhotoId[] {
      return Array.from(state.pollingPhotos);
    },

    /** Returns whether a given photo id is being monitored. */
    isMonitoring(photoId: PhotoId): boolean {
      return state.pollingPhotos.has(photoId);
    },

    /** Optional: surface last error for a photo (useful for UI tooltips). */
    getLastError(photoId: PhotoId): string | undefined {
      return state.meta.get(photoId)?.lastError;
    },
  };
};

