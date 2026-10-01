import * as server from "../server";
import { BookSource } from "./book";

export type Sample = "rich" | "sparse" | "uneven";

export const SAMPLES: { value: Sample; label: string }[] = [
  { value: "rich", label: "Rich" },
  { value: "sparse", label: "Sparse" },
  { value: "uneven", label: "Uneven" },
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

function person(name: string, profilePhotoId = 0): server.Person {
  return {
    id: 0,
    familyId: 0,
    name,
    gender: 1,
    birthday: BIRTHDAY,
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

function milestones(list: M[]): server.Milestone[] {
  return list.map(([offset, category, description, context, photo], i) => ({
    id: i + 1,
    personId: 0,
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

function growth(list: [offset: number, kind: "h" | "w", value: number][]): server.GrowthData[] {
  return list.map(([offset, kind, value], i) => ({
    id: i + 1,
    personId: 0,
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

function rich(): BookSource {
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
    person: person("Juniper", -7),
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

function sparse(): BookSource {
  return {
    person: person("Theo"),
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

function uneven(): BookSource {
  return {
    person: person("Rosie", -3),
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

export function sampleSource(sample: Sample): BookSource {
  if (sample === "sparse") return sparse();
  if (sample === "uneven") return uneven();
  return rich();
}
