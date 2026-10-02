import * as server from "../server";
import {
  BookPlan,
  BookSource,
  Density,
  PRESETS,
  Selection,
<<<<<<< HEAD
  draftSelection,
  firstYearPlan,
=======
  dayOf,
  draftSelection,
  firstBirthday,
>>>>>>> main
} from "./book";

type Writing = { introduction?: string; letter?: string; signature?: string };

interface SampleSource {
  people: server.Person[];
  milestones: server.Milestone[];
  photos: server.Image[];
  growthData: server.GrowthData[];
  tags?: Record<number, number[]>;
  untagged?: number[];
  writing?: Writing;
}

export type Sample = "rich" | "sparse" | "uneven" | "family";

export const SAMPLES: { value: Sample; label: string }[] = [
  { value: "rich", label: "Rich" },
  { value: "sparse", label: "Sparse" },
  { value: "uneven", label: "Uneven" },
  { value: "family", label: "Family year" },
];

export const isSample = (value: string | null): value is Sample =>
  SAMPLES.some(s => s.value === value);

// Sample photos use negative ids; the reader draws them as tinted placeholders.
export const isPlaceholderPhoto = (id: number) => id < 0;

const BIRTHDAY = "2024-03-14T00:00:00Z";

function day(offset: number): string {
  const d = new Date(BIRTHDAY);
  d.setUTCDate(d.getUTCDate() + offset);
  return d.toISOString();
}

function person(name: string, profilePhotoId = 0, id = 1, birthday = BIRTHDAY): server.Person {
  return {
    id,
    familyId: 0,
    name,
    gender: 1,
    birthday,
    age: "",
    profilePhotoId,
    profileCropX: 50,
    profileCropY: 50,
    profileCropScale: 1,
    isPregnancy: false,
    relationship: "",
  };
}

type M = [offset: number, category: string, text: string, context?: string, photo?: number];

function milestones(list: M[], personId = 1, firstId = 1): server.Milestone[] {
  return list.map(([offset, category, description, context, photo], i) => ({
    id: firstId + i,
    personId,
    familyId: 0,
    description,
    category,
    context: context ?? "",
    milestoneDate: day(offset),
    createdAt: day(offset),
    photoIds: photo ? [photo] : [],
    tagIds: [],
  }));
}

type P = [id: number, offset: number, shape?: "tall" | "wide" | "square", caption?: string];

function photos(list: P[]): server.Image[] {
  return list.map(([id, offset, shape, caption]) => ({
    id,
    familyId: 0,
    ownerUserId: 0,
    originalFilename: `IMG_${1000 - id}.jpg`,
    mimeType: "image/jpeg",
    fileSize: 0,
    width: shape === "tall" ? 900 : shape === "square" ? 1000 : 1200,
    height: shape === "tall" ? 1200 : shape === "square" ? 1000 : 800,
    filePath: "",
    title: "",
    description: caption ?? "",
    photoDate: day(offset),
    createdAt: day(offset + 20),
    status: 0,
    analysisStatus: 0,
    tagIds: [],
  }));
}

function growth(
  list: [offset: number, kind: "h" | "w", value: number][],
  personId = 1,
  firstId = 1
): server.GrowthData[] {
  return list.map(([offset, kind, value], i) => ({
    id: firstId + i,
    personId,
    familyId: 0,
    measurementType: kind === "h" ? server.Height : server.Weight,
    value,
    unit: kind === "h" ? "in" : "lbs",
    measurementDate: day(offset),
    createdAt: day(offset),
  }));
}

function burst(startId: number, offset: number, count: number): P[] {
  const shapes: P[2][] = ["wide", "tall", "square", "wide", "tall"];
  return Array.from({ length: count }, (_, i) => [startId - i, offset, shapes[i % shapes.length]]);
}

