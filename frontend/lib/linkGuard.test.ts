import { describe, expect, it } from "vitest";
import { guardHandledLinkClicks } from "./linkGuard";

describe("guardHandledLinkClicks", () => {
  it("keeps a click the page handled from the router, and lets the rest through", () => {
    const page = new EventTarget();
    const routed: string[] = [];
    page.addEventListener("click", event => {
      if ((event as CustomEvent).detail === "handled") event.preventDefault();
    });
    guardHandledLinkClicks(page);
    page.addEventListener("click", event => routed.push((event as CustomEvent).detail));

    page.dispatchEvent(new CustomEvent("click", { cancelable: true, detail: "handled" }));
    page.dispatchEvent(new CustomEvent("click", { cancelable: true, detail: "plain" }));

    expect(routed).toEqual(["plain"]);
  });
});
