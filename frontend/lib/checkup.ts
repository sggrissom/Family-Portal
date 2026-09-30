import * as server from "../server";
import { lbOzToLbs, OZ_PER_LB, formatMeasurement, prefersLbOz } from "./weightFormat";

export type HeightUnit = "in" | "ft-in" | "cm";
export type WeightUnit = "lb" | "lb-oz";

export const HEIGHT_UNITS: { value: HeightUnit; label: string }[] = [
  { value: "in", label: "in" },
  { value: "ft-in", label: "ft/in" },
  { value: "cm", label: "cm" },
];

export const WEIGHT_UNITS: { value: WeightUnit; label: string }[] = [
  { value: "lb", label: "lb" },
  { value: "lb-oz", label: "lb/oz" },
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

function number(value: string): number {
  return value.trim() === "" ? 0 : Number(value);
}

export function checkupValues(entry: CheckupEntry): CheckupValues & { error: string } {
  const values: CheckupValues = { height: null, weight: null };
  const fail = (error: string) => ({ height: null, weight: null, error });

  if (entry.heightUnit === "ft-in") {
    if (!blank(entry.feet, entry.inches)) {
      const feet = number(entry.feet);
      const inches = number(entry.inches);
      const total = feet * 12 + inches;
      if (!(feet >= 0 && inches >= 0 && total > 0))
        return fail("Enter a height in feet and inches");
      values.height = { value: total, unit: "in" };
    }
  } else if (!blank(entry.height)) {
    const value = number(entry.height);
    if (!(value > 0)) return fail("Enter a height above zero");
    values.height = { value, unit: entry.heightUnit };
  }

  if (entry.weightUnit === "lb-oz") {
    if (!blank(entry.pounds, entry.ounces)) {
      const pounds = number(entry.pounds);
      const ounces = number(entry.ounces);
      if (!(pounds >= 0 && ounces >= 0 && ounces < OZ_PER_LB && pounds + ounces > 0)) {
        return fail("Enter a weight in pounds and ounces under 16");
      }
      values.weight = { value: lbOzToLbs(pounds, ounces), unit: "lbs" };
    }
  } else if (!blank(entry.weight)) {
    const value = number(entry.weight);
    if (!(value > 0)) return fail("Enter a weight above zero");
    values.weight = { value, unit: "lbs" };
  }

  if (!values.height && !values.weight) return fail("Enter a height, a weight, or both");
  return { ...values, error: "" };
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
