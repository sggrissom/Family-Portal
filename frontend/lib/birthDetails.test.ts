import { describe, expect, it } from "vitest";
import * as server from "../server";
import { birthDetailsRequest, birthFormFor, formatBirthTime, newBirthForm } from "./birthDetails";

const person = {
  birthday: "2024-03-02T00:00:00Z",
  birthTime: "04:12",
  birthPlace: "Home",
} as server.Person;

const record = (type: server.MeasurementType, value: number, unit: string, date: string) =>
  ({ measurementType: type, value, unit, measurementDate: date }) as server.GrowthData;

describe("birth details", () => {
  it("loads the records on the birthday", () => {
    const form = birthFormFor(person, [
      record(server.Weight, 7.25, "lbs", "2024-03-02T00:00:00Z"),
      record(server.Height, 50, "cm", "2024-03-02T00:00:00Z"),
      record(server.Weight, 9, "lbs", "2024-04-02T00:00:00Z"),
    ]);
    expect(form.open).toBe(true);
    expect(form.entry).toMatchObject({ weightUnit: "lb-oz", pounds: "7", ounces: "4" });
    expect(form.entry).toMatchObject({ heightUnit: "cm", height: "50" });
  });

  it("sends blanks as clears", () => {
    const form = newBirthForm();
    expect(form.open).toBe(false);
    expect(birthDetailsRequest(form)).toEqual({
      details: {
        birthTime: "",
        birthPlace: "",
        birthLength: { value: 0, unit: "" },
        birthWeight: { value: 0, unit: "" },
      },
      error: "",
    });
  });

  it("reports a bad measurement", () => {
    const form = newBirthForm();
    form.entry.ounces = "20";
    expect(birthDetailsRequest(form).error).toMatch(/^Birth weight:/);
  });

  it("formats the time of birth", () => {
    expect(formatBirthTime("04:12")).toBe("4:12 AM");
    expect(formatBirthTime("16:05")).toBe("4:05 PM");
    expect(formatBirthTime("")).toBe("");
  });
});
