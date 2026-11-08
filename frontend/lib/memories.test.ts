import { beforeEach, describe, expect, it } from "vitest";
import * as server from "../server";
import { exactRecordTime } from "./recordTime";
import { currentVisit, markSeen, memoryHref, memoryNames, memoryWhen, saveVisit } from "./memories";

function memory(fields: Partial<server.Memory>): server.Memory {
  return {
    key: "m1",
    kind: "moment",
    personIds: [1],
    photoIds: [],
    text: "Walked",
    context: "",
    category: "first",
    date: "2021-06-04T00:00:00Z",
    recordTime: exactRecordTime("2021-06-04"),
    createdAt: "2026-11-01T00:00:00Z",
    milestoneId: 0,
    interviewId: 0,
    photoId: 0,
    eventId: 0,
    ...fields,
  };
}

const people = new Map<number, server.Person>([
  [1, { id: 1, name: "Clara Whitfield", birthday: "2019-03-04T00:00:00Z" } as server.Person],
  [2, { id: 2, name: "Rowan Whitfield", birthday: "2021-01-10T00:00:00Z" } as server.Person],
  [3, { id: 3, name: "Jake Whitfield", birthday: "2017-01-10T00:00:00Z" } as server.Person],
]);

describe("memoryWhen", () => {
  it("gives the date and the age for one person", () => {
    expect(memoryWhen(memory({}), people)).toBe("June 4, 2021 · 2 years 3 months");
  });

  it("leaves out the age when several people share the memory", () => {
    expect(memoryWhen(memory({ personIds: [1, 2] }), people)).toBe("June 4, 2021");
  });

  it("keeps an approximate date approximate", () => {
    const recordTime = {
      ...exactRecordTime("2019-01-01"),
      precision: "year",
      from: "2019",
      estimated: true,
    };
    expect(memoryWhen(memory({ recordTime }), people)).toBe("Around 2019");
  });
});

describe("memoryNames", () => {
  it("joins first names", () => {
    expect(memoryNames(memory({ personIds: [1] }), people)).toBe("Clara");
    expect(memoryNames(memory({ personIds: [1, 2] }), people)).toBe("Clara & Rowan");
    expect(memoryNames(memory({ personIds: [1, 2, 3] }), people)).toBe("Clara, Rowan & Jake");
  });
});

describe("memoryHref", () => {
  it("opens the record behind the memory", () => {
    expect(memoryHref(memory({ milestoneId: 9 }))).toBe("/milestone/9");
    expect(memoryHref(memory({ eventId: 4 }))).toBe("/competition/4");
    expect(memoryHref(memory({ photoId: 7 }))).toBe("/view-photo/7");
  });
});

describe("visits", () => {
  beforeEach(() => {
    sessionStorage.clear();
    localStorage.clear();
  });

  it("keeps the same seed and position for the whole visit", () => {
    const first = currentVisit();
    saveVisit({ ...first, index: 2 });
    expect(currentVisit()).toEqual({ ...first, index: 2 });
  });

  it("freezes what was seen at the start of the visit", () => {
    markSeen("m1");
    const visit = currentVisit();
    markSeen("m2");
    expect(visit.seen).toEqual(["m1"]);
    expect(currentVisit().seen).toEqual(["m1"]);
    sessionStorage.clear();
    expect(currentVisit().seen).toEqual(["m1", "m2"]);
  });
});
