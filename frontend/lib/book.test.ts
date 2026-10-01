import { describe, expect, it } from "vitest";
import { assembleFirstYear, chapterTitle, firstBirthday, groupMonths, spreadPick } from "./book";
import { sampleSource } from "./bookFixtures";

describe("the first year's range", () => {
  it("ends the day before the first birthday", () => {
    expect(firstBirthday("2024-03-14T00:00:00Z")).toBe("2025-03-14");
    const book = assembleFirstYear(sampleSource("rich"));
    expect(book.dates).toBe("March 14, 2024 – March 13, 2025");
  });

  it("rolls a leap-day birthday to March 1", () => {
    expect(firstBirthday("2024-02-29T00:00:00Z")).toBe("2025-03-01");
  });

  it("keeps first-birthday records for a closing chapter", () => {
    const book = assembleFirstYear(sampleSource("rich"));
    const last = book.chapters[book.chapters.length - 1];
    expect(last.title).toBe("Turning one");
    expect(last.dates).toBe("March 14, 2025");
  });
});

describe("grouping quiet months", () => {
  it("folds quiet months into a neighbour and drops empty edges", () => {
    expect(groupMonths([9, 0, 0, 1, 8, 0, 0, 0, 0, 0, 9])).toEqual([[0], [3, 4], [10]]);
  });

  it("caps a chapter at three months", () => {
    expect(groupMonths([1, 1, 1, 1, 1, 1])).toEqual([
      [0, 1, 2],
      [3, 4, 5],
    ]);
  });

  it("names chapters by age", () => {
    expect(chapterTitle(1, 1)).toBe("One month");
    expect(chapterTitle(4, 6)).toBe("Four to six months");
  });
});

describe("picking photos", () => {
  it("spreads picks across days instead of one busy day", () => {
    const items = [
      ...Array.from({ length: 10 }, () => ({ day: "2024-05-01" })),
      { day: "2024-05-10" },
      { day: "2024-05-20" },
    ];
    const days = spreadPick(items, 3).map(p => p.day);
    expect(days).toEqual(["2024-05-01", "2024-05-10", "2024-05-20"]);
  });

  it("is deterministic and uses each photo at most once", () => {
    const a = assembleFirstYear(sampleSource("rich"));
    const b = assembleFirstYear(sampleSource("rich"));
    expect(a).toEqual(b);
    const ids = a.chapters.flatMap(c =>
      c.blocks.flatMap(block => {
        if (block.kind === "hero") return [block.photo.id];
        if (block.kind === "photos") return block.photos.map(p => p.id);
        if (block.kind === "artwork") return [block.photo.id];
        if (block.kind === "moment" && block.photo) return [block.photo.id];
        return [];
      })
    );
    expect(new Set(ids).size).toBe(ids.length);
    expect(ids).not.toContain(a.cover?.id);
  });

  it("uses more photos as the length grows", () => {
    const brief = assembleFirstYear(sampleSource("rich"), "brief").notes.photosUsed;
    const detailed = assembleFirstYear(sampleSource("rich"), "detailed").notes.photosUsed;
    expect(detailed).toBeGreaterThan(brief);
  });
});

describe("sparse and uneven records", () => {
  it("keeps every recorded milestone", () => {
    for (const sample of ["rich", "sparse", "uneven"] as const) {
      const notes = assembleFirstYear(sampleSource(sample)).notes;
      expect(notes.milestonesUsed).toBe(notes.milestonesInRange);
    }
  });

  it("says nothing about months with no records", () => {
    const book = assembleFirstYear(sampleSource("uneven"));
    expect(book.notes.hiddenMonths.length).toBeGreaterThan(0);
    expect(book.chapters.every(c => c.blocks.length > 0)).toBe(true);
  });

  it("only labels a measurement as at birth when it was taken that day", () => {
    const facts = (sample: "rich" | "sparse") =>
      assembleFirstYear(sampleSource(sample)).chapters[0].blocks.find(b => b.kind === "facts");
    expect(facts("rich")).toMatchObject({
      lines: expect.arrayContaining(["Weighed 7 lb 4 oz at birth"]),
    });
    expect(facts("sparse")).toMatchObject({
      lines: expect.arrayContaining(["Weighed 6 lb 12.8 oz at 3 days old"]),
    });
  });
});
