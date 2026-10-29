export const OutcomeWin = "win";
export const OutcomeLoss = "loss";
export const OutcomeDraw = "draw";

export const outcomeOptions: { value: string; label: string }[] = [
  { value: "", label: "—" },
  { value: OutcomeWin, label: "Won" },
  { value: OutcomeLoss, label: "Lost" },
  { value: OutcomeDraw, label: "Draw" },
];

export const commonRounds = ["Round 1", "Round 2", "Quarterfinal", "Semifinal", "Final"];

export interface Matchup {
  round: string;
  opponent: string;
  outcome: string;
  scoreFor: number | null;
  scoreAgainst: number | null;
}

export interface MatchupDraft {
  round: string;
  opponent: string;
  outcome: string;
  scoreFor: string;
  scoreAgainst: string;
}

export interface OutcomeTally {
  wins: number;
  losses: number;
  draws: number;
}

export function hasMatchup(m: Matchup): boolean {
  return (
    m.round !== "" ||
    m.opponent !== "" ||
    m.outcome !== "" ||
    m.scoreFor != null ||
    m.scoreAgainst != null
  );
}

export function outcomeLabel(outcome: string): string {
  return outcomeOptions.find(o => o.value === outcome && o.value !== "")?.label ?? "";
}

export function scoreText(m: Matchup): string {
  if (m.scoreFor == null && m.scoreAgainst == null) return "";
  return `${m.scoreFor ?? "?"}–${m.scoreAgainst ?? "?"}`;
}

function resultText(m: Matchup): string {
  return [outcomeLabel(m.outcome), scoreText(m)].filter(part => part).join(" ");
}

export function matchupParts(m: Matchup): string[] {
  return [m.round, m.opponent ? `vs ${m.opponent}` : "", resultText(m)].filter(part => part);
}

export function tallyOutcomes(matchups: Matchup[]): OutcomeTally {
  const tally = { wins: 0, losses: 0, draws: 0 };
  for (const m of matchups) {
    if (m.outcome === OutcomeWin) tally.wins++;
    else if (m.outcome === OutcomeLoss) tally.losses++;
    else if (m.outcome === OutcomeDraw) tally.draws++;
  }
  return tally;
}

export function recordText(tally: OutcomeTally): string {
  if (tally.wins + tally.losses + tally.draws === 0) return "";
  const parts = [tally.wins, tally.losses];
  if (tally.draws > 0) parts.push(tally.draws);
  return parts.join("–");
}

export function recordLabel(tally: OutcomeTally): string {
  return tally.draws > 0 ? "W–L–D" : "W–L";
}

export function runSummary(matchups: Matchup[]): string {
  const decided = matchups.filter(m => m.outcome !== "");
  if (decided.length === 0) return "";
  const record = recordText(tallyOutcomes(decided));
  const last = decided[decided.length - 1];
  if (last.round === "") return `Went ${record}`;
  const round = last.round.toLowerCase().startsWith("round") ? last.round : `the ${last.round}`;
  switch (last.outcome) {
    case OutcomeWin:
      return `Went ${record}, won ${round}`;
    case OutcomeLoss:
      return `Went ${record}, lost in ${round}`;
    default:
      return `Went ${record}, drew in ${round}`;
  }
}

export function groupByEntry<T extends { entry: { id: number } }>(rows: T[]): T[][] {
  const groups = new Map<number, T[]>();
  for (const row of rows) {
    const group = groups.get(row.entry.id);
    if (group) group.push(row);
    else groups.set(row.entry.id, [row]);
  }
  return [...groups.values()];
}

export function emptyMatchupDraft(): MatchupDraft {
  return { round: "", opponent: "", outcome: "", scoreFor: "", scoreAgainst: "" };
}

export function matchupToDraft(m: Matchup): MatchupDraft {
  return {
    round: m.round,
    opponent: m.opponent,
    outcome: m.outcome,
    scoreFor: m.scoreFor == null ? "" : String(m.scoreFor),
    scoreAgainst: m.scoreAgainst == null ? "" : String(m.scoreAgainst),
  };
}

function parseScore(value: string): number | null | undefined {
  const trimmed = value.trim();
  if (trimmed === "") return null;
  if (!/^\d+$/.test(trimmed)) return undefined;
  return Number(trimmed);
}

export function draftError(draft: MatchupDraft): string {
  if (parseScore(draft.scoreFor) === undefined || parseScore(draft.scoreAgainst) === undefined) {
    return "A score must be a whole number.";
  }
  if (outcomeContradictsScore(draft)) return "The outcome doesn't match the score.";
  return "";
}

export function draftToMatchup(draft: MatchupDraft): Matchup {
  return {
    round: draft.round.trim(),
    opponent: draft.opponent.trim(),
    outcome: draft.outcome,
    scoreFor: parseScore(draft.scoreFor) ?? null,
    scoreAgainst: parseScore(draft.scoreAgainst) ?? null,
  };
}

export function outcomeFromScore(draft: MatchupDraft): string {
  const ours = parseScore(draft.scoreFor);
  const theirs = parseScore(draft.scoreAgainst);
  if (ours == null || theirs == null) return "";
  if (ours > theirs) return OutcomeWin;
  if (ours < theirs) return OutcomeLoss;
  return OutcomeDraw;
}

export function outcomeContradictsScore(draft: MatchupDraft): boolean {
  const derived = outcomeFromScore(draft);
  if (derived === "" || draft.outcome === "" || draft.outcome === derived) return false;
  return derived !== OutcomeDraw;
}

export function opponentFromEventName(name: string): string {
  const match = /^\s*(?:vs\.?|v\.?|versus|at|@)\s+(.+?)\s*$/i.exec(name);
  return match ? match[1] : "";
}