function rich(): SampleSource {
  const ph: P[] = [
    [-1, 0, "wide", "Ten minutes old"],
    [-2, 1, "tall", "Going home"],
    [-3, 3, "square"],
    [-4, 9, "wide", "Meeting Grandma"],
    [-5, 15, "tall"],
    [-6, 24, "wide"],
    [-7, 42, "square", "That smile"],
    [-8, 50, "wide"],
    [-9, 63, "tall"],
    [-10, 70, "wide", "First trip to the lake"],
    [-11, 88, "square"],
    [-12, 101, "wide"],
    [-13, 120, "tall", "Rolled right off the blanket"],
    [-14, 135, "wide"],
    [-15, 150, "wide", "Easter at the farm"],
    [-16, 170, "square"],
    [-17, 183, "tall", "Sweet potato, mostly on her face"],
    [-18, 190, "wide"],
    [-19, 200, "square", "Handprints for the fridge"],
    [-20, 214, "wide"],
    [-21, 230, "tall"],
    [-22, 241, "wide", "On the move"],
    [-23, 255, "square"],
    [-24, 270, "wide", "Halloween — a very small pumpkin"],
    [-25, 284, "tall"],
    [-26, 300, "wide"],
    [-27, 318, "square", "Standing at the couch"],
    [-28, 330, "tall"],
    [-29, 341, "wide", "Christmas morning"],
    [-30, 352, "square"],
    [-31, 359, "wide"],
    [-32, 365, "wide", "Cake, conquered"],
    [-33, 365, "tall"],
    ...burst(-40, 22, 8),
    ...burst(-60, 155, 10),
    ...burst(-80, 341, 6),
  ];
  return {
    people: [person("Juniper", -7)],
    photos: photos(ph),
    milestones: milestones([
      [
        0,
        "other",
        "Born at 4:12 in the morning after a long night. Dad cut the cord and cried more than you did.",
        "",
        -1,
      ],
      [2, "other", "Came home in the yellow sweater Grandma knit."],
      [9, "other", "Grandma drove eight hours to hold you for the first time."],
      [12, "behavior", "Only falls asleep to the dryer running."],
      [42, "first", "First real smile — at Dad, mid-diaper change.", "", -7],
      [55, "development", "Holds her head up during tummy time for almost a minute."],
      [70, "first", "First trip to the lake. Slept through the whole thing.", "", -10],
      [98, "development", "Found her feet. Will not stop looking at them."],
      [104, "first", "First laugh — a real belly laugh when the dog sneezed."],
      [120, "first", "Rolled over, back to front, then looked shocked about it.", "", -13],
      [151, "behavior", "Hates the car seat. Loves the vacuum."],
      [183, "first", "First solid food: sweet potato. Verdict unclear.", "", -17],
      [200, "artwork", "Handprint painting at daycare", "Painted with Ms. Rosa's class.", -19],
      [214, "health", "First cold. Long nights, lots of rocking."],
      [241, "first", "Crawling! Backwards first, then forwards a week later.", "", -22],
      [252, "first", "First tooth, bottom left."],
      [276, "development", "Waves bye-bye to everyone, including the mailbox."],
      [300, "quote", "Dada!", "Said clearly, and on purpose, while pointing at him."],
      [318, "first", "Pulled up to standing on the couch.", "", -27],
      [333, "quote", "Uh-oh.", "Every single time she drops something. On purpose."],
      [
        350,
        "first",
        "First steps — three of them, from the couch to Mom.",
        "We were all on the floor cheering.",
      ],
      [365, "other", "First birthday party. Twelve people, one smash cake, zero naps.", "", -32],
    ]),
    growthData: growth([
      [0, "w", 7.25],
      [0, "h", 20],
      [5, "w", 7.0],
      [14, "w", 7.6],
      [60, "w", 11.2],
      [60, "h", 23],
      [120, "w", 13.9],
      [120, "h", 24.8],
      [183, "w", 16.1],
      [183, "h", 26.3],
      [274, "w", 18.4],
      [274, "h", 28],
      [365, "w", 20.6],
      [365, "h", 29.5],
    ]),
    writing: {
      introduction:
        "We waited a long time for you. This is the story of your first year, mostly in pictures, partly in the little notes we scribbled down so we would not forget.",
      letter:
        "Juniper, you made this year the best and most tired one of our lives. You are curious, stubborn, and you laugh with your whole body. We can't wait to see who you become.",
      signature: "Love, Mom and Dad",
    },
  };
}

function sparse(): SampleSource {
  return {
    people: [person("Theo")],
    photos: photos([
      [-1, 1, "tall"],
      [-2, 95, "wide"],
      [-3, 210, "square"],
      [-4, 300, "wide"],
      [-5, 365, "wide"],
    ]),
    milestones: milestones([
      [44, "first", "First smile."],
      [190, "first", "Started on solids."],
      [260, "first", "Crawling."],
      [355, "first", "Walked across the kitchen."],
    ]),
    growthData: growth([
      [3, "w", 6.8],
      [182, "w", 15.5],
    ]),
  };
}

function uneven(): SampleSource {
  return {
    people: [person("Rosie", -3)],
    photos: photos([
      ...burst(-1, 0, 6),
      ...burst(-10, 4, 9),
      ...burst(-20, 18, 12),
      ...burst(-40, 33, 7),
      [-50, 160, "wide"],
      ...burst(-60, 330, 9),
      ...burst(-70, 348, 5),
    ]),
    milestones: milestones([
      [0, "other", "Arrived two weeks early, on a snowy Thursday."],
      [3, "behavior", "Hiccups constantly. The pediatrician says this is normal."],
      [6, "other", "Her big brother asked if we could keep her."],
      [11, "health", "Back to birth weight."],
      [19, "behavior", "Best sleep so far: four hours straight!"],
      [27, "development", "Tracks faces across the room."],
      [36, "first", "First smile, at her brother."],
      [334, "first", "First steps at the park."],
      [347, "quote", "Ball!"],
    ]),
    growthData: [],
    writing: {
      introduction:
        "Our second baby, and somehow everything was new again. The middle of this year is a blur — but here is what we caught.",
    },
  };
}

