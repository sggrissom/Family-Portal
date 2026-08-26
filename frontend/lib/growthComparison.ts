import * as server from "../server";
import { ageInMonths, formatAgeAtMeasurement, isValidBirthday } from "./growthPercentiles";

export interface ComparisonPoint {
  ageLabel: string;
  value: number;
  unit: string;
  date: string;
  valueDiff: number;
  ageDiffMonths: number;
}

export interface FamilyComparisonEntry {
  person: server.Person;
  atSameAge: ComparisonPoint | null;
  atSameValue: ComparisonPoint | null;
}

function toMetric(value: number, unit: string): number {
  if (unit === "in") return value * 2.54;
  if (unit === "lbs") return value * 0.453592;
  return value;
}

function fromMetric(valueMetric: number, unit: string): number {
  if (unit === "in") return valueMetric / 2.54;
  if (unit === "lbs") return valueMetric / 0.453592;
  return valueMetric;
}

interface AgedRecord {
  ageMonths: number;
  record: server.GrowthData;
}

function nearestByAge(records: AgedRecord[], targetAgeMonths: number): AgedRecord | null {
  let best: AgedRecord | null = null;
  let bestDiff = Infinity;
  for (const r of records) {
    const diff = Math.abs(r.ageMonths - targetAgeMonths);
    if (diff < bestDiff) {
      bestDiff = diff;
      best = r;
    }
  }
  return best;
}

function nearestByValue(records: AgedRecord[], targetValueMetric: number): AgedRecord | null {
  let best: AgedRecord | null = null;
  let bestDiff = Infinity;
  for (const r of records) {
    const diff = Math.abs(toMetric(r.record.value, r.record.unit) - targetValueMetric);
    if (diff < bestDiff) {
      bestDiff = diff;
      best = r;
    }
  }
  return best;
}

const AGE_TOLERANCE_RATIO = 0.05;
const MIN_AGE_TOLERANCE_MONTHS = 0.5;
const VALUE_TOLERANCE_RATIO = 0.02;

function isAgeMatchRelevant(match: AgedRecord, targetAgeMonths: number): boolean {
  const tolerance = Math.max(MIN_AGE_TOLERANCE_MONTHS, targetAgeMonths * AGE_TOLERANCE_RATIO);
  return Math.abs(match.ageMonths - targetAgeMonths) <= tolerance;
}

function isValueMatchRelevant(match: AgedRecord, targetValueMetric: number): boolean {
  const matchValueMetric = toMetric(match.record.value, match.record.unit);
  const tolerance = targetValueMetric * VALUE_TOLERANCE_RATIO;
  return Math.abs(matchValueMetric - targetValueMetric) <= tolerance;
}

function toPoint(
  r: AgedRecord,
  targetValueMetric: number,
  targetAgeMonths: number
): ComparisonPoint {
  const targetValueInPointUnit = fromMetric(targetValueMetric, r.record.unit);
  return {
    ageLabel: formatAgeAtMeasurement(r.ageMonths),
    value: r.record.value,
    unit: r.record.unit,
    date: r.record.measurementDate,
    valueDiff: targetValueInPointUnit - r.record.value,
    ageDiffMonths: r.ageMonths - targetAgeMonths,
  };
}

function roundTo1Decimal(n: number): number {
  return Math.round(n * 10) / 10;
}

function formatValueDiff(diff: number, unit: string): string {
  const rounded = roundTo1Decimal(Math.abs(diff));
  const formatted = Number.isInteger(rounded) ? rounded.toString() : rounded.toFixed(1);
  return `${formatted} ${unit}`;
}

function formatDurationMonths(absMonths: number): string {
  const totalMonths = Math.round(absMonths);
  if (totalMonths < 12) return totalMonths === 1 ? "1 mo" : `${totalMonths} mo`;
  const yrs = Math.floor(totalMonths / 12);
  const mos = totalMonths % 12;
  if (mos === 0) return yrs === 1 ? "1 yr" : `${yrs} yr`;
  return `${yrs} yr ${mos} mo`;
}

export function describeValueComparison(
  point: ComparisonPoint,
  measurementType: server.MeasurementType
): string {
  const noun = measurementType === server.Height ? "height" : "weight";
  if (roundTo1Decimal(Math.abs(point.valueDiff)) === 0) {
    return `you were about the same ${noun} at this age`;
  }
  const magnitude = formatValueDiff(point.valueDiff, point.unit);
  const comparative =
    measurementType === server.Height
      ? point.valueDiff > 0
        ? "taller"
        : "shorter"
      : point.valueDiff > 0
        ? "heavier"
        : "lighter";
  return `you were ${magnitude} ${comparative} at this age`;
}

export function describeAgeComparison(point: ComparisonPoint): string {
  if (Math.round(Math.abs(point.ageDiffMonths)) < 1) return "at about the same age as you";
  const duration = formatDurationMonths(Math.abs(point.ageDiffMonths));
  return point.ageDiffMonths > 0 ? `${duration} after you` : `${duration} before you`;
}

export function computeFamilyComparisons(
  targetRecord: server.GrowthData,
  targetPerson: server.Person,
  familyMembers: { person: server.Person; growthData: server.GrowthData[] }[]
): FamilyComparisonEntry[] {
  if (!isValidBirthday(targetPerson.birthday)) return [];

  const targetAgeMonths = ageInMonths(targetPerson.birthday, targetRecord.measurementDate);
  const targetValueMetric = toMetric(targetRecord.value, targetRecord.unit);

  const entries: FamilyComparisonEntry[] = [];

  for (const member of familyMembers) {
    if (member.person.id === targetPerson.id) continue;
    if (member.person.isPregnancy) continue;
    if (!isValidBirthday(member.person.birthday)) continue;

    const records: AgedRecord[] = (member.growthData || [])
      .filter(g => g.measurementType === targetRecord.measurementType)
      .map(record => ({
        ageMonths: ageInMonths(member.person.birthday, record.measurementDate),
        record,
      }));

    if (records.length === 0) {
      entries.push({ person: member.person, atSameAge: null, atSameValue: null });
      continue;
    }

    const ageMatch = nearestByAge(records, targetAgeMonths);
    const valueMatch = nearestByValue(records, targetValueMetric);

    const relevantAgeMatch =
      ageMatch && isAgeMatchRelevant(ageMatch, targetAgeMonths) ? ageMatch : null;
    const relevantValueMatch =
      valueMatch && isValueMatchRelevant(valueMatch, targetValueMetric) ? valueMatch : null;

    if (!relevantAgeMatch && !relevantValueMatch) continue;

    entries.push({
      person: member.person,
      atSameAge: relevantAgeMatch
        ? toPoint(relevantAgeMatch, targetValueMetric, targetAgeMonths)
        : null,
      atSameValue: relevantValueMatch
        ? toPoint(relevantValueMatch, targetValueMetric, targetAgeMonths)
        : null,
    });
  }

  return entries;
}
