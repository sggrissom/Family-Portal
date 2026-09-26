import { describe, expect, it } from "vitest";
import {
  loadSequence,
  routeHasSequence,
  saveSequence,
  sequencePosition,
  swipeDirection,
  viewPhotoRoute,
} from "./photoSequence";

describe("sequencePosition", () => {
  it("finds neighbours in the middle of the list", () => {
    expect(sequencePosition([5, 9, 3], 9)).toEqual({ index: 1, total: 3, prevId: 5, nextId: 3 });
  });

  it("has no previous at the start and no next at the end", () => {
    expect(sequencePosition([5, 9, 3], 5)).toMatchObject({ prevId: 0, nextId: 9 });
    expect(sequencePosition([5, 9, 3], 3)).toMatchObject({ prevId: 9, nextId: 0 });
  });

  it("is null when the photo isn't in the list", () => {
    expect(sequencePosition([5, 9, 3], 4)).toBeNull();
  });
});

describe("saved sequence", () => {
  it("round-trips through session storage", () => {
    saveSequence({ ids: [1, 2, 3], backRoute: "/photos?people=4" });
    expect(loadSequence()).toEqual({ ids: [1, 2, 3], backRoute: "/photos?people=4" });
  });

  it("ignores a malformed entry", () => {
    sessionStorage.setItem("photoSequence", "{not json");
    expect(loadSequence()).toBeNull();
    sessionStorage.setItem("photoSequence", JSON.stringify({ ids: "1,2" }));
    expect(loadSequence()).toBeNull();
  });
});

describe("viewer route", () => {
  it("marks routes opened from a grid", () => {
    expect(viewPhotoRoute(7, true)).toBe("/view-photo/7?seq=1");
    expect(routeHasSequence("/view-photo/7?seq=1")).toBe(true);
    expect(routeHasSequence(viewPhotoRoute(7, false))).toBe(false);
  });
});

describe("swipeDirection", () => {
  it("swiping right goes back, left goes forward", () => {
    expect(swipeDirection(120, 10)).toBe("prev");
    expect(swipeDirection(-120, 10)).toBe("next");
  });

  it("ignores short or mostly vertical movement", () => {
    expect(swipeDirection(30, 0)).toBeNull();
    expect(swipeDirection(80, 70)).toBeNull();
  });
});
