import { describe, expect, it } from "vitest";
import * as server from "@app/server";
import {
  CheckupEntry,
  checkupValues,
  defaultHeightUnit,
  defaultWeightUnit,
  describeLast,
  latestOf,
  newUnitPrefs,
  rememberUnits,
  timeAgo,
} from "./checkup";

const growth = (
  id: number,
  type: server.MeasurementType,
  value: number,
  unit: string,
  date: string
): server.GrowthData => ({
  id,
  personId: 1,
  familyId: 1,
  measurementType: type,
  value,
  unit,
  measurementDate: date,
  createdAt: date,
});

const entry = (fields: Partial<CheckupEntry> = {}): CheckupEntry => ({
  heightUnit: "in",
  height: "",
  feet: "",
  inches: "",
  weightUnit: "lb",
  weight: "",
  pounds: "",
  ounces: "",
  ...fields,
});

describe("checkupValues", () => {
  it("needs at least one of height and weight", () => {
    expect(checkupValues(entry()).error).toBe("Enter a height, a weight, or both");
  });

  it("saves both from one entry", () => {
    expect(checkupValues(entry({ height: "38.5", weight: "32" }))).toEqual({
      height: { value: 38.5, unit: "in" },
      weight: { value: 32, unit: "lbs" },
      error: "",
    });
  });

  it("converts feet and inches, and pounds and ounces", () => {
    const { height, weight } = checkupValues(
      entry({
        heightUnit: "ft-in",
        feet: "3",
        inches: "2.5",
        weightUnit: "lb-oz",
        pounds: "7",
        ounces: "8",
      })
    );
    expect(height).toEqual({ value: 38.5, unit: "in" });
    expect(weight).toEqual({ value: 7.5, unit: "lbs" });
  });

  it("keeps centimetres as centimetres", () => {
    expect(checkupValues(entry({ heightUnit: "cm", height: "98" }))).toEqual({
      height: { value: 98, unit: "cm" },
      weight: null,
      error: "",
    });
  });

  it("rejects values that are not measurements", () => {
    expect(checkupValues(entry({ height: "abc" })).error).toBe("Enter a height above zero");
    expect(checkupValues(entry({ weight: "0" })).error).toBe("Enter a weight above zero");
    expect(checkupValues(entry({ weightUnit: "lb-oz", pounds: "7", ounces: "16" })).error).toBe(
      "Enter a weight in pounds and ounces under 16"
    );
  });
});

describe("units", () => {
  it("uses the person's own choice first", () => {
    const prefs = {
      ...newUnitPrefs(),
      height: { 1: "ft-in" as const },
      weight: { 1: "lb" as const },
    };
    expect(defaultHeightUnit(prefs, 1, [])).toBe("ft-in");
    expect(defaultWeightUnit(prefs, 1, 3)).toBe("lb");
  });

  it("then the unit of their last height", () => {
    expect(
      defaultHeightUnit(newUnitPrefs(), 1, [growth(1, server.Height, 98, "cm", "2026-01-01")])
    ).toBe("cm");
  });

  it("weighs babies in pounds and ounces", () => {
    expect(defaultWeightUnit({ ...newUnitPrefs(), lastWeight: "lb" }, 1, 4)).toBe("lb-oz");
    expect(defaultWeightUnit({ ...newUnitPrefs(), lastWeight: "lb" }, 1, 40)).toBe("lb");
  });

  it("falls back to what the family used last", () => {
    expect(defaultHeightUnit({ ...newUnitPrefs(), lastHeight: "cm" }, 1, [])).toBe("cm");
    expect(defaultWeightUnit(newUnitPrefs(), 1, null)).toBe("lb");
  });

  it("remembers only the units that were used", () => {
    const next = rememberUnits(
      newUnitPrefs(),
      5,
      { height: null, weight: { value: 7.5, unit: "lbs" } },
      entry({ heightUnit: "cm", weightUnit: "lb-oz" })
    );
    expect(next).toEqual({
      height: {},
      weight: { 5: "lb-oz" },
      lastHeight: null,
      lastWeight: "lb-oz",
    });
  });
});

describe("describeLast", () => {
  const now = new Date("2026-09-27T12:00:00Z");

  it("picks the latest measurement of the type", () => {
    const list = [
      growth(1, server.Height, 36, "in", "2026-01-01T00:00:00Z"),
      growth(2, server.Height, 37.75, "in", "2026-05-20T00:00:00Z"),
      growth(3, server.Weight, 30, "lbs", "2026-08-01T00:00:00Z"),
    ];
    expect(latestOf(list, server.Height)?.id).toBe(2);
    expect(describeLast(latestOf(list, server.Height), 38, now)).toBe(
      "last: 3 ft 1.75 in, 4 months ago"
    );
  });

  it("is empty with nothing to show", () => {
    expect(describeLast(null, null, now)).toBe("");
  });

  it("says how long ago in words", () => {
    expect(timeAgo("2026-09-27T08:00:00Z", now)).toBe("today");
    expect(timeAgo("2026-09-20T08:00:00Z", now)).toBe("7 days ago");
    expect(timeAgo("2026-08-20T08:00:00Z", now)).toBe("5 weeks ago");
    expect(timeAgo("2023-09-20T08:00:00Z", now)).toBe("3 years ago");
  });
});
