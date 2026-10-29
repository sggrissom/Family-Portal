import { beforeEach, describe, expect, it, vi } from "vitest";
import type { VNode } from "preact";
import { VideoPlayer } from "./VideoPlayer";
import type { Image, GetVideoPlaybackResponse } from "../server";

const mocks = vi.hoisted(() => ({
  state: null as any,
  getPlayback: vi.fn(),
  fetch: vi.fn(),
}));
vi.mock("vlens", () => ({
  declareHook: (init: () => unknown) => () => (mocks.state ??= init()),
  scheduleRedraw: vi.fn(),
}));
vi.mock("vlens/core", () => ({ replaceRoute: vi.fn(), getRoute: () => "" }));
vi.mock("vlens/css", () => ({ block: vi.fn() }));
vi.mock("../server", () => ({ GetVideoPlayback: mocks.getPlayback }));
vi.mock("../hooks/usePhotoStatus", () => ({
  Status: { Unknown: -1, Processing: 1, Failed: 2 },
  usePhotoStatus: () => ({ getStatus: () => -1 }),
}));
vi.mock("./ResponsiveImage", () => ({ FullImage: () => null }));
vi.mock("./VideoTranscript", () => ({ VideoTranscript: () => null }));
vi.mock("../lib/videoUpload", () => ({ uploadUrl: (url: string) => url }));

const playback = {
  playbackUrl: "https://media.example/video.mp4",
  mimeType: 'video/mp4; codecs="avc1.640028"',
  expiresAt: "2099-01-01T00:00:00Z",
} as GetVideoPlaybackResponse;

function render(id = 1) {
  return VideoPlayer({ photo: { id, status: 0 } as Image, playback });
}

function find(node: any, type: string): VNode<any> | undefined {
  if (!node || typeof node !== "object") return;
  if (node.type === type) return node;
  for (const child of [node.props?.children].flat(Infinity)) {
    const result = find(child, type);
    if (result) return result;
  }
}

function mountVideo() {
  const vnode = find(render(), "video")!;
  const video = {
    currentSrc: playback.playbackUrl,
    error: { code: 4 },
  };
  (vnode.ref as (el: unknown) => void)(video);
  return { vnode, video };
}

function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>(r => (resolve = r));
  return { promise, resolve };
}

beforeEach(() => {
  mocks.state = null;
  vi.clearAllMocks();
  vi.stubGlobal("document", { createElement: () => ({ canPlayType: () => "probably" }) });
  vi.stubGlobal("window", { fetch: mocks.fetch });
});

describe("video error recovery", () => {
  it("ignores a probe that finishes after navigation, including back to the same photo", async () => {
    const pending = deferred<any>();
    mocks.fetch.mockReturnValue(pending.promise);
    const { vnode, video } = mountVideo();
    const checking = vnode.props.onError({ currentTarget: video });
    render(2);
    render(1);
    pending.resolve({ status: 206 });
    await checking;
    expect(mocks.state.problem).toBe("");
    expect(mocks.getPlayback).not.toHaveBeenCalled();
  });

  it("ignores a renewed link that finishes after navigation", async () => {
    mocks.fetch.mockResolvedValue({ status: 403 });
    const pending = deferred<any>();
    mocks.getPlayback.mockReturnValue(pending.promise);
    const { vnode, video } = mountVideo();
    const checking = vnode.props.onError({ currentTarget: video });
    await vi.waitFor(() => expect(mocks.getPlayback).toHaveBeenCalled());
    render(2);
    pending.resolve([{ ...playback, playbackUrl: "https://media.example/old.mp4" }, ""]);
    await checking;
    expect(mocks.state.playback).toBe(playback);
    expect(mocks.state.photoId).toBe(2);
  });

  it("recreates the player on retry even when the signed URL is unchanged", async () => {
    const cancel = vi.fn();
    mocks.fetch.mockResolvedValue({ status: 206, body: { cancel } });
    mocks.getPlayback.mockResolvedValue([playback, ""]);
    const { vnode, video } = mountVideo();
    await vnode.props.onError({ currentTarget: video });
    expect(mocks.state.problem).toBe("unsupported");
    const retryButton = find(render(), "button")!;
    expect(retryButton).toBeDefined();
    await retryButton.props.onClick();
    // onClick starts the asynchronous renewal.
    await vi.waitFor(() => expect(mocks.state.problem).toBe(""));
    expect(find(render(), "video")!.key).not.toBe(vnode.key);
    expect(cancel).toHaveBeenCalledOnce();
  });

  it("ignores an error that has cleared while probing", async () => {
    const pending = deferred<any>();
    mocks.fetch.mockReturnValue(pending.promise);
    const { vnode, video } = mountVideo();
    const checking = vnode.props.onError({ currentTarget: video });
    Object.assign(video, { error: null });
    pending.resolve({ status: 206 });
    await checking;
    expect(mocks.state.problem).toBe("");
  });
});
