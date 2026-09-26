import { describe, expect, it } from "vitest";
import {
  failureSummary,
  maxPhotoBytes,
  pendingPhotos,
  photoFileProblem,
  type UploadState,
} from "@app/lib/photoUploadQueue";

describe("photoFileProblem", () => {
  it("accepts images up to the size limit", () => {
    expect(photoFileProblem({ name: "a.jpg", type: "image/jpeg", size: maxPhotoBytes })).toBe("");
  });

  it("names the file that is not an image", () => {
    expect(photoFileProblem({ name: "notes.pdf", type: "application/pdf", size: 10 })).toBe(
      "notes.pdf is not an image"
    );
  });

  it("names the file that is too large", () => {
    expect(photoFileProblem({ name: "big.png", type: "image/png", size: maxPhotoBytes + 1 })).toBe(
      "big.png is over 10MB"
    );
  });
});

function states(...list: UploadState[]) {
  return list.map(state => ({ state }));
}

describe("pendingPhotos", () => {
  it("skips photos already uploaded so a retry does not duplicate them", () => {
    expect(pendingPhotos(states("done", "failed", "queued", "done"))).toEqual(
      states("failed", "queued")
    );
  });
});

describe("failureSummary", () => {
  it("is empty when nothing failed", () => {
    expect(failureSummary(states("done", "done"))).toBe("");
  });

  it("is empty for a single photo, whose own error is shown instead", () => {
    expect(failureSummary(states("failed"))).toBe("");
  });

  it("counts failures across a batch", () => {
    expect(failureSummary(states("done", "failed", "failed"))).toBe(
      "2 of 3 photos could not be uploaded. Fix or remove them, then upload again."
    );
    expect(failureSummary(states("done", "failed"))).toBe(
      "1 of 2 photos could not be uploaded. Fix or remove them, then upload again."
    );
  });
});
