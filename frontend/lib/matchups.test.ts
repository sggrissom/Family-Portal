import { describe, expect, it } from "vitest";
import {
  Matchup,
  draftError,
  draftToMatchup,
  groupByEntry,
  hasMatchup,
  matchupParts,
  matchupToDraft,
  outcomeFromScore,
  recordLabel,
  recordText,
  runSummary,
  tallyOutcomes,
} from "@app/lib/matchups";

function m(over: Partial<Matchup>): Matchup {
  return { round: "", opponent: "", outcome: "", scoreFor: null, scoreAgainst: null, ...over };
}

describe("matchupParts", () => {
  it("reads round, opponent, then the result", () => {
    expect(
      matchupParts(
        m({ round: "Final", opponent: "Crew Z", outcome: "win", scoreFor: 3, scoreAgainst: 1 })
      )
    ).toEqual(["Final", "vs Crew Z", "Won 3–1"]);
  });

  it("leaves out what was not recorded", () => {
    expect(matchupParts(m({ outcome: "loss" }))).toEqual(["Lost"]);
    expect(matchupParts(m({ scoreFor: 2, scoreAgainst: 2 }))).toEqual(["2–2"]);
    expect(matchupParts(m({}))).toEqual([]);
  });
});

describe("hasMatchup", () => {
  it("treats a zero score as recorded", () => {
    expect(hasMatchup(m({}))).toBe(false);
    expect(hasMatchup(m({ scoreAgainst: 0 }))).toBe(true);
  });
});

describe("recordText", () => {
  it("shows draws only when there are any", () => {
    expect(recordText(tallyOutcomes([m({ outcome: "win" }), m({ outcome: "loss" })]))).toBe("1–1");
    expect(recordText(tallyOutcomes([m({ outcome: "win" }), m({ outcome: "draw" })]))).toBe(
      "1–0–1"
    );
    expect(recordText(tallyOutcomes([m({ round: "Final" })]))).toBe("");
  });
});

describe("recordLabel", () => {
  it("names the draw column only when it is shown", () => {
    expect(recordLabel({ wins: 2, losses: 1, draws: 0 })).toBe("W–L");
    expect(recordLabel({ wins: 2, losses: 1, draws: 1 })).toBe("W–L–D");
  });
});

describe("runSummary", () => {
  it("names how the run ended", () => {
    const bracket = [
      m({ round: "Round 1", outcome: "win" }),
      m({ round: "Semifinal", outcome: "win" }),
      m({ round: "Final", outcome: "loss" }),
    ];
    expect(runSummary(bracket)).toBe("Went 2–1, lost in the Final");
    expect(runSummary(bracket.slice(0, 2))).toBe("Went 2–0, won the Semifinal");
    expect(runSummary([m({ round: "Round 2", outcome: "loss" })])).toBe(
      "Went 0–1, lost in Round 2"
    );
  });

  it("ignores undecided rounds and unnamed ones", () => {
    expect(runSummary([m({ outcome: "win" }), m({ round: "Final" })])).toBe("Went 1–0");
    expect(runSummary([m({ round: "Final" })])).toBe("");
  });
});

describe("groupByEntry", () => {
  it("keeps each entry's rows together in first-seen order", () => {
    const rows = [1, 2, 1, 3, 2].map((id, i) => ({ entry: { id }, i }));
    expect(groupByEntry(rows).map(group => group.map(row => row.i))).toEqual([[0, 2], [1, 4], [3]]);
  });
});

describe("drafts", () => {
  it("round-trips a matchup, keeping a zero score", () => {
    const matchup = m({ round: "Final", outcome: "win", scoreFor: 1, scoreAgainst: 0 });
    expect(draftToMatchup(matchupToDraft(matchup))).toEqual(matchup);
  });

  it("trims and rejects non-numeric scores", () => {
    const draft = { ...matchupToDraft(m({})), round: " Final ", scoreFor: "two" };
    expect(draftError(draft)).not.toBe("");
    expect(draftError({ ...draft, scoreFor: " 2 " })).toBe("");
    expect(draftToMatchup({ ...draft, scoreFor: " 2 " })).toMatchObject({
      round: "Final",
      scoreFor: 2,
    });
  });

  it("derives an outcome only from two scores", () => {
    const draft = matchupToDraft(m({}));
    expect(outcomeFromScore({ ...draft, scoreFor: "3", scoreAgainst: "1" })).toBe("win");
    expect(outcomeFromScore({ ...draft, scoreFor: "0", scoreAgainst: "1" })).toBe("loss");
    expect(outcomeFromScore({ ...draft, scoreFor: "2", scoreAgainst: "2" })).toBe("draw");
    expect(outcomeFromScore({ ...draft, scoreFor: "2" })).toBe("");
  });
});
