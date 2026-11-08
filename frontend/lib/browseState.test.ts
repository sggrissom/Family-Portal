import { beforeEach, describe, expect, it } from "vitest";
import { browseKey, recallBrowse, rememberBrowse, rememberScroll } from "./browseState";

describe("browse state", () => {
  beforeEach(() => sessionStorage.clear());

  it("belongs to one history entry", () => {
    const first = browseKey("/history", { ts: 1 });
    const second = browseKey("/history", { ts: 2 });
    rememberBrowse(first, { personIds: [3] });
    rememberScroll(first, 840.4);
    expect(recallBrowse(first)).toEqual({ value: { personIds: [3] }, y: 840 });
    expect(recallBrowse(second)).toBeNull();
  });

  it("keeps the scroll when the filters change again", () => {
    const key = browseKey("/history", { ts: 5 });
    rememberBrowse(key, { personIds: [] });
    rememberScroll(key, 300);
    rememberBrowse(key, { personIds: [1] });
    expect(recallBrowse(key)).toEqual({ value: { personIds: [1] }, y: 300 });
  });

  it("ignores scroll for a page with nothing saved", () => {
    rememberScroll(browseKey("/photos", { ts: 9 }), 100);
    expect(recallBrowse(browseKey("/photos", { ts: 9 }))).toBeNull();
  });
});
