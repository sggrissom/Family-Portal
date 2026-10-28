import * as preact from "preact";
import * as vlens from "vlens";
import * as server from "../server";
import { copy } from "../lib/copy";
import { mediaAccept, mediaFileProblem, uploadMedia } from "../lib/mediaUpload";
import { useVideoSupport } from "../hooks/useVideoSupport";
import "./media-upload-styles";

export type MediaUploadState = {
  uploading: boolean;
  uploadProgress: number;
  uploadError: string;
};

export function newMediaUpload(): MediaUploadState {
  return { uploading: false, uploadProgress: 0, uploadError: "" };
}

type UploadTarget = {
  personIds: number[];
  familyId: number;
  onUploaded: (image: server.Image, file: File) => void;
};

async function onFilesChosen(
  state: MediaUploadState,
  video: server.GetVideoSupportResponse,
  target: UploadTarget,
  event: Event
) {
  const input = event.currentTarget as HTMLInputElement;
  const files = Array.from(input.files ?? []);
  input.value = "";
  if (state.uploading || files.length === 0) return;

  state.uploading = true;
  state.uploadError = "";
  vlens.scheduleRedraw();

  const problems: string[] = [];
  for (const file of files) {
    const problem = await mediaFileProblem(file, video);
    if (problem) {
      problems.push(problem);
      continue;
    }
    state.uploadProgress = 0;
    try {
      const image = await uploadMedia(file, target.personIds, target.familyId, fraction => {
        state.uploadProgress = fraction;
        vlens.scheduleRedraw();
      });
      target.onUploaded(image, file);
    } catch (err) {
      problems.push(`${file.name}: ${err instanceof Error ? err.message : String(err)}`);
    }
    vlens.scheduleRedraw();
  }

  state.uploading = false;
  state.uploadError = problems.join("; ");
  vlens.scheduleRedraw();
}

function uploadingLabel(state: MediaUploadState): string {
  return state.uploadProgress > 0
    ? `${copy.photos.uploading} ${Math.floor(state.uploadProgress * 100)}%`
    : `${copy.photos.uploading}…`;
}

interface MediaUploadButtonProps extends UploadTarget {
  state: MediaUploadState;
  disabled?: boolean;
  label?: string;
  className?: string;
}

// Upload new photos or videos (when the family takes them) right where they
// are needed, handing each finished upload back to attach.
export const MediaUploadButton = ({
  state,
  disabled = false,
  label,
  className = "btn btn-secondary",
  ...target
}: MediaUploadButtonProps): preact.ComponentChild => {
  const video = useVideoSupport(target.familyId);
  const off = disabled || state.uploading;
  return (
    <label className={`media-upload ${className}${disabled ? " disabled" : ""}`}>
      <input
        type="file"
        accept={mediaAccept(video)}
        multiple
        disabled={off}
        onChange={event => onFilesChosen(state, video, target, event)}
      />
      {state.uploading
        ? uploadingLabel(state)
        : (label ?? (video.enabled ? copy.photos.addNewWithVideo : copy.photos.addNew))}
    </label>
  );
};

export const MediaUploadError = ({ state }: { state: MediaUploadState }) =>
  state.uploadError ? (
    <div className="error-message media-upload-error" role="alert">
      {state.uploadError}
    </div>
  ) : null;
