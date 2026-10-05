import { afterEach, describe, expect, it, vi } from "vitest";
import {
  calculateAge,
  formatDate,
  formatDateRange,
  formatDateTime,
  formatLongDate,
} from "./dateUtils";
import { ageInMonths } from "./growthPercentiles";
import { timeAgo } from "./checkup";
import { monthsOld } from "./sameAge";
import { localDateString, whenRequest, newWhen } from "./when";

const zones = ["UTC", "America/Chicago", "America/Los_Angeles", "Asia/Tokyo", "Pacific/Kiritimati"];

afterEach(() => vi.unstubAllEnvs());

describe.each(zones)("calendar dates in %s", zone => {
  it("keeps bare dates and server dates on the same displayed day", () => {
    vi.stubEnv("TZ", zone);
    for (const day of ["2024-01-01", "2024-03-01", "2024-03-10", "2024-11-03"]) {
      const expected = new Date(day + "T12:00:00").toLocaleDateString();
      expect(formatDate(day)).toBe(expected);
      expect(formatDate(day + "T00:00:00Z")).toBe(expected);
      expect(formatDateRange(day, day + "T00:00:00Z")).toBe(expected);
    }
    expect(formatDate("invalid")).toBe("");
    expect(formatLongDate("2024-03-01")).toBe("March 1, 2024");
  });

  it("keeps growth ages stable when only one date crosses a month boundary locally", () => {
    vi.stubEnv("TZ", zone);
    expect(ageInMonths("2024-03-01", "2024-04-15")).toBeCloseTo(1 + 14 / 30.4375);
    expect(ageInMonths("2024-03-01T00:00:00Z", "2024-04-15T00:00:00Z")).toBeCloseTo(
      1 + 14 / 30.4375
    );
    expect(calculateAge("2024-03-01", "2024-04-15")).toBe("1 month");
  });

  it("uses the local day for current ages and entry defaults near midnight", () => {
    vi.stubEnv("TZ", zone);
    for (const hour of [0, 23]) {
      const now = new Date(2024, 3, 1, hour, 30);
      expect(localDateString(now)).toBe("2024-04-01");
      expect(ageInMonths("2024-03-01", now)).toBe(1);
      expect(monthsOld("2024-03-01", localDateString(now))).toBe(1);
      expect(whenRequest(newWhen(), now).date).toBe("2024-04-01");
      expect(timeAgo("2024-04-01T00:00:00Z", now)).toBe("today");
      expect(timeAgo("2024-03-31T00:00:00Z", now)).toBe("yesterday");
    }
  });

  it("counts calendar days through both daylight-saving transitions", () => {
    vi.stubEnv("TZ", zone);
    for (const [month, day, previous] of [
      [2, 11, "2024-03-10"],
      [10, 4, "2024-11-03"],
    ] as const) {
      const now = new Date(2024, month, day, 0, 30);
      expect(timeAgo(previous + "T00:00:00Z", now)).toBe("yesterday");
      expect(whenRequest({ ...newWhen(), mode: "yesterday" }, now).date).toBe(previous);
    }
  });
});

it("still converts actual timestamps to the viewer's local time", () => {
  vi.stubEnv("TZ", "America/Chicago");
  const instant = "2024-03-01T00:00:00Z";
  expect(new Date(instant).getDate()).toBe(29); // Ensure this test actually changes timezone.
  expect(formatDate(instant)).toBe("3/1/2024");
  expect(formatDateTime(instant)).toContain("2/29/2024");
});
