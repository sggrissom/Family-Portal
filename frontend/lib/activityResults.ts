export const ResultKindAdjudication = "adjudication";
export const ResultKindPlacement = "placement";
export const ResultKindAward = "award";
export const ResultKindScore = "score";

export interface ResultLike {
  kind: string;
  label: string;
  rank: number | null;
  outOf: number | null;
  score: number | null;
}

export function ordinal(n: number): string {
  const mod100 = n % 100;
  if (mod100 >= 11 && mod100 <= 13) return `${n}th`;
  switch (n % 10) {
    case 1:
      return `${n}st`;
    case 2:
      return `${n}nd`;
    case 3:
      return `${n}rd`;
    default:
      return `${n}th`;
  }
}

export function resultText(result: ResultLike): string {
  switch (result.kind) {
    case ResultKindPlacement: {
      const place = result.rank == null ? result.label : ordinal(result.rank);
      const field = result.outOf == null ? "" : ` of ${result.outOf}`;
      return `${place}${field}`;
    }
    case ResultKindScore: {
      if (result.score == null) return result.label;
      const score = String(result.score);
      const label = result.label.trim();
      if (!label || label === score) return score;
      return label.startsWith(`${score} `) ? label : `${score} ${label}`;
    }
    default:
      return result.label;
  }
}

export type EventStatus = "upcoming" | "now" | "done";

export function eventStatus(startDay: string, endDay: string, today: string): EventStatus {
  if (!startDay) return "done";
  const end = endDay && endDay >= startDay ? endDay : startDay;
  if (today < startDay) return "upcoming";
  if (today <= end) return "now";
  return "done";
}

const danceAdjudications = ["Platinum", "High Gold", "Gold"];

export function quickLabels(kind: string, used: string[] | null): string[] {
  const labels = (used ?? []).filter(label => label.trim() !== "");
  if (labels.length > 0) return labels.slice(0, 6);
  return kind === "dance" ? danceAdjudications : [];
}

export function bestPlacement<T extends ResultLike>(results: T[]): T | null {
  let best: T | null = null;
  for (const result of results) {
    if (result.kind !== ResultKindPlacement || result.rank == null) continue;
    if (best === null || result.rank < (best.rank ?? Infinity)) best = result;
  }
  return best;
}

export function headlineText(results: ResultLike[]): string {
  const placement = bestPlacement(results);
  const adjudication = results.find(r => r.kind === ResultKindAdjudication && r.label.trim());
  const award = results.find(r => r.kind === ResultKindAward && r.label.trim());
  const parts = [
    placement ? ordinal(placement.rank!) : "",
    adjudication?.label ?? "",
    !placement && !adjudication ? (award?.label ?? "") : "",
  ].filter(part => part);
  if (parts.length > 0) return parts.join(" · ");
  const first = results[0];
  return first ? resultText(first) : "";
}

export interface AppearanceResultsLike {
  appearance: {
    outcome: string;
    round: string;
    opponent: string;
    scoreFor: number | null;
    scoreAgainst: number | null;
  };
  results: ResultLike[] | null;
}

export function cellHeadline(views: AppearanceResultsLike[]): string {
  if (views.length === 0) return "";
  const decided = views.filter(view => view.appearance.outcome !== "");
  if (decided.length > 0) {
    const tally = { win: 0, loss: 0, draw: 0 } as Record<string, number>;
    for (const view of decided) tally[view.appearance.outcome]++;
    if (decided.length === 1) {
      const m = decided[0].appearance;
      const letter = m.outcome === "win" ? "W" : m.outcome === "loss" ? "L" : "D";
      const score =
        m.scoreFor != null && m.scoreAgainst != null ? ` ${m.scoreFor}–${m.scoreAgainst}` : "";
      return `${letter}${score}`;
    }
    const record = [tally.win, tally.loss];
    if (tally.draw > 0) record.push(tally.draw);
    return record.join("–");
  }
  const headline = headlineText(views.flatMap(view => view.results ?? []));
  return headline || "Recorded";
}

export function adjudicationCounts(results: ResultLike[]): { label: string; count: number }[] {
  const counts = new Map<string, number>();
  for (const result of results) {
    if (result.kind !== ResultKindAdjudication) continue;
    const label = result.label.trim();
    if (!label) continue;
    counts.set(label, (counts.get(label) ?? 0) + 1);
  }
  return [...counts.entries()]
    .map(([label, count]) => ({ label, count }))
    .sort((a, b) => (a.count !== b.count ? b.count - a.count : a.label < b.label ? -1 : 1));
}

export function bestFinish<T extends { results: ResultLike[] | null }>(
  views: T[]
): { view: T; result: ResultLike } | null {
  let best: { view: T; result: ResultLike } | null = null;
  for (const view of views) {
    const placement = bestPlacement(view.results ?? []);
    if (placement && (best === null || placement.rank! < best.result.rank!)) {
      best = { view, result: placement };
    }
  }
  return best;
}
