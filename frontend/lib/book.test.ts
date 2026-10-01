import { describe, expect, it } from "vitest";
import * as server from "../server";
import {
  Book,
  additionsSince,
  assembleBook,
  candidatesIn,
  categoryOf,
  sectionOf,
  chapterTitle,
  firstBirthday,
  groupMonths,
  resolveSource,
  spreadPick,
  suggestItems,
} from "./book";
import { sampleBook, sampleSource } from "./bookFixtures";

const build = (
  name: "rich" | "sparse" | "uneven",
  density: "brief" | "balanced" | "detailed" = "balanced"
) => {
  const { source, selection } = sampleBook(name, density);
  return assembleBook(source, selection);
};

const photoIds = (book: Book) =>
  book.chapters.flatMap(c =>
    c.blocks.flatMap(block => {
      if (block.kind === "hero") return [block.photo.id];
      if (block.kind === "photos") return block.photos.map(p => p.id);
      if (block.kind === "artwork") return [block.photo.id];
      if (block.kind === "moment" && block.photo) return [block.photo.id];
      return [];
    })
  );

describe("the first year's range", () => {
  it("ends the day before the first birthday", () => {
    expect(firstBirthday("2024-03-14T00:00:00Z")).toBe("2025-03-14");
    expect(build("rich").dates).toBe("March 14, 2024 – March 13, 2025");
  });

  it("rolls a leap-day birthday to March 1", () => {
    expect(firstBirthday("2024-02-29T00:00:00Z")).toBe("2025-03-01");
  });

  it("keeps first-birthday records for a closing chapter", () => {
    const book = build("rich");
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

describe("suggesting a draft", () => {
  it("spreads picks across days instead of one busy day", () => {
    const items = [
      ...Array.from({ length: 10 }, () => ({ day: "2024-05-01" })),
      { day: "2024-05-10" },
      { day: "2024-05-20" },
    ];
    expect(spreadPick(items, 3).map(p => p.day)).toEqual([
      "2024-05-01",
      "2024-05-10",
      "2024-05-20",
    ]);
  });

  it("is deterministic and uses each photo at most once", () => {
    const a = build("rich");
    expect(a).toEqual(build("rich"));
    const ids = photoIds(a);
    expect(new Set(ids).size).toBe(ids.length);
    expect(ids).not.toContain(a.cover?.id);
  });

  it("uses more photos as the length grows", () => {
    expect(build("rich", "detailed").notes.photosUsed).toBeGreaterThan(
      build("rich", "brief").notes.photosUsed
    );
  });

  it("keeps every recorded milestone", () => {
    for (const name of ["rich", "sparse", "uneven"] as const) {
      const notes = build(name).notes;
      expect(notes.milestonesUsed).toBe(notes.milestonesInRange);
    }
  });

  it("keeps pinned photos, leaves removed ones out, and keeps the order it already had", () => {
    const { source, selection } = sampleBook("rich", "detailed");
    const r = resolveSource(source, selection);
    const photos = selection.items.filter(i => i.kind === server.BookItemPhoto);
    const pinned = { ...photos[5], pinned: true };
    const removed = photos[6];
    const reordered = [pinned, ...selection.items.filter(i => i !== photos[5])];

    const next = suggestItems(r, {
      density: "brief",
      current: reordered,
      excluded: [removed],
      coverPhotoId: selection.coverPhotoId,
    });
    const keys = next.items.map(i => `${i.kind}:${i.sourceId}`);
    expect(next.items[0]).toEqual(pinned);
    expect(keys).not.toContain(`${removed.kind}:${removed.sourceId}`);
    expect(next.items.filter(i => i.kind === server.BookItemPhoto).length).toBeLessThan(
      photos.length
    );
    expect(next.coverPhotoId).toBe(selection.coverPhotoId);
  });
});

describe("reading a saved book", () => {
  it("only shows what was saved, so new uploads do not rearrange it", () => {
    const { source, selection } = sampleBook("rich");
    const before = assembleBook(source, selection);
    const added: server.Image = {
      ...source.photos[0],
      id: -999,
      photoDate: "2024-06-01T00:00:00Z",
    };
    const after = assembleBook({ ...source, photos: [...source.photos, added] }, selection);
    expect(photoIds(after)).toEqual(photoIds(before));
  });

  it("follows the saved order, so moving a photo first makes it the chapter's opening photo", () => {
    const { source, selection } = sampleBook("rich");
    const book = assembleBook(source, selection);
    const chapter = book.chapters.find(c => c.blocks[0].kind === "hero" && c.items.length > 4)!;
    const lastPhoto = [...chapter.items]
      .reverse()
      .find(i => selection.items[i].kind === server.BookItemPhoto)!;
    const moved = selection.items[lastPhoto];
    const items = selection.items.filter(i => i !== moved);
    items.splice(chapter.items[0], 0, moved);

    const reread = assembleBook(source, { ...selection, items });
    const same = reread.chapters.find(c => c.id === chapter.id)!;
    expect(same.blocks[0]).toMatchObject({ kind: "hero", photo: { id: moved.sourceId } });
  });

  it("drops deleted records and counts them for the editor", () => {
    const { source, selection } = sampleBook("rich");
    const gone = source.milestones[3].id;
    const book = assembleBook(
      { ...source, milestones: source.milestones.filter(m => m.id !== gone) },
      selection
    );
    expect(book.notes.missing).toBe(1);
  });

  it("uses a book-only caption over the photo's own", () => {
    const { source, selection } = sampleBook("rich");
    const items = selection.items.map(i =>
      i.kind === server.BookItemPhoto ? { ...i, caption: "Only in the book" } : i
    );
    const book = assembleBook(source, { ...selection, items });
    const hero = book.chapters.flatMap(c => c.blocks).find(b => b.kind === "hero");
    expect(hero).toMatchObject({ photo: { caption: "Only in the book" } });
    expect(sampleSource("rich").photos.every(p => p.description !== "Only in the book")).toBe(true);
  });

  it("says nothing about months with no records", () => {
    const book = build("uneven");
    expect(book.notes.hiddenMonths.length).toBeGreaterThan(0);
    expect(book.chapters.every(c => c.blocks.length > 0)).toBe(true);
  });

  it("only labels a measurement as at birth when it was taken that day", () => {
    const facts = (name: "rich" | "sparse") =>
      build(name).chapters[0].blocks.find(b => b.kind === "facts");
    expect(facts("rich")).toMatchObject({
      lines: expect.arrayContaining(["Weighed 7 lb 4 oz at birth"]),
    });
    expect(facts("sparse")).toMatchObject({
      lines: expect.arrayContaining(["Weighed 6 lb 12.8 oz at 3 days old"]),
    });
  });

  it("leaves the growth chapter out when it is switched off", () => {
    const { source, selection } = sampleBook("rich");
    const book = assembleBook(source, { ...selection, showGrowth: false });
    expect(book.chapters.map(c => c.id)).not.toContain("growth");
  });
});

describe("books about a stretch of time", () => {
  const family = () => sampleBook("family");

  it("names calendar chapters by month", () => {
    const { source, selection } = family();
    const book = assembleBook(source, selection);
    const months = book.chapters.filter(c => c.id.startsWith("together-")).map(c => c.title);
    expect(months.length).toBeGreaterThan(1);
    expect(months.every(t => /^[A-Z][a-z]+( – [A-Z][a-z]+)? 2025$/.test(t))).toBe(true);
    expect(book.dates).toBe("January 1, 2025 – December 31, 2025");
  });

  it("puts a photo of both children in the shared chapters once, not in each child's section", () => {
    const { source, selection } = family();
    const book = assembleBook(source, selection);
    const r = resolveSource(source, selection);
    const both = new Set(
      Object.entries(source.photoPeople)
        .filter(([, people]) => people.length === 2)
        .map(([id]) => Number(id))
    );
    for (const chapter of book.chapters) {
      for (const index of chapter.items) {
        const item = selection.items[index];
        if (item.kind !== server.BookItemPhoto || !both.has(item.sourceId)) continue;
        expect(chapter.id.startsWith("together-")).toBe(true);
        expect(sectionOf(r, item)).toBe(0);
      }
    }
    const ids = photoIds(book);
    expect(new Set(ids).size).toBe(ids.length);
    expect(book.chapters.map(c => c.title)).toEqual(expect.arrayContaining(["Juniper", "Theo"]));
  });

  it("gives each child their own section with their milestones and photos", () => {
    const { source, selection } = family();
    const book = assembleBook(source, selection);
    const theo = book.chapters.find(c => c.title === "Theo")!;
    expect(theo.dates).toBe("Age 3 to 4");
    const people = theo.items.map(i =>
      sectionOf(resolveSource(source, selection), selection.items[i])
    );
    expect(new Set(people)).toEqual(new Set([2]));
  });

  it("leaves photos with nobody tagged for the editor to add by hand", () => {
    const { source, selection } = family();
    const untagged = new Set(source.untagged);
    expect(
      selection.items.some(i => i.kind === server.BookItemPhoto && untagged.has(i.sourceId))
    ).toBe(false);
  });

  it("can ask for only photos with everyone in them", () => {
    const { source, selection } = family();
    const r = resolveSource(source, { ...selection, match: "all" });
    const picked = suggestItems(r, { density: "detailed" }).items.filter(
      i => i.kind === server.BookItemPhoto
    );
    expect(picked.length).toBeGreaterThan(0);
    expect(picked.every(i => source.photoPeople[i.sourceId].length === 2)).toBe(true);
  });

  it("follows the chosen kinds of content", () => {
    const { source, selection } = family();
    const r = resolveSource(source, { ...selection, categories: ["quotes"] });
    const items = suggestItems(r, { density: "balanced" }).items;
    expect(items.length).toBe(2);
    expect(items.every(i => categoryOf(r, i) === "quotes")).toBe(true);
  });

  it("offers records added after the last review, and only those", () => {
    const { source, selection } = family();
    const r = resolveSource(source, selection);
    const reviewedAt = "2026-06-01T00:00:00-05:00";
    const fresh = additionsSince(r, reviewedAt, selection.items, []);
    expect(fresh.length).toBe(0);

    const late: server.Milestone = {
      ...source.milestones[0],
      id: 99,
      createdAt: "2026-06-02T00:00:00Z",
      milestoneDate: "2025-05-20T00:00:00Z",
    };
    const withLate = { ...source, milestones: [...source.milestones, late] };
    const r2 = resolveSource(withLate, selection);
    expect(additionsSince(r2, reviewedAt, selection.items, []).map(i => i.sourceId)).toEqual([99]);
    expect(
      additionsSince(r2, reviewedAt, selection.items, [
        { ...additionsSince(r2, reviewedAt, selection.items, [])[0] },
      ])
    ).toEqual([]);
  });

  it("ends a year book the day before its end date", () => {
    const { source, selection } = family();
    const r = resolveSource(source, selection);
    const nextYear = { ...source.photos[0], id: -500, photoDate: "2026-01-01T00:00:00Z" };
    const r2 = resolveSource({ ...source, photos: [...source.photos, nextYear] }, selection);
    expect(candidatesIn(r2).photos.length).toBe(candidatesIn(r).photos.length);
  });
});
