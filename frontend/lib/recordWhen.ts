import * as server from "../server";
import { civilString, parseCivil, recordTimeLabel, resolveRecordTime } from "./recordTime";
import { formatLongDate, localDateString } from "./dateUtils";

export type RecordWhenMode =
  | "today"
  | "yesterday"
  | "date"
  | "month"
  | "year"
  | "range"
  | "age"
  | "unknown";

export interface RecordWhen {
  mode: RecordWhenMode;
  date: string;
  month: string;
  year: string;
  from: string;
  to: string;
  ageYears: string;
  ageMonths: string;
  ageToYears: string;
  meaning: "" | "occurrence" | "period";
  estimated: boolean;
  around: string;
  anchorPersonId: number;
}

export function newRecordWhen(anchorPersonId = 0): RecordWhen {
  return {
    mode: "today",
    date: "",
    month: "",
    year: "",
    from: "",
    to: "",
    ageYears: "",
    ageMonths: "",
    ageToYears: "",
    meaning: "",
    estimated: false,
    around: "",
    anchorPersonId,
  };
}

export function recordWhenFrom(rt: server.RecordTime | undefined, date: string): RecordWhen {
  const w = newRecordWhen(rt?.anchorPersonId ?? 0);
  if (!rt || rt.basis === "") {
    return { ...w, mode: "date", date: date.slice(0, 10) };
  }
  w.estimated = rt.estimated;
  w.around = rt.around ?? "";
  w.meaning = rt.meaning === "period" ? "period" : rt.basis === "age" ? "occurrence" : "";
  if (rt.basis === "unknown") return { ...w, mode: "unknown" };
  if (rt.basis === "age") {
    w.mode = "age";
    w.ageYears = String(rt.ageYears);
    if (rt.precision === "month") w.ageMonths = String(rt.ageMonths);
    if (rt.precision === "range") w.ageToYears = String(rt.ageToYears);
    return w;
  }
  switch (rt.precision) {
    case "day":
      return { ...w, mode: "date", date: rt.from ?? "" };
    case "month":
      return { ...w, mode: "month", month: rt.from ?? "" };
    case "year":
      return { ...w, mode: "year", year: rt.from ?? "" };
  }
  return { ...w, mode: "range", from: rt.from ?? "", to: rt.to ?? "" };
}

function yesterday(now: Date): string {
  const d = new Date(now);
  d.setDate(d.getDate() - 1);
  return localDateString(d);
}

const isAge = (w: RecordWhen) => w.mode === "age";

const str = (value: unknown) => String(value ?? "").trim();

// Keep visible controls, validation, and serialization on the same rules.
export function recordWhenControls(w: RecordWhen) {
  const exactShortcut = w.mode === "today" || w.mode === "yesterday";
  const editable = !exactShortcut && w.mode !== "unknown";
  return {
    meaning: editable,
    estimate: editable,
    bestGuess: w.mode === "range" || (isAge(w) && str(w.ageToYears) !== ""),
  };
}

export function setRecordWhenMode(w: RecordWhen, mode: RecordWhenMode): void {
  if (w.mode === mode) return;
  w.mode = mode;
  // A central age and a central calendar date aren't interchangeable. Even
  // between calendar modes, don't silently reuse a guess for another window.
  w.around = "";
  const controls = recordWhenControls(w);
  if (!controls.meaning) w.meaning = "";
  if (!controls.estimate) w.estimated = false;
}