function family(): SampleSource {
  const theo = "2021-06-01T00:00:00Z";
  const jan1 = 293;
  const at = (days: number) => jan1 + days;
  const together: P[] = [
    [-1, at(0), "wide", "New Year's morning"],
    [-2, at(45), "wide"],
    [-3, at(90), "square", "Easter egg hunt"],
    [-4, at(150), "wide", "First swim of the summer"],
    [-5, at(152), "tall"],
    [-6, at(200), "wide", "Camping at the lake"],
    [-7, at(203), "wide"],
    [-8, at(240), "square", "First day of school"],
    [-9, at(303), "wide", "Halloween — a dragon and a very small pumpkin"],
    [-10, at(357), "wide", "Christmas morning"],
    [-11, at(358), "tall"],
  ];
  const june: P[] = [
    [-20, at(20), "tall"],
    [-21, at(72), "wide"],
    [-22, at(130), "square", "Strawberry picking"],
    [-23, at(210), "tall"],
    [-24, at(280), "wide"],
  ];
  const theoOnly: P[] = [
    [-30, at(15), "wide", "Snow fort"],
    [-31, at(100), "tall"],
    [-32, at(185), "wide", "Training wheels off"],
    [-33, at(260), "square"],
  ];
  const nobody: P[] = [
    [-40, at(33), "wide", "Grandma's birthday dinner"],
    [-41, at(330), "wide"],
  ];
  const tags: Record<number, number[]> = {};
  together.forEach(([id]) => (tags[id] = [1, 2]));
  june.forEach(([id]) => (tags[id] = [1]));
  theoOnly.forEach(([id]) => (tags[id] = [2]));
  nobody.forEach(([id]) => (tags[id] = []));
  return {
    people: [person("Juniper", 0, 1), person("Theo", 0, 2, theo)],
    photos: photos([...together, ...june, ...theoOnly, ...nobody]),
    tags,
    untagged: nobody.map(([id]) => id),
    milestones: [
      ...milestones(
        [
          [at(10), "first", "Said her first full sentence: “Doggy go outside.”"],
          [
            at(60),
            "quote",
            "I'm not little, I'm medium.",
            "When asked if she was too little for the slide.",
          ],
          [at(125), "first", "Climbed out of the crib. The crib's days are numbered.", "", -22],
          [at(222), "development", "Counts to ten, skipping seven every time."],
          [at(300), "artwork", "Self-portrait with seven arms", "Drawn at preschool.", -24],
        ],
        1,
        1
      ),
      ...milestones(
        [
          [at(5), "achievement", "Learned to zip his own coat."],
          [at(98), "quote", "Why do worms come out when it rains? Are they thirsty?"],
          [
            at(185),
            "first",
            "Rode his bike without training wheels.",
            "Two laps of the cul-de-sac before he noticed.",
            -32,
          ],
          [at(240), "first", "First day of kindergarten. Didn't look back."],
          [
            at(320),
            "other",
            "Lost his first tooth and immediately asked about the tooth fairy's salary.",
          ],
        ],
        2,
        20
      ),
    ],
    growthData: [
      ...growth(
        [
          [at(10), "w", 24.5],
          [at(200), "w", 27.1],
          [at(10), "h", 33],
          [at(200), "h", 35.5],
        ],
        1,
        1
      ),
      ...growth(
        [
          [at(40), "w", 38],
          [at(250), "w", 41.5],
          [at(40), "h", 41],
          [at(250), "h", 43.2],
        ],
        2,
        20
      ),
    ],
    writing: {
      introduction: "Another year of the four of us. Here is what we managed to write down.",
      letter: "To Juniper and Theo: you made this year loud, sticky and wonderful.",
      signature: "Love, Mom and Dad",
    },
  };
}

function sample(name: Sample): SampleSource {
  if (name === "sparse") return sparse();
  if (name === "uneven") return uneven();
  if (name === "family") return family();
  return rich();
}

function toSource(raw: SampleSource): BookSource {
  const photoPeople: Record<number, number[]> = {};
  for (const photo of raw.photos)
    photoPeople[photo.id] = raw.tags?.[photo.id] ?? [raw.people[0].id];
  return {
    people: raw.people,
    milestones: raw.milestones,
    photos: raw.photos,
    growthData: raw.growthData,
    photoPeople,
    untagged: raw.untagged ?? [],
  };
}

export function sampleSource(name: Sample): BookSource {
  return toSource(sample(name));
}

<<<<<<< HEAD
=======
function firstYearPlan(person: server.Person, density: Density): BookPlan {
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

>>>>>>> main
export function samplePlan(name: Sample, density: Density = "balanced"): BookPlan {
  const raw = sample(name);
  if (name !== "family") return firstYearPlan(raw.people[0], density);
  return {
    preset: PRESETS.familyYear,
    title: "Our 2025",
    startDate: "2025-01-01",
    endDate: "2026-01-01",
    categories: [],
    match: "any",
    showGrowth: true,
    density,
  };
}

export function sampleBook(
  name: Sample,
  density: Density = "balanced"
): { source: BookSource; selection: Selection } {
  const raw = sample(name);
  const source = toSource(raw);
  return {
    source,
    selection: {
      ...draftSelection(source, samplePlan(name, density)),
      introduction: raw.writing?.introduction ?? "",
      letter: raw.writing?.letter ?? "",
      signature: raw.writing?.signature ?? "",
    },
  };
}
