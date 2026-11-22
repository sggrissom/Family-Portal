import * as preact from "preact";
import * as vlens from "vlens";
import { Copy, Download, Link, Share2 } from "lucide-preact";
import { Icon } from "./Icon";
import {
  ModalDialogState,
  attrsModalDialog,
  closeModalDialog,
  newModalDialog,
} from "../lib/modalDialog";
import { RenderedShare } from "../lib/share/render";
import { copy } from "../lib/copy";
import "./share-dialog-styles";

export interface ShareDialogState {
  open: boolean;
  dialog: ModalDialogState;
  key: string;
  generation: number;
  rendered: RenderedShare | null;
  rendering: boolean;
  error: string;
  copied: "" | "text" | "link";
}

export function newShareDialog(): ShareDialogState {
  const state: ShareDialogState = {
    open: false,
    dialog: newModalDialog(),
    key: "",
    generation: 0,
    rendered: null,
    rendering: false,
    error: "",
    copied: "",
  };
  state.dialog.onDismiss = () => closeShareDialog(state);
  return state;
}

export function openShareDialog(state: ShareDialogState) {
  state.open = true;
  state.key = "";
  state.copied = "";
  vlens.scheduleRedraw();
}

function release(rendered: RenderedShare | null) {
  for (const url of rendered?.previews ?? []) URL.revokeObjectURL(url);
}

export function closeShareDialog(state: ShareDialogState) {
  if (!state.open) return;
  state.open = false;
  state.generation++;
  release(state.rendered);
  state.rendered = null;
  closeModalDialog(state.dialog);
  vlens.scheduleRedraw();
}

/** Renders the file again whenever what goes in it (`key`) changes while the dialog is open. */
export function syncShareRender(
  state: ShareDialogState,
  key: string,
  render: () => Promise<RenderedShare>
) {
  if (!state.open || state.key === key) return;
  state.key = key;
  const generation = ++state.generation;
  state.rendering = true;
  state.error = "";
  state.copied = "";
  render()
    .then(rendered => {
      if (generation !== state.generation) {
        release(rendered);
        return;
      }
      release(state.rendered);
      state.rendered = rendered;
    })
    .catch(() => {
      if (generation === state.generation) {
        release(state.rendered);
        state.rendered = null;
        state.error = copy.share.renderFailed;
      }
    })
    .finally(() => {
      if (generation === state.generation) state.rendering = false;
      vlens.scheduleRedraw();
    });
}

function canShareFile(file: File): boolean {
  return typeof navigator.canShare === "function" && navigator.canShare({ files: [file] });
}

async function shareFile(state: ShareDialogState) {
  const file = state.rendered?.file;
  if (!file) return;
  try {
    await navigator.share({ files: [file] });
  } catch (err) {
    if ((err as DOMException)?.name !== "AbortError") download(state);
  }
}

function download(state: ShareDialogState) {
  const file = state.rendered?.file;
  if (!file) return;
  const url = URL.createObjectURL(file);
  const link = document.createElement("a");
  link.href = url;
  link.download = file.name;
  document.body.appendChild(link);
  link.click();
  link.remove();
  setTimeout(() => URL.revokeObjectURL(url), 1000);
}

async function copyToClipboard(state: ShareDialogState, what: "text" | "link", value: string) {
  try {
    await navigator.clipboard.writeText(value);
    state.copied = what;
  } catch {
    state.error = copy.share.renderFailed;
  }
  vlens.scheduleRedraw();
}

function backdropClicked(state: ShareDialogState, event: MouseEvent) {
  if (event.target === event.currentTarget) closeShareDialog(state);
}

export interface ShareDialogProps {
  state: ShareDialogState;
  text: string;
  enabled: boolean;
  link?: string | null;
  notes?: preact.ComponentChild;
  children?: preact.ComponentChildren;
}

export const ShareDialog = ({ state, text, enabled, link, notes, children }: ShareDialogProps) => {
  if (!state.open) return null;
  const rendered = enabled ? state.rendered : null;
  const isPdf = rendered?.kind === "pdf";

  return (
    <div className="share-dialog-backdrop" onClick={vlens.cachePartial(backdropClicked, state)}>
      <div
        className="share-dialog"
        role="dialog"
        aria-modal="true"
        aria-labelledby="shareDialogTitle"
        {...attrsModalDialog(state.dialog)}
      >
        <div className="share-dialog-head">
          <h2 id="shareDialogTitle">{copy.share.previewTitle}</h2>
          <button
            type="button"
            className="share-dialog-close"
            aria-label={copy.share.close}
            onClick={vlens.cachePartial(closeShareDialog, state)}
          >
            ×
          </button>
        </div>

        <div className="share-dialog-preview" aria-busy={state.rendering}>
          {!enabled ? (
            <p className="share-dialog-note">{copy.share.nothingIncluded}</p>
          ) : rendered ? (
            rendered.previews.map((url, i) => (
              <img key={url} src={url} alt={isPdf ? `Page ${i + 1}` : copy.share.previewTitle} />
            ))
          ) : (
            <p className="share-dialog-note">{state.error || copy.share.rendering}</p>
          )}
        </div>
        {isPdf && rendered && (
          <p className="share-dialog-note">{copy.share.pages(rendered.previews.length)}</p>
        )}
        {notes}

        {children && <div className="share-dialog-options">{children}</div>}

        <p className="share-dialog-note">{copy.share.forwarding}</p>

        <div className="share-dialog-actions">
          {rendered && canShareFile(rendered.file) && (
            <button
              type="button"
              className="btn btn-primary"
              onClick={vlens.cachePartial(shareFile, state)}
            >
              <Icon symbol={Share2} /> {isPdf ? copy.share.sharePDF : copy.share.shareImage}
            </button>
          )}
          <button
            type="button"
            className="btn btn-secondary"
            disabled={!rendered}
            onClick={vlens.cachePartial(download, state)}
          >
            <Icon symbol={Download} /> {isPdf ? copy.share.downloadPDF : copy.share.downloadImage}
          </button>
          <button
            type="button"
            className="btn btn-secondary"
            disabled={!enabled}
            onClick={vlens.cachePartial(copyToClipboard, state, "text", text)}
          >
            <Icon symbol={Copy} />{" "}
            {state.copied === "text" ? copy.share.copied : copy.share.copyText}
          </button>
          {link && (
            <button
              type="button"
              className="btn btn-secondary"
              onClick={vlens.cachePartial(copyToClipboard, state, "link", link)}
            >
              <Icon symbol={Link} />{" "}
              {state.copied === "link" ? copy.share.copied : copy.share.copyLink}
            </button>
          )}
        </div>
        {link && <p className="share-dialog-note">{copy.share.linkNote}</p>}
      </div>
    </div>
  );
};

export const ShareToggle = ({
  label,
  checked,
  onToggle,
}: {
  label: string;
  checked: boolean;
  onToggle: () => void;
}) => (
  <label className="share-toggle">
    <input type="checkbox" checked={checked} onChange={onToggle} />
    <span>{label}</span>
  </label>
);
