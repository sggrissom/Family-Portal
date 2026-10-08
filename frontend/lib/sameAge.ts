import type { SameAgeRow } from "../server";

export function ageStep(ageMonths: number): number {
  if (ageMonths < 24) return 1;
  if (ageMonths < 72) return 3;
  return 6;
}

export function onAgeGrid(ageMonths: number): boolean {
  return ageMonths % ageStep(ageMonths) === 0;
}

export function nextAge(ageMonths: number, maxMonths: number): number {
  let age = ageMonths + 1;
  while (!onAgeGrid(age)) age++;
  return Math.min(age, Math.max(maxMonths, ageMonths));
}

export function prevAge(ageMonths: number): number {
  let age = ageMonths - 1;
  while (age > 0 && !onAgeGrid(age)) age--;
  return Math.max(age, 0);
}

export function parseAgeParam(value: string | null): number | null {
  if (!value) return null;
  const match = /^(?:(\d+)y)?(?:(\d+)m?)?$/.exec(value.trim());
  if (!match || (!match[1] && !match[2])) return null;
  return parseInt(match[1] ?? "0") * 12 + parseInt(match[2] ?? "0");
}

export type SameAgeView = "portraits" | "details";

export function parseViewParam(value: string | null): SameAgeView {
  return value === "details" ? "details" : "portraits";
}

export function sameAgePath(
  ageMonths: number | null,
  fromPersonId: number,
  view: SameAgeView = "portraits"
): string {
  const params = new URLSearchParams();
  if (ageMonths !== null && ageMonths >= 0) params.set("age", `${ageMonths}m`);
  if (fromPersonId > 0) params.set("from", String(fromPersonId));
  if (view === "details") params.set("view", view);
  const query = params.toString();
  return query ? `/same-age?${query}` : "/same-age";
}

export const AGE_SHORTCUTS = [0, 3, 6, 12];
export const NEWBORN_DAYS = 28;

export function ageTitle(ageMonths: number): string {
  if (ageMonths <= 0) return "Newborn";
  const years = Math.floor(ageMonths / 12);
  const months = ageMonths % 12;
  const y = years === 1 ? "1 year" : `${years} years`;
  const m = months === 1 ? "1 month" : `${months} months`;
  if (years === 0) return m;
  return months === 0 ? y : `${y} ${m}`;
}

export function monthsOld(birthday: string, at: string): number {
  const born = new Date(birthday);
  const then = new Date(at);
  let months =
    (then.getUTCFullYear() - born.getUTCFullYear()) * 12 + then.getUTCMonth() - born.getUTCMonth();
  if (then.getUTCDate() < born.getUTCDate()) months--;
  return months;
}

const DAY_MS = 24 * 60 * 60 * 1000;

const utcDay = (date: Date) =>
  Date.UTC(date.getUTCFullYear(), date.getUTCMonth(), date.getUTCDate());

// photoAge is how old someone actually was in a photo, to the day under two.
export function photoAge(birthday: string, at: string): string {
  const months = monthsOld(birthday, at);
  if (months < 0) return "";
  if (months >= 24) return ageTitle(months);
  const born = new Date(birthday);
  const anchor = Date.UTC(born.getUTCFullYear(), born.getUTCMonth() + months, born.getUTCDate());
  const days = Math.max(0, Math.round((utcDay(new Date(at)) - anchor) / DAY_MS));
  const d = days === 1 ? "1 day" : `${days} days`;
  if (days === 0) return ageTitle(months);
  return months === 0 ? d : `${ageTitle(months)}, ${d}`;
}

export function ageHeading(ageMonths: number): string {
  return ageMonths <= 0 ? "Everyone as newborns" : `Everyone at ${ageTitle(ageMonths)}`;
}

// Newborn through day 27, otherwise the nearest age on the age grid.
export function sameAgeForPhoto(birthday: string, at: string): number | null {
  const born = utcDay(new Date(birthday));
  const days = Math.round((utcDay(new Date(at)) - born) / DAY_MS);
  if (isNaN(days) || days < 0) return null;
  if (days < NEWBORN_DAYS) return 0;
  const months = monthsOld(birthday, at);
  const b = new Date(birthday);
  const distance = (m: number) =>
    Math.abs(
      utcDay(new Date(at)) - Date.UTC(b.getUTCFullYear(), b.getUTCMonth() + m, b.getUTCDate())
    );
  const before = Math.max(1, prevAge(months + 1));
  const after = nextAge(months, Infinity);
  return distance(after) < distance(before) ? after : before;
}

export function portraitOrder(rows: SameAgeRow[]): SameAgeRow[] {
  return [...rows].sort(
    (a, b) => a.person.birthday.localeCompare(b.person.birthday) || a.person.id - b.person.id
  );
}

// Shared by the page and the compact profile strip.
export function hasSameAgeRecords(row: SameAgeRow): boolean {
  return (
    !!row.height ||
    !!row.weight ||
    (row.milestones ?? []).length > 0 ||
    (row.photoIds ?? []).length > 0 ||
    (row.portraits ?? []).length > 0
  );
}

export function nearbyRecordedAge(
  ages: number[],
  current: number,
  direction: -1 | 1
): number | null {
  const matches = ages.filter(age => (direction < 0 ? age < current : age > current));
  if (!matches.length) return null;
  return direction < 0 ? Math.max(...matches) : Math.min(...matches);
}
