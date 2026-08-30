import * as vlens from "vlens";
import { setRoute, setErrorView, type RouteHandler } from "vlens/core";
import * as preact from "preact";
import * as server from "./server";
import * as auth from "./lib/authCache";
import { classifyError } from "./lib/errorDisplay";
import { ErrorDisplay } from "./components/ErrorDisplay";
import { Header, Footer } from "./layout";
import { ErrorBoundary } from "./components/ErrorBoundary";
import { installGlobalErrorHandlers } from "./lib/clientErrors";
import "./styles/global";

function customErrorView(route: string, prefix: string, error: string): preact.ComponentChild {
  if (error === server.ErrAuthFailure) {
    auth.clearAuth();

    setTimeout(() => {
      setRoute("/");
    }, 0);

    return preact.h("div", { className: "auth-failure-redirect" }, [
      preact.h("p", {}, "Redirecting to login..."),
    ]);
  }

  return (
    <div>
      <Header isHome={false} />
      <main id="app">
        <ErrorDisplay error={classifyError(error)} />
      </main>
      <Footer />
    </div>
  );
}

// Keyed by route so a crash on one page does not leave the boundary latched
// after navigating away.
function guarded<Data>(prefix: string, load: () => Promise<RouteHandler<Data>>) {
  return vlens.routeHandler<Data>(prefix, async () => {
    const mod = await load();
    return {
      fetch: mod.fetch,
      view: (route: string, viewPrefix: string, data: Data) => (
        <ErrorBoundary key={route}>{mod.view(route, viewPrefix, data)}</ErrorBoundary>
      ),
    };
  });
}

async function main() {
  setErrorView(customErrorView);
  installGlobalErrorHandlers();

  vlens.initRoutes([
    guarded("/profile/", () => import("@app/pages/profile/profile")),
    guarded("/create-account", () => import("@app/pages/auth/create-account")),
    guarded("/login", () => import("@app/pages/auth/login")),
    guarded("/forgot-password", () => import("@app/pages/auth/forgot-password")),
    guarded("/reset-password", () => import("@app/pages/auth/reset-password")),
    guarded("/verify-email", () => import("@app/pages/auth/verify-email")),
    guarded("/dashboard", () => import("@app/pages/dashboard/dashboard")),
    guarded("/compare", () => import("@app/pages/compare/compare")),
    guarded("/family-timeline", () => import("@app/pages/family-timeline/family-timeline")),
    guarded("/chat", () => import("@app/pages/chat/chat")),
    guarded("/settings", () => import("@app/pages/settings/settings")),
    guarded("/add-person", () => import("@app/pages/people/add-person")),
    guarded("/edit-person/", () => import("@app/pages/people/edit-person")),
    guarded("/add-growth", () => import("@app/pages/growth/add-growth")),
    guarded("/edit-growth", () => import("@app/pages/growth/edit-growth")),
    guarded("/view-growth", () => import("@app/pages/growth/view-growth")),
    guarded("/family-chart", () => import("@app/pages/growth/family-chart")),
    guarded("/add-milestone", () => import("@app/pages/milestones/add-milestone")),
    guarded("/edit-milestone", () => import("@app/pages/milestones/edit-milestone")),
    guarded("/photos", () => import("@app/pages/photos/family-photos")),
    guarded("/add-photo", () => import("@app/pages/photos/add-photo")),
    guarded("/view-photo", () => import("@app/pages/photos/view-photo")),
    guarded("/edit-photo", () => import("@app/pages/photos/edit-photo")),
    guarded("/season/", () => import("@app/pages/activities/season")),
    guarded("/competition/", () => import("@app/pages/activities/competition")),
    guarded("/routine/", () => import("@app/pages/activities/routine")),
    guarded("/person-activities/", () => import("@app/pages/activities/person")),
    guarded("/activities", () => import("@app/pages/activities/activities")),
    guarded("/manage-tags", () => import("@app/pages/tags/manage-tags")),
    guarded("/import", () => import("@app/pages/settings/import")),
    guarded("/admin/users", () => import("@app/pages/admin/users")),
    guarded("/admin/photos", () => import("@app/pages/admin/photos")),
    guarded("/admin/logs", () => import("@app/pages/admin/logs")),
    guarded("/admin/analytics", () => import("@app/pages/admin/analytics")),
    guarded("/admin/push", () => import("@app/pages/admin/push")),
    guarded("/admin/app-versions", () => import("@app/pages/admin/app-versions")),
    guarded("/admin/seed", () => import("@app/pages/admin/seed")),
    guarded("/admin", () => import("@app/pages/admin/admin")),
    guarded("/privacy", () => import("@app/pages/legal/privacy")),
    guarded("/terms", () => import("@app/pages/legal/terms")),
    guarded("/support", () => import("@app/pages/legal/support")),
    guarded("/", () => import("@app/pages/home/home")),
  ]);
}

main();
