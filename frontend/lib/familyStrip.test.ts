import { describe, expect, it } from "vitest";
import { compactAge, dueSummary } from "./familyStrip";

const today = new Date(2026, 8, 27);

describe("compactAge", () => {
  it("counts days and weeks for a newborn", () => {
    expect(compactAge("2026-09-24T00:00:00Z", today)).toBe("3d");
    expect(compactAge("2026-09-06T00:00:00Z", today)).toBe("3w");
  });

  it("counts months under two", () => {
    expect(compactAge("2026-01-10T00:00:00Z", today)).toBe("8m");
    expect(compactAge("2024-10-01T00:00:00Z", today)).toBe("23m");
  });

  it("gives years and months for children, years for adults", () => {
    expect(compactAge("2023-05-02T00:00:00Z", today)).toBe("3y 4m");
    expect(compactAge("2019-09-27T00:00:00Z", today)).toBe("7y");
    expect(compactAge("1985-03-01T00:00:00Z", today)).toBe("41y");
  });

  it("is blank for a date that has not come yet", () => {
    expect(compactAge("2026-12-01T00:00:00Z", today)).toBe("");
  });
});

describe("dueSummary", () => {
  it("counts down in weeks and days", () => {
    expect(dueSummary("2026-11-01T00:00:00Z", today)).toBe("Due in 5w");
    expect(dueSummary("2026-11-03T00:00:00Z", today)).toBe("Due in 5w 2d");
    expect(dueSummary("2026-09-30T00:00:00Z", today)).toBe("Due in 3 days");
    expect(dueSummary("2026-09-27T00:00:00Z", today)).toBe("Due today");
    expect(dueSummary("2026-09-26T00:00:00Z", today)).toBe("Due date passed 1 day ago");
  });
});
