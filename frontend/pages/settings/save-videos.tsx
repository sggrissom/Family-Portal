import * as preact from "preact";
import * as vlens from "vlens";
import * as rpc from "vlens/rpc";
import { Download, FolderDown, RotateCcw } from "lucide-preact";
import * as server from "../../server";
import { Header, Footer } from "../../layout";
import { Icon } from "../../components/Icon";
import { requireAuthInView } from "../../lib/authHelpers";
import { formatBytes } from "../../lib/formatBytes";
import { formatDuration } from "../../lib/videoFiles";
import { uploadUrl } from "../../lib/videoUpload";
import { alreadySaved, linkIsFresh, saveSummary, type SaveState } from "../../lib/saveVideos";
import "./settings-styles";

type Row = {
  video: server.ExportVideo;
  url: string;
  expiresAt: string;
  state: SaveState;
  progress: number;
  error: string;
};

type PageState = {
  rows: Row[];
  running: boolean;
  loadedFor: string;
};

const usePageState = vlens.declareHook(
  (): PageState => ({ rows: [], running: false, loadedFor: "" })
);

type DirectoryHandle = {
  getFileHandle(name: string, options?: { create: boolean }): Promise<FileHandle>;
};
type FileHandle = {
  getFile(): Promise<File>;
  createWritable(): Promise<WritableStream<Uint8Array>>;
};
type DirectoryPicker = (options?: { mode: "readwrite" }) => Promise<DirectoryHandle>;

const directoryPicker = (): DirectoryPicker | null =>
  (window as unknown as { showDirectoryPicker?: DirectoryPicker }).showDirectoryPicker ?? null;

export async function fetch(route: string, prefix: string) {
  const familyId =
    parseInt(new URLSearchParams(route.split("?")[1] ?? "").get("family") ?? "") || 0;
  return server.ListVideoOriginals({ familyId });
}

export function view(
  route: string,
  prefix: string,
  data: server.ListVideoOriginalsResponse
): preact.ComponentChild {
  if (!requireAuthInView()) return;
  const state = usePageState();
  if (state.loadedFor !== route) {
    state.loadedFor = route;
    state.running = false;
    state.rows = data.videos.map(v => ({
      video: v.video,
      url: v.downloadUrl,
      expiresAt: data.expiresAt,
      state: "pending",
      progress: 0,
      error: "",
    }));
  }
  return (
    <div>
      <Header isHome={false} />
      <main id="app" className="settings-container">
        <SaveVideosPage state={state} data={data} />
      </main>
      <Footer />
    </div>
  );
}

async function freshLink(row: Row, force: boolean): Promise<string> {
  if (!force && row.url && linkIsFresh(row.expiresAt, Date.now())) return row.url;
  const [link, err] = await server.GetVideoOriginalLink({ id: row.video.id });
  if (!link) throw new Error(err || "Could not get a download link.");
  row.url = link.downloadUrl;
  row.expiresAt = link.expiresAt;
  return row.url;
}

async function fetchOriginal(row: Row): Promise<Response> {
  for (const force of [false, true]) {
    const resp = await window.fetch(uploadUrl(await freshLink(row, force)), {
      credentials: "omit",
    });
    if (resp.ok) return resp;
    if (resp.status !== 403 && resp.status !== 400) {
      throw new Error(`The server answered ${resp.status}.`);
    }
  }
  throw new Error("The download link was refused.");
}

async function saveInto(dir: DirectoryHandle, row: Row) {
  const name = row.video.file_name;
  const existing = await dir
    .getFileHandle(name)
    .then(h => h.getFile())
    .then(f => f.size)
    .catch(() => null);
  if (alreadySaved(existing, row.video.file_size)) {
    row.state = "skipped";
    return;
  }

  const resp = await fetchOriginal(row);
  const handle = await dir.getFileHandle(name, { create: true });
  const writable = await handle.createWritable();
  const total = row.video.file_size || Number(resp.headers.get("Content-Length")) || 0;
  let received = 0;
  const counter = new TransformStream<Uint8Array, Uint8Array>({
    transform(chunk, controller) {
      received += chunk.byteLength;
      row.progress = total ? received / total : 0;
      vlens.scheduleRedraw();
      controller.enqueue(chunk);
    },
  });
  await resp.body!.pipeThrough(counter).pipeTo(writable);
  row.state = "saved";
}

async function saveAllToFolder(state: PageState) {
  const pick = directoryPicker();
  if (!pick || state.running) return;
  let dir: DirectoryHandle;
  try {
    dir = await pick({ mode: "readwrite" });
  } catch {
    return;
  }
  state.running = true;
  vlens.scheduleRedraw();
  for (const row of state.rows) {
    if (row.state === "saved" || row.state === "skipped") continue;
    row.state = "saving";
    row.progress = 0;
    row.error = "";
    vlens.scheduleRedraw();
    try {
      await saveInto(dir, row);
    } catch (e) {
      row.state = "failed";
      row.error = e instanceof Error ? e.message : "The download failed.";
    }
    vlens.scheduleRedraw();
  }
  state.running = false;
  vlens.scheduleRedraw();
}

