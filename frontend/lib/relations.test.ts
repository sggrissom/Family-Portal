import { describe, expect, it } from "vitest";
import * as server from "@app/server";
import {
  childrenOf,
  coAnchorSuggestions,
  parentsOf,
  partnersOf,
  siblingsOf,
  syncCoAnchors,
} from "@app/lib/relations";

let nextId = 1;
const parent = (fromId: number, toId: number): server.Relation => ({
  id: nextId++,
  fromId,
  toId,
  kind: server.RelationParent,
});
const partner = (fromId: number, toId: number): server.Relation => ({
  id: nextId++,
  fromId,
  toId,
  kind: server.RelationPartner,
});
const sibling = (fromId: number, toId: number): server.Relation => ({
  id: nextId++,
  fromId,
  toId,
  kind: server.RelationSibling,
});

// Dad(1) + Mom(2) are partners with children Ann(3) and Ben(4).
const family = [parent(1, 3), parent(2, 3), parent(1, 4), parent(2, 4), partner(1, 2)];

describe("parentsOf / childrenOf", () => {
  it("reads the parent edge in the direction it was stored", () => {
    expect(parentsOf(family, 3).sort()).toEqual([1, 2]);
    expect(childrenOf(family, 1).sort()).toEqual([3, 4]);
    expect(childrenOf(family, 3)).toEqual([]);
  });
});

describe("partnersOf", () => {
  it("is symmetric regardless of which side the edge was stored on", () => {
    expect(partnersOf(family, 1)).toEqual([2]);
    expect(partnersOf(family, 2)).toEqual([1]);
  });
});

describe("siblingsOf", () => {
  it("infers siblings from a shared parent", () => {
    expect(siblingsOf(family, 3)).toEqual([4]);
  });

  it("never includes the person themselves", () => {
    expect(siblingsOf(family, 3)).not.toContain(3);
  });

  it("merges stated siblings with inferred ones without duplicating", () => {
    // Ben is both stated as Ann's sibling and reachable through both parents.
    const relations = [...family, sibling(3, 4)];
    expect(siblingsOf(relations, 3)).toEqual([4]);
  });

  it("includes a stated sibling who shares no parent", () => {
    const relations = [...family, sibling(3, 5)];
    expect(siblingsOf(relations, 3).sort()).toEqual([4, 5]);
  });
});

describe("coAnchorSuggestions", () => {
  it("preselects the sole partner as a new child's other parent", () => {
    // Adding Cy(9) as Dad's child: Mom is the only candidate, so preselect her.
    expect(coAnchorSuggestions(family, server.StatedChild, 9, 1)).toEqual([
      { anchorId: 2, defaultChecked: true },
    ]);
  });

  it("leaves multiple partners unchecked because the choice is ambiguous", () => {
    const relations = [...family, partner(1, 5)];
    expect(coAnchorSuggestions(relations, server.StatedChild, 9, 1)).toEqual([
      { anchorId: 2, defaultChecked: false },
      { anchorId: 5, defaultChecked: false },
    ]);
  });

  it("suggests the anchor's siblings when stating a parent or sibling", () => {
    // Stating Cy(9) as Ann's sibling: Ben is Ann's sibling, so likely Cy's too.
    expect(coAnchorSuggestions(family, server.StatedSibling, 9, 3)).toEqual([
      { anchorId: 4, defaultChecked: false },
    ]);
  });

  it("drops candidates the relation already holds against", () => {
    const relations = [...family, parent(2, 9)];
    expect(coAnchorSuggestions(relations, server.StatedChild, 9, 1)).toEqual([]);
  });

  it("returns nothing without an anchor, or for a partner relation", () => {
    expect(coAnchorSuggestions(family, server.StatedChild, 9, 0)).toEqual([]);
    expect(coAnchorSuggestions(family, server.StatedPartner, 9, 1)).toEqual([]);
  });
});

describe("syncCoAnchors", () => {
  it("seeds the selection with the preselected suggestions", () => {
    const state = { key: "", ids: [] as number[] };
    syncCoAnchors(state, family, server.StatedChild, 9, 1);
    expect(state.ids).toEqual([2]);
  });

  it("keeps a user's edit while the anchor and suggestions are unchanged", () => {
    const state = { key: "", ids: [] as number[] };
    syncCoAnchors(state, family, server.StatedChild, 9, 1);
    state.ids = [];
    syncCoAnchors(state, family, server.StatedChild, 9, 1);
    expect(state.ids).toEqual([]);
  });

  it("resets the selection when the anchor changes", () => {
    const state = { key: "", ids: [] as number[] };
    syncCoAnchors(state, family, server.StatedChild, 9, 1);
    state.ids = [];
    syncCoAnchors(state, family, server.StatedChild, 9, 2);
    expect(state.ids).toEqual([1]);
  });
});
