import * as server from "../server";
import { formatLongDate } from "./dateUtils";
import { firstName } from "./interviews";
import { isExactRecord, recordTimeLabel } from "./recordTime";
import { photoAge } from "./sameAge";

const VISIT_KEY = "memory-visit";
const SEEN_KEY = "memories-seen";
const SEEN_LIMIT = 80;

export interface MemoryVisit {
  seed: number;
  seen: string[];
  index: number;
}

function readJSON<T>(storage: Storage | undefined, key: string): T | null {
  try {
    const raw = storage?.getItem(key);
    return raw ? (JSON.parse(raw) as T) : null;
  } catch {
    return null;
  }
}

function writeJSON(storage: Storage | undefined, key: string, value: unknown) {
  try {
    storage?.setItem(key, JSON.stringify(value));
  } catch {}
}

const session = () => (typeof sessionStorage === "undefined" ? undefined : sessionStorage);
const local = () => (typeof localStorage === "undefined" ? undefined : localStorage);

export function readSeen(): string[] {
  const seen = readJSON<string[]>(local(), SEEN_KEY);
  return Array.isArray(seen) ? seen : [];
}

// A visit lasts as long as the tab's session, so reloads and returning to
// Home keep the same memories; seen keys are frozen at its start.
export function currentVisit(): MemoryVisit {
  const stored = readJSON<MemoryVisit>(session(), VISIT_KEY);
  if (stored && typeof stored.seed === "number" && Array.isArray(stored.seen)) {
    return { seed: stored.seed, seen: stored.seen, index: stored.index || 0 };
  }
  const visit = { seed: Math.floor(Math.random() * 1_000_000_000), seen: readSeen(), index: 0 };
  writeJSON(session(), VISIT_KEY, visit);
  return visit;
}

export function saveVisit(visit: MemoryVisit) {
  writeJSON(session(), VISIT_KEY, visit);
}

export function markSeen(key: string) {
  const seen = readSeen().filter(k => k !== key);
  seen.push(key);
  writeJSON(local(), SEEN_KEY, seen.slice(-SEEN_LIMIT));
}

export function memoryHref(memory: server.Memory): string {
  if (memory.milestoneId) return `/milestone/${memory.milestoneId}`;
  if (memory.eventId) return `/competition/${memory.eventId}`;
  if (memory.photoId) return `/view-photo/${memory.photoId}`;
  return "/history";
}

export function memoryNames(memory: server.Memory, people: Map<number, server.Person>): string {
  const names = (memory.personIds ?? [])
    .map(id => people.get(id))
    .filter((p): p is server.Person => !!p)
    .map(p => firstName(p.name));
  if (names.length <= 2) return names.join(" & ");
  return `${names.slice(0, -1).join(", ")} & ${names[names.length - 1]}`;
}

// Exact records give the date and, for one person, their age then; anything
// approximate keeps the family's own wording rather than inventing a day.
export function memoryWhen(memory: server.Memory, people: Map<number, server.Person>): string {
  if (!isExactRecord(memory.recordTime)) return recordTimeLabel(memory.recordTime);
  const date = formatLongDate(memory.date);
  const ids = memory.personIds ?? [];
  const person = ids.length === 1 ? people.get(ids[0]) : undefined;
  const age = person?.birthday && !person.isPregnancy ? photoAge(person.birthday, memory.date) : "";
  return age ? `${date} · ${age}` : date;
}
