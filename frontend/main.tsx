import * as vlens from "vlens";
import "./styles.ts";

async function main() {
  vlens.initRoutes([
    vlens.routeHandler("/create-account", () => import("@app/create-account")),
    vlens.routeHandler("/login", () => import("@app/login")),
    vlens.routeHandler("/", () => import("@app/home")),
  ]);
}

main();
