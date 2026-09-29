import * as server from "../server";
import { DayRecords } from "./daySummary";
import { entryYear, isShownYear } from "./photoPages";

export type HistoryType = "milestones" | "measurements" | "photos" | "activities" | "birthdays";

export const HISTORY_TYPES: HistoryType[] = [
  "milestones",
  "measurements",
  "photos",
  "activities",
  "birthdays",
];

export interface HistoryFilters {
  personIds: number[];
  types: HistoryType[];
  tagIds: number[];
}

export interface HistoryView {
  records: DayRecords;
  birthdayPeople: server.Person[];
  range: { from: string; to: string } | null;
}

function appearanceDate(a: server.TimelineAppearance): string {
  const occurred = a.detail.appearance.occurredAt;
  return new Date(occurred).getUTCFullYear() > 1900 ? occurred : a.detail.event.startDate;
}

export function historyView(
  timeline: server.GetFamilyTimelineResponse,
  filters: HistoryFilters,
  stopYear: number | null,
  today: string
): HistoryView {
  const shown = (date: string) => isShownYear(entryYear(date), stopYear, "newest");
  const wantPerson = (id: number) =>
    filters.personIds.length === 0 || filters.personIds.includes(id);
  const wantPeople = (ids: number[]) =>
    filters.personIds.length === 0 || ids.some(id => filters.personIds.includes(id));
  const wantType = (type: HistoryType) => filters.types.includes(type);
  const tagged = (tagIds: number[] | null) =>
    filters.tagIds.length === 0 || (tagIds ?? []).some(id => filters.tagIds.includes(id));
  const tagsOnly = filters.tagIds.length > 0;

  const records: DayRecords = { photos: [], growth: [], milestones: [], appearances: [] };
  const photos = new Map<number, server.PhotoWithPeople>();

  for (const item of timeline.people ?? []) {
    const person = item.person;
    if (wantType("milestones")) {
      for (const m of item.milestones ?? []) {
        if (wantPerson(person.id) && shown(m.milestoneDate) && tagged(m.tagIds)) {
          records.milestones.push(m);
        }
      }
    }
    if (wantType("measurements") && !tagsOnly) {
      for (const g of item.growthData ?? []) {
        if (wantPerson(person.id) && shown(g.measurementDate)) records.growth.push(g);
      }
    }
    if (wantType("photos")) {
      for (const image of item.photos ?? []) {
        if (!shown(image.photoDate) || !tagged(image.tagIds)) continue;
        const entry = photos.get(image.id) ?? { image, people: [], similar: [] };
        entry.people.push(person);
        photos.set(image.id, entry);
      }
    }
  }
  records.photos = [...photos.values()].filter(p => wantPeople(p.people.map(person => person.id)));

  if (wantType("activities") && !tagsOnly) {
    records.appearances = (timeline.appearances ?? []).filter(
      a => shown(appearanceDate(a)) && wantPeople(a.personIds)
    );
  }

  const years = (timeline.years ?? []).filter(year => isShownYear(year, stopYear, "newest"));
  const showBirthdays = wantType("birthdays") && !tagsOnly && years.length > 0;
  return {
    records,
    birthdayPeople: showBirthdays
      ? (timeline.people ?? []).map(item => item.person).filter(p => wantPerson(p.id))
      : [],
    range: showBirthdays ? { from: `${Math.min(...years)}-01-01`, to: today } : null,
  };
}
