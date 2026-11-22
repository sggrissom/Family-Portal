import * as server from "../../server";
import { formatDate, formatDateRange, isRealDate, toDateInputValue } from "../dateUtils";
import { hasMatchup, matchupParts, runSummary } from "../matchups";
import { eventStatus, resultText, sortResults } from "../activityResults";
import { copy } from "../copy";

export type EntryCard = {
  entry: server.Entry;
  personIds: number[];
  appearances: server.AppearanceDetail[];
};

/** The results page's cards: routines with performances in the order they were recorded, then the rest of the season's. */
export function entryCards(
  entries: server.EntryView[],
  appearances: server.AppearanceDetail[]
): EntryCard[] {
  const cards = new Map<number, EntryCard>();
  for (const detail of appearances) {
    const card = cards.get(detail.entry.id);
    if (card) {
      card.appearances.push(detail);
      continue;
    }
    const view = entries.find(row => row.entry.id === detail.entry.id);
    cards.set(detail.entry.id, {
      entry: detail.entry,
      personIds: view?.personIds ?? [],
      appearances: [detail],
    });
  }
  for (const view of entries) {
    if (!cards.has(view.entry.id)) {
      cards.set(view.entry.id, {
        entry: view.entry,
        personIds: view.personIds ?? [],
        appearances: [],
      });
    }
  }
  return [...cards.values()];
}

export function namesOf(people: server.Person[], personIds: number[]): string[] {
  return personIds
    .map(id => people.find(person => person.id === id)?.name)
    .filter((name): name is string => !!name);
}

export function showPerformanceDate(detail: server.AppearanceDetail, event: server.Event): boolean {
  if (!isRealDate(detail.appearance.occurredAt)) return false;
  const multiDay =
    isRealDate(event.endDate) &&
    toDateInputValue(event.endDate) !== toDateInputValue(event.startDate);
  return (
    multiDay || toDateInputValue(detail.appearance.occurredAt) !== toDateInputValue(event.startDate)
  );
}

export interface RecapResult {
  kind: string;
  text: string;
  detail: string | null;
}

export interface RecapPerformance {
  id: number;
  when: string | null;
  matchup: string | null;
  results: RecapResult[];
}

export interface RecapRoutine {
  id: number;
  name: string;
  descriptors: string | null;
  roster: string | null;
  summary: string | null;
  performances: RecapPerformance[];
}

export interface CompetitionShare {
  eventName: string;
  subtitle: string | null;
  place: string | null;
  snapshotLabel: string | null;
  routines: RecapRoutine[];
}

export interface CompetitionShareOptions {
  omitted: Set<number>;
  showsNames: boolean;
  isSnapshot: boolean;
}

export interface CompetitionShareInput {
  event: server.Event;
  seasonName: string;
  entries: server.EntryView[];
  appearances: server.AppearanceDetail[];
  people: server.Person[];
}

/** The routines that can go in a recap: the page's cards that have something recorded. */
export function recapCards(input: CompetitionShareInput): EntryCard[] {
  return entryCards(input.entries, input.appearances).filter(card => card.appearances.length > 0);
}

/** While the event is on or yet to come, or a performance has nothing recorded, a recap starts labelled as results so far. */
export function looksUnfinished(
  event: server.Event,
  appearances: server.AppearanceDetail[],
  today: string
): boolean {
  const status = eventStatus(
    toDateInputValue(event.startDate),
    toDateInputValue(event.endDate),
    today
  );
  if (status !== "done") return true;
  return appearances.some(
    detail => (detail.results ?? []).length === 0 && !hasMatchup(detail.appearance)
  );
}

export function buildCompetitionShare(
  input: CompetitionShareInput,
  options: CompetitionShareOptions,
  sharedOn: string
): CompetitionShare {
  const { event, people } = input;
  const nameOf = (id: number | null) =>
    id == null ? "" : (people.find(person => person.id === id)?.name ?? "");

  const routines = recapCards(input)
    .filter(card => !options.omitted.has(card.entry.id))
    .map((card): RecapRoutine => {
      const entry = card.entry;
      const traits = [entry.format, entry.style, entry.division, entry.level].filter(Boolean);
      const roster = options.showsNames ? namesOf(people, card.personIds) : [];
      const summary =
        card.appearances.length > 1 ? runSummary(card.appearances.map(d => d.appearance)) : "";
      return {
        id: entry.id,
        name: entry.name,
        descriptors: traits.length ? traits.join(" · ") : null,
        roster: roster.length ? roster.join(", ") : null,
        summary: summary || null,
        performances: card.appearances.map(detail => {
          const parts = matchupParts(detail.appearance);
          return {
            id: detail.appearance.id,
            when: showPerformanceDate(detail, event)
              ? formatDate(detail.appearance.occurredAt)
              : null,
            matchup: parts.length ? parts.join(" · ") : null,
            results: sortResults(detail.results).map(result => {
              const name = nameOf(result.personId);
              const who = name ? (options.showsNames ? name : copy.share.individual) : "";
              const detailText = [result.category, who].filter(Boolean).join(" · ");
              return { kind: result.kind, text: resultText(result), detail: detailText || null };
            }),
          };
        }),
      };
    });

  const subtitle = [input.seasonName, formatDateRange(event.startDate, event.endDate)]
    .filter(Boolean)
    .join(" · ");
  const place = [event.location, event.host].filter(Boolean).join(" · ");

  return {
    eventName: event.name,
    subtitle: subtitle || null,
    place: place || null,
    snapshotLabel: options.isSnapshot ? copy.share.asOf(sharedOn) : null,
    routines,
  };
}

export function competitionShareText(share: CompetitionShare): string {
  const lines = [share.eventName];
  for (const line of [share.subtitle, share.place, share.snapshotLabel]) {
    if (line) lines.push(line);
  }
  for (const routine of share.routines) {
    lines.push("", routine.name);
    for (const line of [routine.descriptors, routine.roster, routine.summary]) {
      if (line) lines.push(line);
    }
    for (const performance of routine.performances) {
      const heading = [performance.when, performance.matchup].filter(Boolean).join(" · ");
      const indent = heading ? "  " : "";
      if (heading) lines.push(`• ${heading}`);
      if (performance.results.length === 0 && !performance.matchup) {
        lines.push(`${indent}• No results recorded`);
      }
      for (const result of performance.results) {
        lines.push(`${indent}• ${result.text}${result.detail ? ` (${result.detail})` : ""}`);
      }
    }
  }
  return lines.join("\n");
}

/** The recap as a hosted link stores it. Ids and revisions let the server check each result is this event's and current; it keeps neither. */
export function recapLinkInput(
  share: CompetitionShare,
  appearances: server.AppearanceDetail[]
): server.RecapInput {
  const revisions = new Map(appearances.map(d => [d.appearance.id, d.appearance.revision]));
  return {
    eventName: share.eventName,
    subtitle: share.subtitle ?? "",
    place: share.place ?? "",
    snapshotLabel: share.snapshotLabel ?? "",
    routines: share.routines.map(routine => ({
      entryId: routine.id,
      name: routine.name,
      descriptors: routine.descriptors ?? "",
      roster: routine.roster ?? "",
      summary: routine.summary ?? "",
      performances: routine.performances.map(performance => ({
        appearanceId: performance.id,
        revision: revisions.get(performance.id) ?? 0,
        when: performance.when ?? "",
        matchup: performance.matchup ?? "",
        results: performance.results.map(result => ({
          kind: result.kind,
          text: result.text,
          detail: result.detail ?? "",
        })),
      })),
    })),
  };
}
