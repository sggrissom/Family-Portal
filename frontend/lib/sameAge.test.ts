import { describe, expect, it } from "vitest";
import {
  ageStep,
  ageTitle,
  hasSameAgeRecords,
  nearbyRecordedAge,
  monthsOld,
  nextAge,
  parseAgeParam,
  photoAge,
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

describe("photoAge", () => {
  const born = "2014-03-02T00:00:00Z";

  it("counts days past the month under two", () => {
    expect(photoAge(born, "2014-09-10T15:30:00Z")).toBe("6 months, 8 days");
    expect(photoAge(born, "2014-09-02T08:00:00Z")).toBe("6 months");
    expect(photoAge(born, "2014-03-03T00:00:00Z")).toBe("1 day");
    expect(photoAge(born, "2014-03-02T00:00:00Z")).toBe("Newborn");
    expect(photoAge(born, "2015-04-01T00:00:00Z")).toBe("1 year, 30 days");
  });

  it("drops the days from two on", () => {
    expect(photoAge(born, "2019-05-20T00:00:00Z")).toBe("5 years 2 months");
  });

  it("says nothing for a photo before the birthday", () => {
    expect(photoAge(born, "2014-01-01T00:00:00Z")).toBe("");
  });
});

describe("browsing saved ages", () => {
  it("skips gaps, handles off-grid links, and stops at either end", () => {
    const ages = [0, 6, 42, 78];
    expect(nearbyRecordedAge(ages, 6, 1)).toBe(42);
    expect(nearbyRecordedAge(ages, 41, -1)).toBe(6);
    expect(nearbyRecordedAge(ages, 41, 1)).toBe(42);
    expect(nearbyRecordedAge(ages, 0, -1)).toBeNull();
    expect(nearbyRecordedAge(ages, 78, 1)).toBeNull();
    expect(nearbyRecordedAge([], 12, 1)).toBeNull();
  });

  it("hides empty records while retaining each supported kind of content", () => {
    const empty = {
      height: null,
      weight: null,
      milestones: [],
      photoIds: [],
      portraits: [],
    } as any;
    expect(hasSameAgeRecords(empty)).toBe(false);
    for (const record of [
      { height: { value: 0 } },
      { weight: { value: 10 } },
      { milestones: [{ id: 1 }] },
      { photoIds: [1] },
      { portraits: [{ photoId: 1 }] },
    ]) {
      expect(hasSameAgeRecords({ ...empty, ...record })).toBe(true);
    }
  });
});
