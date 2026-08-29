import * as vlens from "vlens";
import * as core from "vlens/core";

const FOCUSABLE = [
  "a[href]",
  "button:not([disabled])",
  "input:not([disabled]):not([type='hidden'])",
  "select:not([disabled])",
  "textarea:not([disabled])",
  "[tabindex]:not([tabindex='-1'])",
].join(",");

function focusableWithin(root: HTMLElement): HTMLElement[] {
  return Array.from(root.querySelectorAll<HTMLElement>(FOCUSABLE)).filter(
    el => el.getClientRects().length > 0
  );
}

export interface ModalDialogState {
  previouslyFocused: HTMLElement | null;
  previousOverflow: string;
  onDismiss: () => void;
}

export function newModalDialog(): ModalDialogState {
  return {
    previouslyFocused: null,
    previousOverflow: "",
    onDismiss: () => {},
  };
}

export function attrsModalDialog(state: ModalDialogState): any {
  return {
    "listen-create": true,
    oncreate: vlens.cachePartial(dialogCreated, state),
    onKeyDown: vlens.cachePartial(dialogKeyDown, state),
  };
}

let openDialog: ModalDialogState | null = null;
core.registerCleanupFunction(() => {
  if (openDialog) {
    closeModalDialog(openDialog);
  }
});

export function closeModalDialog(state: ModalDialogState) {
  if (openDialog === state) {
    openDialog = null;
  }
  document.body.style.overflow = state.previousOverflow;

  const previous = state.previouslyFocused;
  state.previouslyFocused = null;
  if (previous && document.contains(previous)) {
    previous.focus();
  }
}

function dialogCreated(state: ModalDialogState, event: CustomEvent) {
  const dialog = event.currentTarget as HTMLElement;

  state.previouslyFocused = document.activeElement as HTMLElement | null;

  state.previousOverflow = document.body.style.overflow;
  document.body.style.overflow = "hidden";
  openDialog = state;

  const initial = focusableWithin(dialog)[0];
  if (initial) {
    initial.focus();
  } else {
    dialog.tabIndex = -1;
    dialog.focus();
  }
}

function dialogKeyDown(state: ModalDialogState, event: KeyboardEvent) {
  if (event.key === "Escape") {
    event.stopPropagation();
    state.onDismiss();
    return;
  }
  if (event.key !== "Tab") return;

  const dialog = event.currentTarget as HTMLElement;
  const focusable = focusableWithin(dialog);
  if (focusable.length === 0) {
    event.preventDefault();
    return;
  }
  const first = focusable[0];
  const last = focusable[focusable.length - 1];
  const active = document.activeElement;

  if (event.shiftKey && (active === first || !dialog.contains(active))) {
    event.preventDefault();
    last.focus();
  } else if (!event.shiftKey && active === last) {
    event.preventDefault();
    first.focus();
  }
}
