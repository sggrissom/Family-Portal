import { describe, expect, it } from "vitest";
import * as server from "@app/server";
import { summarizeDays } from "./daySummary";
import { chapterTitle, storyChapters } from "./story";

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

const birthday = "2023-05-02T00:00:00Z";

describe("storyChapters", () => {
  const days = summarizeDays(
    {
      photos: [
        {
          image: { id: 1, photoDate: "2023-04-20T00:00:00Z" } as server.Image,
          people: [],
          similar: [],
        },
      ],
      milestones: [],
      growth: [
        growth(1, server.Height, 35, "in", "2026-05-10T00:00:00Z"),
        growth(2, server.Weight, 28, "lbs", "2026-05-10T00:00:00Z"),
        growth(3, server.Height, 38, "in", "2026-09-20T00:00:00Z"),
        growth(4, server.Weight, 32, "lbs", "2026-09-20T00:00:00Z"),
        growth(5, server.Height, 33, "in", "2025-12-01T00:00:00Z"),
      ],
    },
    []
  );

  it("groups days by age, newest first", () => {
    const chapters = storyChapters(days, birthday);
    expect(chapters.map(c => c.age)).toEqual([3, 2, null]);
    expect(chapters.map(c => chapterTitle(c.age))).toEqual(["Age 3", "Age 2", "Before birth"]);
  });

  it("says how much they grew within an age", () => {
    const [age3, age2] = storyChapters(days, birthday);
    expect(age3.grew).toBe("Grew 3 in and 4 lb");
    expect(age2.grew).toBe("");
  });

  it("names the first year", () => {
    expect(chapterTitle(0)).toBe("First year");
  });
});
