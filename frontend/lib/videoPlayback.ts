export type PlaybackProblem = "expired" | "unsupported" | "network";

export const MEDIA_ERR_NETWORK = 2;

// What a player error most likely means. A link past its expiry, or one the
// server now refuses, is renewed; a link that still answers means the browser
// cannot play this file; anything else is the connection.
export function classifyPlaybackError(
  errorCode: number,
  probeStatus: number | null,
  now: number,
  expiresAt: number
): PlaybackProblem {
  if (now >= expiresAt - 60_000) return "expired";
  if (probeStatus === 401 || probeStatus === 403 || probeStatus === 400) return "expired";
  if (probeStatus !== null && probeStatus >= 200 && probeStatus < 300) {
    return errorCode === MEDIA_ERR_NETWORK ? "network" : "unsupported";
  }
  return "network";
}

export function canProbablyPlay(canPlayType: (type: string) => string, mimeType: string): boolean {
  return (
    canPlayType(mimeType) !== "" || (mimeType.includes("avc1") && canPlayType("video/mp4") !== "")
  );
}
