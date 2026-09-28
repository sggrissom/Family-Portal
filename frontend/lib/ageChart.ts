import * as server from "../server";
import { getPercentileRow } from "./growthPercentiles";
import { monthsOld } from "./sameAge";

export type Metric = "height" | "weight";

export interface ChartPoint {
  id: number;
  ageMonths: number;
  value: number;
}

export interface BandRow {
  ageMonths: number;
  p3: number;
  p15: number;
  p50: number;
  p85: number;
  p97: number;
}

export const METRIC_UNIT: Record<Metric, string> = { height: "in", weight: "lb" };

const CM_PER_IN = 2.54;
const KG_PER_LB = 0.453592;

export function toDisplay(value: number, unit: string): number {
  if (unit === "cm") return value / CM_PER_IN;
  if (unit === "kg") return value / KG_PER_LB;
  return value;
}

export function chartPoints(
  growth: server.GrowthData[],
  birthday: string,
  metric: Metric
): ChartPoint[] {
  const type = metric === "height" ? server.Height : server.Weight;
  return growth
    .filter(g => g.measurementType === type)
    .map(g => ({
      id: g.id,
      ageMonths: Math.max(
        0,
        monthsOld(birthday, g.measurementDate) + fractionOfMonth(birthday, g.measurementDate)
      ),
      value: toDisplay(g.value, g.unit),
    }))
    .sort((a, b) => a.ageMonths - b.ageMonths);
}

function fractionOfMonth(birthday: string, date: string): number {
  const born = new Date(birthday).getUTCDate();
  const day = new Date(date).getUTCDate();
  const diff = day >= born ? day - born : day + 30 - born;
  return Math.min(diff / 30.4375, 0.99);
}

export function percentileBand(
  gender: number,
  metric: Metric,
  fromMonths: number,
  toMonths: number
): BandRow[] {
  const rows: BandRow[] = [];
  const start = Math.max(0, Math.floor(fromMonths));
  const end = Math.min(240, Math.ceil(toMonths));
  const step = end - start > 48 ? 3 : 1;
  const convert = metric === "height" ? 1 / CM_PER_IN : 1 / KG_PER_LB;
  for (let m = start; m <= end; m += step) {
    const row = getPercentileRow(m, gender, metric);
    if (!row) continue;
    rows.push({
      ageMonths: m,
      p3: row.p3 * convert,
      p15: row.p15 * convert,
      p50: row.p50 * convert,
      p85: row.p85 * convert,
      p97: row.p97 * convert,
    });
  }
  return rows;
}

export function niceTicks(min: number, max: number, target = 5): number[] {
  if (!isFinite(min) || !isFinite(max)) return [];
  if (min === max) return [min];
  const rough = (max - min) / target;
  const magnitude = Math.pow(10, Math.floor(Math.log10(rough)));
  const norm = rough / magnitude;
  const step = (norm >= 7.5 ? 10 : norm >= 3.5 ? 5 : norm >= 1.5 ? 2 : 1) * magnitude;
  const ticks: number[] = [];
  for (let v = Math.ceil(min / step) * step; v <= max + 1e-9; v += step) {
    ticks.push(Math.round(v * 1e6) / 1e6);
  }
  return ticks;
}

export function ageTicks(
  minMonths: number,
  maxMonths: number
): { months: number; label: string }[] {
  const span = maxMonths - minMonths;
  const step = span <= 8 ? 1 : span <= 24 ? 3 : span <= 72 ? 12 : span <= 144 ? 24 : 60;
  const ticks = [];
  for (let m = Math.ceil(minMonths / step) * step; m <= maxMonths; m += step) {
    ticks.push({ months: m, label: step >= 12 ? `${Math.round(m / 12)}y` : ageLabel(m) });
  }
  return ticks;
}

function ageLabel(months: number): string {
  if (months < 24) return `${months}m`;
  const rest = months % 12;
  return rest === 0 ? `${months / 12}y` : `${Math.floor(months / 12)}y ${rest}m`;
}

export interface AgeRange {
  from: number;
  to: number;
}

export interface ChartDomain {
  minAge: number;
  maxAge: number;
  minValue: number;
  maxValue: number;
  band: BandRow[];
  zoomed: boolean;
}

export const MIN_ZOOM_MONTHS = 1;

export function chartDomain(
  series: ChartPoint[][],
  band: BandRow[],
  zoom: AgeRange | null
): ChartDomain | null {
  const points = series.flat();
  if (points.length === 0) return null;

  const ages = points.map(p => p.ageMonths);
  let minAge = Math.max(0, Math.min(...ages) - 1);
  let maxAge = Math.max(...ages, minAge + 6) + 1;
  const range = zoom && clampRange(zoom, minAge, maxAge);
  if (range) {
    minAge = range.from;
    maxAge = range.to;
  }

  const values = series.flatMap(s => visibleValues(s, minAge, maxAge));
  const inRange = bandWithin(band, minAge, maxAge);
  values.push(
    ...inRange
      .filter(r => r.ageMonths >= minAge && r.ageMonths <= maxAge)
      .flatMap(r => [r.p3, r.p97])
  );
  if (values.length === 0) return null;

  const pad = Math.max((Math.max(...values) - Math.min(...values)) * 0.06, 0.5);
  return {
    minAge,
    maxAge,
    minValue: Math.max(0, Math.min(...values) - pad),
    maxValue: Math.max(...values) + pad,
    band: inRange,
    zoomed: range !== null,
  };
}

export function clampRange(range: AgeRange, min: number, max: number): AgeRange | null {
  let from = Math.max(min, Math.min(range.from, range.to));
  let to = Math.min(max, Math.max(range.from, range.to));
  if (to - from < MIN_ZOOM_MONTHS) {
    const mid = (from + to) / 2;
    from = Math.max(min, mid - MIN_ZOOM_MONTHS / 2);
    to = Math.min(max, from + MIN_ZOOM_MONTHS);
    from = Math.max(min, to - MIN_ZOOM_MONTHS);
  }
  if (to <= from || (from === min && to === max)) return null;
  return { from, to };
}

function visibleValues(points: ChartPoint[], from: number, to: number): number[] {
  const values = points.filter(p => p.ageMonths >= from && p.ageMonths <= to).map(p => p.value);
  for (const edge of [from, to]) {
    const after = points.findIndex(p => p.ageMonths > edge);
    if (after > 0) {
      const a = points[after - 1];
      const b = points[after];
      values.push(
        a.value + ((edge - a.ageMonths) / (b.ageMonths - a.ageMonths)) * (b.value - a.value)
      );
    }
  }
  return values;
}

function bandWithin(band: BandRow[], from: number, to: number): BandRow[] {
  const start = band.findIndex(r => r.ageMonths >= from);
  if (start === -1) return [];
  const first = Math.max(0, start - 1);
  const past = band.findIndex(r => r.ageMonths > to);
  return band.slice(first, past === -1 ? band.length : past + 1);
}
