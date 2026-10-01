import * as server from "../server";
import { monthsOld, photoAge } from "./sameAge";
import { formatMeasurement } from "./weightFormat";

export type Density = "brief" | "balanced" | "detailed";

export const DENSITIES: Density[] = ["brief", "balanced", "detailed"];

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
}

export interface EditorNotes {
  milestonesUsed: number;
  milestonesInRange: number;
  photosUsed: number;
  photosInRange: number;
  undatedPhotos: number;
  unreadyPhotos: number;
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

export interface BookWriting {
  introduction?: string;
  letter?: string;
  signature?: string;
}

export interface BookSource {
  person: server.Person;
  milestones: server.Milestone[];
  photos: server.Image[];
  growthData: server.GrowthData[];
  writing?: BookWriting;
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

function monthStart(birthday: string, month: number): string {
  const born = new Date(birthday);
  return new Date(Date.UTC(born.getUTCFullYear(), born.getUTCMonth() + month, born.getUTCDate()))
    .toISOString()
    .slice(0, 10);
}

export function chapterTitle(from: number, to: number): string {
  const word = (n: number) => NUMBER_WORDS[n] ?? String(n);
  if (from === to) return from === 1 ? "One month" : `${word(from)} months`;
  return `${word(from)} to ${word(to).toLowerCase()} months`;
}

function toPhoto(image: server.Image, birthday: string): BookPhoto {
  const day = dayOf(image.photoDate);
  return {
    id: image.id,
    width: image.width,
    height: image.height,
    day,
    caption: image.description || (isGeneratedTitle(image) ? "" : image.title),
    age: photoAge(birthday, day + "T00:00:00Z"),
  };
}

function isGeneratedTitle(image: server.Image): boolean {
  const title = (image.title ?? "").trim();
  return !title || title === image.originalFilename || /^(IMG|DSC|PXL|photo)[-_ ]?\d/i.test(title);
}

function toMoment(m: server.Milestone, birthday: string): BookMoment {
  const day = dayOf(m.milestoneDate);
  return {
    text: m.description.trim(),
    context: (m.context ?? "").trim(),
    day,
    age: photoAge(birthday, day + "T00:00:00Z"),
    first: m.category === "first",
  };
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

interface MonthBucket {
  month: number;
  milestones: server.Milestone[];
  photos: BookPhoto[];
}

const bucketWeight = (b: MonthBucket) => b.milestones.length * 3 + Math.min(b.photos.length, 6);

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

function chapterBlocks(
  milestones: server.Milestone[],
  photos: BookPhoto[],
  photoById: Map<number, BookPhoto>,
  birthday: string,
  budget: number,
  perDay: number,
  used: Set<number>
): Block[] {
  type Entry = {
    day: string;
    block: Block | null;
    photo: BookPhoto | null;
    plain: BookMoment | null;
  };
  const entries: Entry[] = [];

  for (const m of [...milestones].sort((a, b) => a.milestoneDate.localeCompare(b.milestoneDate))) {
    const moment = toMoment(m, birthday);
    const photo = (m.photoIds ?? []).map(id => photoById.get(id)).find(p => p && !used.has(p.id));
    if (photo) used.add(photo.id);
    if (m.category === "quote") {
      entries.push({ day: moment.day, block: { kind: "quote", moment }, photo: null, plain: null });
    } else if (m.category === "artwork" && photo) {
      entries.push({
        day: moment.day,
        block: { kind: "artwork", moment, photo },
        photo: null,
        plain: null,
      });
    } else if (photo || moment.first || moment.text.length > 160 || moment.context) {
      entries.push({
        day: moment.day,
        block: { kind: "moment", moment, photo: photo ?? null },
        photo: null,
        plain: null,
      });
    } else {
      entries.push({ day: moment.day, block: null, photo: null, plain: moment });
    }
  }

  const loose = spreadPick(
    photos.filter(p => !used.has(p.id)),
    budget,
    perDay
  );
  loose.forEach(p => used.add(p.id));

  const blocks: Block[] = [];
  const hasPhotoMoment = entries.some(e => e.block?.kind === "moment" && e.block.photo);
  let rest = loose;
  if (loose.length >= 3 || (loose.length && !hasPhotoMoment)) {
    const hero = loose.find(p => p.width >= p.height) ?? loose[0];
    blocks.push({ kind: "hero", photo: hero });
    rest = loose.filter(p => p !== hero);
  }
  for (const p of rest) entries.push({ day: p.day, block: null, photo: p, plain: null });

  entries.sort((a, b) => a.day.localeCompare(b.day) || rank(a) - rank(b));

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
  for (const entry of entries) {
    if (entry.photo) {
      flushNotes();
      pendingPhotos.push(entry.photo);
    } else if (entry.plain) {
      flushPhotos();
      pendingNotes.push(entry.plain);
    } else if (entry.block) {
      flushPhotos();
      flushNotes();
      blocks.push(entry.block);
    }
  }
  flushPhotos();
  flushNotes();
  return blocks;
}

const rank = (e: { block: Block | null; photo: BookPhoto | null }) =>
  e.block ? 0 : e.photo ? 2 : 1;

function growthPoints(
  records: server.GrowthData[],
  type: server.MeasurementType,
  birthday: string
): GrowthPoint[] {
  return records
    .filter(r => r.measurementType === type)
    .map(r => {
      const day = dayOf(r.measurementDate);
      const months = monthsOld(birthday, day + "T00:00:00Z");
      const born = new Date(birthday).getTime();
      const exact = (new Date(day + "T00:00:00Z").getTime() - born) / (30.4375 * 86400000);
      return {
        day,
        months: Math.max(0, exact),
        value: r.value,
        label: formatMeasurement(r.value, r.unit, months),
      };
    })
    .sort((a, b) => a.day.localeCompare(b.day));
}

function birthFacts(
  points: { height: GrowthPoint[]; weight: GrowthPoint[] },
  birthDay: string
): string[] {
  const lines: string[] = [];
  const early = (list: GrowthPoint[]) => list.find(p => p.day <= addDays(birthDay, 3));
  const weight = early(points.weight);
  const height = early(points.height);
  const when = (p: GrowthPoint) => {
    if (p.day === birthDay) return "at birth";
    const days = Math.round((Date.parse(p.day) - Date.parse(birthDay)) / 86400000);
    return `at ${days} ${days === 1 ? "day" : "days"} old`;
  };
  if (weight) lines.push(`Weighed ${weight.label} ${when(weight)}`);
  if (height) lines.push(`Measured ${height.label} long ${when(height)}`);
  return lines;
}

export function assembleFirstYear(source: BookSource, density: Density = "balanced"): Book {
  const { person } = source;
  const birthday = person.birthday;
  const birthDay = dayOf(birthday);
  const endDay = firstBirthday(birthday);
  const budget = PHOTO_BUDGET[density];
  const perDay = PER_DAY[density];
  const inYear = (day: string) => isRealDay(day) && day >= birthDay && day < endDay;
  const onBirthday = (day: string) => day === endDay;

  let undatedPhotos = 0;
  let unreadyPhotos = 0;
  const photoById = new Map<number, BookPhoto>();
  for (const image of source.photos) {
    if (image.status !== 0) {
      unreadyPhotos++;
      continue;
    }
    const day = dayOf(image.photoDate);
    if (!isRealDay(day)) {
      undatedPhotos++;
      continue;
    }
    photoById.set(image.id, toPhoto(image, birthday));
  }
  const allPhotos = [...photoById.values()].sort(
    (a, b) => a.day.localeCompare(b.day) || a.id - b.id
  );
  const yearPhotos = allPhotos.filter(p => inYear(p.day));
  const yearMilestones = source.milestones.filter(m => inYear(dayOf(m.milestoneDate)));

  const used = new Set<number>();
  const profile = person.profilePhotoId ? photoById.get(person.profilePhotoId) : undefined;
  const cover =
    (profile && (inYear(profile.day) || onBirthday(profile.day)) ? profile : undefined) ??
    yearPhotos.find(p => p.width >= p.height && monthsOld(birthday, p.day + "T00:00:00Z") >= 2) ??
    yearPhotos[0] ??
    null;
  if (cover) used.add(cover.id);

  const buckets: MonthBucket[] = Array.from({ length: 12 }, (_, month) => ({
    month,
    milestones: [],
    photos: [],
  }));
  for (const m of yearMilestones) {
    const month = monthsOld(birthday, dayOf(m.milestoneDate) + "T00:00:00Z");
    buckets[Math.min(11, Math.max(0, month))].milestones.push(m);
  }
  for (const p of yearPhotos) {
    const month = monthsOld(birthday, p.day + "T00:00:00Z");
    buckets[Math.min(11, Math.max(0, month))].photos.push(p);
  }

  const growth = {
    height: growthPoints(source.growthData, server.Height, birthday).filter(
      p => inYear(p.day) || onBirthday(p.day)
    ),
    weight: growthPoints(source.growthData, server.Weight, birthday).filter(
      p => inYear(p.day) || onBirthday(p.day)
    ),
  };

  const chapters: Chapter[] = [];
  const writing = source.writing ?? {};

  const welcome: Block[] = [];
  if (writing.introduction)
    welcome.push({ kind: "letter", text: writing.introduction, signature: "" });
  const facts = birthFacts(growth, birthDay);
  facts.unshift(`Born ${longDay(birthDay)}`);
  welcome.push({ kind: "facts", lines: facts });
  const firstWeeks = buckets[0];
  welcome.push(
    ...chapterBlocks(
      firstWeeks.milestones,
      firstWeeks.photos,
      photoById,
      birthday,
      budget,
      perDay,
      used
    )
  );
  chapters.push({
    id: "welcome",
    title: "Welcome to the world",
    dates: monthSpan(birthDay, addDays(monthStart(birthday, 1), -1)),
    blocks: welcome,
  });

  const later = buckets.slice(1);
  const groups = groupMonths(later.map(bucketWeight));
  const hiddenMonths = later.filter(b => bucketWeight(b) === 0).map(b => b.month);
  for (const group of groups) {
    const months = group.map(i => later[i]);
    const from = months[0].month;
    const to = months[months.length - 1].month;
    const blocks = chapterBlocks(
      months.flatMap(b => b.milestones),
      months.flatMap(b => b.photos),
      photoById,
      birthday,
      budget,
      perDay,
      used
    );
    if (!blocks.length) continue;
    chapters.push({
      id: `months-${from}`,
      title: chapterTitle(from, to),
      dates: monthSpan(monthStart(birthday, from), addDays(monthStart(birthday, to + 1), -1)),
      blocks,
    });
  }

  if (growth.height.length + growth.weight.length >= 2) {
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
    });
  }

