import * as server from "../server";
import {
  addDays,
  addYears,
  dayOf,
  isRealDay,
  joinNames,
  monthSpan,
  numberWord,
  PRESETS,
} from "./book";
import { monthsOld } from "./sameAge";

export type YearKind = "calendar" | "past" | "age";

export const YEAR_KINDS: { value: YearKind; label: string }[] = [
  { value: "calendar", label: "Calendar year" },
  { value: "past", label: "Past 12 months" },
  { value: "age", label: "Year of age" },
];

export interface Period {
  start: string;
  end: string;
}

export const calendarYear = (year: number): Period => ({
  start: `${year}-01-01`,
  end: `${year + 1}-01-01`,
});

// pastYear ends tomorrow so that today is the last day covered.
export const pastYear = (today: string): Period => ({
  start: addDays(addYears(today, -1), 1),
  end: addDays(today, 1),
});

export const yearOfAge = (birthday: string, age: number): Period => ({
  start: addYears(dayOf(birthday), age),
  end: addYears(dayOf(birthday), age + 1),
});

export function ageNow(birthday: string, today: string): number {
  return Math.max(0, Math.floor(monthsOld(dayOf(birthday), today + "T00:00:00Z") / 12));
}

export function hasBirthday(person: server.Person): boolean {
  return !person.isPregnancy && isRealDay(dayOf(person.birthday));
}

export function bornBy(person: server.Person, day: string): boolean {
  return hasBirthday(person) && dayOf(person.birthday) < day;
}

const possessive = (name: string) => (name.endsWith("s") ? `${name}'` : `${name}'s`);

export function defaultTitle(
  preset: string,
  people: server.Person[],
  period: Period,
  yearKind: YearKind,
  age: number
): string {
  const first = people[0]?.name ?? "";
  switch (preset) {
    case PRESETS.firstYear:
      return `${possessive(first)} first year`;
    case PRESETS.year:
      if (yearKind === "calendar") return `${possessive(first)} ${period.start.slice(0, 4)}`;
      if (yearKind === "age") {
        return age === 0
          ? `${possessive(first)} first year`
          : `${first} at ${numberWord(age).toLowerCase()}`;
      }
      return `${possessive(first)} year`;
    case PRESETS.familyYear:
      return `Our ${period.start.slice(0, 4)}`;
    default:
      return `${joinNames(people.map(p => p.name))}, ${monthSpan(period.start, addDays(period.end, -1))}`;
  }
}
