import { describe, expect, it } from "vitest";
import * as server from "../../server";
import {
  buildMeasurementShare,
  measurementShareText,
  omitRows,
  shareRowIds,
} from "./measurementShare";
import {
  buildCompetitionShare,
  competitionShareText,
  looksUnfinished,
  recapCards,
  recapLinkInput,
} from "./competitionShare";
import { paginate } from "./pagination";
import { layoutBlock, wrap } from "./cardLayout";
import { jpegPagesToPdf } from "./pdf";
import { competitionGroups, measurementBlocks } from "./render";
import { copy } from "../copy";

function person(id: number, name: string, birthday: string, gender = 1): server.Person {
  return {
    id,
    familyId: 7,
    name,
    gender,
    birthday,
    age: "",
    profilePhotoId: 0,
    profileCropX: 0,
    profileCropY: 0,
    profileCropScale: 0,
    isPregnancy: false,
    relationship: "",
    birthTime: "",
    birthPlace: "",
  };
}

function growth(id: number, personId: number, value: number, date: string): server.GrowthData {
  return {
    id,
    personId,
    familyId: 7,
    measurementType: server.Height,
    value,
    unit: "in",
    measurementDate: `${date}T00:00:00Z`,
    createdAt: "",
    createdBy: 0,
    recordedBy: "",
  };
}

describe("measurement share", () => {
  const mia = person(1, "Mia", "2016-04-11T00:00:00Z");
  const ben = person(2, "Ben", "2019-08-01T00:00:00Z", 0);
  const latest = growth(10, 1, 30, "2017-04-11");
  const bens = growth(20, 2, 29.5, "2020-08-01");
  const build = () =>
    buildMeasurementShare({
      measurements: [latest],
      person: mia,
      familyMembers: [
        { person: mia, growthData: [latest] },
        { person: ben, growthData: [bens] },
      ],
      relationGroups: new Map([[2, "sibling"]]),
    })!;

  it("carries the value, percentile and family rows in the page's order", () => {
    const share = build();
    const section = share.sections[0];
    expect(share.personName).toBe("Mia");
    expect(share.dateText).toBe("April 11, 2017");
    expect(section.label).toBe("Height");
    expect(section.value).toBe("30 in");
    expect(section.rows.map(row => row.heading)).toEqual(["Percentile", "Ben"]);
    const benRow = section.rows[1];
    expect(benRow.group).toBe("Siblings");
    expect(benRow.lines[0]).toEqual({
      title: "At 12 mo old: 29.5 in",
      detail: "Mia was 0.5 in taller at this age",
    });
  });

  it("names the person instead of “you”, so the card reads on its own", () => {
    expect(measurementShareText(build())).not.toMatch(/\byou\b/);
  });

  it("leaves out birth dates", () => {
    expect(measurementShareText(build())).not.toContain("2016");
  });

  it("drops omitted rows from the card and the text alike", () => {
    const share = build();
    const benId = share.sections[0].rows[1].id;
    const shown = omitRows(share, new Set([benId]));
    expect(shareRowIds(shown)).toEqual([`10-percentile`]);
    expect(measurementShareText(shown)).not.toContain("Ben");
    expect(measurementShareText(shown)).not.toContain("Siblings");
    const blocks = measurementBlocks(shown).flatMap(block => block.lines.map(line => line.text));
    expect(blocks.join("\n")).not.toContain("Ben");

    const bare = omitRows(share, new Set(shareRowIds(share)));
    expect(measurementShareText(bare)).toBe("Mia · April 11, 2017 · Age 12 mo\n\nHeight: 30 in");
  });
});

