import * as preact from "preact";
import * as vlens from "vlens";
import * as server from "../server";
import { formatDuration } from "../lib/videoFiles";
import "./video-styles";

const LOW_CONFIDENCE = 0.5;

type TranscriptState = {
  photoId: number;
  transcript: server.GetVideoTranscriptResponse | null;
  editing: boolean;
  drafts: string[];
  confirmRemove: boolean;
  busy: boolean;
  error: string;
};

const useTranscriptState = vlens.declareHook(
  (): TranscriptState => ({
    photoId: 0,
    transcript: null,
    editing: false,
    drafts: [],
    confirmRemove: false,
    busy: false,
    error: "",
  })
);

export function currentSegment(segments: server.TranscriptSegment[], seconds: number): number {
  const ms = seconds * 1000;
  return segments.findIndex(s => ms >= s.startMs && ms < s.endMs);
}

function startEditing(state: TranscriptState) {
  state.editing = true;
  state.drafts = (state.transcript?.segments ?? []).map(s => s.text);
  state.error = "";
  vlens.scheduleRedraw();
}

function cancelEditing(state: TranscriptState) {
  state.editing = false;
  state.confirmRemove = false;
  vlens.scheduleRedraw();
}

async function save(state: TranscriptState) {
  state.busy = true;
  vlens.scheduleRedraw();
  const [resp, err] = await server.UpdateVideoTranscript({
    id: state.photoId,
    texts: state.drafts,
  });
  finish(state, resp, err || "Couldn't save the transcript");
}

async function remove(state: TranscriptState) {
  if (!state.confirmRemove) {
    state.confirmRemove = true;
    vlens.scheduleRedraw();
    return;
  }
  state.busy = true;
  vlens.scheduleRedraw();
  const [resp, err] = await server.RemoveVideoTranscript({ id: state.photoId });
  finish(state, resp, err || "Couldn't remove the transcript");
}

function finish(
  state: TranscriptState,
  resp: server.GetVideoTranscriptResponse | null,
  err: string
) {
  state.busy = false;
  if (resp) {
    state.transcript = resp;
    state.editing = false;
    state.confirmRemove = false;
    state.error = "";
  } else {
    state.error = err;
  }
  vlens.scheduleRedraw();
}

export const VideoTranscript = ({
  photoId,
  transcript,
  position,
  onSeek,
}: {
  photoId: number;
  transcript: server.GetVideoTranscriptResponse | null;
  position: number;
  onSeek: (seconds: number) => void;
}) => {
  const state = useTranscriptState();
  if (state.photoId !== photoId) {
    Object.assign(state, {
      photoId,
      transcript,
      editing: false,
      drafts: [],
      confirmRemove: false,
      busy: false,
      error: "",
    });
  }
  const t = state.transcript;
  if (!t) return null;

  if (t.status !== server.TranscriptDone) {
    return t.pending ? (
      <p className="video-transcript-note">
        A transcript will appear here once the video has been listened to.
      </p>
    ) : null;
  }

  const active = currentSegment(t.segments, position);
  return (
    <details className="video-transcript" open={state.editing || undefined}>
      <summary>
        Transcript
        {t.automatic && (
          <span className="video-transcript-hint">Automatic — may contain mistakes</span>
        )}
      </summary>
      {state.error && (
        <p className="video-transcript-error" role="alert">
          {state.error}
        </p>
      )}
      <ol className="video-transcript-lines">
        {t.segments.map((s, i) => (
          <li
            key={`${s.startMs}-${i}`}
            className={`${i === active ? "active" : ""} ${
              t.automatic && s.confidence < LOW_CONFIDENCE ? "unsure" : ""
            }`}
          >
            <button
              type="button"
              className="video-transcript-time"
              onClick={() => onSeek(s.startMs / 1000)}
            >
              {formatDuration(s.startMs)}
            </button>
            {state.editing ? (
              <textarea
                rows={2}
                value={state.drafts[i]}
                aria-label={`Line at ${formatDuration(s.startMs)}`}
                onInput={e => {
                  state.drafts[i] = (e.currentTarget as HTMLTextAreaElement).value;
                }}
              />
            ) : (
              <span>{s.text}</span>
            )}
          </li>
        ))}
      </ol>
      {t.canEdit && (
        <div className="video-transcript-actions">
          {state.editing ? (
            <>
              <button className="btn btn-primary" disabled={state.busy} onClick={() => save(state)}>
                Save
              </button>
              <button
                className="btn btn-outline"
                disabled={state.busy}
                onClick={() => cancelEditing(state)}
              >
                Cancel
              </button>
              <button
                className="btn btn-danger"
                disabled={state.busy}
                onClick={() => remove(state)}
              >
                {state.confirmRemove ? "Remove it for good" : "Remove transcript"}
              </button>
            </>
          ) : (
            <button className="btn btn-outline" onClick={() => startEditing(state)}>
              Fix mistakes
            </button>
          )}
        </div>
      )}
    </details>
  );
};
