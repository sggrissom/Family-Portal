import { describe, expect, it } from "vitest";
import * as server from "@app/server";
import { ageTicks, chartPoints, niceTicks, percentileBand, toDisplay } from "./ageChart";

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

describe("chartPoints", () => {
  it("plots one metric by age, in inches and pounds, oldest first", () => {
    const points = chartPoints(
      [
        growth(2, server.Height, 97.79, "cm", "2026-09-02T00:00:00Z"),
        growth(1, server.Height, 30, "in", "2024-05-02T00:00:00Z"),
        growth(3, server.Weight, 32, "lbs", "2026-09-02T00:00:00Z"),
      ],
      "2023-05-02T00:00:00Z",
      "height"
    );
    expect(points.map(p => p.id)).toEqual([1, 2]);
    expect(points[0].ageMonths).toBe(12);
    expect(points[1].ageMonths).toBe(40);
    expect(points[1].value).toBeCloseTo(38.5, 1);
  });
});

describe("toDisplay", () => {
  it("converts metric to inches and pounds", () => {
    expect(toDisplay(254, "cm")).toBeCloseTo(100);
    expect(toDisplay(10, "kg")).toBeCloseTo(22.05, 1);
    expect(toDisplay(40, "in")).toBe(40);
  });
});

describe("percentileBand", () => {
  it("covers the requested ages in display units, ordered by percentile", () => {
    const band = percentileBand(1, "height", 0, 24);
    expect(band[0].ageMonths).toBe(0);
    expect(band[band.length - 1].ageMonths).toBe(24);
    for (const row of band) {
      expect(row.p3).toBeLessThan(row.p50);
      expect(row.p50).toBeLessThan(row.p97);
    }
    expect(band[0].p50).toBeCloseTo(19.3, 0);
  });

  it("stops at twenty years", () => {
    const band = percentileBand(0, "weight", 200, 400);
    expect(band[band.length - 1].ageMonths).toBe(240);
  });
});

describe("ticks", () => {
  it("picks round values", () => {
    expect(niceTicks(0, 50)).toEqual([0, 10, 20, 30, 40, 50]);
    expect(niceTicks(30, 41)).toEqual([30, 32, 34, 36, 38, 40]);
  });

  it("labels ages in months for babies and years after", () => {
    expect(ageTicks(0, 12).map(t => t.label)).toEqual(["0m", "3m", "6m", "9m", "12m"]);
    expect(ageTicks(0, 60).map(t => t.label)).toEqual(["0y", "1y", "2y", "3y", "4y", "5y"]);
  });
});
