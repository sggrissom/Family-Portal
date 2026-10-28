import { describe, expect, it } from "vitest";
import {
  ResultKindAdjudication,
  ResultKindAward,
  ResultKindPlacement,
  ResultKindScore,
  ResultLike,
  cellHeadline,
  eventStatus,
  headlineText,
  ordinal,
  quickLabels,
  resultText,
} from "./activityResults";

function result(kind: string, fields: Partial<ResultLike> = {}): ResultLike {
  return { kind, label: "", rank: null, outOf: null, score: null, ...fields };
}

describe("ordinal", () => {
  it("handles the teens", () => {
    expect([1, 2, 3, 4, 11, 12, 13, 21, 22, 111].map(ordinal)).toEqual([
      "1st",
      "2nd",
      "3rd",
      "4th",
      "11th",
      "12th",
      "13th",
      "21st",
      "22nd",
      "111th",
    ]);
  });
});

describe("resultText", () => {
  it("writes a placement with its field", () => {
    expect(resultText(result(ResultKindPlacement, { rank: 3, outOf: 18 }))).toBe("3rd of 18");
    expect(resultText(result(ResultKindPlacement, { rank: 1 }))).toBe("1st");
  });

  it("does not repeat a score whose label is the score", () => {
    expect(resultText(result(ResultKindScore, { score: 285.4, label: "285.4" }))).toBe("285.4");
    expect(resultText(result(ResultKindScore, { score: 2, label: "goals" }))).toBe("2 goals");
    expect(resultText(result(ResultKindScore, { label: "3–1 W" }))).toBe("3–1 W");
  });
});

describe("eventStatus", () => {
  it("places today against a one-day or multi-day event", () => {
    expect(eventStatus("2026-08-28", "2026-08-30", "2026-08-27")).toBe("upcoming");
    expect(eventStatus("2026-08-28", "2026-08-30", "2026-08-29")).toBe("now");
    expect(eventStatus("2026-08-28", "", "2026-08-28")).toBe("now");
    expect(eventStatus("2026-08-28", "", "2026-08-29")).toBe("done");
    expect(eventStatus("2026-08-28", "2026-08-01", "2026-08-29")).toBe("done");
  });
});

describe("quickLabels", () => {
  it("prefers what the family has used, and falls back for dance", () => {
    expect(quickLabels("dance", ["Diamond", "Elite"])).toEqual(["Diamond", "Elite"]);
    expect(quickLabels("dance", [])).toEqual(["Platinum", "High Gold", "Gold"]);
    expect(quickLabels("sport", null)).toEqual([]);
  });
});

describe("headlineText", () => {
  it("leads with the best placement and the adjudication", () => {
    expect(
      headlineText([
        result(ResultKindPlacement, { rank: 4, outOf: 20 }),
        result(ResultKindAdjudication, { label: "Platinum" }),
        result(ResultKindPlacement, { rank: 1, outOf: 6 }),
      ])
    ).toBe("1st · Platinum");
  });

  it("falls back to an award, then to whatever came first", () => {
    expect(headlineText([result(ResultKindAward, { label: "Studio Choice" })])).toBe(
      "Studio Choice"
    );
    expect(headlineText([result(ResultKindScore, { label: "3–1 W" })])).toBe("3–1 W");
    expect(headlineText([])).toBe("");
  });
});

describe("cellHeadline", () => {
  const appearance = (
    outcome = "",
    scoreFor: number | null = null,
    scoreAgainst: number | null = null
  ) => ({
    outcome,
    round: "",
    opponent: "",
    scoreFor,
    scoreAgainst,
  });

  it("is empty when nothing was performed", () => {
    expect(cellHeadline([])).toBe("");
  });

  it("sums a performance's results", () => {
    expect(
      cellHeadline([
        {
          appearance: appearance(),
          results: [
            result(ResultKindAdjudication, { label: "High Gold" }),
            result(ResultKindPlacement, { rank: 3, outOf: 18 }),
          ],
        },
      ])
    ).toBe("3rd · High Gold");
    expect(cellHeadline([{ appearance: appearance(), results: [] }])).toBe("Recorded");
  });

  it("reads a game as its outcome and a bracket as its record", () => {
    expect(cellHeadline([{ appearance: appearance("win", 3, 1), results: [] }])).toBe("W 3–1");
    expect(cellHeadline([{ appearance: appearance("loss"), results: [] }])).toBe("L");
    expect(
      cellHeadline([
        { appearance: appearance("win"), results: [] },
        { appearance: appearance("win"), results: [] },
        { appearance: appearance("loss"), results: [] },
      ])
    ).toBe("2–1");
  });
});
