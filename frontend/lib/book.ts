import * as server from "../server";
import { monthsOld, photoAge } from "./sameAge";
import { formatMeasurement } from "./weightFormat";

export type Density = "brief" | "balanced" | "detailed";

export const DENSITIES: Density[] = ["brief", "balanced", "detailed"];

export const isDensity = (value: string | null | undefined): value is Density =>
  DENSITIES.includes(value as Density);

<<<<<<< HEAD
=======
export const DENSITY_OPTIONS = DENSITIES.map(d => ({
  value: d,
  label: d[0].toUpperCase() + d.slice(1),
}));

export const MATCH_OPTIONS = [
  { value: "any", label: "Photos of any of them" },
  { value: "all", label: "Only photos of all of them" },
];

export const toggled = <T>(list: T[], value: T): T[] =>
  list.includes(value) ? list.filter(x => x !== value) : [...list, value];

>>>>>>> main
export type Category = "milestones" | "quotes" | "artwork" | "photos";

export const CATEGORIES: { value: Category; label: string }[] = [
  { value: "milestones", label: "Milestones" },
  { value: "quotes", label: "Quotes" },
  { value: "artwork", label: "Artwork" },
  { value: "photos", label: "Photos" },
];

export const PRESETS = {
  firstYear: "first-year",
  year: "year",
  familyYear: "family-year",
  custom: "custom",
} as const;

const PHOTO_BUDGET: Record<Density, number> = { brief: 3, balanced: 7, detailed: 14 };
const PER_DAY: Record<Density, number> = { brief: 1, balanced: 2, detailed: 4 };

export interface BookPhoto {
  id: number;
  width: number;
  height: number;
  day: string;
  caption: string;
  detail: string;
}

export interface BookMoment {
  id: number;
  text: string;
  context: string;
  day: string;
  detail: string;
  first: boolean;
}

export interface GrowthPoint {
  day: string;
  months: number;
  value: number;
  label: string;
}

export type Block =
  | { kind: "hero"; photo: BookPhoto }
  | { kind: "photos"; photos: BookPhoto[] }
  | { kind: "moment"; moment: BookMoment; photo: BookPhoto | null }
  | { kind: "quote"; moment: BookMoment }
  | { kind: "artwork"; moment: BookMoment; photo: BookPhoto }
  | { kind: "notes"; moments: BookMoment[] }
  | { kind: "facts"; lines: string[] }
  | { kind: "letter"; text: string; signature: string }
<<<<<<< HEAD
  | { kind: "growth"; height: GrowthPoint[]; weight: GrowthPoint[] };
=======
  | {
      kind: "growth";
      height: GrowthPoint[];
      weight: GrowthPoint[];
      heightLabel: string;
      span: [number, number];
      ticks: [string, string];
    };
>>>>>>> main

export interface Chapter {
  id: string;
  title: string;
  dates: string;
  blocks: Block[];
  items: number[];
}

export interface EditorNotes {
  milestonesUsed: number;
  milestonesInRange: number;
  photosUsed: number;
  photosInRange: number;
  undatedPhotos: number;
  unreadyPhotos: number;
  missing: number;
  hiddenMonths: string[];
}

export interface Book {
  name: string;
  title: string;
  dates: string;
  cover: BookPhoto | null;
  chapters: Chapter[];
  ending: string;
  notes: EditorNotes;
}

export type BookSource = server.BookSources;

export interface Selection {
  preset: string;
  title: string;
  startDate: string;
  endDate: string;
  coverPhotoId: number;
  introduction: string;
  letter: string;
  signature: string;
  showGrowth: boolean;
  categories: string[];
  match: string;
  items: server.BookItem[];
}

const NUMBER_WORDS = [
  "Zero",
  "One",
  "Two",
  "Three",
  "Four",
  "Five",
  "Six",
  "Seven",
  "Eight",
  "Nine",
  "Ten",
  "Eleven",
  "Twelve",
  "Thirteen",
  "Fourteen",
  "Fifteen",
  "Sixteen",
  "Seventeen",
  "Eighteen",
];

export const numberWord = (n: number) => NUMBER_WORDS[n] ?? String(n);

export const dayOf = (iso: string) => (iso ?? "").slice(0, 10);

export const isRealDay = (day: string) => /^\d{4}-\d{2}-\d{2}$/.test(day) && day > "1000";

