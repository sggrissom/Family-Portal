export type SaveState = "pending" | "saving" | "saved" | "skipped" | "started" | "failed";

export function linkIsFresh(expiresAt: string, now: number): boolean {
  const expires = new Date(expiresAt).getTime();
  return isFinite(expires) && now < expires - 60_000;
}

export function alreadySaved(existingSize: number | null, expectedSize: number): boolean {
  return existingSize !== null && existingSize === expectedSize;
}

export function saveSummary(states: SaveState[]): string {
  const total = states.length;
  const saved = states.filter(s => s === "saved" || s === "skipped").length;
  const started = states.filter(s => s === "started").length;
  const failed = states.filter(s => s === "failed").length;
  const noun = total === 1 ? "video" : "videos";
  if (failed > 0) return `${saved + started} of ${total} done; ${failed} failed. Retry them below.`;
  if (saved === total) return `All ${total} ${noun} saved.`;
  if (saved + started === total) {
    return `All ${total} ${noun} handed to your browser. Check its downloads to see them finish.`;
  }
  return `${saved + started} of ${total} done`;
}
