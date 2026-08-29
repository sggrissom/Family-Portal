import * as server from "../server";

export function getIdFromRoute(route: string, position: number = 2): number | null {
  const parts = route.split("/");
  if (parts.length <= position) {
    return null;
  }
  const id = parseInt(parts[position]);
  return isNaN(id) ? null : id;
}

export function personSubtitle(person: server.Person): string {
  const age = person.age ? `Age ${person.age}` : "";
  if (!person.relationship) {
    return age;
  }
  return age ? `${person.relationship} • ${age}` : person.relationship;
}

export const RELATION_OPTIONS: { value: number; label: string; gender: number | null }[] = [
  { value: server.StatedChild, label: "daughter", gender: 1 },
  { value: server.StatedChild, label: "son", gender: 0 },
  { value: server.StatedChild, label: "child", gender: null },
  { value: server.StatedParent, label: "mother", gender: 1 },
  { value: server.StatedParent, label: "father", gender: 0 },
  { value: server.StatedParent, label: "parent", gender: null },
  { value: server.StatedSibling, label: "sister", gender: 1 },
  { value: server.StatedSibling, label: "brother", gender: 0 },
  { value: server.StatedSibling, label: "sibling", gender: null },
  { value: server.StatedPartner, label: "wife", gender: 1 },
  { value: server.StatedPartner, label: "husband", gender: 0 },
  { value: server.StatedPartner, label: "partner", gender: null },
];
