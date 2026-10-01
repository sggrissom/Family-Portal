import { describe, expect, it } from "vitest";
import * as server from "../server";
import { bookDates } from "./book";
import { ageNow, calendarYear, defaultTitle, pastYear, yearOfAge } from "./bookPlans";

const person = (name: string, birthday: string) => ({ name, birthday }) as server.Person;

describe("what a year means", () => {
  it("shows each kind of year as the inclusive dates it covers", () => {
    const show = (p: { start: string; end: string }) => bookDates(p.start, p.end);
    expect(show(calendarYear(2025))).toBe("January 1, 2025 – December 31, 2025");
    expect(show(pastYear("2026-10-01"))).toBe("October 2, 2025 – October 1, 2026");
    expect(show(yearOfAge("2021-06-01T00:00:00Z", 4))).toBe("June 1, 2025 – May 31, 2026");
  });

  it("rolls a leap-day birthday to March 1 in other years", () => {
    expect(yearOfAge("2020-02-29T00:00:00Z", 1)).toEqual({
      start: "2021-03-01",
      end: "2022-03-01",
    });
  });

  it("counts whole years of age", () => {
    expect(ageNow("2021-06-01T00:00:00Z", "2026-05-31")).toBe(4);
    expect(ageNow("2021-06-01T00:00:00Z", "2026-06-01")).toBe(5);
  });
});

describe("default titles", () => {
  const theo = person("Theo", "2021-06-01T00:00:00Z");
  const june = person("Juniper", "2024-03-14T00:00:00Z");

  it("says which year a personal book is", () => {
    expect(defaultTitle("year", [theo], calendarYear(2025), "calendar", 0)).toBe("Theo's 2025");
    expect(defaultTitle("year", [theo], yearOfAge(theo.birthday, 4), "age", 4)).toBe(
      "Theo at four"
    );
    expect(defaultTitle("year", [theo], pastYear("2026-10-01"), "past", 0)).toBe("Theo's year");
  });

  it("names everyone in a custom book", () => {
    expect(
      defaultTitle(
        "custom",
        [june, theo],
        { start: "2025-06-01", end: "2025-09-01" },
        "calendar",
        0
      )
    ).toBe("Juniper and Theo, June – August 2025");
  });

  it("calls a family year ours", () => {
    expect(defaultTitle("family-year", [june, theo], calendarYear(2025), "calendar", 0)).toBe(
      "Our 2025"
    );
  });
});
