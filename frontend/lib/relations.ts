import * as server from "../server";

function peersOf(relations: server.Relation[], personId: number, kind: server.RelationKind) {
  const ids: number[] = [];
  for (const rel of relations) {
    if (rel.kind !== kind) continue;
    if (rel.fromId === personId) ids.push(rel.toId);
    else if (rel.toId === personId) ids.push(rel.fromId);
  }
  return ids;
}

export function partnersOf(relations: server.Relation[], personId: number): number[] {
  return peersOf(relations, personId, server.RelationPartner);
}

export function parentsOf(relations: server.Relation[], personId: number): number[] {
  return relations
    .filter(rel => rel.kind === server.RelationParent && rel.toId === personId)
    .map(rel => rel.fromId);
}

export function childrenOf(relations: server.Relation[], personId: number): number[] {
  return relations
    .filter(rel => rel.kind === server.RelationParent && rel.fromId === personId)
    .map(rel => rel.toId);
}

// Mirrors the backend: siblings stated outright plus those sharing a parent.
export function siblingsOf(relations: server.Relation[], personId: number): number[] {
  const seen = new Set<number>([personId]);
  const ids: number[] = [];
  const add = (id: number) => {
    if (seen.has(id)) return;
    seen.add(id);
    ids.push(id);
  };
  peersOf(relations, personId, server.RelationSibling).forEach(add);
  for (const parentId of parentsOf(relations, personId)) {
    childrenOf(relations, parentId).forEach(add);
  }
  return ids;
}

export function hasStatedRelation(
  relations: server.Relation[],
  stated: server.StatedRelation,
  personId: number,
  anchorId: number
): boolean {
  switch (stated) {
    case server.StatedChild:
      return childrenOf(relations, anchorId).includes(personId);
    case server.StatedParent:
      return parentsOf(relations, anchorId).includes(personId);
    case server.StatedSibling:
      return siblingsOf(relations, anchorId).includes(personId);
    case server.StatedPartner:
      return partnersOf(relations, anchorId).includes(personId);
  }
  return false;
}

export interface CoAnchorSuggestion {
  anchorId: number;
  defaultChecked: boolean;
}

// coAnchorSuggestions names the other people the same stated relation most
// likely also applies to: a child's other parent, or the siblings of whoever
// the relation was stated against.
export function coAnchorSuggestions(
  relations: server.Relation[],
  stated: server.StatedRelation,
  personId: number,
  anchorId: number
): CoAnchorSuggestion[] {
  if (!anchorId) return [];

  let candidates: number[];
  let defaultChecked = false;

  if (stated === server.StatedChild) {
    candidates = partnersOf(relations, anchorId);
    // A single partner is the child's other parent often enough to preselect;
    // more than one is ambiguous, so make the choice explicit.
    defaultChecked = candidates.length === 1;
  } else if (stated === server.StatedParent || stated === server.StatedSibling) {
    candidates = siblingsOf(relations, anchorId);
  } else {
    return [];
  }

  return candidates
    .filter(id => id !== personId && id !== anchorId)
    .filter(id => !hasStatedRelation(relations, stated, personId, id))
    .map(anchorId => ({ anchorId, defaultChecked }));
}

export interface CoAnchorState {
  key: string;
  ids: number[];
}

// syncCoAnchors returns the current suggestions and, whenever they change,
// resets the selection to the ones worth preselecting.
export function syncCoAnchors(
  state: CoAnchorState,
  relations: server.Relation[],
  stated: server.StatedRelation,
  personId: number,
  anchorId: number
): CoAnchorSuggestion[] {
  const suggestions = coAnchorSuggestions(relations, stated, personId, anchorId);
  const key = `${stated}:${anchorId}:${suggestions.map(s => s.anchorId).join(",")}`;
  if (state.key !== key) {
    state.key = key;
    state.ids = suggestions.filter(s => s.defaultChecked).map(s => s.anchorId);
  }
  return suggestions;
}
