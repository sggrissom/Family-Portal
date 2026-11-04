import { describe, expect, it } from "vitest";
import fixtures from "./recordTimeFixtures.json";
import {
  RecordTimeFields,
  isExactDay,
  overlaps,
  recordTimeLabel,
  resolveRecordTime,
} from "./recordTime";

function fields(record: Partial<RecordTimeFields>): RecordTimeFields {
  return {
    meaning: "",
    basis: "",
    precision: "",
    estimated: false,
    ageYears: 0,
    ageMonths: 0,
    ageToYears: 0,
    ...record,
  };
}

describe("record time fixtures", () => {
  for (const f of fixtures.valid) {
    it(f.name, () => {
      const rt = fields(f.record);
      const birthday = "birthday" in f ? (f.birthday as string) : null;
      const range = resolveRecordTime(rt, birthday);
      expect([range.start, range.end]).toEqual([f.expect.start, f.expect.end]);
      expect(range.resolved).toBe(f.expect.start !== "");
      expect(recordTimeLabel(rt)).toBe(f.expect.label);
      expect(isExactDay(rt)).toBe(f.expect.exactDay);
    });
  }
});

describe("overlaps", () => {
  it("is half-open", () => {
    const range = { start: "2024-06-15", end: "2025-06-15", resolved: true };
    expect(overlaps(range, "2024-06-15", "2024-06-16")).toBe(true);
    expect(overlaps(range, "2025-06-15", "2025-06-16")).toBe(false);
    expect(overlaps(range, "2024-06-14", "2024-06-15")).toBe(false);
  });

  it("never matches an unresolved record", () => {
    expect(overlaps({ start: "", end: "", resolved: false }, "0001-01-01", "9999-01-01")).toBe(
      false
    );
  });
});
