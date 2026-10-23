import { describe, expect, it } from "vitest";
import { alreadySaved, linkIsFresh, saveSummary } from "./saveVideos";

describe("saving videos", () => {
  it("treats a link within a minute of expiry as stale", () => {
    const now = Date.parse("2026-10-20T12:00:00Z");
    expect(linkIsFresh("2026-10-20T13:00:00Z", now)).toBe(true);
    expect(linkIsFresh("2026-10-20T12:00:30Z", now)).toBe(false);
    expect(linkIsFresh("", now)).toBe(false);
  });

  it("skips only a file of the same size", () => {
    expect(alreadySaved(1024, 1024)).toBe(true);
    expect(alreadySaved(512, 1024)).toBe(false);
    expect(alreadySaved(null, 1024)).toBe(false);
  });

  it("summarises progress", () => {
    expect(saveSummary(["saved", "skipped"])).toBe("All 2 videos saved.");
    expect(saveSummary(["saved"])).toBe("All 1 video saved.");
    expect(saveSummary(["started", "saved"])).toBe(
      "All 2 videos handed to your browser. Check its downloads to see them finish."
    );
    expect(saveSummary(["saved", "pending", "saving"])).toBe("1 of 3 done");
    expect(saveSummary(["saved", "failed"])).toBe("1 of 2 done; 1 failed. Retry them below.");
  });
});
