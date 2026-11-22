import * as server from "../../server";
import { formatDate, formatLongDate } from "../dateUtils";
import { formatMeasurement } from "../weightFormat";
import {
  ageInMonths,
  computePercentileLabel,
  formatAgeAtMeasurement,
  isValidBirthday,
} from "../growthPercentiles";
import {
  ComparisonPoint,
  FamilyComparisonEntry,
  computeFamilyComparisons,
  describeAgeComparison,
  describeValueComparison,
} from "../growthComparison";

export interface ShareLine {
  title: string;
  detail: string | null;
}

export interface MeasurementShareRow {
  id: string;
  group: string | null;
  heading: string;
  lines: ShareLine[];
}

export interface MeasurementShareSection {
  id: number;
  type: "height" | "weight";
  label: string;
  value: string;
  rows: MeasurementShareRow[];
}

export interface MeasurementShare {
  personName: string;
  dateText: string;
  ageText: string | null;
  sections: MeasurementShareSection[];
}

export function shareRowIds(share: MeasurementShare): string[] {
  return share.sections.flatMap(section => section.rows.map(row => row.id));
}

export function omitRows(share: MeasurementShare, omitted: Set<string>): MeasurementShare {
  return {
    ...share,
    sections: share.sections.map(section => ({
      ...section,
      rows: section.rows.filter(row => !omitted.has(row.id)),
    })),
  };
}

export function measurementShareText(share: MeasurementShare): string {
  const lines = [[share.personName, share.dateText, share.ageText].filter(Boolean).join(" · ")];
  for (const section of share.sections) {
    lines.push("", `${section.label}: ${section.value}`);
    let group: string | null = null;
    for (const row of section.rows) {
      if (row.group && row.group !== group) lines.push(`${row.group}:`);
      group = row.group;
      const body = row.lines
        .map(line => (line.detail ? `${line.title} — ${line.detail}` : line.title))
        .join("; ");
      lines.push(`• ${row.heading}: ${body}`);
    }
  }
  return lines.join("\n");
}

export interface MeasurementShareInput {
  measurements: server.GrowthData[];
  person: server.Person;
  familyMembers: { person: server.Person; growthData: server.GrowthData[] }[];
  relationGroups: Map<number, string>;
}

const groupTitles: [string, (group: string | undefined) => boolean][] = [
  ["Siblings", group => group === "sibling"],
  ["Parents", group => group === "parent"],
  ["Rest of the family", group => group !== "sibling" && group !== "parent"],
];

export function buildMeasurementShare(input: MeasurementShareInput): MeasurementShare | null {
  const { person } = input;
  const measurements = input.measurements.filter(m => m.personId === person.id);
  const first = measurements[0];
  if (!first) return null;

  const hasBirthday = isValidBirthday(person.birthday);
  const firstAge = hasBirthday ? ageInMonths(person.birthday, first.measurementDate) : null;

  return {
    personName: person.name,
    dateText: formatLongDate(first.measurementDate),
    ageText: firstAge !== null && firstAge >= 0 ? `Age ${formatAgeAtMeasurement(firstAge)}` : null,
    sections: measurements.map(growth => section(growth, input, hasBirthday)),
  };
}

function section(
  growth: server.GrowthData,
  input: MeasurementShareInput,
  hasBirthday: boolean
): MeasurementShareSection {
  const { person } = input;
  const type = growth.measurementType === server.Height ? "height" : "weight";
  const ageMonths = hasBirthday ? ageInMonths(person.birthday, growth.measurementDate) : null;
  const rows: MeasurementShareRow[] = [];

  const percentile =
    ageMonths !== null && ageMonths <= 240
      ? computePercentileLabel(growth.value, growth.unit, ageMonths, person.gender, type)
      : null;
  if (percentile) {
    rows.push({
      id: `${growth.id}-percentile`,
      group: null,
      heading: "Percentile",
      lines: [{ title: percentile, detail: null }],
    });
  }

  const matched = computeFamilyComparisons(growth, person, input.familyMembers).filter(
    entry => entry.atSameAge || entry.atSameValue
  );
  for (const [title, inGroup] of groupTitles) {
    for (const entry of matched.filter(e => inGroup(input.relationGroups.get(e.person.id)))) {
      rows.push(familyRow(growth, entry, title, person.name));
    }
  }

  return {
    id: growth.id,
    type,
    label: type === "height" ? "Height" : "Weight",
    value: formatMeasurement(growth.value, growth.unit, ageMonths),
    rows,
  };
}

function format(point: ComparisonPoint): string {
  return formatMeasurement(point.value, point.unit, point.ageMonths);
}

function familyRow(
  growth: server.GrowthData,
  entry: FamilyComparisonEntry,
  group: string,
  subject: string
): MeasurementShareRow {
  const lines: ShareLine[] = [];
  if (entry.atSameAge) {
    const point = entry.atSameAge;
    lines.push({
      title: `At ${point.ageLabel} old: ${format(point)}`,
      detail: describeValueComparison(point, growth.measurementType, subject),
    });
  }
  if (entry.atSameValue) {
    const point = entry.atSameValue;
    lines.push({
      title: `Reached ${format(point)} at ${point.ageLabel} old`,
      detail: `${describeAgeComparison(point, subject)} · ${formatDate(point.date)}`,
    });
  }
  return { id: `${growth.id}-${entry.person.id}`, group, heading: entry.person.name, lines };
}
