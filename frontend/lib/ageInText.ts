export interface AgeInText {
  years: number;
  months: number;
  text: string;
}

const NUMBER = String.raw`(\d{1,2}(?:\.\d)?|one|two|three|four|five|six|seven|eight|nine|ten|eleven|twelve|a|an)`;
const WORDS: Record<string, number> = {
  a: 1,
  an: 1,
  one: 1,
  two: 2,
  three: 3,
  four: 4,
  five: 5,
  six: 6,
  seven: 7,
  eight: 8,
  nine: 9,
  ten: 10,
  eleven: 11,
  twelve: 12,
};
const YEAR = String.raw`(?:years?|yrs?|y)`;
const MONTH = String.raw`(?:months?|mos?|m)`;
const WEEK = String.raw`(?:weeks?|wks?)`;

// "at 14 months", "2 years 3 months old", "a 2-year-old", "at 2.5 years".
// A number with a unit counts only after "at" or before "old", so "two years
// ago" or "in 3 weeks" don't.
const PATTERN = new RegExp(
  String.raw`\b(?:(at|age|aged)\s+)?${NUMBER}[\s-]*(${YEAR}|${MONTH}|${WEEK})` +
    String.raw`(?:(?:\s*,?\s*(?:and\s+)?)${NUMBER}[\s-]*(${MONTH}|${WEEK}))?` +
    String.raw`(?:[\s-]*(old))?\b`,
  "i"
);

const amount = (value: string) => WORDS[value.toLowerCase()] ?? parseFloat(value);

const unitMonths = (unit: string): number => {
  const u = unit.toLowerCase();
  if (/^w/.test(u)) return 12 / 52;
  if (/^(y|yr)/.test(u)) return 12;
  return 1;
};

export function parseAgeFromText(text: string): AgeInText | null {
  const match = PATTERN.exec(text);
  if (!match) return null;
  const [whole, lead, n1, u1, n2, u2, old] = match;
  if (!lead && !old) return null;
  let months = amount(n1) * unitMonths(u1);
  if (n2 && u2) months += amount(n2) * unitMonths(u2);
  if (!isFinite(months) || months < 0 || months > 12 * 25) return null;
  const total = Math.floor(months + 1e-9);
  return { years: Math.floor(total / 12), months: total % 12, text: whole.trim() };
}
