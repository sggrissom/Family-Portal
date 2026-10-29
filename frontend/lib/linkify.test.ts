import { describe, expect, it } from "vitest";
import { linkParts } from "./linkify";

describe("linkParts", () => {
  it("leaves plain text alone", () => {
    expect(linkParts("see you at 5")).toEqual([{ text: "see you at 5" }]);
  });

  it("splits out links and drops trailing punctuation from them", () => {
    expect(linkParts("Photos: https://example.com/a?b=1. Nice!")).toEqual([
      { text: "Photos: " },
      { text: "https://example.com/a?b=1", href: "https://example.com/a?b=1" },
      { text: ". Nice!" },
    ]);
  });

  it("ignores schemes other than http and https", () => {
    expect(linkParts("javascript:alert(1)")).toEqual([{ text: "javascript:alert(1)" }]);
  });
});
