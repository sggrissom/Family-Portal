import { describe, expect, it } from "vitest";
import { ProfileContent, invitesInterview, profileTabs, shownTab } from "./profileTabs";

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

describe("interview invitation", () => {
  const child = { birthday: "2022-06-15T00:00:00Z", isPregnancy: false };

  it("invites editors once the person is three and has no interviews", () => {
    expect(invitesInterview(child, 0, true, "2025-06-15")).toBe(true);
  });

  it("waits until the third birthday", () => {
    expect(invitesInterview(child, 0, true, "2025-06-14")).toBe(false);
  });

  it("stays away once there is an interview, for viewers, and for pregnancies", () => {
    expect(invitesInterview(child, 1, true, "2026-01-01")).toBe(false);
    expect(invitesInterview(child, 0, false, "2026-01-01")).toBe(false);
    expect(invitesInterview({ ...child, isPregnancy: true }, 0, true, "2026-01-01")).toBe(false);
    expect(invitesInterview({ ...child, birthday: "" }, 0, true, "2026-01-01")).toBe(false);
  });
});
