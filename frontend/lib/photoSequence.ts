import * as server from "../server";

// The viewer only trusts the saved list when its route carries SEQUENCE_PARAM,
// so a photo opened from somewhere other than the grid can't pick up a stale one.

const STORAGE_KEY = "photoSequence";
export const SEQUENCE_PARAM = "seq";

export interface PhotoSequence {
  ids: number[];
  backRoute: string;
}

export interface SequencePosition {
  index: number;
  total: number;
  prevId: number;
  nextId: number;
}

export function saveSequence(sequence: PhotoSequence) {
  try {
    sessionStorage.setItem(STORAGE_KEY, JSON.stringify(sequence));
  } catch {}
}

export function loadSequence(): PhotoSequence | null {
  try {
    const raw = sessionStorage.getItem(STORAGE_KEY);
    return raw ? parseSequence(JSON.parse(raw)) : null;
  } catch {
    return null;
  }
}

function parseSequence(value: any): PhotoSequence | null {
  if (!value || !Array.isArray(value.ids) || typeof value.backRoute !== "string") {
    return null;
  }
  return {
    ids: value.ids.filter((id: unknown) => typeof id === "number"),
    backRoute: value.backRoute,
  };
}

export function sequencePosition(ids: number[], photoId: number): SequencePosition | null {
  const index = ids.indexOf(photoId);
  if (index === -1) return null;
  return {
    index,
    total: ids.length,
    prevId: index > 0 ? ids[index - 1] : 0,
    nextId: index < ids.length - 1 ? ids[index + 1] : 0,
  };
}

export const START_PARAM = "t";

export function viewPhotoRoute(photoId: number, inSequence: boolean, startMs = 0): string {
  const params = new URLSearchParams();
  if (inSequence) params.set(SEQUENCE_PARAM, "1");
  if (startMs > 0) params.set(START_PARAM, String(Math.floor(startMs / 1000)));
  const query = params.toString();
  return query ? `/view-photo/${photoId}?${query}` : `/view-photo/${photoId}`;
}

export function routeStartSeconds(route: string): number {
  const seconds = Number(new URLSearchParams(route.split("?")[1] ?? "").get(START_PARAM));
  return Number.isFinite(seconds) && seconds > 0 ? seconds : 0;
}

export function matchStartMs(photo: server.PhotoWithPeople): number {
  return photo.match?.offsetMs ?? 0;
}

export function routeHasSequence(route: string): boolean {
  const query = route.split("?")[1] ?? "";
  return new URLSearchParams(query).has(SEQUENCE_PARAM);
}

export type SwipeDirection = "prev" | "next" | null;

// A swipe has to be mostly horizontal and long enough that a scroll or a tap
// doesn't count.
export function swipeDirection(dx: number, dy: number, minDistance = 50): SwipeDirection {
  if (Math.abs(dx) < minDistance || Math.abs(dx) < Math.abs(dy) * 1.5) return null;
  return dx > 0 ? "prev" : "next";
}
