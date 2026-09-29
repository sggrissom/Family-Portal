import { describe, expect, it } from "vitest";
import { parseAgeFromText } from "./ageInText";

const age = (text: string) => {
  const found = parseAgeFromText(text);
  return found ? [found.years, found.months] : null;
};

describe("parseAgeFromText", () => {
  it("reads ages said with at or old", () => {
    expect(age("Walked at 14 months")).toEqual([1, 2]);
    expect(age("rode a bike at 4 years")).toEqual([4, 0]);
    expect(age("First haircut, 18 months old")).toEqual([1, 6]);
    expect(age("A 2-year-old's tantrum")).toEqual([2, 0]);
    expect(age("at 2.5 years")).toEqual([2, 6]);
    expect(age("at 3 years and 2 months")).toEqual([3, 2]);
    expect(age("smiled at six weeks")).toEqual([0, 1]);
    expect(age("at two years")).toEqual([2, 0]);
  });

  it("ignores durations and times that aren't ages", () => {
    expect(age("Slept through the night for 2 weeks")).toBeNull();
    expect(age("Moved house two years ago")).toBeNull();
    expect(age("Ran a 5K in 25 minutes")).toBeNull();
    expect(age("First goal")).toBeNull();
    expect(age("Saw a whale at a museum")).toBeNull();
    expect(age("Met Santa at 3 pm")).toBeNull();
  });
});
