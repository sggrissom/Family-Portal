import { RecordTime } from "../server";
import { monthsOld } from "./sameAge";

export interface RecordTimeFields {
  meaning: string;
  basis: string;
  precision: string;
  estimated: boolean;
  from?: string;
  to?: string;
  around?: string;
  ageYears: number;
  ageMonths: number;
  ageToYears: number;
}

export interface RecordRange {
  start: string;
  end: string;
  resolved: boolean;
}

interface Civil {
  year: number;
  month: number;
  day: number;
}

function daysIn(year: number, month: number): number {
  return new Date(Date.UTC(year, month, 0)).getUTCDate();
}

function addMonths(d: Civil, n: number): Civil {
  const total = d.year * 12 + (d.month - 1) + n;
  const year = Math.floor(total / 12);
  const month = (total % 12) + 1;
  return { year, month, day: Math.min(d.day, daysIn(year, month)) };
}

function addDays(d: Civil, n: number): Civil {
  const t = new Date(Date.UTC(d.year, d.month - 1, d.day + n));
  return { year: t.getUTCFullYear(), month: t.getUTCMonth() + 1, day: t.getUTCDate() };
}

function civilString(d: Civil): string {
  const pad = (n: number, w: number) => String(n).padStart(w, "0");
  return `${pad(d.year, 4)}-${pad(d.month, 2)}-${pad(d.day, 2)}`;
}

interface ParsedCivil {
  start: Civil;
  end: Civil;
  precision: "day" | "month" | "year";
}

export function parseCivil(value: string | undefined): ParsedCivil | null {
  const match = /^(\d{4})(?:-(\d{2}))?(?:-(\d{2}))?$/.exec(value ?? "");
  if (!match || (match[3] && !match[2])) return null;
  const year = Number(match[1]);
  if (year < 1) return null;
  if (!match[2]) {
    const start = { year, month: 1, day: 1 };
    return { start, end: addMonths(start, 12), precision: "year" };
  }
  const month = Number(match[2]);
  if (month < 1 || month > 12) return null;
  if (!match[3]) {
    const start = { year, month, day: 1 };
    return { start, end: addMonths(start, 1), precision: "month" };
  }
  const day = Number(match[3]);
  if (day < 1 || day > daysIn(year, month)) return null;
  const start = { year, month, day };
  return { start, end: addDays(start, 1), precision: "day" };
}

export function resolveRecordTime(rt: RecordTimeFields, birthday: string | null): RecordRange {
  const unresolved = { start: "", end: "", resolved: false };
  let start: Civil;
  let end: Civil;
  if (rt.basis === "calendar") {
    const from = parseCivil(rt.from);
    const to = rt.precision === "range" ? parseCivil(rt.to) : from;
    if (!from || !to) return unresolved;
    start = from.start;
    end = to.end;
  } else if (rt.basis === "age") {
    const born = birthday ? parseCivil(birthday.slice(0, 10)) : null;
    if (!born) return unresolved;
    const b = born.start;
    if (rt.precision === "month") {
      const months = rt.ageYears * 12 + rt.ageMonths;
      start = addMonths(b, months);
      end = addMonths(b, months + 1);
    } else {
      const last = rt.precision === "range" ? rt.ageToYears : rt.ageYears;
      start = addMonths(b, rt.ageYears * 12);
      end = addMonths(b, (last + 1) * 12);
    }
  } else {
    return unresolved;
  }
  return { start: civilString(start), end: civilString(end), resolved: true };
}

export function isExactDay(rt: RecordTimeFields): boolean {
  return (
    rt.meaning === "occurrence" &&
    rt.basis === "calendar" &&
    rt.precision === "day" &&
    !rt.estimated
  );
}

export function overlaps(range: RecordRange, start: string, end: string): boolean {
  return range.resolved && range.start < end && range.end > start;
}

const monthNames = [
  "January",
  "February",
  "March",
  "April",
  "May",
  "June",
  "July",
  "August",
  "September",
  "October",
  "November",
  "December",
];

function civilLabel(value: string | undefined): string {
  const parsed = parseCivil(value);
  if (!parsed) return value ?? "";
  const { year, month, day } = parsed.start;
  switch (parsed.precision) {
    case "year":
      return String(year);
    case "month":
      return `${monthNames[month - 1]} ${year}`;
  }
  return `${monthNames[month - 1].slice(0, 3)} ${day}, ${year}`;
}

function joinRange(from: string, to: string): string {
  return from.includes(" ") || to.includes(" ") ? `${from} – ${to}` : `${from}–${to}`;
}

function plural(n: number, unit: string): string {
  return n === 1 ? `1 ${unit}` : `${n} ${unit}s`;
}

export function recordTimeLabel(rt: RecordTimeFields): string {
  if (rt.basis === "calendar") {
    let base = civilLabel(rt.from);
    if (rt.precision === "range") base = joinRange(base, civilLabel(rt.to));
    if (!rt.estimated) return base;
    if (rt.around) return `Around ${civilLabel(rt.around)} (${base})`;
    return `Around ${base}`;
  }
  if (rt.basis === "age") {
    let age = "";
    if (rt.precision === "year") age = `age ${rt.ageYears}`;
    else if (rt.precision === "month")
      age = `age ${plural(rt.ageYears, "year")}, ${plural(rt.ageMonths, "month")}`;
    else if (rt.precision === "range") age = `ages ${rt.ageYears}–${rt.ageToYears}`;
    if (rt.estimated) return rt.around ? `Around age ${rt.around} (${age})` : `Around ${age}`;
    return rt.meaning === "period" ? `At ${age}` : `Sometime at ${age}`;
  }
  return "Date unknown";
}

export function exactRecordTime(date: string, source = "family"): RecordTime {
  const from = date.slice(0, 10);
  const range = resolveRecordTime(
    {
      meaning: "occurrence",
      basis: "calendar",
      precision: "day",
      estimated: false,
      from,
      ageYears: 0,
      ageMonths: 0,
      ageToYears: 0,
    },
    null
  );
  return {
    version: 1,
    meaning: "occurrence",
    basis: "calendar",
    precision: "day",
    estimated: false,
    from,
    to: "",
    around: "",
    anchorPersonId: 0,
    ageYears: 0,
    ageMonths: 0,
    ageToYears: 0,
    source,
    note: "",
    changedBy: 0,
    changedAt: "",
    ...range,
    resolverVersion: 1,
  };
}

export function isExactRecord(rt: RecordTime | undefined | null): boolean {
  return !rt || isExactDay(rt);
}

export function recordDateLabel(
  rt: RecordTime | undefined | null,
  date: string,
  formatExact: (date: string) => string
): string {
  return isExactRecord(rt) ? formatExact(date) : recordTimeLabel(rt!);
}

export function recordGroupLabel(rt: RecordTime): string {
  const label = recordTimeLabel(rt);
  if (rt.basis !== "calendar" || rt.estimated || rt.meaning === "period") return label;
  return rt.precision === "range" ? `Sometime ${label}` : `Sometime in ${label}`;
}

export function recordMonthsOld(
  rt: RecordTime | undefined | null,
  date: string,
  birthday: string
): number {
  return isExactRecord(rt) ? monthsOld(birthday, date) : -1;
}
