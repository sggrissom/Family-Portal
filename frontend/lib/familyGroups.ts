import * as server from "../server";
import { parentsOf, partnersOf, siblingsOf } from "./relations";

export interface PersonGroup {
  key: string;
  title: string;
  people: server.Person[];
}

// Read off the bottom of the tree, so the youngest generation present is always
// "Children" and whoever is above them are their parents.
const GENERATION_TITLES = ["Children", "Parents", "Grandparents", "Great-grandparents"];
const OLDER_TITLE = "Earlier generations";
const LONE_GENERATION_TITLE = "Family";
const UNLINKED_TITLE = "Not linked yet";

function birthdayTime(person: server.Person): number {
  const time = new Date(person.birthday).getTime();
  return isNaN(time) ? Number.MAX_SAFE_INTEGER : time;
}

function byAge(a: server.Person, b: server.Person): number {
  return birthdayTime(a) - birthdayTime(b) || a.name.localeCompare(b.name);
}

// generationDepths places everyone one step below their parents, and level with
// their partners and siblings, by relaxing those three rules until they hold.
// Passes are capped at the number of people so a parent cycle still terminates.
function generationDepths(
  people: server.Person[],
  relations: server.Relation[]
): Map<number, number> {
  const depths = new Map<number, number>();
  people.forEach(person => depths.set(person.id, 0));

  const depthOf = (id: number) => depths.get(id) ?? 0;

  for (let pass = 0; pass < people.length; pass++) {
    let changed = false;
    for (const person of people) {
      let depth = depthOf(person.id);
      for (const parentId of parentsOf(relations, person.id)) {
        depth = Math.max(depth, depthOf(parentId) + 1);
      }
      for (const peerId of partnersOf(relations, person.id)) {
        depth = Math.max(depth, depthOf(peerId));
      }
      for (const peerId of siblingsOf(relations, person.id)) {
        depth = Math.max(depth, depthOf(peerId));
      }
      if (depth > depthOf(person.id)) {
        depths.set(person.id, depth);
        changed = true;
      }
    }
    if (!changed) break;
  }
  return depths;
}

// orderGeneration lists a generation under the parents it belongs to: everyone
// follows whoever came first among their parents, and partners stay side by
// side. Positions of the generation above are already in placed.
function orderGeneration(
  members: server.Person[],
  relations: server.Relation[],
  placed: Map<number, number>
): server.Person[] {
  const parentRank = (person: server.Person) =>
    parentsOf(relations, person.id).reduce(
      (rank, parentId) => Math.min(rank, placed.get(parentId) ?? Number.MAX_SAFE_INTEGER),
      Number.MAX_SAFE_INTEGER
    );

  const sorted = [...members].sort((a, b) => parentRank(a) - parentRank(b) || byAge(a, b));

  const byId = new Map(members.map(person => [person.id, person]));
  const ordered: server.Person[] = [];
  const seen = new Set<number>();
  for (const person of sorted) {
    if (seen.has(person.id)) continue;
    seen.add(person.id);
    ordered.push(person);
    for (const partnerId of partnersOf(relations, person.id)) {
      const partner = byId.get(partnerId);
      if (partner && !seen.has(partner.id)) {
        seen.add(partner.id);
        ordered.push(partner);
      }
    }
  }
  return ordered;
}

function generationTitle(depth: number, deepest: number): string {
  if (deepest === 0) return LONE_GENERATION_TITLE;
  return GENERATION_TITLES[deepest - depth] ?? OLDER_TITLE;
}

// groupFamily lays the family out oldest generation first, so parents come
// before their children. People no relationship reaches go last, since nothing
// says where they belong.
export function groupFamily(people: server.Person[], relations: server.Relation[]): PersonGroup[] {
  const visible = new Set(people.map(person => person.id));
  const edges = relations.filter(rel => visible.has(rel.fromId) && visible.has(rel.toId));

  const linkedIds = new Set<number>();
  for (const rel of edges) {
    linkedIds.add(rel.fromId);
    linkedIds.add(rel.toId);
  }
  const linked = people.filter(person => linkedIds.has(person.id));
  const unlinked = people.filter(person => !linkedIds.has(person.id)).sort(byAge);

  const depths = generationDepths(linked, edges);
  const generations = new Map<number, server.Person[]>();
  for (const person of linked) {
    const depth = depths.get(person.id) ?? 0;
    const members = generations.get(depth);
    if (members) members.push(person);
    else generations.set(depth, [person]);
  }
  const deepest = Math.max(0, ...generations.keys());

  const groups: PersonGroup[] = [];
  const placed = new Map<number, number>();
  for (const depth of [...generations.keys()].sort((a, b) => a - b)) {
    const ordered = orderGeneration(generations.get(depth)!, edges, placed);
    ordered.forEach(person => placed.set(person.id, placed.size));
    groups.push({
      key: `generation-${depth}`,
      title: generationTitle(depth, deepest),
      people: ordered,
    });
  }

  if (unlinked.length > 0) {
    groups.push({ key: "unlinked", title: UNLINKED_TITLE, people: unlinked });
  }
  return groups;
}
