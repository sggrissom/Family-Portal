import * as server from "../server";

/**
 * Extract an integer ID from a route path
 * @param route - The route string (e.g., "/edit-milestone/123")
 * @param position - Position in the path (default: 2 for /segment/segment/ID)
 * @returns The parsed ID or null if not found/invalid
 */
export function getIdFromRoute(route: string, position: number = 2): number | null {
  const parts = route.split("/");
  if (parts.length <= position) {
    return null;
  }
  const id = parseInt(parts[position]);
  return isNaN(id) ? null : id;
}

/**
 * Split a list of people into children and parents
 * @param people - Array of Person objects
 * @returns Object with children and parents arrays
 */
export function splitPeopleByType(people: server.Person[]) {
  return {
    children: people.filter(p => p.type === server.Child),
    parents: people.filter(p => p.type === server.Parent),
  };
}
