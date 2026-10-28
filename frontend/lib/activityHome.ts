import { toDateInputValue } from "./dateUtils";
import { eventStatus } from "./activityResults";

export type SeasonPhase = "upcoming" | "current" | "past";

export function seasonPhase(startDay: string, endDay: string, today: string): SeasonPhase {
  if (startDay && today < startDay) return "upcoming";
  if (endDay && today > endDay) return "past";
  return "current";
}

export interface HomeEventLike {
  event: { id: number; startDate: string; endDate: string };
  appearances: number;
}

export type EventFocus = "latest" | "needs-results" | "now" | "next";

export interface FocusedEvent<T> {
  item: T;
  focus: EventFocus;
}

export interface SeasonProgress<T> {
  focus: FocusedEvent<T>[];
  held: number;
  recorded: number;
}

export function seasonProgress<T extends HomeEventLike>(
  events: T[],
  hasEntries: boolean,
  today: string
): SeasonProgress<T> {
  const focus: FocusedEvent<T>[] = [];
  let held = 0;
  let recorded = 0;
  let latest: T | null = null;
  let next: T | null = null;

  for (const item of events) {
    const status = eventStatus(
      toDateInputValue(item.event.startDate),
      toDateInputValue(item.event.endDate),
      today
    );
    if (status === "now") {
      focus.push({ item, focus: "now" });
    } else if (status === "upcoming") {
      if (next === null) next = item;
    } else {
      held++;
      if (item.appearances > 0) {
        recorded++;
        latest = item;
      } else if (hasEntries) {
        focus.push({ item, focus: "needs-results" });
      }
    }
  }

  if (latest !== null) focus.unshift({ item: latest, focus: "latest" });
  if (next !== null) focus.push({ item: next, focus: "next" });
  focus.sort((a, b) => a.item.event.startDate.localeCompare(b.item.event.startDate));
  return { focus, held, recorded };
}
