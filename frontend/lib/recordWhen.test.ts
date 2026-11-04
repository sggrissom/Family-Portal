import { describe, expect, it } from "vitest";
import {
  RecordWhen,
  ageDate,
  exactWhenProblem,
  exactWhenRequest,
  isApproximateMode,
  newRecordWhen,
  recordWhenControls,
  recordWhenFrom,
  recordWhenPreview,
  recordWhenProblem,
  recordWhenLookup,
  recordWhenTime,
  setRecordWhenMode,
} from "./recordWhen";
import { recordTimeLabel, resolveRecordTime } from "./recordTime";

const now = new Date("2026-11-02T12:00:00");
const w = (fields: Partial<RecordWhen>): RecordWhen => ({ ...newRecordWhen(7), ...fields });

describe("recordWhenTime", () => {
  it.each(["today", "yesterday", "unknown"] as const)(
    "%s ignores hidden period and estimate fields even without a mode-change event",
    mode => {
      const when = w({ mode, meaning: "period", estimated: true, around: "invalid" });
      const rt = recordWhenTime(when, now, true);
      expect(rt.meaning).toBe("occurrence");
      expect(rt.estimated).toBe(false);
      expect(rt.around).toBe("");
      expect(recordWhenProblem(when, true, undefined, now)).toBe("");
    }
  );

  it("blank months is a whole year of age, typed zero is month precision", () => {
    const year = recordWhenTime(w({ mode: "age", ageYears: "4", meaning: "period" }), now, true);
    expect([year.precision, year.anchorPersonId, recordTimeLabel(year)]).toEqual([
      "year",
      7,
      "At age 4",
    ]);
    const zero = recordWhenTime(w({ mode: "age", ageYears: "4", ageMonths: "0" }), now, true);
    expect(zero.precision).toBe("month");
  });

  it("builds the plan's examples", () => {
    const cases: [Partial<RecordWhen>, boolean, string][] = [
      [{ mode: "month", month: "2022-10" }, true, "October 2022"],
      [{ mode: "year", year: "1970" }, false, "1970"],
      [
        { mode: "range", from: "1968", to: "1972", estimated: true, around: "1970" },
        false,
        "Around 1970 (1968–1972)",
      ],
      [{ mode: "age", ageYears: "4", meaning: "occurrence" }, true, "Sometime at age 4"],
      [
        { mode: "age", ageYears: "6", ageToYears: "9", estimated: true, around: "8" },
        false,
        "Around age 8 (ages 6–9)",
      ],
      [{ mode: "unknown", estimated: true }, true, "Date unknown"],
    ];
    for (const [fields, isMoment, label] of cases) {
      expect(recordTimeLabel(recordWhenTime(w(fields), now, isMoment))).toBe(label);
    }
  });

  it("never lets a photo describe a period", () => {
    const rt = recordWhenTime(w({ mode: "age", ageYears: "4", meaning: "period" }), now, false);
    expect(rt.meaning).toBe("occurrence");
  });

  it("round-trips through the edit form", () => {
    for (const fields of [
      { mode: "month", month: "2022-10" },
      { mode: "range", from: "1968", to: "1972-06", estimated: true, around: "1970" },
      { mode: "age", ageYears: "4", ageMonths: "3", meaning: "period" },
      { mode: "age", ageYears: "6", ageToYears: "9", meaning: "occurrence" },
      { mode: "unknown" },
    ] as Partial<RecordWhen>[]) {
      const rt = recordWhenTime(w(fields), now, true);
      expect(recordWhenTime(recordWhenFrom(rt, ""), now, true)).toEqual(rt);
    }
  });
});

