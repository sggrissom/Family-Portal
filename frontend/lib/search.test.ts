import { describe, expect, it } from "vitest";
import * as server from "../server";
import {
  activityHref,
  addRecent,
  dateLabel,
  fold,
  highlightPieces,
  highlightRanges,
  nextIndex,
  photosHref,
  queryOfRoute,
  searchRoute,
  searchTerms,
  understoodChips,
} from "./search";

const understood = (over: Partial<server.SearchUnderstood>): server.SearchUnderstood => ({
  personIds: [],
  ageFromMonth: null,
  ageToMonth: null,
  ageLabel: "",
  year: 0,
  month: 0,
  category: "",
  tagIds: [],
  text: "",
  ...over,
});

describe("fold and searchTerms", () => {
  it("drops case, accents and filler", () => {
    expect(fold("Café ZOË")).toBe("cafe zoe");
    expect(searchTerms("The walk at the Café!")).toEqual(["walk", "cafe"]);
    expect(searchTerms("mother-in-law")).toEqual(["mother", "law"]);
  });
});

describe("highlightRanges", () => {
  it("marks the matched prefix of each word", () => {
    expect(highlightRanges("Walked to the park, walking", ["walk"])).toEqual([
      [0, 4],
      [20, 24],
    ]);
  });

  it("does not match inside a word", () => {
    expect(highlightRanges("Sidewalk chalk", ["walk"])).toEqual([]);
  });

  it("maps folded matches back onto accented text", () => {
    const text = "At the Café Zoë";
    expect(highlightPieces(text, ["cafe", "zo"])).toEqual([
      { text: "At the ", mark: false },
      { text: "Café", mark: true },
      { text: " ", mark: false },
      { text: "Zo", mark: true },
      { text: "ë", mark: false },
    ]);
  });

  it("uses the longest term that fits", () => {
    expect(highlightRanges("parkway", ["par", "park"])).toEqual([[0, 4]]);
  });

  it("returns the text unmarked without terms", () => {
    expect(highlightPieces("hello", [])).toEqual([{ text: "hello", mark: false }]);
  });
});

describe("addRecent", () => {
  it("puts the newest first, dedupes ignoring case and caps the list", () => {
    let recent: string[] = [];
    for (const q of ["a1", "a2", "a3", "a4", "a5", "a6", "a7", "a8", "a9"]) {
      recent = addRecent(recent, q);
    }
    expect(recent).toHaveLength(8);
    expect(recent[0]).toBe("a9");
    expect(addRecent(["Beach", "ada"], " beach ")).toEqual(["beach", "ada"]);
    expect(addRecent(["ada"], "  ")).toEqual(["ada"]);
  });
});

describe("understoodChips", () => {
  const people = [
    { id: 1, name: "Ada Grissom" },
    { id: 3, name: "Ada Smith" },
  ] as server.Person[];
  const tags = [{ id: 7, name: "Beach" }] as server.Tag[];

  it("labels one chip per kind", () => {
    const chips = understoodChips(
      understood({
        personIds: [1, 3],
        ageLabel: "age 2",
        year: 2024,
        month: 6,
        category: "quote",
        tagIds: [7],
      }),
      people,
      tags
    );
    expect(chips).toEqual([
      { kind: "person", label: "Ada or Ada" },
      { kind: "age", label: "age 2" },
      { kind: "date", label: "June 2024" },
      { kind: "category", label: "Quotes" },
      { kind: "tag", label: "Beach" },
    ]);
  });

  it("is empty when nothing was understood", () => {
    expect(understoodChips(understood({ text: "walk" }), people, tags)).toEqual([]);
  });
});

describe("dateLabel", () => {
  it("handles a year, a month, or both", () => {
    expect(dateLabel(2024, 0)).toBe("2024");
    expect(dateLabel(0, 12)).toBe("December");
    expect(dateLabel(2023, 2)).toBe("February 2023");
  });
});

describe("routes", () => {
  it("round-trips the query through the route", () => {
    expect(searchRoute("  ada & ben ")).toBe("/search?q=ada%20%26%20ben");
    expect(queryOfRoute(searchRoute("ada & ben"))).toBe("ada & ben");
    expect(searchRoute("")).toBe("/search");
    expect(queryOfRoute("/search")).toBe("");
  });

  it("builds the Photos page link from what was understood", () => {
    expect(photosHref(understood({ personIds: [1, 3], text: "beach" }))).toBe(
      "/photos?people=1,3&q=beach"
    );
    expect(photosHref(understood({ year: 2024 }))).toBe("/photos?from=2024-01-01&to=2024-12-31");
    expect(photosHref(understood({ year: 2024, month: 2, tagIds: [7] }))).toBe(
      "/photos?tags=7&from=2024-02-01&to=2024-02-29"
    );
  });

  it("links each kind of activity hit", () => {
    const hit = (kind: string) => ({ kind, id: 4, name: "", detail: "", date: "" });
    expect(activityHref(hit("season"))).toBe("/season/4");
    expect(activityHref(hit("event"))).toBe("/competition/4");
    expect(activityHref(hit("activity"))).toBe("/activities");
  });
});

describe("nextIndex", () => {
  it("steps within bounds and enters from either end", () => {
    expect(nextIndex(-1, 1, 3)).toBe(0);
    expect(nextIndex(-1, -1, 3)).toBe(2);
    expect(nextIndex(2, 1, 3)).toBe(2);
    expect(nextIndex(0, -1, 3)).toBe(0);
    expect(nextIndex(0, 1, 0)).toBe(-1);
  });
});
