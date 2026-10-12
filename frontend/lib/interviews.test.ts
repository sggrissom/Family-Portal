import { describe, expect, it } from "vitest";
import * as server from "@app/server";
import {
  ALSO_ASKED,
  ageYearsAt,
  changesSincePrevious,
  columnLabel,
  filterTimelines,
  interviewGrid,
  normalizeAnswer,
  questionTimelines,
  rowLabel,
  sortTimelines,
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

describe("over-time views", () => {
  const q = (
    id: number,
    section: string,
    position: number,
    extra: Partial<server.InterviewQuestion> = {}
  ) =>
    ({
      id,
      section,
      position,
      text: `Q${id}`,
      oneOff: false,
      ...extra,
    }) as server.InterviewQuestion;
  const iv = (id: number, date: string, questionIds: number[], skippedIds: number[] = []) =>
    ({ id, date, questionIds, skippedIds }) as server.Interview;
  const a = (id: number, interviewId: number, questionId: number, description: string) =>
    ({
      id,
      interviewId,
      questionId,
      description,
      context: `Asked Q${questionId}`,
    }) as server.Milestone;

  const bundle = {
    person: { birthday: "2018-05-10T00:00:00Z" } as server.Person,
    interviews: [
      iv(1, "2023-05-10T00:00:00Z", [10, 11, 12]),
      iv(2, "2024-05-10T00:00:00Z", [10, 11], [11]),
      iv(3, "2025-05-10T00:00:00Z", [10, 11, 13]),
    ],
    questions: [
      q(13, "Fun", 0, { oneOff: true }),
      q(10, "Favorites", 1),
      q(11, "Favorites", 2),
      q(12, "People", 3, { archived: true }),
    ],
    answers: [
      a(1, 1, 10, "Pizza"),
      a(2, 2, 10, "pizza!"),
      a(3, 3, 10, "Tacos"),
      a(4, 1, 11, "blue"),
      a(5, 3, 11, "Blue"),
      a(6, 1, 12, "Sam"),
      a(7, 3, 13, "a dragon"),
    ],
  } as server.GetPersonInterviewsResponse;

  it("lays questions out by section with one-offs last and marks changes", () => {
    const { sections } = interviewGrid(bundle);
    expect(sections.map(s => s.name)).toEqual(["Favorites", "People", ALSO_ASKED]);
    const food = sections[0].rows[0];
    expect(food.cells.map(c => c.trend)).toEqual(["first", "same", "changed"]);
    const color = sections[0].rows[1];
    expect(color.cells.map(c => [c.trend, c.skipped])).toEqual([
      ["first", false],
      ["", true],
      ["same", false],
    ]);
    expect(sections[1].rows[0].cells.map(c => c.asked)).toEqual([true, false, false]);
    expect(rowLabel(food)).toBe("Asked Q10");
  });

  it("builds per-question timelines that sort by consistency or change", () => {
    const timelines = questionTimelines(bundle);
    expect(timelines[0].points.map(p => [p.age, p.answer.description])).toEqual([
      [5, "Pizza"],
      [6, "pizza!"],
      [7, "Tacos"],
    ]);
    expect(timelines.map(t => [t.streak, t.distinct])).toEqual([
      [2, 2],
      [2, 1],
      [1, 1],
      [1, 1],
    ]);
    expect(sortTimelines(timelines, "changed")[0].question.id).toBe(10);
    expect(sortTimelines(timelines, "consistent")[0].question.id).toBe(10);
    expect(filterTimelines(timelines, "TACO").map(t => t.question.id)).toEqual([10]);
    expect(filterTimelines(timelines, "q12").map(t => t.question.id)).toEqual([12]);
  });

  it("finds what changed since the most recent earlier interview", () => {
    const qs = [q(10, "Favorites", 1), q(11, "Favorites", 2)];
    const answers = new Map([
      [10, { description: "Tacos" } as server.Milestone],
      [11, { description: "blue" } as server.Milestone],
    ]);
    const prev = (interviewId: number, date: string, text: string) =>
      ({ interviewId, date, text }) as server.PreviousAnswer;
    const changes = changesSincePrevious(qs, answers, {
      10: [prev(2, "2024-05-10T00:00:00Z", "pizza"), prev(1, "2023-05-10T00:00:00Z", "pizza")],
      11: [prev(1, "2023-05-10T00:00:00Z", "green")],
    });
    expect(changes.map(c => [c.before, c.after])).toEqual([["pizza", "Tacos"]]);
    expect(changesSincePrevious(qs, answers, {})).toEqual([]);
  });

  it("normalizes case, punctuation and accents", () => {
    expect(normalizeAnswer("  Crème brûlée!! ")).toBe("creme brulee");
    expect(columnLabel(bundle.interviews[2], bundle.person)).toBe("Age 7 · 2025");
  });
});
