import { describe, expect, it } from "vitest";
import { faceCropLayout, hasFaceBox } from "./faceCrop";

describe("faceCropLayout", () => {
  it("scales and offsets the photo so the box fills the frame", () => {
    const layout = faceCropLayout({ left: 0.25, top: 0.5, right: 0.5, bottom: 0.75 }, 0);
    expect(layout).toEqual({
      width: "400.000%",
      height: "400.000%",
      left: "-100.000%",
      top: "-200.000%",
    });
  });

  it("pads around the face", () => {
    const layout = faceCropLayout({ left: 0.4, top: 0.4, right: 0.6, bottom: 0.6 }, 0.5);
    expect(layout).toEqual({
      width: "250.000%",
      height: "250.000%",
      left: "-75.000%",
      top: "-75.000%",
    });
  });
});

describe("hasFaceBox", () => {
  it("rejects the empty box a legacy analysis leaves", () => {
    expect(hasFaceBox({ left: 0, top: 0, right: 0, bottom: 0 })).toBe(false);
    expect(hasFaceBox({ left: 0.1, top: 0.1, right: 0.2, bottom: 0.2 })).toBe(true);
  });
});
