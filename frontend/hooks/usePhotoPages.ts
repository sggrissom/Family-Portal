import * as vlens from "vlens";
import * as server from "../server";
import { PHOTO_PAGE_SIZE, photosRequest } from "../lib/photoPages";

export interface PhotoPages {
  query: string;
  seed: unknown;
  photos: server.PhotoWithPeople[];
  cursor: string;
  loading: boolean;
  started: boolean;
  error: string;
}

export const usePhotoPages = vlens.declareHook(
  (key: string): PhotoPages => ({
    query: "",
    seed: null,
    photos: [],
    cursor: "",
    loading: false,
    started: false,
    error: "",
  })
);

export const hasMorePhotos = (pages: PhotoPages) => !pages.started || pages.cursor !== "";

function reset(pages: PhotoPages, query: string) {
  pages.query = query;
  pages.photos = [];
  pages.cursor = "";
  pages.started = false;
  pages.error = "";
}

// Adopts a first page the route fetched, once per fetch.
export function seedPhotoPages(
  pages: PhotoPages,
  fields: Partial<server.ListFamilyPhotosRequest>,
  first: server.ListFamilyPhotosResponse
) {
  if (pages.seed === first) return;
  reset(pages, JSON.stringify(fields));
  pages.seed = first;
  pages.photos = first.photos ?? [];
  pages.cursor = first.nextCursor;
  pages.started = true;
}

// Starts over when the filters differ from the ones the loaded pages were cut
// with, and loads the first page if nothing has been loaded yet.
export function syncPhotoPages(pages: PhotoPages, fields: Partial<server.ListFamilyPhotosRequest>) {
  const query = JSON.stringify(fields);
  if (pages.query !== query) {
    reset(pages, query);
  }
  if (!pages.started && !pages.loading && !pages.error) {
    loadMorePhotos(pages);
  }
}

export async function loadMorePhotos(pages: PhotoPages) {
  if (pages.loading || !hasMorePhotos(pages)) return;

  const query = pages.query;
  const fields = query ? JSON.parse(query) : {};
  pages.loading = true;
  vlens.scheduleRedraw();

  const [resp, err] = await server.ListFamilyPhotos(
    photosRequest({ ...fields, limit: PHOTO_PAGE_SIZE, cursor: pages.cursor })
  );

  pages.loading = false;
  if (pages.query !== query) {
    // The filters changed while this page was in flight; its rows belong to
    // the old query.
    vlens.scheduleRedraw();
    return;
  }
  if (resp) {
    const ids = new Set(pages.photos.map(photo => photo.image.id));
    pages.photos = [...pages.photos, ...(resp.photos ?? []).filter(p => !ids.has(p.image.id))];
    pages.cursor = resp.nextCursor;
    pages.started = true;
    pages.error = "";
  } else {
    pages.error = err || "Failed to load photos";
  }
  vlens.scheduleRedraw();
}
