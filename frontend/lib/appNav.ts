export type Destination = "home" | "photos" | "growth" | "history" | "chat";

const DESTINATION_PREFIXES: [string, Destination][] = [
  ["/dashboard", "home"],
  ["/profile", "home"],
  ["/photos", "photos"],
  ["/view-photo", "photos"],
  ["/edit-photo", "photos"],
  ["/growth", "growth"],
  ["/view-growth", "growth"],
  ["/edit-growth", "growth"],
  ["/history", "history"],
  ["/chat", "chat"],
];

function underPrefix(route: string, prefix: string): boolean {
  return route === prefix || route.startsWith(prefix + "/") || route.startsWith(prefix + "?");
}

export function activeDestination(path: string): Destination | null {
  const match = DESTINATION_PREFIXES.find(([prefix]) => underPrefix(path, prefix));
  return match ? match[1] : null;
}

export const LEGACY_ROUTES: Record<string, string> = {
  "/family-timeline": "/history",
  "/family-chart": "/growth",
};

export function legacyRedirect(route: string): string | null {
  for (const [from, to] of Object.entries(LEGACY_ROUTES)) {
    if (underPrefix(route, from)) {
      return to + route.slice(from.length);
    }
  }
  return null;
}

export function contextPersonId(path: string): number | null {
  const match = /^\/(?:profile|edit-person|person-activities)\/(\d+)/.exec(path);
  return match ? parseInt(match[1]) : null;
}

export function addPath(
  form: "/add-photo" | "/add-growth" | "/add-milestone",
  personId: number | null
) {
  return personId ? `${form}/${personId}` : form;
}