describe("competition share", () => {
  const event: server.Event = {
    id: 3,
    seasonId: 1,
    familyId: 7,
    name: "Nuvo Nashville",
    host: "Nuvo",
    location: "Nashville, TN",
    startDate: "2025-11-08T00:00:00Z",
    endDate: "2025-11-08T00:00:00Z",
    notes: "Hotel is the Marriott",
    createdAt: "",
  };
  const summary: server.EventSummary = {
    id: 3,
    name: event.name,
    host: event.host,
    location: event.location,
    startDate: event.startDate,
    endDate: event.endDate,
  };
  const entry = (id: number, name: string, format: string): server.Entry => ({
    id,
    seasonId: 1,
    familyId: 7,
    name,
    format,
    style: "Jazz",
    division: "Junior",
    level: "",
    notes: "",
    createdAt: "",
  });
  const group = entry(1, "Rise Up", "group");
  const solo = entry(2, "Firework", "solo");
  const unrecorded = entry(4, "Tap Trio", "trio");
  const result = (
    id: number,
    kind: string,
    label: string,
    fields: Partial<server.Result> = {}
  ): server.Result => ({
    id,
    appearanceId: 0,
    familyId: 7,
    kind,
    label,
    rank: null,
    outOf: null,
    category: "",
    score: null,
    personId: null,
    notes: "",
    sortOrder: 0,
    createdAt: "",
    ...fields,
  });
  const appearance = (id: number, entryId: number, notes = ""): server.Appearance => ({
    id,
    eventId: 3,
    entryId,
    familyId: 7,
    occurredAt: "0001-01-01T00:00:00Z",
    notes,
    createdAt: "",
    round: "",
    opponent: "",
    outcome: "",
    scoreFor: null,
    scoreAgainst: null,
    revision: 1,
  });
  const appearances = (soloResults?: server.Result[]): server.AppearanceDetail[] => [
    {
      appearance: appearance(11, 2, "Fell on the turn"),
      results: soloResults ?? [
        result(22, "adjudication", "Platinum", { notes: "Judge 2 loved it" }),
        result(21, "placement", "", { rank: 1, outOf: 14 }),
      ],
      photoIds: [],
      entry: solo,
      event: summary,
    },
    {
      appearance: appearance(10, 1),
      results: [
        result(20, "adjudication", "High Gold"),
        result(23, "award", "Judges' Choice", { category: "Technique", personId: 5 }),
      ],
      photoIds: [],
      entry: group,
      event: summary,
    },
  ];
  const input = (details = appearances()) => ({
    event,
    seasonName: "2025-26 Season",
    entries: [
      { entry: group, personIds: [5, 6] },
      { entry: solo, personIds: [5] },
      { entry: unrecorded, personIds: [] },
    ],
    appearances: details,
    people: [person(5, "Clara", ""), person(6, "Ada", "")],
  });
  const options = { omitted: new Set<number>(), showsNames: false, isSnapshot: false };
  const build = (overrides: Partial<typeof options> = {}) =>
    buildCompetitionShare(input(), { ...options, ...overrides }, "Nov 21, 2026");

  it("keeps the page's routine order and leaves out routines with nothing recorded", () => {
    expect(recapCards(input()).map(card => card.entry.name)).toEqual(["Firework", "Rise Up"]);
    const share = build();
    expect(share.routines.map(r => r.name)).toEqual(["Firework", "Rise Up"]);
    expect(share.routines[0].performances[0].results.map(r => r.text)).toEqual([
      "1st of 14",
      "Platinum",
    ]);
    expect(share.place).toBe("Nashville, TN · Nuvo");
  });

  it("drops an omitted routine from the recap, its text and its blocks", () => {
    const share = build({ omitted: new Set([2]) });
    expect(share.routines.map(r => r.name)).toEqual(["Rise Up"]);
    expect(competitionShareText(share)).not.toContain("Firework");
    expect(competitionGroups(share)).toHaveLength(2);
  });

  it("keeps names out unless asked, while a dancer's own award still reads as one", () => {
    const hidden = build();
    const award = hidden.routines[1].performances[0].results[1];
    expect(award.detail).toBe(`Technique · ${copy.share.individual}`);
    expect(hidden.routines[1].roster).toBeNull();
    expect(competitionShareText(hidden)).not.toContain("Clara");

    const shown = build({ showsNames: true });
    expect(shown.routines[1].roster).toBe("Clara, Ada");
    expect(shown.routines[1].performances[0].results[1].detail).toBe("Technique · Clara");
  });

  it("never includes notes", () => {
    const text = competitionShareText(build({ showsNames: true }));
    expect(text).not.toContain("Marriott");
    expect(text).not.toContain("Fell on the turn");
    expect(text).not.toContain("Judge 2");
  });

  it("labels a snapshot with the day it was shared", () => {
    expect(build({ isSnapshot: true }).snapshotLabel).toBe(copy.share.asOf("Nov 21, 2026"));
    expect(build().snapshotLabel).toBeNull();
  });

  it("starts as a snapshot while the event is on or a performance is blank", () => {
    expect(looksUnfinished(event, appearances(), "2025-12-01")).toBe(false);
    expect(looksUnfinished(event, appearances(), "2025-11-08")).toBe(true);
    expect(looksUnfinished(event, appearances([]), "2025-12-01")).toBe(true);
  });

  it("puts every included result in the attachment's blocks, the last routine's award too", () => {
    const text = competitionGroups(build())
      .flat()
      .flatMap(block => block.lines.map(line => line.text))
      .join("\n");
    expect(text).toContain("• Judges' Choice");
    expect(text).toContain("• 1st of 14");
  });

  it("sends a recap link exactly what the preview holds, with each performance's revision", () => {
    const details = appearances();
    details[1].appearance.revision = 4;
    const share = buildCompetitionShare(
      input(details),
      { ...options, omitted: new Set([2]) },
      "Nov 21, 2026"
    );
    const sent = recapLinkInput(share, details);
    expect(sent.routines.map(r => r.entryId)).toEqual([1]);
    expect(sent.routines[0].roster).toBe("");
    expect(sent.snapshotLabel).toBe("");
    expect(sent.routines[0].performances).toEqual([
      {
        appearanceId: 10,
        revision: 4,
        when: "",
        matchup: "",
        results: [
          { kind: "adjudication", text: "High Gold", detail: "" },
          { kind: "award", text: "Judges' Choice", detail: `Technique · ${copy.share.individual}` },
        ],
      },
    ]);
    expect(JSON.stringify(sent)).not.toContain("Clara");
  });
});