describe("date mode transitions", () => {
  it.each(["today", "yesterday", "unknown"] as const)(
    "clears incompatible choices when switching an estimated age period to %s",
    mode => {
      const when = w({
        mode: "age",
        ageYears: "6",
        ageToYears: "9",
        meaning: "period",
        estimated: true,
        around: "8",
      });
      setRecordWhenMode(when, mode);
      expect([when.meaning, when.estimated, when.around]).toEqual(["", false, ""]);
      const rt = recordWhenTime(when, now, true);
      expect(rt.meaning).toBe("occurrence");
      expect(rt.estimated).toBe(false);
      expect(rt.basis).toBe(mode === "unknown" ? "unknown" : "calendar");
      if (mode !== "unknown") {
        expect(rt.precision).toBe("day");
        expect(rt.from).toBe(mode === "today" ? "2026-11-02" : "2026-11-01");
        expect(isApproximateMode(when)).toBe(false);
      }
    }
  );

  it("doesn't carry a central age into an estimated calendar range", () => {
    const when = w({
      mode: "age",
      ageYears: "6",
      ageToYears: "9",
      meaning: "period",
      estimated: true,
      around: "8",
    });
    setRecordWhenMode(when, "range");
    when.from = "1968";
    when.to = "1972";
    expect(when.around).toBe("");
    expect(recordWhenProblem(when, true, undefined, now)).toBe("");
    expect(recordTimeLabel(recordWhenTime(when, now, true))).toBe("Around 1968–1972");
    when.around = "1970";
    expect(recordWhenTime(when, now, true).around).toBe("1970");
    setRecordWhenMode(when, "age");
    expect(when.around).toBe("");
  });

  it("hides and ignores a best guess when an age range becomes a single age", () => {
    const when = w({ mode: "age", ageYears: "4", ageToYears: "6", estimated: true, around: "5" });
    when.ageToYears = "";
    expect(recordWhenControls(when).bestGuess).toBe(false);
    expect(recordWhenTime(when, now, true).around).toBe("");
    expect(recordTimeLabel(recordWhenTime(when, now, true))).toBe("Around age 4");
  });

  it("doesn't expose a stale age range's best guess in calendar date entry", () => {
    const when = w({ mode: "age", ageYears: "4", ageToYears: "6", estimated: true, around: "5" });
    setRecordWhenMode(when, "date");
    when.date = "2022-10-31";
    expect(recordWhenControls(when).bestGuess).toBe(false);
    expect(recordWhenProblem(when, true, undefined, now)).toBe("");
    expect(recordTimeLabel(recordWhenTime(when, now, true))).toBe("Around Oct 31, 2022");
  });

  it("treats a period on a calendar day as approximate for photo suggestions", () => {
    const when = w({ mode: "date", date: "2022-10-31", meaning: "period" });
    expect(isApproximateMode(when)).toBe(true);
    expect(recordWhenTime(when, now, true).meaning).toBe("period");
    setRecordWhenMode(when, "today");
    expect(isApproximateMode(when)).toBe(false);
  });
});

