// An allowlist on purpose: a route added later and forgotten here stays noindex,
// which is the failure that costs nothing. A denylist would leak the opposite way.
const PUBLIC_ROUTES = new Set([
  "/",
  "/login",
  "/create-account",
  "/forgot-password",
  "/privacy",
  "/terms",
  "/support",
]);

const SITE_ORIGIN = "https://familyrecord.app";

export function isPublicRoute(path: string): boolean {
  return PUBLIC_ROUTES.has(normalizePath(path));
}

export function applyPageMetadata(path: string): void {
  const normalized = normalizePath(path);
  const isPublic = isPublicRoute(normalized);

  setMetaContent("robots", isPublic ? "index, follow" : "noindex, nofollow");

  setCanonical(isPublic ? SITE_ORIGIN + normalized : SITE_ORIGIN + "/");
}

function normalizePath(path: string): string {
  const withoutQuery = path.split("?")[0].split("#")[0];
  if (withoutQuery.length > 1 && withoutQuery.endsWith("/")) {
    return withoutQuery.slice(0, -1);
  }
  return withoutQuery || "/";
}

function setMetaContent(name: string, content: string): void {
  let tag = document.querySelector(`meta[name="${name}"]`);
  if (!tag) {
    tag = document.createElement("meta");
    tag.setAttribute("name", name);
    document.head.appendChild(tag);
  }
  tag.setAttribute("content", content);
}

function setCanonical(href: string): void {
  let link = document.querySelector('link[rel="canonical"]');
  if (!link) {
    link = document.createElement("link");
    link.setAttribute("rel", "canonical");
    document.head.appendChild(link);
  }
  link.setAttribute("href", href);
}
