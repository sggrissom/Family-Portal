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
import { VideoTranscript } from "./VideoTranscript";
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
  element: HTMLVideoElement | null;
  attempt: number;
  renewing: boolean;
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
    element: null,
    attempt: 0,
    renewing: false,
  })
);

const MAX_RENEWALS = 3;

async function probe(url: string): Promise<number | null> {
  try {
    const resp = await window.fetch(uploadUrl(url), {
      headers: { Range: "bytes=0-0" },
      credentials: "omit",
    });
    // Some servers ignore Range; don't keep downloading an entire video.
    await resp.body?.cancel();
    return resp.status;
  } catch {
    return null;
  }
}

async function renew(state: PlayerState) {
  if (state.renewing) return;
  state.renewing = true;
  const photoId = state.photoId;
  const attempt = ++state.attempt;
  state.resumeAt = state.position;
  state.resumePlaying = state.playing;
  state.renewals++;
  const [fresh] = await server.GetVideoPlayback({ id: photoId });
  if (state.photoId !== photoId || state.attempt !== attempt) return;
  state.renewing = false;
  if (fresh) {
    state.playback = fresh;
    state.problem = "";
    // A redraw during the request may already have mounted the old URL.
    state.attempt++;
  } else {
    state.problem = "network";
  }
  vlens.scheduleRedraw();
}

async function onPlaybackError(state: PlayerState, event: Event) {
  const video = event.currentTarget as HTMLVideoElement;
  const playback = state.playback;
  if (!playback || state.element !== video || state.renewing) return;
  const attempt = state.attempt;
  const errorCode = video.error?.code ?? 0;
  const status = await probe(video.currentSrc || playback.playbackUrl);
  if (
    state.attempt !== attempt ||
    state.playback !== playback ||
    state.element !== video ||
    !video.error
  )
    return;
  const problem = classifyPlaybackError(
    errorCode,
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

function seek(state: PlayerState, seconds: number) {
  const video = state.element;
  if (!video) return;
  video.currentTime = seconds;
  video.play().catch(() => {});
}

export const VideoPlayer = ({
  photo,
  playback,
  startAt = 0,
  transcript = null,
}: {
  photo: server.Image;
  playback: server.GetVideoPlaybackResponse | null;
  startAt?: number;
  transcript?: server.GetVideoTranscriptResponse | null;
}) => {
  const state = usePlayerState();
  if (state.photoId !== photo.id) {
    state.attempt++;
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
      element: null,
      renewing: false,
    });
  }
  const transcriptPanel = (
    <VideoTranscript
      photoId={photo.id}
      transcript={transcript}
      position={state.position}
      onSeek={seconds => seek(state, seconds)}
    />
  );

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
  if (
    state.problem === "unsupported" ||
    state.problem === "decode" ||
    (!likely && !state.forcePlay)
  ) {
    return (
      <div className="video-player">
        <Poster photo={photo} />
        <p className="video-player-note">
          <strong>
            {state.problem === "decode"
              ? "Playback stopped"
              : "This browser couldn't play this video"}
          </strong>
          {state.problem === "decode"
            ? "The video couldn't be decoded. Try loading it again."
            : "Try loading it again. If it still won't play, its format may not be supported on this device. You can also download the original."}
        </p>
        <div className="video-player-actions">
          <DownloadLink playback={p} />
          {state.problem && (
            <button
              className="btn btn-outline"
              disabled={state.renewing}
              onClick={() => retry(state)}
            >
              <Icon symbol={RotateCcw} /> Try again
            </button>
          )}
          {!likely && state.problem !== "unsupported" && (
            <button className="btn btn-outline" onClick={() => playAnyway(state)}>
              Try playing anyway
            </button>
          )}
        </div>
        {transcriptPanel}
      </div>
    );
  }

  return (
    <div className="video-player">
      <video
        key={`${photo.id}:${state.attempt}:${p.playbackUrl}`}
        ref={el => {
          state.element = el;
        }}
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
      >
        {transcript?.captionUrl && (
          <track
            kind="captions"
            src={transcript.captionUrl}
            srcLang={transcript.language || "en"}
            label={transcript.automatic ? "English (automatic)" : "English"}
          />
        )}
      </video>
      {state.problem === "network" && (
        <p className="video-player-note" role="alert">
          <strong>Playback stopped</strong>
          The connection to the video was lost.{" "}
          <button className="link-button" onClick={() => retry(state)}>
            <Icon symbol={RotateCcw} size={14} /> Try again
          </button>
        </p>
      )}
      {transcriptPanel}
    </div>
  );
};
