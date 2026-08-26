import * as server from "../server";

export function getIdFromRoute(route: string, position: number = 2): number | null {
  const parts = route.split("/");
  if (parts.length <= position) {
    return null;
  }
  const id = parseInt(parts[position]);
  return isNaN(id) ? null : id;
}

export function splitPeopleByType(people: server.Person[]) {
  return {
    children: people.filter(p => p.type === server.Child),
    parents: people.filter(p => p.type === server.Parent),
  };
}
