export const OZ_PER_LB = 16;
const LB_OZ_MAX_AGE_MONTHS = 24;
const LB_OZ_MAX_LBS = 25;

export function prefersLbOz(value: number, unit: string, ageMonths?: number | null): boolean {
  if (unit !== "lbs") return false;
  if (ageMonths !== undefined && ageMonths !== null && ageMonths >= 0) {
    return ageMonths < LB_OZ_MAX_AGE_MONTHS;
  }
  return value < LB_OZ_MAX_LBS;
}

function roundTo(n: number, places: number): number {
  const f = Math.pow(10, places);
  return Math.round(n * f) / f;
}

export function splitLbOz(lbs: number): { lb: number; oz: number } {
  let lb = Math.floor(lbs);
  let oz = roundTo((lbs - lb) * OZ_PER_LB, 1);
  if (oz >= OZ_PER_LB) {
    lb += 1;
    oz -= OZ_PER_LB;
  }
  return { lb, oz };
}

export function lbOzToLbs(lb: number, oz: number): number {
  return lb + oz / OZ_PER_LB;
}

export function formatLbOz(lbs: number): string {
  const { lb, oz } = splitLbOz(lbs);
  if (lb === 0) return `${oz} oz`;
  if (oz === 0) return `${lb} lb`;
  return `${lb} lb ${oz} oz`;
}

export function formatMeasurement(value: number, unit: string, ageMonths?: number | null): string {
  if (prefersLbOz(value, unit, ageMonths)) return formatLbOz(value);
  return `${roundTo(value, 2)} ${unit}`;
}
