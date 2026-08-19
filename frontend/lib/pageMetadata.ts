// Per-route indexing and canonical URLs.
//
// Every route is served the same index.html, so the tags in that file describe
// the whole application rather than any one page. The static defaults are the
// safe ones — noindex, nofollow, canonical to the site root — and this narrows
// them for the pages that are genuinely public.
//
// The direction matters. A route added later and forgotten here stays out of
// the index, which is the failure that costs nothing. The opposite default
// would put somebody's children in a search result.

/**
 * PUBLIC_ROUTES are the pages a stranger is meant to be able to find: the
 * landing page, the two ways in, and the policies. Everything else — every
 * dashboard, profile, photo, chart, and settings page — is somebody's family.
 */
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

/**
 * applyPageMetadata sets the robots directive and canonical link for a route.
 * Called on every navigation, because a single-page application changes route
 * without ever reloading the document.
 */
export function applyPageMetadata(path: string): void {
  const normalized = normalizePath(path);
  const isPublic = isPublicRoute(normalized);

  setMetaContent("robots", isPublic ? "index, follow" : "noindex, nofollow");

  // A canonical URL on a private page would be an invitation to index it, so
  // only public pages get one that is not the site root.
  setCanonical(isPublic ? SITE_ORIGIN + normalized : SITE_ORIGIN + "/");
}

/**
 * Trailing slashes and query strings are noise here: /login and /login?next=x
 * are the same page, and the canonical should say so.
 */
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
