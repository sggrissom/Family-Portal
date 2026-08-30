import { afterEach, describe, expect, it, vi } from "vitest";
import {
  calculateAge,
  formatDateRange,
  formatRelativeTime,
  isRealDate,
  toDateInputValue,
} from "@app/lib/dateUtils";

describe("calculateAge", () => {
  it("returns an empty string when either date is missing", () => {
    expect(calculateAge("", "2024-01-01")).toBe("");
    expect(calculateAge("2024-01-01", "")).toBe("");
  });

  it("calls the first month of life Newborn", () => {
    expect(calculateAge("2024-01-01", "2024-01-20")).toBe("Newborn");
  });

  it("singularises one month and one year", () => {
    expect(calculateAge("2024-01-01", "2024-02-01")).toBe("1 month");
    expect(calculateAge("2023-01-01", "2024-01-01")).toBe("1 year");
  });

  it("reports months alone under a year", () => {
    expect(calculateAge("2024-01-01", "2024-08-15")).toBe("7 months");
  });

  it("reports whole years alone, then years and months together", () => {
    expect(calculateAge("2020-03-01", "2024-03-10")).toBe("4 years");
    expect(calculateAge("2020-03-01", "2024-08-10")).toBe("4 years 5 months");
  });

  it("borrows a year when the month has not come round yet", () => {
    expect(calculateAge("2020-11-01", "2024-02-01")).toBe("3 years 3 months");
  });

  describe("waiting for the day of the month to come round", () => {
    it("does not count a month until the day is reached", () => {
      expect(calculateAge("2024-01-15", "2024-08-01")).toBe("6 months");
      expect(calculateAge("2024-01-15", "2024-08-14")).toBe("6 months");
    });

    it("counts the month on the day itself", () => {
      expect(calculateAge("2024-01-15", "2024-08-15")).toBe("7 months");
    });

    it("does not round a child up to the next year before their birthday", () => {
      expect(calculateAge("2023-03-20", "2024-03-01")).toBe("11 months");
      expect(calculateAge("2023-03-20", "2024-03-19")).toBe("11 months");
      expect(calculateAge("2023-03-20", "2024-03-20")).toBe("1 year");
    });

    it("borrows across the turn of the year", () => {
      expect(calculateAge("2020-11-20", "2024-02-10")).toBe("3 years 2 months");
      expect(calculateAge("2020-11-20", "2024-02-20")).toBe("3 years 3 months");
    });

    it("keeps a baby a Newborn for their whole first month", () => {
      expect(calculateAge("2024-01-15", "2024-02-01")).toBe("Newborn");
      expect(calculateAge("2024-01-15", "2024-02-14")).toBe("Newborn");
      expect(calculateAge("2024-01-15", "2024-02-15")).toBe("1 month");
    });

    it("rolls a month-end birthday into the following month", () => {
      // February has no 31st, so the month completes on 1 March.
      expect(calculateAge("2024-01-31", "2024-02-29")).toBe("Newborn");
      expect(calculateAge("2024-01-31", "2024-03-01")).toBe("1 month");
      expect(calculateAge("2024-01-31", "2024-03-31")).toBe("2 months");
    });

    it("handles a leap-day birthday in a common year", () => {
      expect(calculateAge("2024-02-29", "2025-02-28")).toBe("11 months");
      expect(calculateAge("2024-02-29", "2025-03-01")).toBe("1 year");
    });
  });

  describe("across timezones", () => {
    afterEach(() => {
      vi.unstubAllEnvs();
    });

    // A date-only or Z-suffixed string parses to UTC midnight. Read back with
    // local getters it slides to the previous day west of UTC, which pushed the
    // age a whole month out for anyone in the Americas.
    const zones = [
      "UTC",
      "America/Chicago",
      "America/Los_Angeles",
      "Asia/Tokyo",
      "Pacific/Auckland",
    ];

    const inEveryZone = (birthday: string, target: string) =>
      zones.map(zone => {
        vi.stubEnv("TZ", zone);
        return [zone, calculateAge(birthday, target)] as const;
      });

    it("reports the same age wherever the reader is", () => {
      const results = inEveryZone("2024-01-01T00:00:00Z", "2024-08-15T00:00:00Z");
      expect(results).toEqual(zones.map(zone => [zone, "7 months"]));
    });

    it("does not shift a birthday across a month boundary", () => {
      const results = inEveryZone("2024-03-01T00:00:00Z", "2025-03-01T00:00:00Z");
      expect(results).toEqual(zones.map(zone => [zone, "1 year"]));
    });

    it("keeps a bare date stable too", () => {
      const results = inEveryZone("2020-01-01", "2024-01-01");
      expect(results).toEqual(zones.map(zone => [zone, "4 years"]));
    });
  });

  describe("before the birthday, where the person is still a pregnancy", () => {
    it("counts back from a 40-week term", () => {
      // 140 days out is 20 weeks, so 20 weeks along.
      expect(calculateAge("2024-06-01", "2024-01-13")).toBe("20 weeks");
    });

    it("singularises the first week", () => {
      // 273 days out rounds to 39 weeks remaining, leaving 1 week along.
      expect(calculateAge("2024-06-01", "2023-09-02")).toBe("1 week");
    });

    it("never reports negative weeks for a due date far in the future", () => {
      expect(calculateAge("2030-01-01", "2024-01-01")).toBe("0 weeks");
    });
  });
});