export function recordWhenTime(w: RecordWhen, now: Date, isMoment: boolean): server.RecordTime {
  const controls = recordWhenControls(w);
  const estimated = controls.estimate && w.estimated;
  const rt: server.RecordTime = {
    version: 1,
    meaning: isMoment && controls.meaning && w.meaning === "period" ? "period" : "occurrence",
    basis: "calendar",
    precision: "day",
    estimated,
    from: "",
    to: "",
    around: estimated && controls.bestGuess ? str(w.around) : "",
    anchorPersonId: 0,
    ageYears: 0,
    ageMonths: 0,
    ageToYears: 0,
    source: "family",
    note: "",
    changedBy: 0,
    changedAt: "0001-01-01T00:00:00Z",
    start: "",
    end: "",
    resolved: false,
    resolverVersion: 0,
  };
  switch (w.mode) {
    case "today":
      return { ...rt, from: localDateString(now) };
    case "yesterday":
      return { ...rt, from: yesterday(now) };
    case "date":
      return { ...rt, from: w.date };
    case "month":
      return { ...rt, precision: "month", from: w.month };
    case "year":
      return { ...rt, precision: "year", from: str(w.year) };
    case "range":
      return { ...rt, precision: "range", from: str(w.from), to: str(w.to) };
    case "unknown":
      return { ...rt, meaning: "occurrence", basis: "unknown", precision: "unknown", around: "" };
  }
  const months = str(w.ageMonths);
  const toYears = str(w.ageToYears);
  return {
    ...rt,
    basis: "age",
    precision: toYears ? "range" : months ? "month" : "year",
    anchorPersonId: w.anchorPersonId,
    ageYears: parseInt(str(w.ageYears)),
    ageMonths: months && !toYears ? parseInt(months) : 0,
    ageToYears: toYears ? parseInt(toYears) : 0,
  };
}

export function recordWhenProblem(
  w: RecordWhen,
  isMoment: boolean,
  birthday: string | undefined,
  now: Date
): string {
  const whole = (value: unknown) => /^\d+$/.test(str(value));
  switch (w.mode) {
    case "date":
      if (!w.date) return "Pick a date";
      break;
    case "month":
      if (parseCivil(w.month)?.precision !== "month") return "Pick a month";
      break;
    case "year":
      if (parseCivil(str(w.year))?.precision !== "year") return "Enter a year, like 1970";
      break;
    case "range": {
      const from = parseCivil(str(w.from));
      const to = parseCivil(str(w.to));
      if (!from || !to) return "Enter both ends, like 1968 and 1972";
      if (civilString(to.end) < civilString(from.end)) return "The range ends before it starts";
      break;
    }
    case "age":
      if (!whole(w.ageYears)) return "Enter an age in years";
      if (str(w.ageMonths) && str(w.ageToYears)) return "Use months or an age range, not both";
      if (str(w.ageMonths) && (!whole(w.ageMonths) || parseInt(str(w.ageMonths)) > 11)) {
        return "Months should be 0 to 11";
      }
      if (str(w.ageToYears) && (!whole(w.ageToYears) || +str(w.ageToYears) < +str(w.ageYears))) {
        return "The age range ends before it starts";
      }
      if (!w.anchorPersonId) return "Choose whose age this is";
      if (isMoment && !w.meaning) return "Choose whether it happened then or describes that age";
      break;
  }
  const rt = recordWhenTime(w, now, isMoment);
  if (rt.around) {
    const ok = isAge(w) ? whole(rt.around) : !!parseCivil(rt.around);
    if (!ok) return isAge(w) ? "The best guess should be an age in years" : "Check the best guess";
    if (!aroundInRange(rt)) {
      return isAge(w)
        ? "The best guess should fall inside the age range"
        : "The best guess should fall inside the range";
    }
  }
  if (w.mode === "unknown") return "";
  const range = resolveRecordTime(rt, birthday ?? null);
  if (range.resolved && range.start > localDateString(now)) {
    return isAge(w) ? "That age hasn't happened yet" : "That date hasn't happened yet";
  }
  return "";
}

function aroundInRange(rt: server.RecordTime): boolean {
  if (rt.basis === "age") {
    const around = parseInt(rt.around ?? "");
    const last = rt.precision === "range" ? rt.ageToYears : rt.ageYears;
    return around >= rt.ageYears && around <= last;
  }
  const around = parseCivil(rt.around);
  const from = parseCivil(rt.from);
  const to = rt.precision === "range" ? parseCivil(rt.to) : from;
  if (!around || !from || !to) return true;
  return (
    civilString(around.start) >= civilString(from.start) &&
    civilString(around.end) <= civilString(to.end)
  );
}

