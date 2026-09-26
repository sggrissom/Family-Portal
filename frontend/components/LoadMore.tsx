import * as preact from "preact";
import "./load-more-styles";

const callbacks = new WeakMap<Element, () => void>();
let observer: IntersectionObserver | null = null;

function watch(el: Element | null, onVisible: () => void) {
  if (!el || typeof IntersectionObserver === "undefined") return;
  if (!observer) {
    observer = new IntersectionObserver(
      entries => {
        for (const entry of entries) {
          if (entry.isIntersecting) callbacks.get(entry.target)?.();
        }
      },
      { rootMargin: "600px" }
    );
  }
  // Observing afresh on every render reports the sentinel's current state, so
  // a page too short to push it off screen goes on to load the next one.
  callbacks.set(el, onVisible);
  observer.unobserve(el);
  observer.observe(el);
}

// Loads the next page when scrolled near, and stays a button for anyone who
// gets there first or whose browser has no IntersectionObserver.
export const LoadMore = ({
  loading,
  onLoad,
  label = "Load more",
}: {
  loading: boolean;
  onLoad: () => void;
  label?: string;
}): preact.ComponentChild => (
  <div className="load-more" ref={el => watch(el, () => !loading && onLoad())}>
    <button type="button" className="btn btn-secondary" disabled={loading} onClick={onLoad}>
      {loading ? "Loading..." : label}
    </button>
  </div>
);