describe("isRealDate", () => {
  it("rejects missing values and the Go zero date", () => {
    expect(isRealDate(null)).toBe(false);
    expect(isRealDate(undefined)).toBe(false);
    expect(isRealDate("")).toBe(false);
    expect(isRealDate("0001-01-01T00:00:00Z")).toBe(false);
  });

  it("rejects an unparseable string instead of throwing", () => {
    expect(isRealDate("not a date")).toBe(false);
  });

  it("accepts a real date", () => {
    expect(isRealDate("2024-05-05T00:00:00Z")).toBe(true);
  });
});

describe("toDateInputValue", () => {
  it("strips the time so the value fits an <input type=date>", () => {
    expect(toDateInputValue("2024-05-05T13:45:00Z")).toBe("2024-05-05");
  });

  it("returns an empty string for a date the form should leave blank", () => {
    expect(toDateInputValue("0001-01-01T00:00:00Z")).toBe("");
    expect(toDateInputValue(null)).toBe("");
  });
});

describe("formatDateRange", () => {
  it("joins two distinct dates with an en dash", () => {
    expect(formatDateRange("2024-05-05T00:00:00Z", "2024-06-06T00:00:00Z")).toBe(
      "5/5/2024 – 6/6/2024"
    );
  });

  it("collapses a range whose ends are the same day", () => {
    expect(formatDateRange("2024-05-05T00:00:00Z", "2024-05-05T00:00:00Z")).toBe("5/5/2024");
  });

  it("falls back to whichever end is a real date", () => {
    expect(formatDateRange("2024-05-05T00:00:00Z", "0001-01-01T00:00:00Z")).toBe("5/5/2024");
    expect(formatDateRange("", "2024-06-06T00:00:00Z")).toBe("6/6/2024");
  });
});

describe("formatRelativeTime", () => {
  afterEach(() => {
    vi.useRealTimers();
  });

  const at = (now: string, timestamp: string) => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date(now));
    return formatRelativeTime(timestamp);
  };

  it("steps up through seconds, minutes, hours, and days", () => {
    expect(at("2024-05-05T12:00:30Z", "2024-05-05T12:00:00Z")).toBe("just now");
    expect(at("2024-05-05T12:05:00Z", "2024-05-05T12:00:00Z")).toBe("5m ago");
    expect(at("2024-05-05T15:00:00Z", "2024-05-05T12:00:00Z")).toBe("3h ago");
    expect(at("2024-05-08T12:00:00Z", "2024-05-05T12:00:00Z")).toBe("3d ago");
  });

  it("does not report a clock skew into the future as time elapsed", () => {
    expect(at("2024-05-05T12:00:00Z", "2024-05-05T12:05:00Z")).toBe("just now");
  });

  it("falls back for a timestamp it cannot parse", () => {
    expect(formatRelativeTime("not a date", "never")).toBe("never");
    expect(formatRelativeTime("not a date")).toBe("not a date");
  });
});
