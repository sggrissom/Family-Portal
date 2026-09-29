import { describe, expect, it } from "vitest";
import * as server from "@app/server";
import { birthdaysBetween, dayKey, dayLabel, groupByMonth, summarizeDays } from "./daySummary";

const person = (
  id: number,
  name: string,
  birthday: string,
  isPregnancy = false
): server.Person => ({
  id,
  familyId: 1,
  name,
  gender: server.Unknown,
  birthday,
  age: "",
  profilePhotoId: 0,
  profileCropX: 0,
  profileCropY: 0,
  profileCropScale: 1,
  isPregnancy,
  relationship: "",
});

const growth = (
  id: number,
  personId: number,
  type: server.MeasurementType,
  value: number,
  date: string
): server.GrowthData => ({
  id,
  personId,
  familyId: 1,
  measurementType: type,
  value,
  unit: type === server.Height ? "in" : "lbs",
  measurementDate: date,
  createdAt: date,
});

const photo = (id: number, date: string, people: server.Person[]): server.PhotoWithPeople => ({
  image: { id, photoDate: date } as server.Image,
  people,
  similar: [],
});

const milestone = (id: number, personId: number, date: string): server.Milestone =>
  ({ id, personId, milestoneDate: date, description: "First steps" }) as server.Milestone;

const clara = person(1, "Clara", "2023-05-02T00:00:00Z");
const jake = person(2, "Jake", "2019-09-26T00:00:00Z");

describe("summarizeDays", () => {
  it("merges a height and weight on one day into one checkup", () => {
    const days = summarizeDays(
      {
        photos: [],
        milestones: [],
        growth: [
          growth(1, 1, server.Height, 38.5, "2026-09-20T00:00:00Z"),
          growth(2, 1, server.Weight, 32, "2026-09-20T00:00:00Z"),
        ],
      },
      [clara]
    );
    expect(days).toHaveLength(1);
    expect(days[0].checkups).toEqual([
      expect.objectContaining({
        personId: 1,
        height: expect.objectContaining({ id: 1 }),
        weight: expect.objectContaining({ id: 2 }),
      }),
    ]);
  });

  it("turns a day of photos into one mosaic of the newest four", () => {
    const photos = [1, 2, 3, 4, 5, 6].map(id =>
      photo(id, `2026-09-21T0${id}:00:00Z`, id % 2 ? [clara] : [jake])
    );
    const [day] = summarizeDays({ photos, milestones: [], growth: [] }, [clara, jake]);
    expect(day.photos).toEqual({ count: 6, photoIds: [6, 5, 4, 3], personIds: [2, 1] });
  });

  it("orders days newest first and keeps milestones whole", () => {
    const days = summarizeDays(
      {
        photos: [photo(9, "2026-09-18T12:00:00Z", [])],
        milestones: [milestone(4, 1, "2026-09-22T00:00:00Z")],
        growth: [],
      },
      [clara]
    );
    expect(days.map(d => d.day)).toEqual(["2026-09-22", "2026-09-18"]);
    expect(days[0].milestones.map(m => m.id)).toEqual([4]);
  });

  it("adds birthdays in the range, even on quiet days", () => {
    const days = summarizeDays({ photos: [], milestones: [], growth: [] }, [clara, jake], {
      from: "2026-09-14",
      to: "2026-09-27",
    });
    expect(days).toEqual([
      {
        day: "2026-09-26",
        birthdays: [{ personId: 2, age: 7 }],
        milestones: [],
        events: [],
        checkups: [],
        photos: null,
      },
    ]);
  });
});

describe("activity events", () => {
  const appearance = (id: number, eventId: number, occurredAt: string, startDate: string) =>
    ({
      detail: {
        appearance: { id, occurredAt },
        event: { id: eventId, startDate },
        entry: { name: `Entry ${id}` },
        results: [],
        photoIds: [],
      },
      personIds: [1],
    }) as unknown as server.TimelineAppearance;

  it("puts appearances at one event on one card, dated by when they happened", () => {
    const days = summarizeDays(
      {
        photos: [],
        growth: [],
        milestones: [],
        appearances: [
          appearance(1, 10, "2026-09-20T15:00:00Z", "2026-09-19T00:00:00Z"),
          appearance(2, 10, "2026-09-20T17:00:00Z", "2026-09-19T00:00:00Z"),
          appearance(3, 11, "0001-01-01T00:00:00Z", "2026-09-12T00:00:00Z"),
        ],
      },
      []
    );
    expect(days.map(d => [d.day, d.events.map(e => [e.event.id, e.appearances.length])])).toEqual([
      ["2026-09-20", [[10, 2]]],
      ["2026-09-12", [[11, 1]]],
    ]);
  });
});

describe("groupByMonth", () => {
  it("keeps days in order under their month", () => {
    const day = (d: string) => ({
      day: d,
      birthdays: [],
      milestones: [],
      events: [],
      checkups: [],
      photos: null,
    });
    const groups = groupByMonth([day("2026-09-20"), day("2026-09-02"), day("2026-08-30")]);
    expect(groups.map(g => [g.label, g.days.length])).toEqual([
      ["September 2026", 2],
      ["August 2026", 1],
    ]);
  });
});

describe("birthdaysBetween", () => {
  it("skips pregnancies and the day someone was born", () => {
    const due = person(3, "Baby", "2026-09-20T00:00:00Z", true);
    const newborn = person(4, "Newborn", "2026-09-21T00:00:00Z");
    expect(birthdaysBetween([due, newborn], "2026-09-14", "2026-09-27").size).toBe(0);
  });
});

describe("dayKey", () => {
  it("uses the UTC calendar day", () => {
    expect(dayKey("2026-09-20T23:30:00Z")).toBe("2026-09-20");
    expect(dayKey("not a date")).toBe("");
  });
});

describe("dayLabel", () => {
  it("names recent days and dates older ones", () => {
    expect(dayLabel("2026-09-27", "2026-09-27")).toBe("Today");
    expect(dayLabel("2026-09-26", "2026-09-27")).toBe("Yesterday");
    expect(dayLabel("2026-09-19", "2026-09-27")).toBe("Saturday, Sep 19");
    expect(dayLabel("2025-12-31", "2026-01-02")).toBe("Wednesday, Dec 31, 2025");
  });
});
