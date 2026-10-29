import * as core from "vlens/core";

let pendingRoute: string | null = null;
core.registerCleanupFunction(() => {
  pendingRoute = null;
});

// Goes through vlens's popstate path, as a link click does, so the new page
// opens at the top instead of inheriting the old page's scroll position.
export function navigate(route: string) {
  if (pendingRoute === route) return;
  pendingRoute = route;
  const ts = Date.now();
  history.pushState({ ts }, "", route);
  window.dispatchEvent(new PopStateEvent("popstate", { state: { ts } }));
}
