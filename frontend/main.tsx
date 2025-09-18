import * as vlens from "vlens";
import "./styles.ts";

async function main() {
  vlens.initRoutes([
    vlens.routeHandler("/profile/", () => import("@app/profile")),
    vlens.routeHandler("/create-account", () => import("@app/create-account")),
    vlens.routeHandler("/login", () => import("@app/login")),
    vlens.routeHandler("/dashboard", () => import("@app/dashboard")),
    vlens.routeHandler("/add-person", () => import("@app/add-person")),
    vlens.routeHandler("/", () => import("@app/home")),
  ]);
}

main();