describe("recordWhenProblem", () => {
  it("requires the moment meaning for an age", () => {
    expect(recordWhenProblem(w({ mode: "age", ageYears: "4" }), true, undefined, now)).toMatch(
      /happened then/
    );
    expect(recordWhenProblem(w({ mode: "age", ageYears: "4" }), false, undefined, now)).toBe("");
  });

  it("checks ranges and ages", () => {
    const problem = (fields: Partial<RecordWhen>) =>
      recordWhenProblem(w({ meaning: "occurrence", ...fields }), true, "2020-06-15", now);
    expect(problem({ mode: "range", from: "1972", to: "1968" })).toMatch(/ends before/);
    expect(problem({ mode: "range", from: "1968", to: "" })).toMatch(/both ends/);
    expect(problem({ mode: "year", year: "70" })).toMatch(/year/);
    expect(problem({ mode: "age", ageYears: "4", ageMonths: "12" })).toMatch(/0 to 11/);
    expect(problem({ mode: "age", ageYears: "9", ageToYears: "6" })).toMatch(/ends before/);
    expect(problem({ mode: "age", ageYears: "7" })).toMatch(/hasn't happened/);
    expect(problem({ mode: "age", ageYears: "6" })).toBe("");
    expect(problem({ mode: "age", ageYears: "4", anchorPersonId: 0 })).toMatch(/whose age/);
  });

  it("keeps the best guess inside the range", () => {
    const problem = (fields: Partial<RecordWhen>) =>
      recordWhenProblem(
        w({ meaning: "occurrence", estimated: true, ...fields }),
        true,
        "2020-06-15",
        now
      );
    const range = { mode: "range", from: "2026-01", to: "2026-03" } as const;
    expect(problem({ ...range, around: "2025-01" })).toMatch(/inside the range/);
    expect(problem({ ...range, around: "2026-04" })).toMatch(/inside the range/);
    expect(problem({ ...range, around: "2026" })).toMatch(/inside the range/);
    expect(problem({ ...range, around: "2026-03-31" })).toBe("");
    expect(problem({ ...range, around: "2026-02" })).toBe("");
    const ages = { mode: "age", ageYears: "2", ageToYears: "4" } as const;
    expect(problem({ ...ages, around: "5" })).toMatch(/inside the age range/);
    expect(problem({ ...ages, around: "1" })).toMatch(/inside the age range/);
    expect(problem({ ...ages, around: "4" })).toBe("");
  });
});

describe("recordWhenPreview", () => {
  it("shows the age window from the birthday", () => {
    const preview = recordWhenPreview(
      w({ mode: "age", ageYears: "4", meaning: "period" }),
      true,
      "2020-06-15T00:00:00Z",
      now
    );
    expect(preview.label).toBe("At age 4");
    expect(preview.bounds).toMatch(/^Based on birthday: .*2024.* – .*2025/);
  });
});

describe("recordWhenLookup", () => {
  it("searches around an exact day only", () => {
    expect(recordWhenLookup(w({ mode: "date", date: "2022-10-31" }), now)).toMatchObject({
      basis: "calendar",
      precision: "day",
      estimated: false,
      from: "2022-10-31",
    });
    expect(recordWhenLookup(w({ mode: "year", year: "1970" }), now)).toBeNull();
    expect(recordWhenLookup(w({ mode: "date", date: "" }), now)).toBeNull();
    expect(recordWhenLookup(w({ mode: "age", ageYears: "2" }), now)).toBeNull();
  });
});

describe("exactWhenRequest", () => {
  const today = new Date(2026, 8, 27, 9, 30);

  it("sends today and yesterday as local dates, not the server's", () => {
    expect(exactWhenRequest(w({ mode: "today" }), today)).toEqual({
      inputType: "date",
      date: "2026-09-27",
      ageYears: null,
      ageMonths: null,
    });
    expect(exactWhenRequest(w({ mode: "yesterday" }), today).date).toBe("2026-09-26");
    expect(exactWhenRequest(w({ mode: "yesterday" }), new Date(2026, 2, 1)).date).toBe(
      "2026-02-28"
    );
  });

  it("sends a picked date as is", () => {
    expect(exactWhenRequest(w({ mode: "date", date: "2025-12-24" }), today)).toMatchObject({
      inputType: "date",
      date: "2025-12-24",
    });
  });

  it("treats blank months as zero", () => {
    expect(exactWhenRequest(w({ mode: "age", ageYears: "3" }), today)).toMatchObject({
      inputType: "age",
      ageYears: 3,
      ageMonths: 0,
    });
  });
});

describe("exactWhenProblem", () => {
  it("needs a date or an age when those are chosen", () => {
    expect(exactWhenProblem(w({ mode: "today" }))).toBe("");
    expect(exactWhenProblem(w({ mode: "date" }))).toBe("Pick a date");
    expect(exactWhenProblem(w({ mode: "age" }))).toBe("Enter an age in years");
    expect(exactWhenProblem(w({ mode: "age", ageYears: "2", ageMonths: "12" }))).toBe(
      "Months should be 0 to 11"
    );
  });

  it("refuses an age that hasn't happened yet", () => {
    const when = w({ mode: "age", ageYears: "9" });
    const sixYearsAgo = `${new Date().getUTCFullYear() - 6}-01-01T00:00:00Z`;
    expect(exactWhenProblem(when, sixYearsAgo)).toBe("That age hasn't happened yet");
    expect(exactWhenProblem({ ...when, ageYears: "5" }, sixYearsAgo)).toBe("");
    expect(exactWhenProblem(when)).toBe("");
  });
});

describe("ageDate", () => {
  it("resolves an age against a stored birthday", () => {
    expect(ageDate("2020-02-14T00:00:00Z", w({ mode: "age", ageYears: "2", ageMonths: "3" }))).toBe(
      "2022-05-14"
    );
  });
});

describe("recordWhenTime bounds", () => {
  it("sends the assertion without derived bounds", () => {
    const rt = recordWhenTime(w({ mode: "year", year: "1970" }), now, false);
    expect(rt.from).toBe("1970");
    expect(rt.resolved).toBe(false);
    expect(resolveRecordTime(rt, null).start).toBe("1970-01-01");
  });
});

describe("half-typed input", () => {
  it("has no preview and never throws", () => {
    for (const fields of [
      { mode: "age", ageYears: "" },
      { mode: "range", from: "19", to: "" },
      { mode: "month", month: "" },
      { mode: "year", year: "abc" },
    ] as Partial<RecordWhen>[]) {
      expect(recordWhenPreview(w(fields), true, "2020-06-15T00:00:00Z", now)).toEqual({
        label: "",
        bounds: "",
      });
    }
  });
});
