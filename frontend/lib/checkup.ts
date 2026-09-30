import * as server from "../server";
import { lbOzToLbs, OZ_PER_LB, formatMeasurement, prefersLbOz, splitLbOz } from "./weightFormat";

export type HeightUnit = "in" | "ft-in" | "cm";
export type WeightUnit = "lb" | "lb-oz" | "kg";

export const HEIGHT_UNITS: { value: HeightUnit; label: string }[] = [
  { value: "in", label: "in" },
  { value: "ft-in", label: "ft/in" },
  { value: "cm", label: "cm" },
];

export const WEIGHT_UNITS: { value: WeightUnit; label: string }[] = [
  { value: "lb", label: "lb" },
  { value: "lb-oz", label: "lb/oz" },
];

export const EDIT_WEIGHT_UNITS: { value: WeightUnit; label: string }[] = [
  ...WEIGHT_UNITS,
  { value: "kg", label: "kg" },
];

export interface CheckupEntry {
  heightUnit: HeightUnit;
  height: string;
  feet: string;
  inches: string;
  weightUnit: WeightUnit;
  weight: string;
  pounds: string;
  ounces: string;
}

export interface UnitPrefs {
  height: Record<string, HeightUnit>;
  weight: Record<string, WeightUnit>;
  lastHeight: HeightUnit | null;
  lastWeight: WeightUnit | null;
}

export interface CheckupValues {
  height: server.CheckupValue | null;
  weight: server.CheckupValue | null;
}

export function newEntry(): CheckupEntry {
  return {
    heightUnit: "in",
    height: "",
    feet: "",
    inches: "",
    weightUnit: "lb",
    weight: "",
    pounds: "",
    ounces: "",
  };
}

export function entryForRecord(record: server.GrowthData): CheckupEntry {
  const entry = newEntry();
  const value = String(record.value);
  if (record.measurementType === server.Height) {
    entry.heightUnit = record.unit === "cm" ? "cm" : "in";
    entry.height = value;
  } else if (record.unit === "kg") {
    entry.weightUnit = "kg";
    entry.weight = value;
  } else if (prefersLbOz(record.value, record.unit)) {
    const { lb, oz } = splitLbOz(record.value);
    entry.weightUnit = "lb-oz";
    entry.pounds = String(lb);
    entry.ounces = String(oz);
  } else {
    entry.weight = value;
  }
  return entry;
}

export function newUnitPrefs(): UnitPrefs {
  return { height: {}, weight: {}, lastHeight: null, lastWeight: null };
}

export function latestOf(
  growth: server.GrowthData[],
  type: server.MeasurementType
): server.GrowthData | null {
  let latest: server.GrowthData | null = null;
  for (const g of growth) {
    if (g.measurementType !== type) continue;
    if (!latest || g.measurementDate > latest.measurementDate) latest = g;
  }
  return latest;
}

export function defaultHeightUnit(
  prefs: UnitPrefs,
  personId: number,
  personGrowth: server.GrowthData[]
): HeightUnit {
  const stored = prefs.height[personId];
  if (stored) return stored;
  const last = latestOf(personGrowth, server.Height);
  if (last) return last.unit === "cm" ? "cm" : "in";
  return prefs.lastHeight ?? "in";
}

export function defaultWeightUnit(
  prefs: UnitPrefs,
  personId: number,
  ageMonths: number | null
): WeightUnit {
  const stored = prefs.weight[personId];
  if (stored) return stored;
  if (ageMonths !== null && prefersLbOz(0, "lbs", ageMonths)) return "lb-oz";
  return prefs.lastWeight ?? "lb";
}

export function rememberUnits(
  prefs: UnitPrefs,
  personId: number,
  values: CheckupValues,
  entry: CheckupEntry
): UnitPrefs {
  const next: UnitPrefs = {
    height: { ...prefs.height },
    weight: { ...prefs.weight },
    lastHeight: prefs.lastHeight,
    lastWeight: prefs.lastWeight,
  };
  if (values.height) {
    next.height[personId] = entry.heightUnit;
    next.lastHeight = entry.heightUnit;
  }
  if (values.weight) {
    next.weight[personId] = entry.weightUnit;
    next.lastWeight = entry.weightUnit;
  }
  return next;
}

function blank(...values: string[]): boolean {
  return values.every(v => v.trim() === "");
}

// Plain decimals only: "12", "12.5", ".5". Anything else is NaN.
export function parseAmount(text: string): number {
  const t = text.trim();
  if (t === "") return 0;
  return /^(\d+\.?\d*|\.\d+)$/.test(t) ? Number(t) : NaN;
}

type Parsed = { value: server.CheckupValue | null; error: string };

export function heightValue(entry: CheckupEntry): Parsed {
  if (entry.heightUnit === "ft-in") {
    if (blank(entry.feet, entry.inches)) return { value: null, error: "" };
    const total = parseAmount(entry.feet) * 12 + parseAmount(entry.inches);
    if (!(total > 0)) return { value: null, error: "Enter a height in feet and inches" };
    return { value: { value: total, unit: "in" }, error: "" };
  }
  if (blank(entry.height)) return { value: null, error: "" };
  const value = parseAmount(entry.height);
  if (!(value > 0)) return { value: null, error: "Enter a height above zero" };
  return { value: { value, unit: entry.heightUnit }, error: "" };
}

export function weightValue(entry: CheckupEntry): Parsed {
  if (entry.weightUnit === "lb-oz") {
    if (blank(entry.pounds, entry.ounces)) return { value: null, error: "" };
    const pounds = parseAmount(entry.pounds);
    const ounces = parseAmount(entry.ounces);
    if (!(ounces < OZ_PER_LB && pounds + ounces > 0)) {
      return { value: null, error: "Enter a weight in pounds and ounces under 16" };
    }
    return { value: { value: lbOzToLbs(pounds, ounces), unit: "lbs" }, error: "" };
  }
  if (blank(entry.weight)) return { value: null, error: "" };
  const value = parseAmount(entry.weight);
  if (!(value > 0)) return { value: null, error: "Enter a weight above zero" };
  return { value: { value, unit: entry.weightUnit === "kg" ? "kg" : "lbs" }, error: "" };
}

export function checkupValues(entry: CheckupEntry): CheckupValues & { error: string } {
  const height = heightValue(entry);
  const weight = weightValue(entry);
  const error =
    height.error ||
    weight.error ||
    (!height.value && !weight.value ? "Enter a height, a weight, or both" : "");
  if (error) return { height: null, weight: null, error };
  return { height: height.value, weight: weight.value, error: "" };
}

export function timeAgo(date: string, now: Date): string {
  const days = Math.floor((now.getTime() - new Date(date).getTime()) / 86_400_000);
  if (days < 1) return "today";
  if (days === 1) return "yesterday";
  if (days < 14) return `${days} days ago`;
  if (days < 60) return `${Math.floor(days / 7)} weeks ago`;
  const months = Math.floor(days / 30.4375);
  if (months < 24) return `${months} months ago`;
  return `${Math.floor(months / 12)} years ago`;
}

export function describeLast(
  last: server.GrowthData | null,
  ageMonthsThen: number | null,
  now: Date
): string {
  if (!last) return "";
  return `last: ${formatMeasurement(last.value, last.unit, ageMonthsThen)}, ${timeAgo(last.measurementDate, now)}`;
}
