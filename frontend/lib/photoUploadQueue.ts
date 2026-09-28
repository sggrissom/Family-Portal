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

export function peopleChanges(
  taggedAtUpload: number[],
  selected: number[]
): { add: number[]; remove: number[] } {
  return {
    add: selected.filter(id => !taggedAtUpload.includes(id)),
    remove: taggedAtUpload.filter(id => !selected.includes(id)),
  };
}

const MONTHS = ["Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"];

export function takenLabel(dates: string[]): string {
  const days = dates
    .map(d => new Date(d))
    .filter(d => !isNaN(d.getTime()))
    .sort((a, b) => a.getTime() - b.getTime());
  if (days.length === 0) return "";
  const first = days[0];
  const last = days[days.length - 1];
  const [fy, fm, fd] = [first.getUTCFullYear(), first.getUTCMonth(), first.getUTCDate()];
  const [ly, lm, ld] = [last.getUTCFullYear(), last.getUTCMonth(), last.getUTCDate()];

  if (fy === ly && fm === lm && fd === ld) return `${MONTHS[fm]} ${fd}, ${fy}`;
  if (fy === ly && fm === lm) return `${MONTHS[fm]} ${fd} – ${ld}, ${fy}`;
  if (fy === ly) return `${MONTHS[fm]} ${fd} – ${MONTHS[lm]} ${ld}, ${fy}`;
  return `${MONTHS[fm]} ${fd}, ${fy} – ${MONTHS[lm]} ${ld}, ${ly}`;
}
