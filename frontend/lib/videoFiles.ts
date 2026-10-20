import { formatBytes } from "./formatBytes";

export const videoTypes = ["video/mp4", "video/quicktime", "video/x-m4v"];
const videoExtensions = [".mp4", ".mov", ".m4v"];

export function isVideoFile(file: { name: string; type: string }): boolean {
  if (file.type.startsWith("video/")) return true;
  const name = file.name.toLowerCase();
  return videoExtensions.some(ext => name.endsWith(ext));
}

export function videoFileProblem(
  file: { name: string; type: string; size: number },
  support: { enabled: boolean; reason: string; maxBytes: number }
): string {
  if (!support.enabled) {
    return `${file.name}: ${support.reason || "videos can't be added here"}`;
  }
  const name = file.name.toLowerCase();
  if (!videoTypes.includes(file.type) && !videoExtensions.some(ext => name.endsWith(ext))) {
    return `${file.name} is not an MP4 or MOV video`;
  }
  if (file.size > support.maxBytes) {
    return `${file.name} is over ${formatBytes(support.maxBytes)}`;
  }
  return "";
}

export function durationProblem(name: string, seconds: number | null, maxSeconds: number): string {
  if (seconds === null || !isFinite(seconds) || seconds <= maxSeconds + 1) return "";
  return `${name} is ${formatDuration(seconds * 1000)} long; videos can be at most ${Math.round(maxSeconds / 60)} minutes`;
}

export function formatDuration(ms: number): string {
  const total = Math.max(0, Math.round(ms / 1000));
  const hours = Math.floor(total / 3600);
  const minutes = Math.floor((total % 3600) / 60);
  const seconds = String(total % 60).padStart(2, "0");
  return hours > 0
    ? `${hours}:${String(minutes).padStart(2, "0")}:${seconds}`
    : `${minutes}:${seconds}`;
}
