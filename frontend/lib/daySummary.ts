import * as server from "../server";

export interface PhotoGroup {
  count: number;
  photoIds: number[];
  personIds: number[];
}

export interface Checkup {
  personId: number;
  height: server.GrowthData | null;
  weight: server.GrowthData | null;
}

export interface Birthday {
  personId: number;
  age: number;
}

export interface DaySummary {
  day: string;
  birthdays: Birthday[];
  milestones: server.Milestone[];
  appearances: server.AppearanceDetail[];
  checkups: Checkup[];
  photos: PhotoGroup | null;
}

export interface DayRecords {
  photos: server.PhotoWithPeople[];
  growth: server.GrowthData[];
  milestones: server.Milestone[];
  appearances?: server.AppearanceDetail[];
}

const MOSAIC_SIZE = 4;

export function dayKey(date: string): string {
  const parsed = new Date(date);
  return isNaN(parsed.getTime()) ? "" : parsed.toISOString().slice(0, 10);
}

function eachDay(from: string, to: string): string[] {
  const days: string[] = [];
  const cursor = new Date(from + "T00:00:00Z");
  const end = new Date(to + "T00:00:00Z");
  while (cursor <= end) {
    days.push(cursor.toISOString().slice(0, 10));
    cursor.setUTCDate(cursor.getUTCDate() + 1);
  }
  return days;
}

export function birthdaysBetween(
  people: server.Person[],
  from: string,
  to: string
): Map<string, Birthday[]> {
  const byDay = new Map<string, Birthday[]>();
  for (const day of eachDay(from, to)) {
    const [year, month, date] = day.split("-").map(Number);
    for (const person of people) {
      if (person.isPregnancy) continue;
      const born = new Date(person.birthday);
      if (isNaN(born.getTime())) continue;
      const age = year - born.getUTCFullYear();
      if (age <= 0 || born.getUTCMonth() + 1 !== month || born.getUTCDate() !== date) continue;
      const list = byDay.get(day) ?? [];
      list.push({ personId: person.id, age });
      byDay.set(day, list);
    }
  }
  return byDay;
}

export function summarizeDays(
  records: DayRecords,
  people: server.Person[],
  range?: { from: string; to: string }
): DaySummary[] {
  const days = new Map<string, DaySummary>();
  const summary = (day: string) => {
    let found = days.get(day);
    if (!found) {
      found = { day, birthdays: [], milestones: [], appearances: [], checkups: [], photos: null };
      days.set(day, found);
    }
    return found;
  };

  if (range) {
    for (const [day, birthdays] of birthdaysBetween(people, range.from, range.to)) {
      summary(day).birthdays = birthdays;
    }
  }

  for (const milestone of records.milestones) {
    const day = dayKey(milestone.milestoneDate);
    if (day) summary(day).milestones.push(milestone);
  }

  for (const detail of records.appearances ?? []) {
    const occurred = new Date(detail.appearance.occurredAt);
    const day = dayKey(
      occurred.getUTCFullYear() > 1900 ? detail.appearance.occurredAt : detail.event.startDate
    );
    if (day) summary(day).appearances.push(detail);
  }

  const newestFirst = (a: server.GrowthData, b: server.GrowthData) =>
    b.measurementDate.localeCompare(a.measurementDate) || b.id - a.id;
  for (const growth of [...records.growth].sort(newestFirst)) {
    const day = dayKey(growth.measurementDate);
    if (!day) continue;
    const checkups = summary(day).checkups;
    let checkup = checkups.find(c => c.personId === growth.personId);
    if (!checkup) {
      checkup = { personId: growth.personId, height: null, weight: null };
      checkups.push(checkup);
    }
    const slot = growth.measurementType === server.Height ? "height" : "weight";
    if (!checkup[slot]) checkup[slot] = growth;
  }

  const photosNewestFirst = [...records.photos].sort(
    (a, b) => b.image.photoDate.localeCompare(a.image.photoDate) || b.image.id - a.image.id
  );
  for (const { image, people: inPhoto } of photosNewestFirst) {
    const day = dayKey(image.photoDate);
    if (!day) continue;
    const entry = summary(day);
    entry.photos ??= { count: 0, photoIds: [], personIds: [] };
    entry.photos.count++;
    if (entry.photos.photoIds.length < MOSAIC_SIZE) entry.photos.photoIds.push(image.id);
    for (const person of inPhoto ?? []) {
      if (!entry.photos.personIds.includes(person.id)) entry.photos.personIds.push(person.id);
    }
  }

  return [...days.values()].sort((a, b) => b.day.localeCompare(a.day));
}

const WEEKDAYS = ["Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"];
const MONTHS = ["Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"];

export function dayLabel(day: string, today: string): string {
  const date = new Date(day + "T00:00:00Z");
  const diff = Math.round((new Date(today + "T00:00:00Z").getTime() - date.getTime()) / 86_400_000);
  if (diff === 0) return "Today";
  if (diff === 1) return "Yesterday";
  const label = `${WEEKDAYS[date.getUTCDay()]}, ${MONTHS[date.getUTCMonth()]} ${date.getUTCDate()}`;
  return date.getUTCFullYear() === new Date(today + "T00:00:00Z").getUTCFullYear()
    ? label
    : `${label}, ${date.getUTCFullYear()}`;
}
