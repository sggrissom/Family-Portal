import { describe, expect, it } from "vitest";
import {
  formatLbOz,
  formatMeasurement,
  lbOzToLbs,
  prefersLbOz,
  splitLbOz,
} from "@app/lib/weightFormat";

describe("splitLbOz", () => {
  it("splits whole and fractional pounds", () => {
    expect(splitLbOz(7.5)).toEqual({ lb: 7, oz: 8 });
    expect(splitLbOz(lbOzToLbs(7, 8.5))).toEqual({ lb: 7, oz: 8.5 });
  });

  it("carries ounces that round up to a full pound", () => {
    expect(splitLbOz(7.999)).toEqual({ lb: 8, oz: 0 });
  });
});

describe("formatLbOz", () => {
  it("omits zero parts", () => {
    expect(formatLbOz(8)).toBe("8 lb");
    expect(formatLbOz(0.75)).toBe("12 oz");
    expect(formatLbOz(7.25)).toBe("7 lb 4 oz");
  });
});

describe("prefersLbOz", () => {
  it("uses age when known", () => {
    expect(prefersLbOz(30, "lbs", 6)).toBe(true);
    expect(prefersLbOz(20, "lbs", 36)).toBe(false);
  });

  it("falls back to weight when age is unknown", () => {
    expect(prefersLbOz(9, "lbs")).toBe(true);
    expect(prefersLbOz(150, "lbs", null)).toBe(false);
  });

  it("never applies to other units", () => {
    expect(prefersLbOz(3, "kg", 1)).toBe(false);
    expect(prefersLbOz(20, "in", 1)).toBe(false);
  });
});

describe("formatMeasurement", () => {
  it("formats infant weights as lb/oz", () => {
    expect(formatMeasurement(7.5, "lbs", 0)).toBe("7 lb 8 oz");
  });

  it("rounds other values to two decimals", () => {
    expect(formatMeasurement(150.53125, "lbs", 400)).toBe("150.53 lbs");
    expect(formatMeasurement(67.5, "in")).toBe("67.5 in");
  });
});
