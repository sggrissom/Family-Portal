import { describe, expect, it } from "vitest";
import {
  failureSummary,
  maxPhotoBytes,
  peopleChanges,
  pendingPhotos,
  takenLabel,
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

describe("peopleChanges", () => {
  it("adds who was picked after upload and removes who was unpicked", () => {
    expect(peopleChanges([1, 2], [2, 3])).toEqual({ add: [3], remove: [1] });
    expect(peopleChanges([], [])).toEqual({ add: [], remove: [] });
  });
});

describe("takenLabel", () => {
  it("names one day, a span in a month, and a span across months", () => {
    expect(takenLabel(["2026-09-03T14:00:00Z", "2026-09-03T09:00:00Z"])).toBe("Sep 3, 2026");
    expect(takenLabel(["2026-09-03T14:00:00Z", "2026-09-01T09:00:00Z"])).toBe("Sep 1 – 3, 2026");
    expect(takenLabel(["2026-08-30T00:00:00Z", "2026-09-03T00:00:00Z"])).toBe(
      "Aug 30 – Sep 3, 2026"
    );
    expect(takenLabel(["2025-12-30T00:00:00Z", "2026-01-02T00:00:00Z"])).toBe(
      "Dec 30, 2025 – Jan 2, 2026"
    );
  });

  it("is empty before anything has uploaded", () => {
    expect(takenLabel([])).toBe("");
  });
});
