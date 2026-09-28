import { describe, expect, it } from "vitest";
import * as server from "@app/server";
import {
  ChartPoint,
  ageTicks,
  chartDomain,
  chartPoints,
  clampRange,
  niceTicks,
  percentileBand,
  toDisplay,
} from "./ageChart";

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

  it("labels zoomed-in ages past two with years and months", () => {
    expect(ageTicks(30, 42).map(t => t.label)).toEqual(["2y 6m", "2y 9m", "3y", "3y 3m", "3y 6m"]);
    expect(ageTicks(12, 16).map(t => t.label)).toEqual(["12m", "13m", "14m", "15m", "16m"]);
  });
});

const pts = (...pairs: [number, number][]): ChartPoint[] =>
  pairs.map(([ageMonths, value], i) => ({ id: i, ageMonths, value }));

describe("chartDomain", () => {
  const a = pts([0, 20], [12, 30], [24, 34], [60, 44]);
  const b = pts([0, 19], [12, 29], [24, 33.5], [60, 43]);

  it("fits every point when not zoomed", () => {
    const d = chartDomain([a, b], [], null)!;
    expect(d.zoomed).toBe(false);
    expect(d.minAge).toBe(0);
    expect(d.maxAge).toBe(61);
    expect(d.minValue).toBeLessThan(19);
    expect(d.maxValue).toBeGreaterThan(44);
  });

  it("refits values to the zoomed ages, including where lines cross the edges", () => {
    const d = chartDomain([a, b], [], { from: 18, to: 30 })!;
    expect(d.zoomed).toBe(true);
    expect(d.minAge).toBe(18);
    expect(d.maxAge).toBe(30);
    expect(d.minValue).toBeGreaterThan(30);
    expect(d.maxValue).toBeLessThan(37);
  });

  it("keeps band rows just past each edge so the band reaches the axes", () => {
    const band = percentileBand(1, "height", 0, 60).filter(r => r.ageMonths % 3 === 0);
    const d = chartDomain([a], band, { from: 19, to: 29 })!;
    expect(d.band[0].ageMonths).toBe(18);
    expect(d.band[d.band.length - 1].ageMonths).toBe(30);
  });
});

describe("clampRange", () => {
  it("orders and clamps to the data", () => {
    expect(clampRange({ from: 30, to: 12 }, 0, 20)).toEqual({ from: 12, to: 20 });
    expect(clampRange({ from: 15, to: 5 }, 0, 20)).toEqual({ from: 5, to: 15 });
  });

  it("widens a tiny selection to a month", () => {
    expect(clampRange({ from: 10, to: 10.2 }, 0, 20)).toEqual({ from: 9.6, to: 10.6 });
  });

  it("ignores a selection covering everything", () => {
    expect(clampRange({ from: 0, to: 20 }, 0, 20)).toBeNull();
  });
});
