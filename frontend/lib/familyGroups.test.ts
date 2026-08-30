import { describe, expect, it } from "vitest";
import * as server from "@app/server";
import { groupFamily } from "@app/lib/familyGroups";

let nextRelationId = 1;
const parent = (fromId: number, toId: number): server.Relation => ({
  id: nextRelationId++,
  fromId,
  toId,
  kind: server.RelationParent,
});
const partner = (fromId: number, toId: number): server.Relation => ({
  id: nextRelationId++,
  fromId,
  toId,
  kind: server.RelationPartner,
});

const person = (id: number, name: string, birthday: string): server.Person => ({
  id,
  familyId: 1,
  name,
  gender: server.Unknown,
  birthday,
  age: "",
  profilePhotoId: 0,
  profileCropX: 0,
  profileCropY: 0,
  profileCropScale: 1,
  isPregnancy: false,
  relationship: "",
});

const dad = person(1, "Dad", "1980-01-01");
const mom = person(2, "Mom", "1982-01-01");
const ann = person(3, "Ann", "2010-01-01");
const ben = person(4, "Ben", "2012-01-01");
const gran = person(5, "Gran", "1955-01-01");

const names = (groups: ReturnType<typeof groupFamily>) =>
  groups.map(g => [g.title, g.people.map(p => p.name)] as const);

describe("groupFamily", () => {
  it("titles generations from the youngest up and lists the oldest first", () => {
    const groups = groupFamily(
      [ann, dad, ben, mom],
      [parent(1, 3), parent(2, 3), parent(1, 4), parent(2, 4), partner(1, 2)]
    );
    expect(names(groups)).toEqual([
      ["Parents", ["Dad", "Mom"]],
      ["Children", ["Ann", "Ben"]],
    ]);
  });

  it("renames the generations as an older one is added", () => {
    const groups = groupFamily([ann, dad, gran], [parent(5, 1), parent(1, 3)]);
    expect(names(groups)).toEqual([
      ["Grandparents", ["Gran"]],
      ["Parents", ["Dad"]],
      ["Children", ["Ann"]],
    ]);
  });

  it("calls a single generation Family rather than Children", () => {
    const groups = groupFamily([dad, mom], [partner(1, 2)]);
    expect(names(groups)).toEqual([["Family", ["Dad", "Mom"]]]);
  });

  it("keeps partners side by side even when ages would separate them", () => {
    // Two couples whose ages interleave: by age alone the order would be
    // Amy, Cal, Bob, Dee, splitting both pairs.
    const amy = person(10, "Amy", "1970-01-01");
    const bob = person(11, "Bob", "1990-01-01");
    const cal = person(12, "Cal", "1975-01-01");
    const dee = person(13, "Dee", "1995-01-01");
    const groups = groupFamily([bob, dee, amy, cal], [partner(10, 11), partner(12, 13)]);
    expect(names(groups)).toEqual([["Family", ["Amy", "Bob", "Cal", "Dee"]]]);
  });

  it("levels partners into the same generation as their spouse", () => {
    // Mom has no parent edge of her own; her partnership with Dad places her.
    const groups = groupFamily([dad, mom, gran], [parent(5, 1), partner(1, 2)]);
    expect(names(groups)).toEqual([
      ["Parents", ["Gran"]],
      ["Children", ["Dad", "Mom"]],
    ]);
  });

  it("sorts an unlinked generation by age, oldest first", () => {
    const groups = groupFamily([ben, ann], []);
    expect(names(groups)).toEqual([["Not linked yet", ["Ann", "Ben"]]]);
  });

  it("puts people no relation reaches in a trailing group", () => {
    const groups = groupFamily([ann, dad, gran], [parent(1, 3)]);
    expect(names(groups)).toEqual([
      ["Parents", ["Dad"]],
      ["Children", ["Ann"]],
      ["Not linked yet", ["Gran"]],
    ]);
  });

  it("ignores relations pointing at people outside the given list", () => {
    const groups = groupFamily([ann, dad], [parent(1, 3), parent(99, 1)]);
    expect(names(groups)).toEqual([
      ["Parents", ["Dad"]],
      ["Children", ["Ann"]],
    ]);
  });

  it("terminates on a parent cycle instead of looping forever", () => {
    const groups = groupFamily([dad, mom], [parent(1, 2), parent(2, 1)]);
    expect(groups.flatMap(g => g.people.map(p => p.name)).sort()).toEqual(["Dad", "Mom"]);
  });

  it("returns no groups for an empty family", () => {
    expect(groupFamily([], [])).toEqual([]);
  });
});
