import { useEffect, useRef } from "preact/hooks";

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
 * Wires the keyboard behaviour a modal dialog owes its user: focus moves into
 * the dialog on open, Tab cycles inside it rather than escaping to the page
 * behind, Escape dismisses, and whatever was focused before the dialog opened
 * gets focus back on close.
 *
 * Returns the ref to attach to the dialog's outermost element.
 */
export function useModalDialog(onDismiss: () => void) {
  const dialogRef = useRef<HTMLDivElement>(null);
  // The handler is read at event time, so a re-render with a new closure does
  // not need to tear the listeners down and put them back.
  const dismissRef = useRef(onDismiss);
  dismissRef.current = onDismiss;

  useEffect(() => {
    const dialog = dialogRef.current;
    if (!dialog) return;

    const previouslyFocused = document.activeElement as HTMLElement | null;

    const initial = focusableWithin(dialog)[0];
    if (initial) {
      initial.focus();
    } else {
      // Nothing focusable yet — make the dialog itself the focus target so the
      // reading position is inside it rather than back at the top of the page.
      dialog.tabIndex = -1;
      dialog.focus();
    }

    const onKeyDown = (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        e.stopPropagation();
        dismissRef.current();
        return;
      }
      if (e.key !== "Tab") return;

      const focusable = focusableWithin(dialog);
      if (focusable.length === 0) {
        e.preventDefault();
        return;
      }
      const first = focusable[0];
      const last = focusable[focusable.length - 1];
      const active = document.activeElement;

      if (e.shiftKey && (active === first || !dialog.contains(active))) {
        e.preventDefault();
        last.focus();
      } else if (!e.shiftKey && active === last) {
        e.preventDefault();
        first.focus();
      }
    };

    dialog.addEventListener("keydown", onKeyDown);
    return () => {
      dialog.removeEventListener("keydown", onKeyDown);
      // The element that opened the dialog can be gone by the time it closes.
      if (previouslyFocused && document.contains(previouslyFocused)) {
        previouslyFocused.focus();
      }
    };
  }, []);

  return dialogRef;
}
