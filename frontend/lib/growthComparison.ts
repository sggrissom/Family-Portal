import * as server from "../server";
import { ageInMonths, formatAgeAtMeasurement, isValidBirthday } from "./growthPercentiles";

export interface ComparisonPoint {
  ageLabel: string;
  value: number;
  unit: string;
  date: string;
}

export interface FamilyComparisonEntry {
  person: server.Person;
  // What this person's measurement was at (approximately) the same age as the target record.
  atSameAge: ComparisonPoint | null;
  // The age at which this person had (approximately) the same measurement value.
  atSameValue: ComparisonPoint | null;
}

// Convert a height/weight value to a common metric unit (cm or kg) for comparison purposes only.
// Display values always use the unit the record was actually saved in.
function toMetric(value: number, unit: string): number {
  if (unit === "in") return value * 2.54;
  if (unit === "lbs") return value * 0.453592;
  return value;
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

// A comparison is only meaningful if the matched record is actually from around
// the same point: someone who was already as old (or as tall/heavy) as the target
// record clearly passed through that point, so their match is always relevant.
// Someone who falls short is only relevant if they're close enough that the gap is
// negligible (e.g. a record from a day younger) rather than a real mismatch (e.g. a
// sibling who's only ever been a toddler being compared to a six-year-old's data).
const AGE_TOLERANCE_RATIO = 0.05;
const MIN_AGE_TOLERANCE_MONTHS = 0.5;
const VALUE_TOLERANCE_RATIO = 0.02;

function isAgeMatchRelevant(match: AgedRecord, targetAgeMonths: number): boolean {
  const tolerance = Math.max(MIN_AGE_TOLERANCE_MONTHS, targetAgeMonths * AGE_TOLERANCE_RATIO);
  return match.ageMonths >= targetAgeMonths - tolerance;
}

function isValueMatchRelevant(match: AgedRecord, targetValueMetric: number): boolean {
  const matchValueMetric = toMetric(match.record.value, match.record.unit);
  const tolerance = targetValueMetric * VALUE_TOLERANCE_RATIO;
  return matchValueMetric >= targetValueMetric - tolerance;
}

function toPoint(r: AgedRecord): ComparisonPoint {
  return {
    ageLabel: formatAgeAtMeasurement(r.ageMonths),
    value: r.record.value,
    unit: r.record.unit,
    date: r.record.measurementDate,
  };
}

/**
 * For a given growth measurement, find how every other family member compares:
 * what they measured at the same age, and when they reached the same measurement.
 * Family members without a valid birthday or without any records of the same
 * measurement type are still included (with null comparison points) so the caller
 * can surface "no data yet" for them.
 */
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

    const relevantAgeMatch = ageMatch && isAgeMatchRelevant(ageMatch, targetAgeMonths) ? ageMatch : null;
    const relevantValueMatch =
      valueMatch && isValueMatchRelevant(valueMatch, targetValueMetric) ? valueMatch : null;

    // Records existed but none were close enough to be a meaningful comparison
    // (e.g. a toddler sibling with no data anywhere near a six-year-old's data point).
    if (!relevantAgeMatch && !relevantValueMatch) continue;

    entries.push({
      person: member.person,
      atSameAge: relevantAgeMatch ? toPoint(relevantAgeMatch) : null,
      atSameValue: relevantValueMatch ? toPoint(relevantValueMatch) : null,
    });
  }

  return entries;
}
