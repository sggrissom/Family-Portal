import { describe, expect, it } from "vitest";
import { activeDestination, addPath, contextPersonId, legacyRedirect } from "./appNav";

describe("activeDestination", () => {
  it("maps each destination and its detail pages", () => {
    expect(activeDestination("/dashboard")).toBe("home");
    expect(activeDestination("/profile/12")).toBe("home");
    expect(activeDestination("/photos")).toBe("photos");
    expect(activeDestination("/view-photo/3")).toBe("photos");
    expect(activeDestination("/growth")).toBe("growth");
    expect(activeDestination("/view-growth/9")).toBe("growth");
    expect(activeDestination("/history")).toBe("history");
    expect(activeDestination("/chat")).toBe("chat");
  });

  it("does not match a longer word that shares a prefix", () => {
    expect(activeDestination("/photos-archive")).toBeNull();
    expect(activeDestination("/growthy")).toBeNull();
  });

  it("leaves pages outside the bar unmarked", () => {
    expect(activeDestination("/settings")).toBeNull();
    expect(activeDestination("/add-photo")).toBeNull();
  });
});

describe("legacyRedirect", () => {
  it("sends old URLs to their new names, keeping the rest of the route", () => {
    expect(legacyRedirect("/family-timeline")).toBe("/history");
    expect(legacyRedirect("/family-timeline?person=4")).toBe("/history?person=4");
    expect(legacyRedirect("/family-chart")).toBe("/growth");
    expect(legacyRedirect("/compare")).toBe("/same-age");
    expect(legacyRedirect("/person-activities/7")).toBe("/profile/7?tab=activities");
  });

  it("leaves current URLs alone", () => {
    expect(legacyRedirect("/history")).toBeNull();
    expect(legacyRedirect("/family-timelines")).toBeNull();
  });
});

describe("contextPersonId", () => {
  it("reads the person from pages about one person", () => {
    expect(contextPersonId("/profile/7")).toBe(7);
    expect(contextPersonId("/edit-person/7")).toBe(7);
  });

  it("is null elsewhere", () => {
    expect(contextPersonId("/dashboard")).toBeNull();
    expect(contextPersonId("/view-photo/7")).toBeNull();
  });
});

describe("addPath", () => {
  it("carries the person into the form", () => {
    expect(addPath("/add-growth", 5)).toBe("/add-growth/5");
    expect(addPath("/add-photo", null)).toBe("/add-photo");
  });
});
