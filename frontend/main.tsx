import * as vlens from "vlens";
import { setRoute, setErrorView } from "vlens/core";
import * as preact from "preact";
import * as server from "./server";
import * as auth from "./authCache";
import "./styles.ts";

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
      preact.h("p", {}, "Redirecting to login...")
    ]);
  }

  // For other errors, show a custom error page
  return preact.h("div", { className: "error-container" }, [
    preact.h("main", { className: "error-page" }, [
      preact.h("h1", {}, "Oops! Something went wrong"),
      preact.h("p", { className: "error-message" }, error),
      preact.h("div", { className: "error-actions" }, [
        preact.h("a", { href: "/", className: "btn btn-primary" }, "Go Home"),
        preact.h("a", { href: "/dashboard", className: "btn btn-secondary" }, "Dashboard")
      ])
    ])
  ]);
}

async function main() {
  // Set up custom error handling
  setErrorView(customErrorView);

  vlens.initRoutes([
    vlens.routeHandler("/profile/", () => import("@app/profile")),
    vlens.routeHandler("/create-account", () => import("@app/create-account")),
    vlens.routeHandler("/login", () => import("@app/login")),
    vlens.routeHandler("/dashboard", () => import("@app/dashboard")),
    vlens.routeHandler("/add-person", () => import("@app/add-person")),
    vlens.routeHandler("/add-growth", () => import("@app/add-growth")),
    vlens.routeHandler("/", () => import("@app/home")),
  ]);
}

main();
