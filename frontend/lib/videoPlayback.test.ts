import { describe, expect, it } from "vitest";
import { canProbablyPlay, classifyPlaybackError } from "./videoPlayback";
import { durationProblem, formatDuration, isVideoFile, videoFileProblem } from "./videoFiles";

describe("classifyPlaybackError", () => {
  const now = 1_000_000;
  const later = now + 3_600_000;

  it("renews a link at or near its expiry without probing", () => {
    expect(classifyPlaybackError(4, null, later, later)).toBe("expired");
    expect(classifyPlaybackError(2, null, later - 30_000, later)).toBe("expired");
  });

  it("renews a link the server refuses", () => {
    expect(classifyPlaybackError(4, 403, now, later)).toBe("expired");
    expect(classifyPlaybackError(4, 400, now, later)).toBe("expired");
  });

  it("calls a reachable file the browser won't decode unsupported", () => {
    expect(classifyPlaybackError(4, 206, now, later)).toBe("unsupported");
    expect(classifyPlaybackError(3, 200, now, later)).toBe("decode");
  });

  it("does not call an aborted or cleared error an unsupported format", () => {
    expect(classifyPlaybackError(1, 206, now, later)).toBe("network");
    expect(classifyPlaybackError(0, 206, now, later)).toBe("network");
  });

  it("treats an unreachable file as a connection problem", () => {
    expect(classifyPlaybackError(2, null, now, later)).toBe("network");
    expect(classifyPlaybackError(4, 502, now, later)).toBe("network");
    expect(classifyPlaybackError(2, 206, now, later)).toBe("network");
  });
});

describe("canProbablyPlay", () => {
  it("asks with the codecs", () => {
    const seen: string[] = [];
    canProbablyPlay(t => (seen.push(t), "probably"), 'video/mp4; codecs="hvc1.1.6.L93.B0"');
    expect(seen[0]).toBe('video/mp4; codecs="hvc1.1.6.L93.B0"');
  });

  it("trusts plain MP4 support for H.264 but not for HEVC", () => {
    const mp4Only = (t: string) => (t === "video/mp4" ? "maybe" : "");
    expect(canProbablyPlay(mp4Only, 'video/mp4; codecs="avc1.640028, mp4a.40.2"')).toBe(true);
    expect(canProbablyPlay(mp4Only, 'video/mp4; codecs="hvc1.2.4.L153.B0"')).toBe(false);
  });
});

describe("video files", () => {
  const support = { enabled: true, reason: "", maxBytes: 1024 };

  it("recognises videos by type or extension", () => {
    expect(isVideoFile({ name: "IMG_1.MOV", type: "" })).toBe(true);
    expect(isVideoFile({ name: "clip", type: "video/mp4" })).toBe(true);
    expect(isVideoFile({ name: "a.jpg", type: "image/jpeg" })).toBe(false);
  });

  it("explains why a video can't be added", () => {
    expect(videoFileProblem({ name: "a.mov", type: "video/quicktime", size: 10 }, support)).toBe(
      ""
    );
    expect(
      videoFileProblem({ name: "a.mov", type: "video/quicktime", size: 2048 }, support)
    ).toContain("over 1.0 KB");
    expect(videoFileProblem({ name: "a.webm", type: "video/webm", size: 10 }, support)).toContain(
      "not an MP4 or MOV"
    );
    expect(
      videoFileProblem(
        { name: "a.mov", type: "video/quicktime", size: 10 },
        { ...support, enabled: false, reason: "Video uploads are not turned on for this family." }
      )
    ).toBe("a.mov: Video uploads are not turned on for this family.");
  });

  it("checks duration with a second of slack", () => {
    expect(durationProblem("a.mov", 600.5, 600)).toBe("");
    expect(durationProblem("a.mov", null, 600)).toBe("");
    expect(durationProblem("a.mov", 725, 600)).toBe(
      "a.mov is 12:05 long; videos can be at most 10 minutes"
    );
  });

  it("formats durations", () => {
    expect(formatDuration(0)).toBe("0:00");
    expect(formatDuration(65_400)).toBe("1:05");
    expect(formatDuration(3_725_000)).toBe("1:02:05");
  });
});
