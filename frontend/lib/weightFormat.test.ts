import { describe, expect, it } from "vitest";
import {
  formatFtIn,
  formatLbOz,
  formatMeasurement,
  lbOzToLbs,
  prefersFtIn,
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

describe("prefersFtIn", () => {
  it("uses age when known", () => {
    expect(prefersFtIn(40, "in", 12)).toBe(false);
    expect(prefersFtIn(33, "in", 24)).toBe(true);
  });

  it("falls back to height when age is unknown", () => {
    expect(prefersFtIn(30, "in")).toBe(false);
    expect(prefersFtIn(36, "in", null)).toBe(true);
  });

  it("never applies to other units", () => {
    expect(prefersFtIn(170, "cm", 400)).toBe(false);
  });
});

describe("formatFtIn", () => {
  it("omits zero parts and carries rounded inches", () => {
    expect(formatFtIn(72)).toBe("6 ft");
    expect(formatFtIn(11.5)).toBe("11.5 in");
    expect(formatFtIn(71.999)).toBe("6 ft");
  });
});

describe("formatMeasurement", () => {
  it("formats infant weights as lb/oz", () => {
    expect(formatMeasurement(7.5, "lbs", 0)).toBe("7 lb 8 oz");
  });

  it("rounds other values to two decimals", () => {
    expect(formatMeasurement(150.53125, "lbs", 400)).toBe("150.53 lbs");
    expect(formatMeasurement(20.5, "in", 0)).toBe("20.5 in");
    expect(formatMeasurement(170.25, "cm", 400)).toBe("170.25 cm");
  });

  it("formats heights from age two as ft/in", () => {
    expect(formatMeasurement(64.75, "in", 400)).toBe("5 ft 4.75 in");
    expect(formatMeasurement(67.5, "in")).toBe("5 ft 7.5 in");
  });
});