describe("pagination", () => {
  it("keeps routines that fit together", () => {
    expect(paginate([[100], [200], [200]], 700, 10)).toEqual([
      [
        { group: 0, item: 0 },
        { group: 1, item: 0 },
        { group: 2, item: 0 },
      ],
    ]);
  });

  it("starts a routine that would straddle a break on the next page", () => {
    expect(paginate([[100], [300, 200], [150, 150]], 700, 10)).toEqual([
      [
        { group: 0, item: 0 },
        { group: 1, item: 0 },
        { group: 1, item: 1 },
      ],
      [
        { group: 2, item: 0 },
        { group: 2, item: 1 },
      ],
    ]);
  });

  it("runs a routine taller than a page on, a performance at a time", () => {
    expect(paginate([[100], [400, 400, 400]], 700, 10)).toEqual([
      [
        { group: 0, item: 0 },
        { group: 1, item: 0 },
      ],
      [{ group: 1, item: 1 }],
      [{ group: 1, item: 2 }],
    ]);
  });

  it("places every block exactly once, in order", () => {
    const groups = [[80], [900], [120, 60], [700], [10, 10, 10]];
    const placed = paginate(groups, 700, 16).flat();
    const expected = groups.flatMap((items, group) => items.map((_, item) => ({ group, item })));
    expect(placed).toEqual(expected);
  });
});

describe("card layout", () => {
  const measure = (text: string) => text.length * 10;

  it("wraps on words, and breaks a word only when it is wider than a row", () => {
    expect(wrap("one two three", 70, "", measure)).toEqual(["one two", "three"]);
    expect(wrap("abcdefghij", 40, "", measure)).toEqual(["abcd", "efgh", "ij"]);
    expect(wrap("", 40, "", measure)).toEqual([""]);
  });

  it("measures a block's height from its wrapped rows and padding", () => {
    const laid = layoutBlock(
      { padding: 10, lines: [{ text: "one two three", size: 10 }] },
      90,
      measure
    );
    expect(laid.lines[0].rows).toEqual(["one two", "three"]);
    expect(laid.height).toBe(10 * 2 + 13 * 2);
  });
});

describe("pdf", () => {
  it("writes an xref whose offsets point at each object", () => {
    const jpeg = new Uint8Array([0xff, 0xd8, 0x00, 0x01, 0xff, 0xd9]);
    const bytes = jpegPagesToPdf([
      { jpeg, widthPx: 10, heightPx: 12 },
      { jpeg, widthPx: 10, heightPx: 12 },
    ]);
    const text = new TextDecoder("latin1").decode(bytes);
    expect(text.startsWith("%PDF-1.4")).toBe(true);
    expect(text).toContain("/Count 2");

    const startxref = Number(text.match(/startxref\n(\d+)/)![1]);
    expect(text.slice(startxref, startxref + 4)).toBe("xref");
    const entries = text
      .slice(startxref)
      .split("\n")
      .slice(3)
      .filter(line => / n $/.test(line));
    expect(entries).toHaveLength(8);
    entries.forEach((line, i) => {
      const offset = Number(line.slice(0, 10));
      expect(text.slice(offset, offset + `${i + 1} 0 obj`.length)).toBe(`${i + 1} 0 obj`);
    });
  });
});
