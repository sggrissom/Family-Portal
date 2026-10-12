import { describe, expect, it } from "vitest";
import * as server from "@app/server";
import {
  ALSO_ASKED,
  ageYearsAt,
  changesSincePrevious,
  compareByAge,
  duplicateRefs,
  importDrafts,
  importRequest,
  columnLabel,
  filterTimelines,
  groupInterviewAnswers,
  shortQuestion,
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
import { summarizeDays } from "./daySummary";

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
    expect(ageYearsAt(person.birthday, "2017-01-01T00:00:00Z")).toBe(-1);
    expect(ageYearsAt("", "2025-05-10T00:00:00Z")).toBe(-1);
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
      a(2, 2, 10, "pizza"),
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
      [6, "pizza"],
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
    expect(changes.map(c => [c.before, c.after])).toEqual([
      ["pizza", "Tacos"],
      ["green", "blue"],
    ]);
    expect(changesSincePrevious(qs, answers, {})).toEqual([]);
  });

  it("normalizes case and whitespace without losing punctuation or accents", () => {
    expect(normalizeAnswer("  Crème brûlée!! ")).toBe("crème brûlée!!");
    expect(columnLabel(bundle.interviews[2], bundle.person)).toBe("Age 7 · 2025");
  });
});

describe("import review", () => {
  const proposals = [
    {
      header: "2019",
      date: "",
      ageYears: -1,
      rows: [
        { raw: "Fav food", answer: "pasta", ref: "key:fav-food", confidence: "exact" },
        { raw: "Favorite dinosaur", answer: "T. rex", ref: "", confidence: "none" },
        { raw: "Notes from the day", answer: "", ref: "", confidence: "none" },
        { raw: "Food again", answer: "rice", ref: "key:fav-food", confidence: "close" },
      ],
    },
  ] as server.ImportProposal[];

  it("starts unmatched answers as new questions, blank lines as ignored, and flags duplicates", () => {
    const [draft] = importDrafts(proposals, "2019-06-01");
    expect(draft.date).toBe("");
    expect(draft.rows.map(r => r.choice)).toEqual(["key:fav-food", "new", "ignore", ""]);
    expect([...duplicateRefs(draft)]).toEqual([]);
    draft.rows[3].choice = "key:fav-food";
    expect([...duplicateRefs(draft)]).toEqual(["key:fav-food"]);
    draft.rows[3].choice = "ignore";
    expect(duplicateRefs(draft).size).toBe(0);

    draft.rows[1].keep = false;
    const req = importRequest(7, [draft]);
    expect(req.interviews[0].answers).toEqual([
      { ref: "key:fav-food", answer: "pasta", newText: "", keep: false, rawQuestion: "Fav food" },
      {
        ref: "",
        answer: "T. rex",
        newText: "Favorite dinosaur",
        keep: false,
        rawQuestion: "Favorite dinosaur",
      },
    ]);
  });

  it("leaves skipped interviews out", () => {
    const drafts = importDrafts(proposals, "");
    drafts[0].skip = true;
    expect(importRequest(7, drafts).interviews).toEqual([]);
  });
});

describe("folding answers into interview cards", () => {
  const m = (id: number, interviewId: number, extra: Partial<server.Milestone> = {}) =>
    ({
      id,
      interviewId,
      personId: 1,
      milestoneDate: "2025-05-10T00:00:00Z",
      context: "What's your favorite food?",
      description: `a${id}`,
      ...extra,
    }) as server.Milestone;

  it("groups answers by interview and leaves other moments loose", () => {
    const { groups, loose } = groupInterviewAnswers([m(5, 2), m(1, 0), m(3, 2), m(4, 9), m(2, 0)]);
    expect(groups.map(g => [g.interviewId, g.answers.map(a => a.id)])).toEqual([
      [2, [3, 5]],
      [9, [4]],
    ]);
    expect(loose.map(x => x.id)).toEqual([1, 2]);
  });

  it("shortens questions for a card's preview lines", () => {
    expect(shortQuestion("What's your favorite food?")).toBe("favorite food");
    expect(shortQuestion("Who is your best friend?")).toBe("best friend");
    expect(shortQuestion("What do you want to be when you grow up?")).toBe(
      "What do you want to be when you grow up"
    );
  });

  it("puts one card on the interview's day in a day summary", () => {
    const days = summarizeDays(
      { photos: [], growth: [], milestones: [m(1, 2), m(2, 2), m(3, 0)] },
      []
    );
    expect(days).toHaveLength(1);
    expect(days[0].milestones.map(x => x.id)).toEqual([3]);
    expect(days[0].interviews.map(g => g.answers.length)).toEqual([2]);
  });
});

describe("compareByAge", () => {
  it("lines each person's answers up by age", () => {
    const row = (id: number, name: string, answers: [number, string][]) =>
      ({
        person: { id, name } as server.Person,
        answers: answers.map(([ageYears, text], i) => ({
          ageYears,
          text,
          milestoneId: id * 10 + i,
          date: "",
        })),
      }) as server.CompareRow;
    const groups = compareByAge([
      row(1, "Clara", [
        [6, "pizza"],
        [7, "tacos"],
      ]),
      row(2, "Ben", [[6, "sushi"]]),
    ]);
    expect(groups.map(g => [g.age, g.answers.map(a => `${a.person.name}: ${a.text}`)])).toEqual([
      [6, ["Clara: pizza", "Ben: sushi"]],
      [7, ["Clara: tacos"]],
    ]);
  });
});
