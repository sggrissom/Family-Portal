import { describe, expect, it } from "vitest";
import { HomeEventLike, seasonPhase, seasonProgress } from "./activityHome";

function event(id: number, start: string, appearances: number, end = ""): HomeEventLike {
  return {
    event: {
      id,
      startDate: `${start}T00:00:00Z`,
      endDate: end ? `${end}T00:00:00Z` : "0001-01-01T00:00:00Z",
    },
    appearances,
  };
}

describe("seasonPhase", () => {
  it("treats an open-ended season as current once it starts", () => {
    expect(seasonPhase("2026-01-01", "", "2026-10-01")).toBe("current");
    expect(seasonPhase("2026-01-01", "2026-06-01", "2026-10-01")).toBe("past");
    expect(seasonPhase("2026-11-01", "", "2026-10-01")).toBe("upcoming");
    expect(seasonPhase("", "", "2026-10-01")).toBe("current");
  });
});

describe("seasonProgress", () => {
  const events = [
    event(1, "2026-01-10", 2),
    event(2, "2026-03-10", 1),
    event(3, "2026-05-10", 0),
    event(4, "2026-06-01", 0, "2026-06-03"),
    event(5, "2026-08-10", 0),
    event(6, "2026-09-10", 0),
  ];

  it("surfaces the latest result, what is missing, what is on, and what is next", () => {
    const progress = seasonProgress(events, true, "2026-06-02");
    expect(progress.focus.map(f => [f.item.event.id, f.focus])).toEqual([
      [2, "latest"],
      [3, "needs-results"],
      [4, "now"],
      [5, "next"],
    ]);
    expect(progress.held).toBe(3);
    expect(progress.recorded).toBe(2);
  });

  it("asks for no results while the season has nothing entered", () => {
    const progress = seasonProgress(events, false, "2026-07-01");
    expect(progress.focus.map(f => f.focus)).toEqual(["latest", "next"]);
  });

  it("handles a season that has not started", () => {
    const progress = seasonProgress(events, true, "2025-12-01");
    expect(progress.focus.map(f => [f.item.event.id, f.focus])).toEqual([[1, "next"]]);
    expect(progress.held).toBe(0);
  });
});
