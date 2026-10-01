import * as server from "../server";
import { monthsOld, photoAge } from "./sameAge";
import { formatMeasurement } from "./weightFormat";

export type Density = "brief" | "balanced" | "detailed";

export const DENSITIES: Density[] = ["brief", "balanced", "detailed"];

export const isDensity = (value: string | null | undefined): value is Density =>
  DENSITIES.includes(value as Density);

const PHOTO_BUDGET: Record<Density, number> = { brief: 3, balanced: 7, detailed: 14 };
const PER_DAY: Record<Density, number> = { brief: 1, balanced: 2, detailed: 4 };

export interface BookPhoto {
  id: number;
  width: number;
  height: number;
  day: string;
  caption: string;
  age: string;
}

export interface BookMoment {
  id: number;
  text: string;
  context: string;
  day: string;
  age: string;
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
  | { kind: "growth"; height: GrowthPoint[]; weight: GrowthPoint[] };

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
  hiddenMonths: number[];
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

export interface BookSource {
  person: server.Person;
  milestones: server.Milestone[];
  photos: server.Image[];
  growthData: server.GrowthData[];
}

export interface Selection {
  title: string;
  startDate: string;
  endDate: string;
  coverPhotoId: number;
  introduction: string;
  letter: string;
  signature: string;
  showGrowth: boolean;
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
];

export const dayOf = (iso: string) => (iso ?? "").slice(0, 10);

const isRealDay = (day: string) => /^\d{4}-\d{2}-\d{2}$/.test(day) && day > "1000";

export function firstBirthday(birthday: string): string {
  const born = new Date(birthday);
  return new Date(Date.UTC(born.getUTCFullYear() + 1, born.getUTCMonth(), born.getUTCDate()))
    .toISOString()
    .slice(0, 10);
}

export function addDays(day: string, n: number): string {
  const d = new Date(day + "T00:00:00Z");
  d.setUTCDate(d.getUTCDate() + n);
  return d.toISOString().slice(0, 10);
}

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

export const bookDates = (startDate: string, endDate: string) =>
  `${longDay(dayOf(startDate))} – ${longDay(addDays(dayOf(endDate), -1))}`;

function monthSpan(from: string, to: string): string {
  const fmt = (day: string, year: boolean) =>
    new Date(day + "T00:00:00Z").toLocaleDateString("en-US", {
      month: "long",
      year: year ? "numeric" : undefined,
      timeZone: "UTC",
    });
  if (from.slice(0, 7) === to.slice(0, 7)) return fmt(from, true);
  if (from.slice(0, 4) === to.slice(0, 4)) return `${fmt(from, false)} – ${fmt(to, true)}`;
  return `${fmt(from, true)} – ${fmt(to, true)}`;
}

function monthStart(start: string, month: number): string {
  const born = new Date(start + "T00:00:00Z");
  return new Date(Date.UTC(born.getUTCFullYear(), born.getUTCMonth() + month, born.getUTCDate()))
    .toISOString()
    .slice(0, 10);
}

export function chapterTitle(from: number, to: number): string {
  const word = (n: number) => NUMBER_WORDS[n] ?? String(n);
  if (from === to) return from === 1 ? "One month" : `${word(from)} months`;
  return `${word(from)} to ${word(to).toLowerCase()} months`;
}

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

const BIRTHDAY = 12;

interface Range {
  start: string;
  end: string;
  birthday: string;
}

// slot is the month of age 0–11, or BIRTHDAY for records dated on the first birthday.
function slotOf(range: Range, day: string): number {
  if (day >= range.end) return BIRTHDAY;
  if (day < range.start) return 0;
  return Math.min(11, Math.max(0, monthsOld(range.start, day + "T00:00:00Z")));
}

export interface Resolved {
  person: server.Person;
  range: Range;
  photos: Map<number, server.Image>;
  milestones: Map<number, server.Milestone>;
  undatedPhotos: number;
  unreadyPhotos: number;
}

export function resolveSource(source: BookSource, startDate: string, endDate: string): Resolved {
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
  return {
    person: source.person,
    range: {
      start: dayOf(startDate),
      end: dayOf(endDate),
      birthday: source.person.birthday,
    },
    photos,
    milestones,
    undatedPhotos,
    unreadyPhotos,
  };
}

const inRange = (r: Range, day: string) => isRealDay(day) && day >= r.start && day <= r.end;

export function firstYearSelection(source: BookSource): Pick<Selection, "startDate" | "endDate"> {
  const start = dayOf(source.person.birthday);
  return { startDate: start + "T00:00:00Z", endDate: firstBirthday(start) + "T00:00:00Z" };
}

export function itemDay(r: Resolved, item: server.BookItem): string | null {
  if (item.kind === server.BookItemMilestone) {
    const m = r.milestones.get(item.sourceId);
    return m ? dayOf(m.milestoneDate) : null;
  }
  const p = r.photos.get(item.sourceId);
  return p ? dayOf(p.photoDate) : null;
}

export interface Candidates {
  milestones: server.Milestone[];
  photos: server.Image[];
}

export function candidatesIn(r: Resolved): Candidates {
  return {
    milestones: [...r.milestones.values()]
      .filter(m => inRange(r.range, dayOf(m.milestoneDate)))
      .sort((a, b) => a.milestoneDate.localeCompare(b.milestoneDate) || a.id - b.id),
    photos: [...r.photos.values()]
      .filter(p => inRange(r.range, dayOf(p.photoDate)))
      .sort((a, b) => a.photoDate.localeCompare(b.photoDate) || a.id - b.id),
  };
}

function chooseCover(r: Resolved, picked: server.Image[]): number {
  const profile = r.photos.get(r.person.profilePhotoId);
  if (profile && inRange(r.range, dayOf(profile.photoDate))) return profile.id;
  const older = picked.find(p => p.width >= p.height && monthsOld(r.range.start, p.photoDate) >= 2);
  return (older ?? picked[0])?.id ?? 0;
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

// Milestones are always offered; photos are budgeted per month of age. Pinned
// items survive, excluded ones never return, and a re-suggestion keeps the
// order of items it keeps.
export function suggestItems(r: Resolved, options: SuggestOptions): Suggestion {
  const excluded = new Set((options.excluded ?? []).map(itemKey));
  const current = new Map((options.current ?? []).map(item => [itemKey(item), item]));
  const { milestones, photos } = candidatesIn(r);
  const budget = PHOTO_BUDGET[options.density];
  const perDay = PER_DAY[options.density];

  const allCandidatePhotos = photos.filter(p => !excluded.has(`${server.BookItemPhoto}:${p.id}`));
  const cover =
    options.coverPhotoId && r.photos.has(options.coverPhotoId)
      ? options.coverPhotoId
      : chooseCover(
          r,
          allCandidatePhotos.filter(p => slotOf(r.range, dayOf(p.photoDate)) !== BIRTHDAY)
        );

  const used = new Set<number>([cover]);
  const chosen: server.BookItem[] = [];

  for (const item of current.values()) {
    if (item.kind === server.BookItemMilestone && item.photoId) used.add(item.photoId);
    if (item.kind === server.BookItemPhoto && item.pinned) used.add(item.sourceId);
  }

  for (const m of milestones) {
    const key = `${server.BookItemMilestone}:${m.id}`;
    if (excluded.has(key)) continue;
    const kept = current.get(key);
    if (kept) {
      chosen.push(kept);
      continue;
    }
    const photoId = (m.photoIds ?? []).find(id => r.photos.has(id) && !used.has(id)) ?? 0;
    if (photoId) used.add(photoId);
    chosen.push({
      kind: server.BookItemMilestone,
      sourceId: m.id,
      photoId,
      caption: "",
      pinned: false,
    });
  }

  type Loose = { image: server.Image; day: string };
  const bySlot = new Map<number, Loose[]>();
  for (const image of allCandidatePhotos) {
    if (used.has(image.id)) continue;
    const day = dayOf(image.photoDate);
    const slot = slotOf(r.range, day);
    const list = bySlot.get(slot) ?? [];
    list.push({ image, day });
    bySlot.set(slot, list);
  }
  for (const item of current.values()) {
    if (item.kind === server.BookItemPhoto && item.pinned && r.photos.has(item.sourceId)) {
      chosen.push(item);
    }
  }
  const pinnedPerSlot = new Map<number, number>();
  for (const item of chosen) {
    if (item.kind !== server.BookItemPhoto) continue;
    const slot = slotOf(r.range, itemDay(r, item)!);
    pinnedPerSlot.set(slot, (pinnedPerSlot.get(slot) ?? 0) + 1);
  }
  for (const [slot, list] of bySlot) {
    const room = Math.max(0, budget - (pinnedPerSlot.get(slot) ?? 0));
    for (const { image } of spreadPick(list, room, perDay)) {
      const key = `${server.BookItemPhoto}:${image.id}`;
      chosen.push(
        current.get(key) ?? {
          kind: server.BookItemPhoto,
          sourceId: image.id,
          photoId: 0,
          caption: "",
          pinned: false,
        }
      );
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

function heroFirst(r: Resolved, items: server.BookItem[]): server.BookItem[] {
  const sorted = [...items].sort(byDay(r));
  const out: server.BookItem[] = [];
  let i = 0;
  while (i < sorted.length) {
    const slot = slotOf(r.range, itemDay(r, sorted[i])!);
    let j = i;
    while (j < sorted.length && slotOf(r.range, itemDay(r, sorted[j])!) === slot) j++;
    const segment = sorted.slice(i, j);
    const loose = segment.filter(item => item.kind === server.BookItemPhoto);
    const hasMomentPhoto = segment.some(
      item => item.kind === server.BookItemMilestone && item.photoId
    );
    if (loose.length >= 3 || (loose.length && !hasMomentPhoto)) {
      const hero =
        loose.find(item => {
          const p = r.photos.get(item.sourceId)!;
          return p.width >= p.height;
        }) ?? loose[0];
      out.push(hero, ...segment.filter(item => item !== hero));
    } else {
      out.push(...segment);
    }
    i = j;
  }
  return out;
}

function mergeInOrder(
  r: Resolved,
  current: server.BookItem[],
  chosen: server.BookItem[]
): server.BookItem[] {
  const chosenKeys = new Set(chosen.map(itemKey));
  const out = current.filter(item => chosenKeys.has(itemKey(item)));
  const present = new Set(out.map(itemKey));
  const order = byDay(r);
  for (const item of [...chosen].sort(order)) {
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

function toPhoto(image: server.Image, r: Resolved, caption?: string): BookPhoto {
  const day = dayOf(image.photoDate);
  return {
    id: image.id,
    width: image.width,
    height: image.height,
    day,
    caption: caption || originalCaption(image),
    age: photoAge(r.range.birthday, day + "T00:00:00Z"),
  };
}

function toMoment(m: server.Milestone, r: Resolved): BookMoment {
  const day = dayOf(m.milestoneDate);
  return {
    id: m.id,
    text: m.description.trim(),
    context: (m.context ?? "").trim(),
    day,
    age: photoAge(r.range.birthday, day + "T00:00:00Z"),
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
  const loose = entries.filter(e => e.item.kind === server.BookItemPhoto);
  const hasMomentPhoto = entries.some(
    e => e.item.kind === server.BookItemMilestone && e.item.photoId && r.photos.has(e.item.photoId)
  );
  let rest = entries;
  const first = entries[0];
  if (first && first.item.kind === server.BookItemPhoto && (loose.length >= 3 || !hasMomentPhoto)) {
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

function growthPoints(records: server.GrowthData[], type: server.MeasurementType, r: Resolved) {
  const born = new Date(r.range.start + "T00:00:00Z").getTime();
  return records
    .filter(g => g.measurementType === type && inRange(r.range, dayOf(g.measurementDate)))
    .map(g => {
      const day = dayOf(g.measurementDate);
      const months = monthsOld(r.range.start, day + "T00:00:00Z");
      const exact = (new Date(day + "T00:00:00Z").getTime() - born) / (30.4375 * 86400000);
      return {
        day,
        months: Math.max(0, exact),
        value: g.value,
        label: formatMeasurement(g.value, g.unit, months),
      };
    })
    .sort((a, b) => a.day.localeCompare(b.day));
}

function birthFacts(points: { height: GrowthPoint[]; weight: GrowthPoint[] }, birthDay: string) {
  const lines: string[] = [];
  const early = (list: GrowthPoint[]) => list.find(p => p.day <= addDays(birthDay, 3));
  const when = (p: GrowthPoint) => {
    if (p.day === birthDay) return "at birth";
    const days = Math.round((Date.parse(p.day) - Date.parse(birthDay)) / 86400000);
    return `at ${days} ${days === 1 ? "day" : "days"} old`;
  };
  const weight = early(points.weight);
  const height = early(points.height);
  if (weight) lines.push(`Weighed ${weight.label} ${when(weight)}`);
  if (height) lines.push(`Measured ${height.label} long ${when(height)}`);
  return lines;
}

const WEIGHT: Record<number, number> = { [server.BookItemMilestone]: 3, [server.BookItemPhoto]: 1 };

export function assembleBook(source: BookSource, selection: Selection): Book {
  const r = resolveSource(source, selection.startDate, selection.endDate);
  const { start, end } = r.range;

  let missing = 0;
  const slots: Entry[][] = Array.from({ length: BIRTHDAY + 1 }, () => []);
  selection.items.forEach((item, index) => {
    const day = itemDay(r, item);
    if (day === null || !isRealDay(day)) {
      missing++;
      return;
    }
    slots[slotOf(r.range, day)].push({ index, item, day });
  });

  const cover = r.photos.get(selection.coverPhotoId);
  const chapters: Chapter[] = [];
  const growth = {
    height: growthPoints(source.growthData ?? [], server.Height, r),
    weight: growthPoints(source.growthData ?? [], server.Weight, r),
  };

  const welcome: Block[] = [];
  if (selection.introduction) {
    welcome.push({ kind: "letter", text: selection.introduction, signature: "" });
  }
  welcome.push({ kind: "facts", lines: [`Born ${longDay(start)}`, ...birthFacts(growth, start)] });
  welcome.push(...chapterBlocks(slots[0], r));
  chapters.push({
    id: "welcome",
    title: "Welcome to the world",
    dates: monthSpan(start, addDays(monthStart(start, 1), -1)),
    blocks: welcome,
    items: slots[0].map(e => e.index),
  });

  const weights = slots.slice(1, BIRTHDAY).map(entries =>
    Math.min(
      entries.reduce((n, e) => n + WEIGHT[e.item.kind], 0),
      entries.filter(e => e.item.kind === server.BookItemMilestone).length * 3 + 6
    )
  );
  const hiddenMonths = weights.map((w, i) => (w === 0 ? i + 1 : -1)).filter(m => m > 0);
  for (const group of groupMonths(weights)) {
    const from = group[0] + 1;
    const to = group[group.length - 1] + 1;
    const entries = group.flatMap(i => slots[i + 1]).sort((a, b) => a.index - b.index);
    chapters.push({
      id: `months-${from}`,
      title: chapterTitle(from, to),
      dates: monthSpan(monthStart(start, from), addDays(monthStart(start, to + 1), -1)),
      blocks: chapterBlocks(entries, r),
      items: entries.map(e => e.index),
    });
  }

  if (selection.showGrowth && growth.height.length + growth.weight.length >= 2) {
    chapters.push({
      id: "growth",
      title: "How you grew",
      dates: "",
      blocks: [
        {
          kind: "growth",
          height: growth.height.length >= 2 ? growth.height : [],
          weight: growth.weight.length >= 2 ? growth.weight : [],
        },
      ],
      items: [],
    });
  }

  const birthday = slots[BIRTHDAY];
  const closing = chapterBlocks(birthday, r);
  if (selection.letter) {
    closing.push({ kind: "letter", text: selection.letter, signature: selection.signature });
  }
  if (closing.length) {
    chapters.push({
      id: "birthday",
      title: birthday.length ? "Turning one" : "A letter for you",
      dates: birthday.length ? longDay(end) : "",
      blocks: closing,
      items: birthday.map(e => e.index),
    });
  }

  const candidates = candidatesIn(r);
  const placed = slots.flat();
  return {
    name: r.person.name,
    title: selection.title || `${r.person.name}'s first year`,
    dates: bookDates(selection.startDate, selection.endDate),
    cover: cover ? toPhoto(cover, r) : null,
    chapters,
    ending: `${r.person.name}, one year old`,
    notes: {
      milestonesUsed: placed.filter(e => e.item.kind === server.BookItemMilestone).length,
      milestonesInRange: candidates.milestones.length,
      photosUsed:
        placed.filter(e => e.item.kind === server.BookItemPhoto).length +
        placed.filter(e => e.item.kind === server.BookItemMilestone && e.item.photoId).length +
        (cover ? 1 : 0),
      photosInRange: candidates.photos.length,
      undatedPhotos: r.undatedPhotos,
      unreadyPhotos: r.unreadyPhotos,
      missing,
      hiddenMonths,
    },
  };
}

export function draftSelection(source: BookSource, density: Density = "balanced"): Selection {
  const range = firstYearSelection(source);
  const r = resolveSource(source, range.startDate, range.endDate);
  const { items, coverPhotoId } = suggestItems(r, { density });
  return {
    title: `${source.person.name}'s first year`,
    ...range,
    coverPhotoId,
    introduction: "",
    letter: "",
    signature: "",
    showGrowth: true,
    items,
  };
}
