import { describe, expect, it } from "vitest";
import { newWhen, whenProblem, whenRequest } from "./when";

const now = new Date(2026, 8, 27, 9, 30);

describe("whenRequest", () => {
  it("sends today as the local date, not the server's", () => {
    expect(whenRequest(newWhen(), now)).toEqual({
      inputType: "date",
      date: "2026-09-27",
      ageYears: null,
      ageMonths: null,
    });
  });

  it("sends yesterday as a local date", () => {
    expect(whenRequest({ ...newWhen(), mode: "yesterday" }, now).date).toBe("2026-09-26");
    expect(whenRequest({ ...newWhen(), mode: "yesterday" }, new Date(2026, 2, 1)).date).toBe(
      "2026-02-28"
    );
  });

  it("sends a picked date as is", () => {
    const when = { ...newWhen(), mode: "date" as const, date: "2025-12-24" };
    expect(whenRequest(when, now)).toMatchObject({ inputType: "date", date: "2025-12-24" });
  });

  it("treats blank months as zero", () => {
    const when = { ...newWhen(), mode: "age" as const, ageYears: "3" };
    expect(whenRequest(when, now)).toMatchObject({ inputType: "age", ageYears: 3, ageMonths: 0 });
  });
});

describe("whenProblem", () => {
  it("needs a date or an age when those are chosen", () => {
    expect(whenProblem(newWhen())).toBe("");
    expect(whenProblem({ ...newWhen(), mode: "date" })).toBe("Pick a date");
    expect(whenProblem({ ...newWhen(), mode: "age" })).toBe("Enter an age in years");
    expect(whenProblem({ ...newWhen(), mode: "age", ageYears: "2", ageMonths: "12" })).toBe(
      "Months should be 0 to 11"
    );
  });
});
