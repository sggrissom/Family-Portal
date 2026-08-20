import * as vlens from "vlens";
import * as core from "vlens/core";

/** Every element that can hold focus inside a dialog, in document order. */
const FOCUSABLE = [
  "a[href]",
  "button:not([disabled])",
  "input:not([disabled]):not([type='hidden'])",
  "select:not([disabled])",
  "textarea:not([disabled])",
  "[tabindex]:not([tabindex='-1'])",
].join(",");

function focusableWithin(root: HTMLElement): HTMLElement[] {
  // getClientRects rather than offsetParent: a fixed-position element reports a
  // null offsetParent even when it is plainly on screen.
  return Array.from(root.querySelectorAll<HTMLElement>(FOCUSABLE)).filter(
    el => el.getClientRects().length > 0
  );
}

/**
 * What a modal dialog has to remember between opening and closing: the page
 * state it took over, so closing can put it back.
 */
export interface ModalDialogState {
  previouslyFocused: HTMLElement | null;
  previousOverflow: string;
  /**
   * Called when the user presses Escape. Read at event time so a redraw with a
   * fresh closure does not need the listeners torn down and put back.
   */
  onDismiss: () => void;
}

export function newModalDialog(): ModalDialogState {
  return {
    previouslyFocused: null,
    previousOverflow: "",
    onDismiss: () => {},
  };
}

/**
 * Wires the keyboard behaviour a modal dialog owes its user: focus moves into
 * the dialog on open, Tab cycles inside it rather than escaping to the page
 * behind, and Escape dismisses. Spread the result onto the dialog's outermost
 * element, and call closeModalDialog when it goes away.
 */
export function attrsModalDialog(state: ModalDialogState): any {
  return {
    // vlens dispatches "create" once the element lands in the document, which
    // is the only moment we get to take focus — there is no mount callback.
    "listen-create": true,
    oncreate: vlens.cachePartial(dialogCreated, state),
    onKeyDown: vlens.cachePartial(dialogKeyDown, state),
  };
}

// Navigating away takes the dialog off the page without running any of its
// close paths, so the scroll lock it installed has to be released here as well
// — otherwise the page it returns to cannot be scrolled.
let openDialog: ModalDialogState | null = null;
core.registerCleanupFunction(() => {
  if (openDialog) {
    closeModalDialog(openDialog);
  }
});

/** Hands the page back what the dialog took: scrolling, and the focus position. */
export function closeModalDialog(state: ModalDialogState) {
  if (openDialog === state) {
    openDialog = null;
  }
  document.body.style.overflow = state.previousOverflow;

  const previous = state.previouslyFocused;
  state.previouslyFocused = null;
  // The element that opened the dialog can be gone by the time it closes.
  if (previous && document.contains(previous)) {
    previous.focus();
  }
}

function dialogCreated(state: ModalDialogState, event: CustomEvent) {
  const dialog = event.currentTarget as HTMLElement;

  state.previouslyFocused = document.activeElement as HTMLElement | null;

  // Keep the page behind the dialog fixed, particularly while manipulating
  // controls inside it on touch devices.
  state.previousOverflow = document.body.style.overflow;
  document.body.style.overflow = "hidden";
  openDialog = state;

  const initial = focusableWithin(dialog)[0];
  if (initial) {
    initial.focus();
  } else {
    // Nothing focusable yet — make the dialog itself the focus target so the
    // reading position is inside it rather than back at the top of the page.
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