export function recordWhenPreview(
  w: RecordWhen,
  isMoment: boolean,
  birthday: string | undefined,
  now: Date
): { label: string; bounds: string } {
  if (
    recordWhenProblem({ ...w, meaning: "occurrence", anchorPersonId: 1 }, false, undefined, now)
  ) {
    return { label: "", bounds: "" };
  }
  const rt = recordWhenTime(w, now, isMoment);
  const label = recordTimeLabel(rt);
  const range = resolveRecordTime(rt, birthday ?? null);
  if (!range.resolved || (rt.precision === "day" && !rt.estimated)) return { label, bounds: "" };
  const last = new Date(range.end + "T00:00:00Z");
  last.setUTCDate(last.getUTCDate() - 1);
  const lastDay = last.toISOString().slice(0, 10);
  const span =
    range.start === lastDay
      ? formatLongDate(range.start)
      : `${formatLongDate(range.start)} – ${formatLongDate(lastDay)}`;
  return { label, bounds: rt.basis === "age" ? `Based on birthday: ${span}` : `Matches ${span}` };
}

export function isApproximateMode(w: RecordWhen): boolean {
  const controls = recordWhenControls(w);
  return (
    !["today", "yesterday", "date"].includes(w.mode) ||
    (controls.estimate && w.estimated) ||
    (controls.meaning && w.meaning === "period")
  );
}

// Photo suggestions only search around an exact day.
export function recordWhenLookup(w: RecordWhen, now: Date): server.RecordTime | null {
  if (isApproximateMode(w) || recordWhenProblem(w, false, undefined, now)) return null;
  return recordWhenTime(w, now, false);
}

// Measurements and interviews are dated by an exact day or an age, which the
// server resolves against the birthday. RecordWhenControl's exact mode offers
// only those.
export interface WhenRequest {
  inputType: "date" | "age";
  date: string | null;
  ageYears: number | null;
  ageMonths: number | null;
}

export function ageDate(birthday: string, w: RecordWhen): string | null {
  const years = parseInt(str(w.ageYears));
  const months = str(w.ageMonths) === "" ? 0 : parseInt(str(w.ageMonths));
  const born = new Date(birthday);
  if (isNaN(years) || isNaN(months) || isNaN(born.getTime()) || born.getUTCFullYear() < 1000) {
    return null;
  }
  const date = new Date(
    Date.UTC(born.getUTCFullYear() + years, born.getUTCMonth() + months, born.getUTCDate())
  );
  return date.toISOString().slice(0, 10);
}

export function exactWhenProblem(w: RecordWhen, birthday?: string): string {
  if (w.mode === "date" && !w.date) return "Pick a date";
  if (w.mode === "age") {
    const years = parseInt(str(w.ageYears));
    const months = str(w.ageMonths) === "" ? 0 : parseInt(str(w.ageMonths));
    if (isNaN(years) || years < 0) return "Enter an age in years";
    if (isNaN(months) || months < 0 || months > 11) return "Months should be 0 to 11";
    const date = birthday ? ageDate(birthday, w) : null;
    if (date && date > localDateString(new Date())) return "That age hasn't happened yet";
  }
  return "";
}

export function exactWhenRequest(w: RecordWhen, now: Date): WhenRequest {
  const day = (date: string): WhenRequest => ({
    inputType: "date",
    date,
    ageYears: null,
    ageMonths: null,
  });
  switch (w.mode) {
    case "today":
      return day(localDateString(now));
    case "yesterday":
      return day(yesterday(now));
    case "age":
      return {
        inputType: "age",
        date: null,
        ageYears: parseInt(str(w.ageYears)),
        ageMonths: str(w.ageMonths) === "" ? 0 : parseInt(str(w.ageMonths)),
      };
  }
  return day(w.date);
}