<<<<<<< HEAD
export function addYears(day: string, n: number): string {
  const d = new Date(day + "T00:00:00Z");
  return new Date(Date.UTC(d.getUTCFullYear() + n, d.getUTCMonth(), d.getUTCDate()))
=======
export function addMonths(day: string, n: number): string {
  const d = new Date(day + "T00:00:00Z");
  return new Date(Date.UTC(d.getUTCFullYear(), d.getUTCMonth() + n, d.getUTCDate()))
>>>>>>> main
    .toISOString()
    .slice(0, 10);
}

<<<<<<< HEAD
=======
export const addYears = (day: string, n: number) => addMonths(day, n * 12);

>>>>>>> main
export const firstBirthday = (birthday: string) => addYears(dayOf(birthday), 1);

export function addDays(day: string, n: number): string {
  const d = new Date(day + "T00:00:00Z");
  d.setUTCDate(d.getUTCDate() + n);
  return d.toISOString().slice(0, 10);
}

<<<<<<< HEAD
export function longDay(day: string): string {
  return new Date(day + "T00:00:00Z").toLocaleDateString("en-US", {
    month: "long",
    day: "numeric",
    year: "numeric",
    timeZone: "UTC",
  });
}

export function shortDay(day: string): string {
  return new Date(day + "T00:00:00Z").toLocaleDateString("en-US", {
    month: "long",
    day: "numeric",
    timeZone: "UTC",
  });
}
=======
const formatDay = (day: string, options: Intl.DateTimeFormatOptions) =>
  new Date(day + "T00:00:00Z").toLocaleDateString("en-US", { ...options, timeZone: "UTC" });

export const longDay = (day: string) =>
  formatDay(day, { month: "long", day: "numeric", year: "numeric" });

export const shortDay = (day: string) => formatDay(day, { month: "long", day: "numeric" });

export const monthName = (day: string, year = true) =>
  formatDay(day, { month: "long", year: year ? "numeric" : undefined });
>>>>>>> main

export const bookDates = (startDate: string, endDate: string) =>
  `${longDay(dayOf(startDate))} – ${longDay(addDays(dayOf(endDate), -1))}`;

<<<<<<< HEAD
const monthName = (day: string, year: boolean) =>
  new Date(day + "T00:00:00Z").toLocaleDateString("en-US", {
    month: "long",
    year: year ? "numeric" : undefined,
    timeZone: "UTC",
  });

export function monthSpan(from: string, to: string): string {
  if (from.slice(0, 7) === to.slice(0, 7)) return monthName(from, true);
  if (from.slice(0, 4) === to.slice(0, 4))
    return `${monthName(from, false)} – ${monthName(to, true)}`;
  return `${monthName(from, true)} – ${monthName(to, true)}`;
}

function monthStart(start: string, month: number): string {
  const d = new Date(start + "T00:00:00Z");
  return new Date(Date.UTC(d.getUTCFullYear(), d.getUTCMonth() + month, d.getUTCDate()))
    .toISOString()
    .slice(0, 10);
=======
export function monthSpan(from: string, to: string): string {
  if (from.slice(0, 7) === to.slice(0, 7)) return monthName(from);
  if (from.slice(0, 4) === to.slice(0, 4)) return `${monthName(from, false)} – ${monthName(to)}`;
  return `${monthName(from)} – ${monthName(to)}`;
>>>>>>> main
}

const firstOfMonth = (day: string) => day.slice(0, 8) + "01";

export function chapterTitle(from: number, to: number): string {
  if (from === to) return from === 1 ? "One month" : `${numberWord(from)} months`;
  return `${numberWord(from)} to ${numberWord(to).toLowerCase()} months`;
}

export const joinNames = (names: string[]) =>
  names.length <= 2
    ? names.join(" and ")
    : `${names.slice(0, -1).join(", ")} and ${names[names.length - 1]}`;

function isGeneratedTitle(image: server.Image): boolean {
  const title = (image.title ?? "").trim();
  return !title || title === image.originalFilename || /^(IMG|DSC|PXL|photo)[-_ ]?\d/i.test(title);
}

export function originalCaption(image: server.Image): string {
  return image.description || (isGeneratedTitle(image) ? "" : image.title);
}

// Spread picks across distinct days so one prolific afternoon cannot fill a chapter.
export function spreadPick<T extends { day: string }>(
  items: T[],
  budget: number,
  perDay = Infinity
): T[] {
  const byDay = new Map<string, T[]>();
  for (const item of items) {
    const list = byDay.get(item.day) ?? [];
    list.push(item);
    byDay.set(item.day, list);
  }
  const days = [...byDay.keys()].sort();
  const picked: T[] = [];
  for (let round = 0; round < perDay && picked.length < budget; round++) {
    const available = days.filter(d => byDay.get(d)!.length > round);
    if (!available.length) break;
    const want = Math.min(budget - picked.length, available.length);
    for (let k = 0; k < want; k++) {
      const day = available[Math.floor(((k + 0.5) * available.length) / want)];
      picked.push(byDay.get(day)![round]);
    }
  }
  return picked.sort((a, b) => a.day.localeCompare(b.day));
}

// Quiet months fold into their neighbours instead of standing alone as empty pages.
export function groupMonths(weights: number[], target = 7, maxSpan = 3): number[][] {
  const groups: number[][] = [];
  let current: number[] = [];
  let weight = 0;
  for (let month = 0; month < weights.length; month++) {
    if (!current.length && weights[month] === 0) continue;
    current.push(month);
    weight += weights[month];
    if (weight >= target || current.length >= maxSpan) {
      groups.push(current);
      current = [];
      weight = 0;
    }
  }
  if (current.length) {
    const last = groups[groups.length - 1];
    if (weight < 2 && last && last.length + current.length <= maxSpan + 1) last.push(...current);
    else groups.push(current);
  }
  return groups
    .map(g => {
      let end = g.length;
      while (end > 0 && weights[g[end - 1]] === 0) end--;
      return g.slice(0, end);
    })
    .filter(g => g.length > 0);
}

export const itemKey = (item: { kind: server.BookItemKind; sourceId: number }) =>
  `${item.kind}:${item.sourceId}`;

<<<<<<< HEAD
const newItem = (kind: server.BookItemKind, sourceId: number): server.BookItem => ({
=======
export const newItem = (kind: server.BookItemKind, sourceId: number): server.BookItem => ({
>>>>>>> main
  kind,
  sourceId,
  photoId: 0,
  caption: "",
  pinned: false,
});

export interface Resolved {
<<<<<<< HEAD
  preset: string;
=======
>>>>>>> main
  people: server.Person[];
  person: Map<number, server.Person>;
  start: string;
  end: string;
  limit: string;
  firstYear: boolean;
  months: number;
  photos: Map<number, server.Image>;
  milestones: Map<number, server.Milestone>;
  photoPeople: Map<number, number[]>;
  untagged: Set<number>;
  categories: Set<string>;
  match: string;
  undatedPhotos: number;
  unreadyPhotos: number;
}

const monthIndex = (day: string) => parseInt(day.slice(0, 4)) * 12 + parseInt(day.slice(5, 7)) - 1;

export function resolveSource(source: BookSource, selection: Selection): Resolved {
  const photos = new Map<number, server.Image>();
  let undatedPhotos = 0;
  let unreadyPhotos = 0;
  for (const image of source.photos ?? []) {
    if (image.status !== 0) unreadyPhotos++;
    else if (!isRealDay(dayOf(image.photoDate))) undatedPhotos++;
    else photos.set(image.id, image);
  }
  const milestones = new Map<number, server.Milestone>();
  for (const m of source.milestones ?? []) milestones.set(m.id, m);
  const photoPeople = new Map<number, number[]>();
  for (const [id, people] of Object.entries(source.photoPeople ?? {})) {
    photoPeople.set(Number(id), people ?? []);
  }
  const people = source.people ?? [];
  const start = dayOf(selection.startDate);
  const end = dayOf(selection.endDate);
  const firstYear = selection.preset === PRESETS.firstYear;
  return {
<<<<<<< HEAD
    preset: selection.preset,
=======
>>>>>>> main
    people,
    person: new Map(people.map(p => [p.id, p])),
    start,
    end,
    limit: firstYear ? addDays(end, 1) : end,
    firstYear,
    months: firstYear ? 12 : Math.max(1, monthIndex(addDays(end, -1)) - monthIndex(start) + 1),
    photos,
    milestones,
    photoPeople,
    untagged: new Set(source.untagged ?? []),
    categories: new Set(
      selection.categories?.length ? selection.categories : CATEGORIES.map(c => c.value)
    ),
    match: selection.match === "all" ? "all" : "any",
    undatedPhotos,
    unreadyPhotos,
  };
}

const inRange = (r: Resolved, day: string) => isRealDay(day) && day >= r.start && day < r.limit;

const BIRTHDAY = -1;

// The slot is the month a record falls in: months of age in a first-year book
// (BIRTHDAY for the day itself), calendar months from the start otherwise.
function slotOf(r: Resolved, day: string): number {
  if (r.firstYear) {
    if (day >= r.end) return BIRTHDAY;
    if (day < r.start) return 0;
    return Math.min(11, Math.max(0, monthsOld(r.start, day + "T00:00:00Z")));
  }
  return Math.min(r.months - 1, Math.max(0, monthIndex(day) - monthIndex(r.start)));
}

export function itemDay(r: Resolved, item: server.BookItem): string | null {
  if (item.kind === server.BookItemMilestone) {
    const m = r.milestones.get(item.sourceId);
    return m ? dayOf(m.milestoneDate) : null;
  }
  const p = r.photos.get(item.sourceId);
  return p ? dayOf(p.photoDate) : null;
}

export function categoryOf(r: Resolved, item: server.BookItem): Category {
  if (item.kind === server.BookItemPhoto) return "photos";
  const category = r.milestones.get(item.sourceId)?.category;
  if (category === "quote") return "quotes";
  if (category === "artwork") return "artwork";
  return "milestones";
}

export const isMulti = (r: Resolved) => r.people.length > 1;

// sectionOf says where an item belongs in a book about several people: shared
// moments together (0), everything about one person in that person's section.
export function sectionOf(r: Resolved, item: server.BookItem): number {
  if (!isMulti(r)) return 0;
  if (item.kind === server.BookItemMilestone) {
    return r.milestones.get(item.sourceId)?.personId ?? 0;
  }
  const tagged = r.photoPeople.get(item.sourceId) ?? [];
  return tagged.length === 1 ? tagged[0] : 0;
}

export function photoMatches(r: Resolved, id: number): boolean {
  const tagged = r.photoPeople.get(id) ?? [];
  if (!tagged.length) return false;
  return r.match === "all" ? tagged.length === r.people.length : true;
}

<<<<<<< HEAD
=======
// The milestone with the first of its photos the book can show and has not used yet.
export function milestoneItem(
  r: Resolved,
  m: server.Milestone,
  used = new Set<number>()
): server.BookItem {
  const item = newItem(server.BookItemMilestone, m.id);
  item.photoId = (m.photoIds ?? []).find(id => r.photos.has(id) && !used.has(id)) ?? 0;
  return item;
}

>>>>>>> main
export interface Candidates {
  milestones: server.Milestone[];
  photos: server.Image[];
}

export function candidatesIn(r: Resolved): Candidates {
  return {
    milestones: [...r.milestones.values()]
      .filter(m => inRange(r, dayOf(m.milestoneDate)))
      .sort((a, b) => a.milestoneDate.localeCompare(b.milestoneDate) || a.id - b.id),
    photos: [...r.photos.values()]
      .filter(p => inRange(r, dayOf(p.photoDate)))
      .sort((a, b) => a.photoDate.localeCompare(b.photoDate) || a.id - b.id),
  };
}

function chooseCover(r: Resolved, picked: server.Image[]): number {
  if (!isMulti(r)) {
    const profile = r.photos.get(r.people[0]?.profilePhotoId ?? 0);
    const day = profile ? dayOf(profile.photoDate) : "";
    if (profile && inRange(r, day) && slotOf(r, day) !== BIRTHDAY) return profile.id;
  }
  const pool = isMulti(r)
    ? picked.filter(p => (r.photoPeople.get(p.id) ?? []).length > 1)
    : picked.filter(p => slotOf(r, dayOf(p.photoDate)) !== BIRTHDAY);
  const wide = (p: server.Image) => p.width >= p.height;
  const settled = r.firstYear ? pool.filter(p => monthsOld(r.start, p.photoDate) >= 2) : pool;
  return (settled.find(wide) ?? pool.find(wide) ?? pool[0] ?? picked[0])?.id ?? 0;
}

export interface SuggestOptions {
  density: Density;
  current?: server.BookItem[];
  excluded?: server.BookItem[];
  coverPhotoId?: number;
}

export interface Suggestion {
  items: server.BookItem[];
  coverPhotoId: number;
}

const bucketOf = (r: Resolved, item: server.BookItem) =>
  `${sectionOf(r, item)}:${slotOf(r, itemDay(r, item)!)}`;

// Milestones in the chosen categories are always offered; photos are budgeted
// per month, and per person in a book about several people. Kept items survive,
// excluded ones never return, and a re-suggestion keeps the order it already had.
export function suggestItems(r: Resolved, options: SuggestOptions): Suggestion {
  const excluded = new Set((options.excluded ?? []).map(itemKey));
  const current = new Map((options.current ?? []).map(item => [itemKey(item), item]));
  const { milestones, photos } = candidatesIn(r);
  const budget = PHOTO_BUDGET[options.density];
  const room = (section: number) =>
    !isMulti(r) ? budget : section === 0 ? Math.ceil(budget / 2) : Math.ceil(budget / 3);

  const matching = photos.filter(
    p => photoMatches(r, p.id) && !excluded.has(`${server.BookItemPhoto}:${p.id}`)
  );
  const cover =
    options.coverPhotoId && r.photos.has(options.coverPhotoId)
      ? options.coverPhotoId
      : chooseCover(r, matching);

  const used = new Set<number>([cover]);
  const chosen: server.BookItem[] = [];
  for (const item of current.values()) {
    if (item.kind === server.BookItemMilestone && item.photoId) used.add(item.photoId);
    if (item.pinned && itemDay(r, item) !== null) {
      if (item.kind === server.BookItemPhoto) used.add(item.sourceId);
      chosen.push(item);
    }
  }

  for (const m of milestones) {
    const key = `${server.BookItemMilestone}:${m.id}`;
<<<<<<< HEAD
    const kept = current.get(key);
    const item = kept ?? newItem(server.BookItemMilestone, m.id);
    if (excluded.has(key) || item.pinned || !r.categories.has(categoryOf(r, item))) continue;
    if (!kept) {
      item.photoId = (m.photoIds ?? []).find(id => r.photos.has(id) && !used.has(id)) ?? 0;
      if (item.photoId) used.add(item.photoId);
    }
=======
    const item = current.get(key) ?? milestoneItem(r, m, used);
    if (excluded.has(key) || item.pinned || !r.categories.has(categoryOf(r, item))) continue;
    if (item.photoId) used.add(item.photoId);
>>>>>>> main
    chosen.push(item);
  }

  if (r.categories.has("photos")) {
    const buckets = new Map<string, { image: server.Image; day: string }[]>();
    for (const image of matching) {
      if (used.has(image.id)) continue;
      const key = bucketOf(r, newItem(server.BookItemPhoto, image.id));
      const list = buckets.get(key) ?? [];
      list.push({ image, day: dayOf(image.photoDate) });
      buckets.set(key, list);
    }
    const pinnedIn = new Map<string, number>();
    for (const item of chosen) {
      if (item.kind !== server.BookItemPhoto) continue;
      const key = bucketOf(r, item);
      pinnedIn.set(key, (pinnedIn.get(key) ?? 0) + 1);
    }
    for (const [key, list] of buckets) {
      const section = Number(key.split(":")[0]);
      const space = Math.max(0, room(section) - (pinnedIn.get(key) ?? 0));
      for (const { image } of spreadPick(list, space, PER_DAY[options.density])) {
        chosen.push(
          current.get(`${server.BookItemPhoto}:${image.id}`) ??
            newItem(server.BookItemPhoto, image.id)
        );
      }
    }
  }

  const items = options.current?.length
    ? mergeInOrder(r, options.current, chosen)
    : heroFirst(r, chosen);
  return { items, coverPhotoId: cover };
}

function byDay(r: Resolved) {
  return (a: server.BookItem, b: server.BookItem) =>
    (itemDay(r, a) ?? "").localeCompare(itemDay(r, b) ?? "") ||
    a.kind - b.kind ||
    a.sourceId - b.sourceId;
}

<<<<<<< HEAD
=======
// A chapter opens on a full-width photo when it has several loose photos, or
// loose photos and no milestone photo to lead with.
function wantsHero(r: Resolved, items: server.BookItem[]): boolean {
  const loose = items.filter(item => item.kind === server.BookItemPhoto).length;
  const momentPhoto = items.some(
    item => item.kind === server.BookItemMilestone && item.photoId && r.photos.has(item.photoId)
  );
  return loose >= 3 || (loose > 0 && !momentPhoto);
}

>>>>>>> main
// heroFirst moves each month's best opening photo to the front of that month.
function heroFirst(r: Resolved, items: server.BookItem[]): server.BookItem[] {
  const sorted = [...items].sort(byDay(r));
  const groups = new Map<string, server.BookItem[]>();
  for (const item of sorted) {
    const key = bucketOf(r, item);
    groups.set(key, [...(groups.get(key) ?? []), item]);
  }
  const heroes = new Map<string, server.BookItem>();
  for (const [key, group] of groups) {
<<<<<<< HEAD
    const loose = group.filter(item => item.kind === server.BookItemPhoto);
    const momentPhoto = group.some(item => item.kind === server.BookItemMilestone && item.photoId);
    if (loose.length >= 3 || (loose.length && !momentPhoto)) {
=======
    if (wantsHero(r, group)) {
      const loose = group.filter(item => item.kind === server.BookItemPhoto);
>>>>>>> main
      heroes.set(
        key,
        loose.find(item => {
          const p = r.photos.get(item.sourceId)!;
          return p.width >= p.height;
        }) ?? loose[0]
      );
    }
  }
  const out: server.BookItem[] = [];
  const placed = new Set<string>();
  for (const item of sorted) {
    const key = bucketOf(r, item);
    const hero = heroes.get(key);
    if (hero && !placed.has(key)) {
      placed.add(key);
      out.push(hero);
    }
    if (item !== hero) out.push(item);
  }
  return out;
}

function mergeInOrder(r: Resolved, current: server.BookItem[], chosen: server.BookItem[]) {
  const chosenKeys = new Set(chosen.map(itemKey));
  const out = current.filter(item => chosenKeys.has(itemKey(item)));
  const present = new Set(out.map(itemKey));
  for (const item of [...chosen].sort(byDay(r))) {
    if (present.has(itemKey(item))) continue;
    insertByDay(r, out, item);
    present.add(itemKey(item));
  }
  return out;
}

export function insertByDay(r: Resolved, items: server.BookItem[], item: server.BookItem) {
  const day = itemDay(r, item) ?? "";
  const at = items.findIndex(other => (itemDay(r, other) ?? "") > day);
  if (at < 0) items.push(item);
  else items.splice(at, 0, item);
}

export function photoDetail(r: Resolved, image: server.Image): string {
  const day = dayOf(image.photoDate);
  if (isMulti(r)) {
    const names = (r.photoPeople.get(image.id) ?? [])
      .map(id => r.person.get(id)?.name ?? "")
      .filter(Boolean);
    return names.length > 1 ? joinNames(names) : "";
  }
  const person = r.people[0];
  return person && isRealDay(dayOf(person.birthday)) && day >= dayOf(person.birthday)
    ? photoAge(person.birthday, day + "T00:00:00Z")
    : "";
}

function toPhoto(image: server.Image, r: Resolved, caption?: string): BookPhoto {
  return {
    id: image.id,
    width: image.width,
    height: image.height,
    day: dayOf(image.photoDate),
    caption: caption || originalCaption(image),
    detail: photoDetail(r, image),
  };
}

function toMoment(m: server.Milestone, r: Resolved): BookMoment {
  const day = dayOf(m.milestoneDate);
  const person = r.person.get(m.personId);
  const born = person ? dayOf(person.birthday) : "";
  return {
    id: m.id,
    text: m.description.trim(),
    context: (m.context ?? "").trim(),
    day,
    detail: isRealDay(born) && day >= born ? photoAge(born, day + "T00:00:00Z") : "",
    first: m.category === "first",
  };
}

interface Entry {
  index: number;
  item: server.BookItem;
  day: string;
}

function chapterBlocks(entries: Entry[], r: Resolved): Block[] {
  const blocks: Block[] = [];
<<<<<<< HEAD
  const loose = entries.filter(e => e.item.kind === server.BookItemPhoto);
  const hasMomentPhoto = entries.some(
    e => e.item.kind === server.BookItemMilestone && e.item.photoId && r.photos.has(e.item.photoId)
  );
  let rest = entries;
  const first = entries[0];
  if (first && first.item.kind === server.BookItemPhoto && (loose.length >= 3 || !hasMomentPhoto)) {
=======
  let rest = entries;
  const first = entries[0];
  if (
    first?.item.kind === server.BookItemPhoto &&
    wantsHero(
      r,
      entries.map(e => e.item)
    )
  ) {
>>>>>>> main
    blocks.push({
      kind: "hero",
      photo: toPhoto(r.photos.get(first.item.sourceId)!, r, first.item.caption),
    });
    rest = entries.slice(1);
  }

  let pendingPhotos: BookPhoto[] = [];
  let pendingNotes: BookMoment[] = [];
  const flushPhotos = () => {
    for (let i = 0; i < pendingPhotos.length; i += 4) {
      blocks.push({ kind: "photos", photos: pendingPhotos.slice(i, i + 4) });
    }
    pendingPhotos = [];
  };
  const flushNotes = () => {
    if (pendingNotes.length) blocks.push({ kind: "notes", moments: pendingNotes });
    pendingNotes = [];
  };

  for (const { item } of rest) {
    if (item.kind === server.BookItemPhoto) {
      flushNotes();
      pendingPhotos.push(toPhoto(r.photos.get(item.sourceId)!, r, item.caption));
      continue;
    }
    const m = r.milestones.get(item.sourceId)!;
    const moment = toMoment(m, r);
    const image = item.photoId ? r.photos.get(item.photoId) : undefined;
    const photo = image ? toPhoto(image, r, item.caption) : null;
    let block: Block | null = null;
    if (m.category === "quote") block = { kind: "quote", moment };
    else if (m.category === "artwork" && photo) block = { kind: "artwork", moment, photo };
    else if (photo || moment.first || moment.text.length > 160 || moment.context || item.pinned)
      block = { kind: "moment", moment, photo };
    if (block) {
      flushPhotos();
      flushNotes();
      blocks.push(block);
    } else {
      flushPhotos();
      pendingNotes.push(moment);
    }
  }
  flushPhotos();
  flushNotes();
  return blocks;
}

<<<<<<< HEAD
=======
const exactMonths = (from: string, day: string) =>
  (Date.parse(day + "T00:00:00Z") - Date.parse(from + "T00:00:00Z")) / (30.4375 * 86400000);

// Growth is charted against age, or against the book's own dates for someone
// without a birthday.
const ageOrigin = (r: Resolved, person: server.Person) =>
  isRealDay(dayOf(person.birthday)) ? dayOf(person.birthday) : r.start;

>>>>>>> main
function growthPoints(
  records: server.GrowthData[],
  type: server.MeasurementType,
  r: Resolved,
  person: server.Person
): GrowthPoint[] {
<<<<<<< HEAD
  const birthday = dayOf(person.birthday);
  const from = isRealDay(birthday) ? birthday : r.start;
  const born = new Date(from + "T00:00:00Z").getTime();
=======
  const from = ageOrigin(r, person);
>>>>>>> main
  return records
    .filter(
      g =>
        g.personId === person.id &&
        g.measurementType === type &&
        inRange(r, dayOf(g.measurementDate))
    )
    .map(g => {
      const day = dayOf(g.measurementDate);
<<<<<<< HEAD
      const months = monthsOld(from, day + "T00:00:00Z");
      const exact = (new Date(day + "T00:00:00Z").getTime() - born) / (30.4375 * 86400000);
      return {
        day,
        months: Math.max(0, exact),
        value: g.value,
        label: formatMeasurement(g.value, g.unit, months),
=======
      return {
        day,
        months: Math.max(0, exactMonths(from, day)),
        value: g.value,
        label: formatMeasurement(g.value, g.unit, monthsOld(from, day + "T00:00:00Z")),
>>>>>>> main
      };
    })
    .sort((a, b) => a.day.localeCompare(b.day));
}

function growthBlock(source: BookSource, r: Resolved, person: server.Person): Block | null {
  const height = growthPoints(source.growthData ?? [], server.Height, r, person);
  const weight = growthPoints(source.growthData ?? [], server.Weight, r, person);
  if (height.length < 2 && weight.length < 2) return null;
<<<<<<< HEAD
=======
  const from = ageOrigin(r, person);
>>>>>>> main
  return {
    kind: "growth",
    height: height.length >= 2 ? height : [],
    weight: weight.length >= 2 ? weight : [],
<<<<<<< HEAD
=======
    heightLabel: r.firstYear ? "Length" : "Height",
    span: [Math.max(0, exactMonths(from, r.start)), exactMonths(from, r.end)],
    ticks: r.firstYear
      ? ["birth", "one year"]
      : [monthName(r.start), monthName(addDays(r.end, -1))],
>>>>>>> main
  };
}

function birthFacts(source: BookSource, r: Resolved, person: server.Person): string[] {
  const birthDay = r.start;
  const lines = [`Born ${longDay(birthDay)}`];
  const when = (p: GrowthPoint) => {
    if (p.day === birthDay) return "at birth";
    const days = Math.round((Date.parse(p.day) - Date.parse(birthDay)) / 86400000);
    return `at ${days} ${days === 1 ? "day" : "days"} old`;
  };
  const early = (type: server.MeasurementType) =>
    growthPoints(source.growthData ?? [], type, r, person).find(p => p.day <= addDays(birthDay, 3));
  const weight = early(server.Weight);
  const height = early(server.Height);
  if (weight) lines.push(`Weighed ${weight.label} ${when(weight)}`);
  if (height) lines.push(`Measured ${height.label} long ${when(height)}`);
  return lines;
}

const WEIGHT: Record<number, number> = { [server.BookItemMilestone]: 3, [server.BookItemPhoto]: 1 };

function bySlot(r: Resolved, entries: Entry[]): Map<number, Entry[]> {
  const slots = new Map<number, Entry[]>();
  for (const e of entries) {
    const slot = slotOf(r, e.day);
    slots.set(slot, [...(slots.get(slot) ?? []), e]);
  }
  return slots;
}

function slotWeights(slots: Map<number, Entry[]>, from: number, count: number): number[] {
  return Array.from({ length: count }, (_, i) => {
    const entries = slots.get(from + i) ?? [];
    const milestones = entries.filter(e => e.item.kind === server.BookItemMilestone).length;
    return Math.min(
      entries.reduce((n, e) => n + WEIGHT[e.item.kind], 0),
      milestones * 3 + 6
    );
  });
}

function calendarChapters(r: Resolved, entries: Entry[], idPrefix: string): Chapter[] {
  const slots = bySlot(r, entries);
  const base = firstOfMonth(r.start);
  const maxSpan = Math.max(3, Math.ceil(r.months / 12));
  return groupMonths(slotWeights(slots, 0, r.months), 7, maxSpan).map(group => {
    const from = group[0];
    const to = group[group.length - 1];
    const picked = group.flatMap(i => slots.get(i) ?? []).sort((a, b) => a.index - b.index);
    return {
      id: `${idPrefix}${from}`,
<<<<<<< HEAD
      title: monthSpan(monthStart(base, from), monthStart(base, to)),
=======
      title: monthSpan(addMonths(base, from), addMonths(base, to)),
>>>>>>> main
      dates: "",
      blocks: chapterBlocks(picked, r),
      items: picked.map(e => e.index),
    };
  });
}

function hiddenCalendarMonths(r: Resolved, entries: Entry[]): string[] {
  const used = new Set(entries.map(e => slotOf(r, e.day)));
  const base = firstOfMonth(r.start);
  const hidden: string[] = [];
  for (let slot = 0; slot < r.months; slot++) {
<<<<<<< HEAD
    if (!used.has(slot)) hidden.push(monthName(monthStart(base, slot), true));
=======
    if (!used.has(slot)) hidden.push(monthName(addMonths(base, slot)));
>>>>>>> main
  }
  return hidden;
}

function ageLine(person: server.Person, r: Resolved): string {
  const birthday = dayOf(person.birthday);
  if (!isRealDay(birthday) || birthday > r.start) return "";
  const from = Math.floor(monthsOld(birthday, r.start + "T00:00:00Z") / 12);
  const to = Math.floor(monthsOld(birthday, addDays(r.end, -1) + "T00:00:00Z") / 12);
  if (from === to) return from === 0 ? "Under one" : `Age ${from}`;
  return `Age ${from} to ${to}`;
}

function firstYearChapters(
  source: BookSource,
  selection: Selection,
  r: Resolved,
  entries: Entry[]
): { chapters: Chapter[]; hiddenMonths: string[] } {
  const subject = r.people[0];
  const slots = bySlot(r, entries);
  const chapters: Chapter[] = [];
  const welcomeEntries = slots.get(0) ?? [];
  const welcome: Block[] = [];
  if (selection.introduction) {
    welcome.push({ kind: "letter", text: selection.introduction, signature: "" });
  }
  welcome.push({ kind: "facts", lines: birthFacts(source, r, subject) });
  welcome.push(...chapterBlocks(welcomeEntries, r));
  chapters.push({
    id: "welcome",
    title: "Welcome to the world",
<<<<<<< HEAD
    dates: monthSpan(r.start, addDays(monthStart(r.start, 1), -1)),
=======
    dates: monthSpan(r.start, addDays(addMonths(r.start, 1), -1)),
>>>>>>> main
    blocks: welcome,
    items: welcomeEntries.map(e => e.index),
  });

  const weights = slotWeights(slots, 1, 11);
<<<<<<< HEAD
  const hiddenMonths = weights.map((w, i) => (w === 0 ? `${i + 1} months` : "")).filter(Boolean);
=======
  const hiddenMonths = weights
    .map((w, i) => (w > 0 ? "" : i === 0 ? "1 month old" : `${i + 1} months old`))
    .filter(Boolean);
>>>>>>> main
  for (const group of groupMonths(weights)) {
    const from = group[0] + 1;
    const to = group[group.length - 1] + 1;
    const picked = group.flatMap(i => slots.get(i + 1) ?? []).sort((a, b) => a.index - b.index);
    chapters.push({
      id: `months-${from}`,
      title: chapterTitle(from, to),
<<<<<<< HEAD
      dates: monthSpan(monthStart(r.start, from), addDays(monthStart(r.start, to + 1), -1)),
=======
      dates: monthSpan(addMonths(r.start, from), addDays(addMonths(r.start, to + 1), -1)),
>>>>>>> main
      blocks: chapterBlocks(picked, r),
      items: picked.map(e => e.index),
    });
  }

  const growth = selection.showGrowth ? growthBlock(source, r, subject) : null;
  if (growth) {
    chapters.push({ id: "growth", title: "How you grew", dates: "", blocks: [growth], items: [] });
  }
  const birthday = slots.get(BIRTHDAY) ?? [];
  const closing = chapterBlocks(birthday, r);
  if (selection.letter) {
    closing.push({ kind: "letter", text: selection.letter, signature: selection.signature });
  }
  if (closing.length) {
    chapters.push({
      id: "birthday",
      title: birthday.length ? "Turning one" : "A letter for you",
      dates: birthday.length ? longDay(r.end) : "",
      blocks: closing,
      items: birthday.map(e => e.index),
    });
  }
  return { chapters, hiddenMonths };
}

function periodChapters(
  source: BookSource,
  selection: Selection,
  r: Resolved,
  entries: Entry[]
): { chapters: Chapter[]; hiddenMonths: string[] } {
  const chapters: Chapter[] = [];
  if (selection.introduction) {
    chapters.push({
      id: "introduction",
      title: "",
      dates: "",
      blocks: [{ kind: "letter", text: selection.introduction, signature: "" }],
      items: [],
    });
  }
  const shared = entries.filter(e => sectionOf(r, e.item) === 0);
  chapters.push(...calendarChapters(r, shared, isMulti(r) ? "together-" : "months-"));

  if (isMulti(r)) {
    for (const person of r.people) {
      const own = entries
        .filter(e => sectionOf(r, e.item) === person.id)
        .sort((a, b) => a.index - b.index);
      const growth = selection.showGrowth ? growthBlock(source, r, person) : null;
      if (!own.length && !growth) continue;
      const blocks = chapterBlocks(own, r);
      if (growth) blocks.push(growth);
      chapters.push({
        id: `person-${person.id}`,
        title: person.name,
        dates: ageLine(person, r),
        blocks,
        items: own.map(e => e.index),
      });
    }
  } else if (selection.showGrowth && r.people[0]) {
    const growth = growthBlock(source, r, r.people[0]);
    if (growth) {
      chapters.push({
        id: "growth",
        title: `How ${r.people[0].name} grew`,
        dates: "",
        blocks: [growth],
        items: [],
      });
    }
  }

  if (selection.letter) {
    chapters.push({
      id: "letter",
      title: "",
      dates: "",
      blocks: [{ kind: "letter", text: selection.letter, signature: selection.signature }],
      items: [],
    });
  }
  return { chapters, hiddenMonths: isMulti(r) ? [] : hiddenCalendarMonths(r, shared) };
}

export function assembleBook(source: BookSource, selection: Selection): Book {
  const r = resolveSource(source, selection);

  let missing = 0;
  const entries: Entry[] = [];
  selection.items.forEach((item, index) => {
    const day = itemDay(r, item);
    if (day === null || !isRealDay(day)) missing++;
    else entries.push({ index, item, day });
  });

<<<<<<< HEAD
  const { chapters, hiddenMonths } =
    r.firstYear && r.people.length === 1
      ? firstYearChapters(source, selection, r, entries)
      : periodChapters(source, selection, r, entries);
=======
  const { chapters, hiddenMonths } = r.firstYear
    ? firstYearChapters(source, selection, r, entries)
    : periodChapters(source, selection, r, entries);
>>>>>>> main

  const candidates = candidatesIn(r);
  const cover = r.photos.get(selection.coverPhotoId);
  const names = joinNames(r.people.map(p => p.name));
  return {
    name: names,
    title: selection.title || names,
    dates: bookDates(selection.startDate, selection.endDate),
    cover: cover ? toPhoto(cover, r) : null,
    chapters,
    ending:
      r.firstYear && r.people[0] ? `${r.people[0].name}, one year old` : selection.title || names,
    notes: {
      milestonesUsed: entries.filter(e => e.item.kind === server.BookItemMilestone).length,
      milestonesInRange: candidates.milestones.length,
      photosUsed:
        entries.filter(e => e.item.kind === server.BookItemPhoto).length +
        entries.filter(e => e.item.kind === server.BookItemMilestone && e.item.photoId).length +
        (cover ? 1 : 0),
      photosInRange: candidates.photos.length,
      undatedPhotos: r.undatedPhotos,
      unreadyPhotos: r.unreadyPhotos,
      missing,
      hiddenMonths,
    },
  };
}

export interface BookPlan {
  preset: string;
  title: string;
  startDate: string;
  endDate: string;
  categories: string[];
  match: string;
  showGrowth: boolean;
  density: Density;
}

export function draftSelection(source: BookSource, plan: BookPlan): Selection {
  const base: Selection = {
    preset: plan.preset,
    title: plan.title,
    startDate: plan.startDate,
    endDate: plan.endDate,
    coverPhotoId: 0,
    introduction: "",
    letter: "",
    signature: "",
    showGrowth: plan.showGrowth,
    categories: plan.categories,
    match: plan.match,
    items: [],
  };
  const { items, coverPhotoId } = suggestItems(resolveSource(source, base), {
    density: plan.density,
  });
  return { ...base, items, coverPhotoId };
}

<<<<<<< HEAD
export function firstYearPlan(person: server.Person, density: Density = "balanced"): BookPlan {
  const start = dayOf(person.birthday);
  return {
    preset: PRESETS.firstYear,
    title: `${person.name}'s first year`,
    startDate: start,
    endDate: firstBirthday(start),
    categories: [],
    match: "any",
    showGrowth: true,
    density,
  };
}

=======
>>>>>>> main
// additionsSince lists records the editor has not seen yet: added to the family
// record after the book was last reviewed, and neither in the book nor left out.
export function additionsSince(
  r: Resolved,
  reviewedAt: string,
  items: server.BookItem[],
  excluded: server.BookItem[]
): server.BookItem[] {
  const since = Date.parse(reviewedAt);
  const isNew = (createdAt: string) => Date.parse(createdAt) > since;
  const known = new Set([...items, ...excluded].map(itemKey));
  const usedPhotos = new Set(items.map(i => i.photoId).filter(Boolean));
  const { milestones, photos } = candidatesIn(r);
  const out: server.BookItem[] = [];
  for (const m of milestones) {
<<<<<<< HEAD
    const item = newItem(server.BookItemMilestone, m.id);
    if (isNew(m.createdAt) && !known.has(itemKey(item))) {
      item.photoId = (m.photoIds ?? []).find(id => r.photos.has(id)) ?? 0;
      out.push(item);
    }
=======
    const item = milestoneItem(r, m);
    if (isNew(m.createdAt) && !known.has(itemKey(item))) out.push(item);
>>>>>>> main
  }
  for (const p of photos) {
    const item = newItem(server.BookItemPhoto, p.id);
    const relevant = photoMatches(r, p.id) || r.untagged.has(p.id);
    if (isNew(p.createdAt) && relevant && !known.has(itemKey(item)) && !usedPhotos.has(p.id)) {
      out.push(item);
    }
  }
  return out;
}
