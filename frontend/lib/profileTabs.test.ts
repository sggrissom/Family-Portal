import { describe, expect, it } from "vitest";
import { ProfileContent, profileTabs, shownTab } from "./profileTabs";

const empty: ProfileContent = {
  isPregnancy: false,
  hasBirthday: true,
  quotes: 0,
  artwork: 0,
  interviews: 0,
  seasons: 0,
};

describe("profile tabs", () => {
  it("shows only story, photos and growth when nothing else is recorded", () => {
    expect(profileTabs(empty)).toEqual(["story", "photos", "growth"]);
  });

  it("adds a tab once it has something in it", () => {
    expect(profileTabs({ ...empty, quotes: 2, interviews: 1, seasons: 1, artwork: 3 })).toEqual([
      "story",
      "quotes",
      "artwork",
      "interviews",
      "photos",
      "growth",
      "activities",
    ]);
  });

  it("drops growth without a birthday", () => {
    expect(profileTabs({ ...empty, hasBirthday: false })).toEqual(["story", "photos"]);
  });

  it("keeps a pregnancy to story and photos", () => {
    expect(profileTabs({ ...empty, isPregnancy: true, quotes: 1 })).toEqual(["story", "photos"]);
  });

  it("falls back to story for a hidden or unknown tab", () => {
    const tabs = profileTabs(empty);
    expect(shownTab("photos", tabs)).toBe("photos");
    expect(shownTab("interviews", tabs)).toBe("story");
    expect(shownTab("nonsense", tabs)).toBe("story");
    expect(shownTab(null, tabs)).toBe("story");
  });
});
