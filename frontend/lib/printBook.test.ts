import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { printBook } from "./printBook";

describe("book printing", () => {
  const print = vi.fn();
  let image: EventTarget & { complete: boolean; loading: string };
  let reader: { isConnected: boolean; querySelectorAll: () => (typeof image)[] };
  let location: { href: string };
  let querySelector: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    vi.useFakeTimers();
    print.mockClear();
    image = Object.assign(new EventTarget(), { complete: false, loading: "lazy" });
    reader = { isConnected: true, querySelectorAll: () => [image] };
    location = { href: "https://family.example/book/1" };
    querySelector = vi.fn(() => reader);
    vi.stubGlobal("document", { querySelector });
    vi.stubGlobal("window", { print, location });
  });

  afterEach(() => {
    vi.clearAllTimers();
    vi.useRealTimers();
    vi.unstubAllGlobals();
  });

  it("loads lazy photos before printing the current reader", async () => {
    const pending = printBook();
    expect(image.loading).toBe("eager");
    expect(print).not.toHaveBeenCalled();
    image.dispatchEvent(new Event("load"));
    await pending;
    expect(print).toHaveBeenCalledOnce();
  });

  it("does not print another page when the reader is removed", async () => {
    const pending = printBook();
    reader.isConnected = false;
    image.dispatchEvent(new Event("load"));
    await pending;
    expect(print).not.toHaveBeenCalled();
  });

  it("does not print a replacement book when the image wait times out", async () => {
    const pending = printBook();
    querySelector.mockReturnValue({ ...reader });
    await vi.advanceTimersByTimeAsync(15000);
    await pending;
    expect(print).not.toHaveBeenCalled();
  });

  it("does not print after navigation reuses the reader element", async () => {
    const pending = printBook();
    location.href = "https://family.example/book/2";
    image.dispatchEvent(new Event("error"));
    await pending;
    expect(print).not.toHaveBeenCalled();
  });
});
