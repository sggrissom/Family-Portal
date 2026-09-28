import * as server from "../server";
import { DaySummary } from "./daySummary";
import { toDisplay } from "./ageChart";
import { monthsOld } from "./sameAge";

export interface Chapter {
  age: number | null;
  days: DaySummary[];
  grew: string;
}

function round(value: number, places: number): string {
  const f = Math.pow(10, places);
  return String(Math.round(value * f) / f);
}

function change(records: server.GrowthData[]): number | null {
  if (records.length < 2) return null;
  const sorted = [...records].sort((a, b) => a.measurementDate.localeCompare(b.measurementDate));
  const first = sorted[0];
  const last = sorted[sorted.length - 1];
  return toDisplay(last.value, last.unit) - toDisplay(first.value, first.unit);
}

export function grewLine(days: DaySummary[]): string {
  const heights = days
    .flatMap(d => d.checkups.map(c => c.height))
    .filter(Boolean) as server.GrowthData[];
  const weights = days
    .flatMap(d => d.checkups.map(c => c.weight))
    .filter(Boolean) as server.GrowthData[];
  const parts: string[] = [];
  const h = change(heights);
  if (h !== null && h >= 0.25) parts.push(`${round(h, 1)} in`);
  const w = change(weights);
  if (w !== null && w >= 0.5) parts.push(`${round(w, w < 10 ? 1 : 0)} lb`);
  return parts.length ? `Grew ${parts.join(" and ")}` : "";
}

export function storyChapters(days: DaySummary[], birthday: string): Chapter[] {
  const chapters: Chapter[] = [];
  for (const day of days) {
    const months = monthsOld(birthday, day.day + "T00:00:00Z");
    const age = months < 0 ? null : Math.floor(months / 12);
    let chapter = chapters[chapters.length - 1];
    if (!chapter || chapter.age !== age) {
      chapter = { age, days: [], grew: "" };
      chapters.push(chapter);
    }
    chapter.days.push(day);
  }
  for (const chapter of chapters) chapter.grew = grewLine(chapter.days);
  return chapters;
}

export function chapterTitle(age: number | null): string {
  if (age === null) return "Before birth";
  if (age === 0) return "First year";
  return `Age ${age}`;
}
