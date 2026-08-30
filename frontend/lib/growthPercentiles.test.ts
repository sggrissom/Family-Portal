import { describe, expect, it } from "vitest";
import * as server from "@app/server";
import {
  ageInMonths,
  computePercentileLabel,
  formatAgeAtMeasurement,
  getPercentileRow,
  interpolatePercentiles,
  isValidBirthday,
  whoHeightBoys,
  type PercentileRow,
} from "@app/lib/growthPercentiles";

const row = (month: number, base: number): PercentileRow => ({
  month,
  p3: base,
  p15: base + 1,
  p50: base + 2,
  p85: base + 3,
  p97: base + 4,
});

describe("interpolatePercentiles", () => {
  const table = [row(0, 10), row(10, 20)];

  it("clamps to the ends rather than extrapolating past the table", () => {
    expect(interpolatePercentiles(table, -5)).toBe(table[0]);
    expect(interpolatePercentiles(table, 99)).toBe(table[1]);
  });

  it("interpolates linearly between two rows", () => {
    expect(interpolatePercentiles(table, 5)).toEqual({
      month: 5,
      p3: 15,
      p15: 16,
      p50: 17,
      p85: 18,
      p97: 19,
    });
  });

  it("returns null for an empty table", () => {
    expect(interpolatePercentiles([], 5)).toBeNull();
  });
});

describe("getPercentileRow", () => {
  it("uses the WHO tables through 24 months", () => {
    expect(getPercentileRow(0, server.Male, "height")).toEqual(whoHeightBoys[0]);
  });

  it("switches to the CDC tables past 24 months", () => {
    // WHO stops at 24 months, so a 36-month row can only come from CDC.
    const row = getPercentileRow(36, server.Male, "height");
    expect(row).not.toBeNull();
    expect(row!.p50).toBeGreaterThan(whoHeightBoys[whoHeightBoys.length - 1].p50);
  });

  it("averages the two tables when the gender is unknown", () => {
    const male = getPercentileRow(12, server.Male, "weight")!;
    const female = getPercentileRow(12, server.Female, "weight")!;
    const unknown = getPercentileRow(12, server.Unknown, "weight")!;
    expect(unknown.p50).toBeCloseTo((male.p50 + female.p50) / 2, 10);
  });

  it("refuses ages outside the charted range", () => {
    expect(getPercentileRow(-1, server.Male, "height")).toBeNull();
    expect(getPercentileRow(241, server.Male, "height")).toBeNull();
  });
});

describe("computePercentileLabel", () => {
  const newbornBoy = whoHeightBoys[0];

  it("reports the median as the 50th percentile", () => {
    expect(computePercentileLabel(newbornBoy.p50, "cm", 0, server.Male, "height")).toBe(
      "~50th %ile"
    );
  });

  it("converts inches to centimetres before comparing", () => {
    const inches = newbornBoy.p50 / 2.54;
    expect(computePercentileLabel(inches, "in", 0, server.Male, "height")).toBe("~50th %ile");
  });

  it("converts pounds to kilograms before comparing", () => {
    const kg = getPercentileRow(0, server.Male, "weight")!.p50;
    expect(computePercentileLabel(kg / 0.453592, "lbs", 0, server.Male, "weight")).toBe(
      "~50th %ile"
    );
  });

  it("reports anything under the 3rd percentile as off the bottom of the chart", () => {
    expect(computePercentileLabel(newbornBoy.p3 - 5, "cm", 0, server.Male, "height")).toBe(
      "<3rd %ile"
    );
  });

  it("extrapolates above the 97th but caps the label at 99.9", () => {
    expect(computePercentileLabel(newbornBoy.p97 + 0.5, "cm", 0, server.Male, "height")).toMatch(
      /^~9[789]\.\dth %ile$/
    );
    expect(computePercentileLabel(newbornBoy.p97 + 500, "cm", 0, server.Male, "height")).toBe(
      "~99.9th %ile"
    );
  });

  it("uses the right ordinal suffix", () => {
    expect(computePercentileLabel(newbornBoy.p3, "cm", 0, server.Male, "height")).toBe("~3rd %ile");
    expect(computePercentileLabel(newbornBoy.p15, "cm", 0, server.Male, "height")).toBe(
      "~15th %ile"
    );
  });

  it("returns null outside the charted age range", () => {
    expect(computePercentileLabel(50, "cm", 300, server.Male, "height")).toBeNull();
  });
});

describe("isValidBirthday", () => {
  it("rejects empty, missing, and Go zero-value dates", () => {
    expect(isValidBirthday(null)).toBe(false);
    expect(isValidBirthday(undefined)).toBe(false);
    expect(isValidBirthday("")).toBe(false);
    expect(isValidBirthday("0001-01-01T00:00:00Z")).toBe(false);
  });

  it("accepts a real date", () => {
    expect(isValidBirthday("2020-06-15T00:00:00Z")).toBe(true);
  });
});

describe("ageInMonths", () => {
  it("counts whole months between the same day of month", () => {
    expect(ageInMonths("2020-01-15", "2020-04-15")).toBeCloseTo(3, 10);
  });

  it("adds a fraction of a month for the extra days", () => {
    expect(ageInMonths("2020-01-15", "2020-04-30")).toBeCloseTo(3 + 15 / 30.4375, 6);
  });
});

describe("formatAgeAtMeasurement", () => {
  it("uses months below two years and singularises one", () => {
    expect(formatAgeAtMeasurement(1)).toBe("1 mo");
    expect(formatAgeAtMeasurement(18.4)).toBe("18 mo");
  });

  it("uses years and months from two years up", () => {
    expect(formatAgeAtMeasurement(24)).toBe("2 yr");
    expect(formatAgeAtMeasurement(30)).toBe("2 yr 6 mo");
    expect(formatAgeAtMeasurement(12 * 5)).toBe("5 yr");
  });

  it("shows a dash for a measurement dated before the birthday", () => {
    expect(formatAgeAtMeasurement(-1)).toBe("—");
  });
});
