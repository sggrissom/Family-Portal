import * as server from "../server";
import { photoFileProblem } from "./photoUploadQueue";
import { uploadPhoto } from "./photoUpload";
import { isVideoFile, videoTypes } from "./videoFiles";
import { durationProblem, readVideoDuration, uploadVideo, videoFileProblem } from "./videoUpload";

export const noVideo: server.GetVideoSupportResponse = {
  enabled: false,
  reason: "",
  maxBytes: 0,
  maxDurationSeconds: 0,
  contentTypes: [],
};

export function mediaAccept(video: { enabled: boolean }): string {
  return video.enabled ? ["image/*", ...videoTypes, ".mov", ".mp4", ".m4v"].join(",") : "image/*";
}

export async function mediaFileProblem(
  file: File,
  video: server.GetVideoSupportResponse
): Promise<string> {
  if (!isVideoFile(file)) return photoFileProblem(file);
  return (
    videoFileProblem(file, video) ||
    durationProblem(file.name, await readVideoDuration(file), video.maxDurationSeconds)
  );
}

export function uploadMedia(
  file: File,
  personIds: number[],
  familyId: number | undefined,
  onProgress: (fraction: number) => void
): Promise<server.Image> {
  return isVideoFile(file)
    ? uploadVideo(file, personIds, familyId, onProgress)
    : uploadPhoto(file, personIds, familyId);
}
