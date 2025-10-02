import * as vlens from "vlens";
import { setRoute, setErrorView } from "vlens/core";
import * as preact from "preact";
import * as server from "./server";
import * as auth from "./lib/authCache";
import "./styles/global";

function customErrorView(route: string, prefix: string, error: string): preact.ComponentChild {
  // Handle AuthFailure by redirecting to landing page
  if (error === server.ErrAuthFailure) {
    // Clear auth cache and redirect to landing page
    auth.clearAuth();

    // Use setTimeout to avoid setting route during render
    setTimeout(() => {
      setRoute("/");
    }, 0);

    return preact.h("div", { className: "auth-failure-redirect" }, [
      preact.h("p", {}, "Redirecting to login..."),
    ]);
  }

  // For other errors, show a custom error page
  return preact.h("div", { className: "error-container" }, [
    preact.h("main", { className: "error-page" }, [
      preact.h("h1", {}, "Oops! Something went wrong"),
      preact.h("p", { className: "error-message" }, error),
      preact.h("div", { className: "error-actions" }, [
        preact.h("a", { href: "/", className: "btn btn-primary" }, "Go Home"),
        preact.h("a", { href: "/dashboard", className: "btn btn-secondary" }, "Dashboard"),
      ]),
    ]),
  ]);
}

async function main() {
  // Set up custom error handling
  setErrorView(customErrorView);

  vlens.initRoutes([
    vlens.routeHandler("/profile/", () => import("@app/pages/profile/profile")),
    vlens.routeHandler("/create-account", () => import("@app/pages/auth/create-account")),
    vlens.routeHandler("/login", () => import("@app/pages/auth/login")),
    vlens.routeHandler("/dashboard", () => import("@app/pages/dashboard/dashboard")),
    vlens.routeHandler("/compare", () => import("@app/pages/compare/compare")),
    vlens.routeHandler("/chat", () => import("@app/pages/chat/chat")),
    vlens.routeHandler("/settings", () => import("@app/pages/settings/settings")),
    vlens.routeHandler("/add-person", () => import("@app/pages/people/add-person")),
    vlens.routeHandler("/add-growth", () => import("@app/pages/growth/add-growth")),
    vlens.routeHandler("/edit-growth", () => import("@app/pages/growth/edit-growth")),
    vlens.routeHandler("/add-milestone", () => import("@app/pages/milestones/add-milestone")),
    vlens.routeHandler("/edit-milestone", () => import("@app/pages/milestones/edit-milestone")),
    vlens.routeHandler("/photos", () => import("@app/pages/photos/family-photos")),
    vlens.routeHandler("/add-photo", () => import("@app/pages/photos/add-photo")),
    vlens.routeHandler("/view-photo", () => import("@app/pages/photos/view-photo")),
    vlens.routeHandler("/edit-photo", () => import("@app/pages/photos/edit-photo")),
    vlens.routeHandler("/import", () => import("@app/pages/settings/import")),
    vlens.routeHandler("/admin/users", () => import("@app/pages/admin/users")),
    vlens.routeHandler("/admin/photos", () => import("@app/pages/admin/photos")),
    vlens.routeHandler("/admin/logs", () => import("@app/pages/admin/logs")),
    vlens.routeHandler("/admin/analytics", () => import("@app/pages/admin/analytics")),
    vlens.routeHandler("/admin", () => import("@app/pages/admin/admin")),
    vlens.routeHandler("/", () => import("@app/pages/home/home")),
  ]);
}

main();
