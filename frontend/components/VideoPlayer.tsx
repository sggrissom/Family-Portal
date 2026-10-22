import * as preact from "preact";
import * as vlens from "vlens";
import * as core from "vlens/core";
import { Download, RotateCcw } from "lucide-preact";
import * as server from "../server";
import { Icon } from "./Icon";
import { FullImage } from "./ResponsiveImage";
import { usePhotoStatus, Status } from "../hooks/usePhotoStatus";
import { canProbablyPlay, classifyPlaybackError, type PlaybackProblem } from "../lib/videoPlayback";
import { uploadUrl } from "../lib/videoUpload";
import "./video-styles";

type PlayerState = {
  photoId: number;
  playback: server.GetVideoPlaybackResponse | null;
  problem: PlaybackProblem | "";
  forcePlay: boolean;
  resumeAt: number;
  resumePlaying: boolean;
  position: number;
  playing: boolean;
  renewals: number;
  watching: boolean;
};

const usePlayerState = vlens.declareHook(
  (): PlayerState => ({
    photoId: 0,
    playback: null,
    problem: "",
    forcePlay: false,
    resumeAt: 0,
    resumePlaying: false,
    position: 0,
    playing: false,
    renewals: 0,
    watching: false,
  })
);

const MAX_RENEWALS = 3;

async function probe(url: string): Promise<number | null> {
  try {
    const resp = await window.fetch(uploadUrl(url), {
      headers: { Range: "bytes=0-0" },
      credentials: "omit",
    });
    return resp.status;
  } catch {
    return null;
  }
}

async function renew(state: PlayerState) {
  state.resumeAt = state.position;
  state.resumePlaying = state.playing;
  state.renewals++;
  const [fresh] = await server.GetVideoPlayback({ id: state.photoId });
  if (fresh) {
    state.playback = fresh;
    state.problem = "";
  } else {
    state.problem = "network";
  }
  vlens.scheduleRedraw();
}

async function onPlaybackError(state: PlayerState, event: Event) {
  const video = event.currentTarget as HTMLVideoElement;
  const playback = state.playback;
  if (!playback) return;
  const status = await probe(video.currentSrc || playback.playbackUrl);
  const problem = classifyPlaybackError(
    video.error?.code ?? 0,
    status,
    Date.now(),
    new Date(playback.expiresAt).getTime()
  );
  if (problem === "expired" && state.renewals < MAX_RENEWALS) {
    await renew(state);
    return;
  }
  state.problem = problem === "expired" ? "network" : problem;
  vlens.scheduleRedraw();
}

// Only the current link's progress counts: a failed load resets the element
// to the start before its error arrives.
function onProgress(state: PlayerState, event: Event) {
  const video = event.currentTarget as HTMLVideoElement;
  if (video.error || video.readyState < 1 || !state.playback) return;
  if (!video.currentSrc.endsWith(state.playback.playbackUrl)) return;
  state.position = video.currentTime;
  state.playing = !video.paused;
}

function onLoadedMetadata(state: PlayerState, event: Event) {
  const video = event.currentTarget as HTMLVideoElement;
  if (state.resumeAt > 0) {
    video.currentTime = state.resumeAt;
    state.resumeAt = 0;
  }
  if (state.resumePlaying) {
    state.resumePlaying = false;
    video.play().catch(() => {});
  }
}

function retry(state: PlayerState) {
  state.renewals = 0;
  renew(state);
}

function playAnyway(state: PlayerState) {
  state.forcePlay = true;
  state.problem = "";
  vlens.scheduleRedraw();
}

const Poster = ({ photo }: { photo: server.Image }) =>
  photo.filePath ? (
    <div className="video-player-poster">
      <FullImage photoId={photo.id} alt={photo.title} />
    </div>
  ) : null;

const DownloadLink = ({ playback }: { playback: server.GetVideoPlaybackResponse }) =>
  playback.downloadUrl ? (
    <a
      className="btn btn-secondary"
      href={uploadUrl(playback.downloadUrl)}
      download={playback.originalFilename}
    >
      <Icon symbol={Download} /> Download original
    </a>
  ) : null;

export const VideoPlayer = ({
  photo,
  playback,
  startAt = 0,
}: {
  photo: server.Image;
  playback: server.GetVideoPlaybackResponse | null;
  startAt?: number;
}) => {
  const state = usePlayerState();
  if (state.photoId !== photo.id) {
    Object.assign(state, {
      photoId: photo.id,
      playback,
      problem: "",
      forcePlay: false,
      resumeAt: startAt,
      resumePlaying: false,
      position: 0,
      playing: false,
      renewals: 0,
      watching: false,
    });
  }

  const photoStatus = usePhotoStatus();
  const status = photoStatus.getStatus(photo.id);
  const current = status === Status.Unknown ? photo.status : status;

  if (state.watching && current !== Status.Processing && photo.status === Status.Processing) {
    state.watching = false;
    core.replaceRoute(core.getRoute());
  }

  if (current === Status.Processing) {
    if (!state.watching) {
      state.watching = true;
      photoStatus.startMonitoring(photo.id, photo.status);
    }
    return (
      <div className="video-player">
        <Poster photo={photo} />
        <p className="video-player-note" role="status">
          <strong>Still processing</strong>
          This video is being prepared for playback. A long video can take a few minutes; this page
          will update when it's ready.
        </p>
      </div>
    );
  }

  const p = state.playback;
  if (current === Status.Failed || !p) {
    return (
      <div className="video-player">
        <Poster photo={photo} />
        <p className="video-player-note" role="alert">
          <strong>This video can't be played</strong>
          {photo.processingError || p?.processingError || "It couldn't be prepared for playback."}
        </p>
        {p && (
          <div className="video-player-actions">
            <DownloadLink playback={p} />
          </div>
        )}
      </div>
    );
  }

  const likely = canProbablyPlay(
    type => document.createElement("video").canPlayType(type),
    p.mimeType
  );
  if (state.problem === "unsupported" || (!likely && !state.forcePlay)) {
    return (
      <div className="video-player">
        <Poster photo={photo} />
        <p className="video-player-note">
          <strong>This browser can't play this video</strong>
          It was recorded in a format this browser or device doesn't support (often HEVC from an
          iPhone on a computer without HEVC playback). Download the original to watch it, or open it
          on another device.
        </p>
        <div className="video-player-actions">
          <DownloadLink playback={p} />
          {!likely && state.problem !== "unsupported" && (
            <button className="btn btn-outline" onClick={() => playAnyway(state)}>
              Try playing anyway
            </button>
          )}
        </div>
      </div>
    );
  }

  return (
    <div className="video-player">
      <video
        key={p.playbackUrl}
        src={uploadUrl(p.playbackUrl)}
        poster={photo.filePath ? `/api/photo/${photo.id}/xlarge` : undefined}
        controls
        playsInline
        preload="metadata"
        width={photo.width || undefined}
        height={photo.height || undefined}
        onError={e => onPlaybackError(state, e)}
        onLoadedMetadata={e => onLoadedMetadata(state, e)}
        onTimeUpdate={e => onProgress(state, e)}
        onPlay={e => onProgress(state, e)}
        onPause={e => onProgress(state, e)}
      />
      {state.problem === "network" && (
        <p className="video-player-note" role="alert">
          <strong>Playback stopped</strong>
          The connection to the video was lost.{" "}
          <button className="link-button" onClick={() => retry(state)}>
            <Icon symbol={RotateCcw} size={14} /> Try again
          </button>
        </p>
      )}
    </div>
  );
};
