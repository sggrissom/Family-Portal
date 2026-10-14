import * as server from "../server";
import { MILESTONE_CATEGORIES } from "./milestoneHelpers";

const FILLER = new Set([
  "a",
  "an",
  "the",
  "and",
  "at",
  "in",
  "on",
  "of",
  "with",
  "to",
  "for",
  "photo",
  "photos",
  "picture",
  "pictures",
  "when",
  "was",
  "old",
]);

const WORD = /[\p{L}\p{N}]+/gu;

export function fold(text: string): string {
  return text
    .normalize("NFD")
    .replace(/\p{Mn}/gu, "")
    .toLowerCase();
}

export function searchTerms(text: string): string[] {
  return (fold(text).match(WORD) ?? []).filter(w => !FILLER.has(w));
}

export type Range = [number, number];

// Where each term starts a word of text, as [start, end) offsets into the
// original text, sorted and merged.
export function highlightRanges(text: string, terms: string[]): Range[] {
  if (terms.length === 0) return [];
  const ranges: Range[] = [];
  for (const match of text.matchAll(WORD)) {
    const word = match[0];
    const folded = fold(word);
    const longest = terms
      .filter(t => folded.startsWith(t))
      .reduce((n, t) => Math.max(n, t.length), 0);
    if (longest === 0) continue;
    let end = 0;
    let length = 0;
    for (const ch of word) {
      if (length >= longest) break;
      length += fold(ch).length;
      end += ch.length;
    }
    ranges.push([match.index!, match.index! + end]);
  }
  return ranges;
}

export type Piece = { text: string; mark: boolean };

export function highlightPieces(text: string, terms: string[]): Piece[] {
  const pieces: Piece[] = [];
  let at = 0;
  for (const [start, end] of highlightRanges(text, terms)) {
    if (start > at) pieces.push({ text: text.slice(at, start), mark: false });
    pieces.push({ text: text.slice(start, end), mark: true });
    at = end;
  }
  if (at < text.length) pieces.push({ text: text.slice(at), mark: false });
  return pieces;
}

export const RECENT_LIMIT = 8;

export function addRecent(recent: string[], query: string): string[] {
  const trimmed = query.trim();
  if (!trimmed) return recent;
  const key = fold(trimmed);
  return [trimmed, ...recent.filter(q => fold(q) !== key)].slice(0, RECENT_LIMIT);
}

const RECENT_KEY = "recent-searches";

export function loadRecent(): string[] {
  try {
    const stored = JSON.parse(localStorage.getItem(RECENT_KEY) ?? "[]");
    return Array.isArray(stored)
      ? stored.filter((q): q is string => typeof q === "string").slice(0, RECENT_LIMIT)
      : [];
  } catch {
    return [];
  }
}

export function saveRecent(recent: string[]) {
  try {
    localStorage.setItem(RECENT_KEY, JSON.stringify(recent));
  } catch {}
}

export type ChipKind = "person" | "age" | "date" | "category";

export type Chip = { kind: ChipKind; label: string };

const MONTHS = [
  "January",
  "February",
  "March",
  "April",
  "May",
  "June",
  "July",
  "August",
  "September",
  "October",
  "November",
  "December",
];

export function dateLabel(year: number, month: number): string {
  const name = month > 0 ? MONTHS[month - 1] : "";
  return [name, year > 0 ? String(year) : ""].filter(Boolean).join(" ");
}

const CATEGORY_CHIPS: Record<string, string> = {
  quote: "Quotes",
  artwork: "Artwork",
  interview: "Interview answers",
};

function categoryLabel(category: string): string {
  return (
    CATEGORY_CHIPS[category] ??
    MILESTONE_CATEGORIES.find(c => c.value === category)?.label ??
    category
  );
}

// One chip per filter the server understood, since removing a chip ignores
// that whole kind. Tags are not filters, so they get no chip.
export function understoodChips(
  understood: server.SearchUnderstood,
  people: server.Person[]
): Chip[] {
  const chips: Chip[] = [];
  const names = (understood.personIds ?? [])
    .map(id => people.find(p => p.id === id)?.name.split(" ")[0])
    .filter((n): n is string => !!n);
  if (names.length > 0) chips.push({ kind: "person", label: names.join(" or ") });
  if (understood.ageLabel) chips.push({ kind: "age", label: understood.ageLabel });
  const date = dateLabel(understood.year, understood.month);
  if (date) chips.push({ kind: "date", label: date });
  if (understood.category)
    chips.push({ kind: "category", label: categoryLabel(understood.category) });
  return chips;
}

export function hasResults(resp: server.SearchResponse): boolean {
  return (
    (resp.people?.length ?? 0) +
      (resp.moments?.length ?? 0) +
      (resp.photos?.length ?? 0) +
      (resp.tags?.length ?? 0) +
      (resp.places?.length ?? 0) +
      (resp.activities?.length ?? 0) +
      (resp.books?.length ?? 0) >
    0
  );
}

export function searchRoute(query: string): string {
  const trimmed = query.trim();
  return trimmed ? `/search?q=${encodeURIComponent(trimmed)}` : "/search";
}

export function queryOfRoute(route: string): string {
  return (new URLSearchParams(route.split("?")[1] ?? "").get("q") ?? "").slice(0, 200);
}

export function activityHref(hit: server.ActivityHit): string {
  if (hit.kind === "season") return `/season/${hit.id}`;
  if (hit.kind === "event") return `/competition/${hit.id}`;
  return "/activities";
}

function pad(n: number): string {
  return String(n).padStart(2, "0");
}

// The Photos page showing what a search's photo group would, as far as its
// filters reach: people, a year or a month of a year, and the text.
export function photosHref(understood: server.SearchUnderstood): string {
  const params = new URLSearchParams();
  const people = understood.personIds ?? [];
  if (people.length > 0) params.set("people", people.join(","));
  if (understood.year > 0) {
    const month = understood.month;
    const lastDay = month > 0 ? new Date(Date.UTC(understood.year, month, 0)).getUTCDate() : 31;
    params.set("from", `${understood.year}-${pad(month || 1)}-01`);
    params.set("to", `${understood.year}-${pad(month || 12)}-${pad(lastDay)}`);
  }
  if (understood.text) params.set("q", understood.text);
  const search = params.toString().replace(/%2C/g, ",");
  return search ? `/photos?${search}` : "/photos";
}

export function nextIndex(current: number, delta: number, count: number): number {
  if (count === 0) return -1;
  if (current < 0) return delta > 0 ? 0 : count - 1;
  return Math.min(count - 1, Math.max(0, current + delta));
}

export function isTypingTarget(target: EventTarget | null): boolean {
  if (!(target instanceof HTMLElement)) return false;
  const tag = target.tagName;
  return tag === "INPUT" || tag === "TEXTAREA" || tag === "SELECT" || target.isContentEditable;
}
