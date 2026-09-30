import { describe, expect, it } from "vitest";
import * as server from "@app/server";
import { HISTORY_TYPES, historyView } from "./history";

const person = (id: number, name: string) =>
  ({ id, name, birthday: "2020-06-15T00:00:00Z", isPregnancy: false }) as server.Person;
const clara = person(1, "Clara");
const jake = person(2, "Jake");

const photo = (id: number, date: string, tagIds: number[] = []) =>
  ({ id, photoDate: date, tagIds }) as server.Image;

const timeline: server.GetFamilyTimelineResponse = {
  people: [
    {
      person: clara,
      milestones: [
        {
          id: 1,
          personId: 1,
          milestoneDate: "2026-03-01T00:00:00Z",
          tagIds: [9],
        } as server.Milestone,
        {
          id: 2,
          personId: 1,
          milestoneDate: "2024-03-01T00:00:00Z",
          tagIds: [] as number[],
        } as server.Milestone,
      ],
      growthData: [
        { id: 3, personId: 1, measurementDate: "2026-02-01T00:00:00Z" } as server.GrowthData,
      ],
      photos: [photo(10, "2026-04-01T00:00:00Z", [9]), photo(11, "2026-05-01T00:00:00Z")],
    },
    {
      person: jake,
      milestones: [],
      growthData: [],
      photos: [photo(10, "2026-04-01T00:00:00Z", [9])],
    },
  ],
  relations: [],
  years: [2026, 2024],
  appearances: [
    {
      detail: {
        appearance: { id: 5, occurredAt: "0001-01-01T00:00:00Z" },
        event: { id: 6, startDate: "2026-06-01T00:00:00Z" },
      } as server.AppearanceDetail,
      personIds: [2],
    },
  ],
  untaggedPhotos: [photo(20, "2026-07-01T00:00:00Z", [9]), photo(21, "2024-07-01T00:00:00Z")],
};

const all = { personIds: [], types: HISTORY_TYPES, tagIds: [] };

describe("historyView", () => {
  it("shows only the loaded years", () => {
    const view = historyView(timeline, all, 2024, "2026-09-27");
    expect(view.records.milestones.map(m => m.id)).toEqual([1]);
    expect(view.range).toEqual({ from: "2026-01-01", to: "2026-09-27" });
  });

  it("lists a photo once with everyone in it", () => {
    const view = historyView(timeline, all, null, "2026-09-27");
    const shared = view.records.photos.find(p => p.image.id === 10)!;
    expect(shared.people.map(p => p.name)).toEqual(["Clara", "Jake"]);
    expect(view.records.photos.map(p => p.image.id)).toEqual([10, 11, 20, 21]);
  });

  it("shows photos with nobody tagged in the loaded years", () => {
    const view = historyView(timeline, all, 2024, "2026-09-27");
    const untagged = view.records.photos.find(p => p.image.id === 20)!;
    expect(untagged.people).toEqual([]);
    expect(view.records.photos.map(p => p.image.id)).not.toContain(21);
  });

  it("filters to the chosen people", () => {
    const view = historyView(timeline, { ...all, personIds: [2] }, null, "2026-09-27");
    expect(view.records.milestones).toEqual([]);
    expect(view.records.photos.map(p => p.image.id)).toEqual([10]);
    expect(view.records.appearances?.map(a => a.detail.appearance.id)).toEqual([5]);
    expect(view.birthdayPeople.map(p => p.id)).toEqual([2]);
  });

  it("with tags, keeps only tagged milestones and photos", () => {
    const view = historyView(timeline, { ...all, tagIds: [9] }, null, "2026-09-27");
    expect(view.records.milestones.map(m => m.id)).toEqual([1]);
    expect(view.records.photos.map(p => p.image.id)).toEqual([10, 20]);
    expect(view.records.growth).toEqual([]);
    expect(view.records.appearances).toEqual([]);
    expect(view.range).toBeNull();
  });

  it("drops the types that are switched off", () => {
    const view = historyView(timeline, { ...all, types: ["measurements"] }, null, "2026-09-27");
    expect(view.records.growth.map(g => g.id)).toEqual([3]);
    expect(view.records.milestones).toEqual([]);
    expect(view.records.photos).toEqual([]);
    expect(view.range).toBeNull();
  });
});
