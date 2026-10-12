import { describe, expect, it } from "vitest";
import * as server from "@app/server";
import {
  ageYearsAt,
  clearDraft,
  interviewTitle,
  loadDrafts,
  pendingQuestions,
  previousLine,
  questionFitsAge,
  questionStatus,
  saveDraft,
} from "./interviews";

const interview = (extra: Partial<server.Interview> = {}) =>
  ({
    id: 9,
    title: "",
    date: "2025-05-03T00:00:00Z",
    skippedIds: [2],
    ...extra,
  }) as server.Interview;
const person = { name: "Clara Whitfield", birthday: "2018-05-10T00:00:00Z" } as server.Person;

describe("ageYearsAt", () => {
  it("counts whole years, turning over on the birthday itself", () => {
    expect(ageYearsAt(person.birthday, "2025-05-09T00:00:00Z")).toBe(6);
    expect(ageYearsAt(person.birthday, "2025-05-10T00:00:00Z")).toBe(7);
    expect(ageYearsAt(person.birthday, "2017-01-01T00:00:00Z")).toBe(0);
    expect(ageYearsAt("", "2025-05-10T00:00:00Z")).toBe(0);
  });
});

describe("interviewTitle", () => {
  it("uses the title when there is one, else the first name and age", () => {
    expect(interviewTitle(interview(), person)).toBe("Clara's interview at 6");
    expect(interviewTitle(interview({ title: "Birthday interview" }), person)).toBe(
      "Birthday interview"
    );
  });
});

describe("question status", () => {
  const answers = new Map([[1, {}]]);
  const qs = [1, 2, 3].map(id => ({ id }) as server.InterviewQuestion);
  it("is answered, skipped or pending", () => {
    expect(qs.map(q => questionStatus(interview(), answers, q.id))).toEqual([
      "answered",
      "skipped",
      "pending",
    ]);
    expect(pendingQuestions(interview(), qs, answers).map(q => q.id)).toEqual([3]);
  });
});

describe("previousLine", () => {
  it("lists the latest few answers with ages", () => {
    const prev = [6, 5, 4, 3].map(
      age => ({ ageYears: age, text: `a${age}` }) as server.PreviousAnswer
    );
    expect(previousLine(prev)).toBe("Age 6: a6 · Age 5: a5 · Age 4: a4");
    expect(previousLine(undefined)).toBe("");
  });
});

describe("questionFitsAge", () => {
  it("treats zero as no bound", () => {
    const q = (minAgeYears: number, maxAgeYears: number) =>
      ({ minAgeYears, maxAgeYears }) as server.InterviewQuestion;
    expect(questionFitsAge(q(0, 0), 2)).toBe(true);
    expect(questionFitsAge(q(5, 0), 4)).toBe(false);
    expect(questionFitsAge(q(0, 10), 11)).toBe(false);
    expect(questionFitsAge(q(4, 10), 7)).toBe(true);
  });
});

describe("drafts", () => {
  it("are kept per interview and question until cleared", () => {
    saveDraft(9, 1, "pizz");
    saveDraft(9, 2, "gold");
    saveDraft(10, 1, "other interview");
    clearDraft(9, 2);
    expect([...loadDrafts(9)]).toEqual([[1, "pizz"]]);
  });
});
