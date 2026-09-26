export const maxPhotoBytes = 10 * 1024 * 1024;

export type UploadState = "queued" | "uploading" | "done" | "failed";

export type QueuedPhoto = {
  file: File;
  previewUrl: string;
  state: UploadState;
  error: string;
};

export function photoFileProblem(file: { name: string; type: string; size: number }): string {
  if (!file.type.startsWith("image/")) {
    return `${file.name} is not an image`;
  }
  if (file.size > maxPhotoBytes) {
    return `${file.name} is over 10MB`;
  }
  return "";
}

export function pendingPhotos<T extends { state: UploadState }>(photos: T[]): T[] {
  return photos.filter(p => p.state !== "done");
}

export function failureSummary(photos: { state: UploadState }[]): string {
  const failed = photos.filter(p => p.state === "failed").length;
  if (failed === 0) return "";
  if (photos.length === 1) return "";
  return `${failed} of ${photos.length} photos could not be uploaded. Fix or remove them, then upload again.`;
}
