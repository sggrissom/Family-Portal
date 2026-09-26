import * as server from "../server";

export const PHOTO_PAGE_SIZE = 60;

export function photosRequest(
  fields: Partial<server.ListFamilyPhotosRequest> = {}
): server.ListFamilyPhotosRequest {
  return {
    personId: 0,
    limit: 0,
    cursor: "",
    personIds: [],
    tagIds: [],
    dateFrom: "",
    dateTo: "",
    ...fields,
  };
}

export function timelineRequest(
  fields: Partial<server.GetFamilyTimelineRequest> = {}
): server.GetFamilyTimelineRequest {
  return { from: "", to: "", skipMilestones: false, skipPhotos: false, ...fields };
}

// Timeline entries are bucketed by UTC year on the server, so the client has to
// agree or an entry dated Jan 1 lands under the wrong banner.
export function entryYear(date: string): number {
  return new Date(date).getUTCFullYear();
}

export function yearRange(fromYear: number, toYear: number): server.GetFamilyTimelineRequest {
  return timelineRequest({ from: `${fromYear}-01-01`, to: `${toYear}-12-31` });
}

// Years are loaded in any order (a jump can skip ahead), but the page only
// shows an unbroken run from where the sort starts. The first year with data
// that isn't loaded yet is where that run stops.
export function firstUnloadedYear(
  years: number[],
  loaded: number[],
  order: "newest" | "oldest"
): number | null {
  const walk =
    order === "newest" ? [...years].sort((a, b) => b - a) : [...years].sort((a, b) => a - b);
  return walk.find(year => !loaded.includes(year)) ?? null;
}

export function isShownYear(
  year: number,
  stop: number | null,
  order: "newest" | "oldest"
): boolean {
  if (stop === null) return true;
  return order === "newest" ? year > stop : year < stop;
}

// The years a jump to `target` has to load so that the run from the sort's
// starting end reaches it without a gap.
export function yearsToReach(
  target: number,
  years: number[],
  loaded: number[],
  order: "newest" | "oldest"
): number[] {
  return years.filter(
    year => !loaded.includes(year) && (order === "newest" ? year >= target : year <= target)
  );
}

// Arrays can arrive as null (the caller can't see that scope), so both sides
// are treated as possibly missing.
const mergeById = <T extends { id: number }>(a: T[] | null, b: T[] | null): T[] => {
  if (!a) return b as T[];
  if (!b) return a;
  const ids = new Set(a.map(item => item.id));
  return [...a, ...b.filter(item => !ids.has(item.id))];
};

export function mergeTimeline(
  base: server.GetFamilyTimelineResponse,
  more: server.GetFamilyTimelineResponse
): server.GetFamilyTimelineResponse {
  const extra = new Map((more.people ?? []).map(item => [item.person.id, item]));
  return {
    ...base,
    people: (base.people ?? []).map(item => {
      const add = extra.get(item.person.id);
      if (!add) return item;
      return {
        ...item,
        milestones: mergeById(item.milestones, add.milestones),
        growthData: mergeById(item.growthData, add.growthData),
        photos: mergeById(item.photos, add.photos),
      };
    }),
  };
}
