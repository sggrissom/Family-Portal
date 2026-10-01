import * as server from "../server";
import { usePhotoStatus } from "../hooks/usePhotoStatus";

async function uploadErrorMessage(response: Response): Promise<string> {
  const fallback = `Upload failed with status ${response.status}`;
  const body = await response.text();
  if (!body) return fallback;

  try {
    const parsed = JSON.parse(body);
    return parsed?.error?.message || fallback;
  } catch {
    return body;
  }
}

export async function uploadPhoto(
  file: File,
  personIds: number[],
  familyId: number | undefined
): Promise<server.Image> {
  const formData = new FormData();
  formData.append("personIds", JSON.stringify(personIds));
  if (familyId !== undefined) {
    formData.append("familyId", String(familyId));
  }
  formData.append("inputType", "auto");
  formData.append("photo", file);

  const response = await window.fetch("/api/upload-photo", {
    method: "POST",
    credentials: "include",
    body: formData,
  });

  if (!response.ok) {
    throw new Error(await uploadErrorMessage(response));
  }

  const image: server.Image = (await response.json()).image;
  if (image.status === 1) {
    usePhotoStatus().startMonitoring(image.id, image.status);
  }
  return image;
}
