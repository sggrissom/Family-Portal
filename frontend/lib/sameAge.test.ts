import { describe, expect, it } from "vitest";
import {
  ageStep,
  ageTitle,
  monthsOld,
  nextAge,
  parseAgeParam,
  prevAge,
  sameAgePath,
} from "./sameAge";

describe("stepping the age", () => {
  it("moves a month at a time under two", () => {
    expect(ageStep(10)).toBe(1);
    expect(nextAge(10, 200)).toBe(11);
    expect(prevAge(10)).toBe(9);
  });

  it("moves by three months from two to six, and six after", () => {
    expect(nextAge(23, 200)).toBe(24);
    expect(nextAge(24, 200)).toBe(27);
    expect(nextAge(70, 200)).toBe(72);
    expect(nextAge(72, 200)).toBe(78);
    expect(prevAge(78)).toBe(72);
    expect(prevAge(72)).toBe(69);
    expect(prevAge(24)).toBe(23);
  });

  it("snaps an off-grid age onto the grid", () => {
    expect(nextAge(41, 200)).toBe(42);
    expect(prevAge(41)).toBe(39);
  });

  it("stops at zero and at the oldest age anyone has reached", () => {
    expect(prevAge(0)).toBe(0);
    expect(nextAge(78, 80)).toBe(80);
  });
});

describe("the age in the URL", () => {
  it("reads months, years, or both", () => {
    expect(parseAgeParam("40m")).toBe(40);
    expect(parseAgeParam("3y4m")).toBe(40);
    expect(parseAgeParam("3y")).toBe(36);
    expect(parseAgeParam("40")).toBe(40);
    expect(parseAgeParam("0m")).toBe(0);
    expect(parseAgeParam("soon")).toBeNull();
    expect(parseAgeParam(null)).toBeNull();
  });

  it("writes months and the person", () => {
    expect(sameAgePath(40, 7)).toBe("/same-age?age=40m&from=7");
    expect(sameAgePath(0, 7)).toBe("/same-age?age=0m&from=7");
    expect(sameAgePath(null, 0)).toBe("/same-age");
  });
});

describe("ageTitle", () => {
  it("spells the age out", () => {
    expect(ageTitle(40)).toBe("3 years 4 months");
    expect(ageTitle(13)).toBe("1 year 1 month");
    expect(ageTitle(8)).toBe("8 months");
    expect(ageTitle(24)).toBe("2 years");
    expect(ageTitle(0)).toBe("Newborn");
  });
});

describe("monthsOld", () => {
  it("counts whole months between two dates", () => {
    expect(monthsOld("2020-06-15T00:00:00Z", "2024-01-15T00:00:00Z")).toBe(43);
    expect(monthsOld("2020-06-15T00:00:00Z", "2024-01-14T00:00:00Z")).toBe(42);
  });
});