  const birthdayMilestones = source.milestones.filter(m => onBirthday(dayOf(m.milestoneDate)));
  const birthdayPhotos = allPhotos.filter(p => onBirthday(p.day));
  const closing = chapterBlocks(
    birthdayMilestones,
    birthdayPhotos,
    photoById,
    birthday,
    budget,
    perDay,
    used
  );
  if (writing.letter) {
    closing.push({ kind: "letter", text: writing.letter, signature: writing.signature ?? "" });
  }
  if (closing.length) {
    chapters.push({
      id: "birthday",
      title:
        birthdayMilestones.length || birthdayPhotos.length ? "Turning one" : "A letter for you",
      dates: birthdayMilestones.length || birthdayPhotos.length ? longDay(endDay) : "",
      blocks: closing,
    });
  }

  const usedMilestones = chapters
    .flatMap(c => c.blocks)
    .reduce((n, b) => n + ("moment" in b ? 1 : b.kind === "notes" ? b.moments.length : 0), 0);

  return {
    name: person.name,
    title: `${person.name}'s first year`,
    dates: `${longDay(birthDay)} – ${longDay(addDays(endDay, -1))}`,
    cover,
    chapters,
    ending: `${person.name}, one year old`,
    notes: {
      milestonesUsed: usedMilestones,
      milestonesInRange: yearMilestones.length + birthdayMilestones.length,
      photosUsed: used.size,
      photosInRange: yearPhotos.length + birthdayPhotos.length,
      undatedPhotos,
      unreadyPhotos,
      hiddenMonths,
    },
  };
}
