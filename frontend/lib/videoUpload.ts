import * as server from "../server";
import { localDateString } from "./when";
import { usePhotoStatus } from "../hooks/usePhotoStatus";

export { isVideoFile, videoFileProblem, durationProblem, formatDuration } from "./videoFiles";

export function readVideoDuration(file: File): Promise<number | null> {
  return new Promise(resolve => {
    const video = document.createElement("video");
    const url = URL.createObjectURL(file);
    const done = (value: number | null) => {
      URL.revokeObjectURL(url);
      video.removeAttribute("src");
      resolve(value);
    };
    video.preload = "metadata";
    video.onloadedmetadata = () => done(video.duration);
    video.onerror = () => done(null);
    setTimeout(() => done(null), 10000);
    video.src = url;
  });
}

export function uploadUrl(url: string): string {
  return url.startsWith("/") ? window.location.origin + url : url;
}

function sendFile(
  file: File,
  upload: server.UploadInstructions,
  onProgress: (fraction: number) => void
): Promise<void> {
  return new Promise((resolve, reject) => {
    const xhr = new XMLHttpRequest();
    xhr.open(upload.method, uploadUrl(upload.url));
    xhr.withCredentials = false;
    for (const [name, value] of Object.entries(upload.headers || {})) {
      xhr.setRequestHeader(name, value);
    }
    xhr.upload.onprogress = event => {
      if (event.lengthComputable) onProgress(event.loaded / event.total);
    };
    xhr.onload = () => {
      if (xhr.status >= 200 && xhr.status < 300) {
        resolve();
        return;
      }
      let message = `The upload failed (${xhr.status}).`;
      try {
        message = JSON.parse(xhr.responseText)?.error?.message || message;
      } catch {}
      reject(new Error(message));
    };
    xhr.onerror = () => reject(new Error("The connection dropped during the upload."));
    xhr.send(file);
  });
}

async function complete(uploadId: number): Promise<server.Image> {
  let lastError = "";
  for (let attempt = 0; attempt < 4; attempt++) {
    if (attempt > 0) await new Promise(r => setTimeout(r, 1000 * attempt));
    const [resp, err] = await server.CompleteVideoUpload({ uploadId });
    if (resp) return resp.image;
    lastError = err;
    if (err === server.ErrVideoUploadExpired || err === server.ErrVideoUploadNotFound) break;
  }
  throw new Error(lastError || "The upload could not be finished.");
}

export async function uploadVideo(
  file: File,
  personIds: number[],
  familyId: number | undefined,
  onProgress: (fraction: number) => void
): Promise<server.Image> {
  const [begun, beginErr] = await server.BeginVideoUpload({
    familyId: familyId ?? 0,
    personIds,
    filename: file.name,
    contentType: file.type,
    size: file.size,
    title: "",
    description: "",
    fallbackDate: localDateString(new Date()),
  });
  if (!begun) throw new Error(beginErr || "The upload could not be started.");

  try {
    await sendFile(file, begun.upload, onProgress);
  } catch (error) {
    server.CancelVideoUpload({ uploadId: begun.uploadId });
    throw error;
  }

  const image = await complete(begun.uploadId);
  if (image.status === 1) {
    usePhotoStatus().startMonitoring(image.id, image.status);
  }
  return image;
}
