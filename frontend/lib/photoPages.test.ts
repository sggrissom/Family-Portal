import { describe, expect, it } from "vitest";
import * as server from "../server";
import {
  entryYear,
  firstUnloadedYear,
  isShownYear,
  mergeTimeline,
  yearRange,
  yearsToReach,
} from "./photoPages";

describe("timeline year windows", () => {
  const years = [2025, 2024, 2021, 2019];

  it("stops the newest-first run at the first gap", () => {
    expect(firstUnloadedYear(years, [2025, 2021], "newest")).toBe(2024);
    expect(firstUnloadedYear(years, [2025, 2024, 2021, 2019], "newest")).toBeNull();
  });

  it("walks from the other end when sorted oldest first", () => {
    expect(firstUnloadedYear(years, [2025, 2024], "oldest")).toBe(2019);
  });

  it("shows only the years on the loaded side of the stop", () => {
    expect(isShownYear(2025, 2024, "newest")).toBe(true);
    expect(isShownYear(2021, 2024, "newest")).toBe(false);
    expect(isShownYear(2019, 2021, "oldest")).toBe(true);
    expect(isShownYear(1990, null, "newest")).toBe(true);
  });

  it("loads every missing year between the edge and a jump target", () => {
    expect(yearsToReach(2019, years, [2025, 2021], "newest")).toEqual([2024, 2019]);
    expect(yearsToReach(2021, years, [2019], "oldest")).toEqual([2021]);
  });

  it("buckets by UTC year, as the server does", () => {
    expect(entryYear("2024-01-01T00:00:00Z")).toBe(2024);
    expect(yearRange(2019, 2021)).toMatchObject({ from: "2019-01-01", to: "2021-12-31" });
  });
});

describe("mergeTimeline", () => {
  const person = { id: 1 } as server.Person;
  const item = (milestones: number[], photos: number[] | null): server.FamilyTimelineItem => ({
    person,
    growthData: [],
    milestones: milestones.map(id => ({ id }) as server.Milestone),
    photos: (photos && photos.map(id => ({ id }) as server.Image)) as server.Image[],
  });
  const timeline = (people: server.FamilyTimelineItem[]): server.GetFamilyTimelineResponse => ({
    people,
    relations: [],
    years: [2024, 2023],
  });

  it("appends a window's entries to each person once", () => {
    const merged = mergeTimeline(timeline([item([1], [10])]), timeline([item([1, 2], [11])]));
    expect(merged.people[0].milestones.map(m => m.id)).toEqual([1, 2]);
    expect(merged.people[0].photos.map(p => p.id)).toEqual([10, 11]);
  });

  it("keeps a scope the caller can't see as null", () => {
    const merged = mergeTimeline(timeline([item([], null)]), timeline([item([], null)]));
    expect(merged.people[0].photos).toBeNull();
  });
});