async function downloadOne(row: Row) {
  try {
    const url = await freshLink(row, false);
    const link = document.createElement("a");
    link.href = uploadUrl(url);
    link.download = row.video.file_name;
    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);
    row.state = "started";
  } catch (e) {
    row.state = "failed";
    row.error = e instanceof Error ? e.message : "The download failed.";
  }
  vlens.scheduleRedraw();
}

async function downloadAll(state: PageState) {
  if (state.running) return;
  state.running = true;
  for (const row of state.rows) {
    if (row.state === "started") continue;
    await downloadOne(row);
    await new Promise(r => setTimeout(r, 1500));
  }
  state.running = false;
  vlens.scheduleRedraw();
}

function downloadList(data: server.ListVideoOriginalsResponse) {
  const manifest = {
    note: "Links expire at expiresAt. Open this page again for fresh ones.",
    expiresAt: data.expiresAt,
    videos: data.videos,
  };
  const blob = new Blob([JSON.stringify(manifest, null, 2)], { type: "application/json" });
  const url = URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = url;
  link.download = `family-videos-${new Date().toISOString().split("T")[0]}.json`;
  link.click();
  URL.revokeObjectURL(url);
}

function rowStatus(row: Row): string {
  switch (row.state) {
    case "pending":
      return "";
    case "saving":
      return `Saving ${Math.floor(row.progress * 100)}%`;
    case "saved":
      return "Saved";
    case "skipped":
      return "Already in the folder";
    case "started":
      return "Download started";
    case "failed":
      return row.error || "Failed";
  }
}

const SaveVideosPage = ({
  state,
  data,
}: {
  state: PageState;
  data: server.ListVideoOriginalsResponse;
}) => {
  const canPickFolder = directoryPicker() !== null;
  const touched = state.rows.some(r => r.state !== "pending");
  return (
    <div className="settings-page">
      <div className="settings-header">
        <h1>Save all videos</h1>
        <p>
          {data.videos.length === 0
            ? "This family has no videos to save."
            : `${data.videos.length} original video${data.videos.length === 1 ? "" : "s"}, ${formatBytes(data.totalBytes)} in all`}
        </p>
      </div>
      <div className="settings-card">
        <p className="section-description">
          These are the files exactly as they were uploaded. The family export's data.json lists
          them under the same file names. <a href="/settings">Back to settings</a>
        </p>
        {data.videos.length > 0 && (
          <div className="save-videos-actions">
            {canPickFolder ? (
              <button
                className="btn btn-primary"
                disabled={state.running}
                onClick={() => saveAllToFolder(state)}
              >
                <Icon symbol={FolderDown} />{" "}
                {touched ? "Continue saving" : "Choose a folder and save all"}
              </button>
            ) : (
              <button
                className="btn btn-primary"
                disabled={state.running}
                onClick={() => downloadAll(state)}
              >
                <Icon symbol={Download} /> Download all
              </button>
            )}
            <button className="btn btn-secondary" onClick={() => downloadList(data)}>
              Download the list with links
            </button>
          </div>
        )}
        {canPickFolder && data.videos.length > 0 && (
          <p className="save-videos-hint">
            Saving into a folder can be stopped and continued: files already there at full size are
            skipped.
          </p>
        )}
        {touched && <p role="status">{saveSummary(state.rows.map(r => r.state))}</p>}
        <ul className="save-videos-list">
          {state.rows.map(row => (
            <li key={row.video.id} className={`save-video save-video-${row.state}`}>
              <span className="save-video-name">
                <a href={`/view-photo/${row.video.id}`}>{row.video.title || row.video.file_name}</a>
                <small>
                  {row.video.file_name} · {formatBytes(row.video.file_size)}
                  {row.video.duration_ms > 0 && ` · ${formatDuration(row.video.duration_ms)}`}
                </small>
              </span>
              <span className="save-video-status">{rowStatus(row)}</span>
              {(row.state === "failed" || (!canPickFolder && row.state === "pending")) && (
                <button
                  className="btn btn-outline btn-small"
                  disabled={state.running}
                  onClick={() => downloadOne(row)}
                >
                  {row.state === "failed" ? <Icon symbol={RotateCcw} size={14} /> : null}
                  {row.state === "failed" ? " Retry" : "Download"}
                </button>
              )}
            </li>
          ))}
        </ul>
      </div>
    </div>
  );
};
