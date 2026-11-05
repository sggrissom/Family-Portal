import * as preact from "preact";

const revealed = new WeakMap<HTMLElement, string>();

function revealOnChange(message: string) {
  return (el: HTMLElement | null) => {
    if (!el || revealed.get(el) === message) return;
    revealed.set(el, message);
    const reduce = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
    el.scrollIntoView({ behavior: reduce ? "auto" : "smooth", block: "center" });
  };
}

export const FormError = ({ message }: { message: string }) =>
  message ? (
    <div className="error-message" role="alert" ref={revealOnChange(message)}>
      {message}
    </div>
  ) : null;
