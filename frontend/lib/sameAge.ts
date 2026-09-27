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

export function sameAgePath(ageMonths: number | null, fromPersonId: number): string {
  const params = new URLSearchParams();
  if (ageMonths !== null && ageMonths >= 0) params.set("age", `${ageMonths}m`);
  if (fromPersonId > 0) params.set("from", String(fromPersonId));
  const query = params.toString();
  return query ? `/same-age?${query}` : "/same-age";
}

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
