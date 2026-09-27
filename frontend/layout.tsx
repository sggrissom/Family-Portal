import * as preact from "preact";
import * as vlens from "vlens";
import * as auth from "./lib/authCache";
import { applyPageMetadata } from "./lib/pageMetadata";
import { VerifyEmailBanner } from "./components/VerifyEmailBanner";
import { AddSheet, BottomNav, TopNav } from "./components/AppNav";
import { Ref } from "vlens/refs";

type HeaderData = {
  isMenuOpen: boolean;
};

const useHeader = vlens.declareHook((): HeaderData => {
  const stored = localStorage.getItem("theme") as "light" | "dark" | null;
  const defaultTheme = window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light";
  document.documentElement.setAttribute("data-theme", stored ?? defaultTheme);

  return {
    isMenuOpen: false,
  };
});

let lastMetadataPath = "";

function syncPageMetadata() {
  const path = window.location.pathname;
  if (path === lastMetadataPath) {
    return;
  }
  lastMetadataPath = path;
  applyPageMetadata(path);
}

const Brand = ({ href }: { href: string }) => (
  <a className="brand" href={href}>
    <svg className="brand-mark" viewBox="132 82 248 348" aria-hidden="true">
      <g stroke="currentColor" stroke-width="44" stroke-linecap="round" fill="none">
        <path d="M156 106 V406" />
        <path d="M156 166 H356" />
        <path d="M156 286 H286" />
      </g>
    </svg>
    Family Record
  </a>
);

export const Header = ({ isHome }: { isHome: boolean }) => {
  syncPageMetadata();
  const headerData = useHeader();
  const currentAuth = auth.getAuth();

  if (currentAuth && currentAuth.id > 0) {
    return (
      <>
        <header className="site-header">
          <a className="skip-link" href="#app" onClick={skipToContent}>
            Skip to main content
          </a>
          <nav className="nav app-topbar" aria-label="Main navigation">
            <Brand href="/dashboard" />
            <TopNav user={currentAuth} />
          </nav>
          <VerifyEmailBanner />
        </header>
        <BottomNav />
        <AddSheet />
      </>
    );
  }

  const menuRef = vlens.ref(headerData, "isMenuOpen");
  const navLinksClass = ["nav-links", "nav-links-guest", vlens.refGet(menuRef) ? "" : "hidden"]
    .filter(Boolean)
    .join(" ");

  return (
    <header className="site-header">
      <a className="skip-link" href="#app" onClick={skipToContent}>
        Skip to main content
      </a>
      <nav className="nav" aria-label="Main navigation">
        <Brand href="/" />
        <button
          className={vlens.refGet(menuRef) ? "nav-toggle open" : "nav-toggle"}
          id="navToggle"
          aria-label="Toggle menu"
          aria-expanded={vlens.refGet(menuRef) ? "true" : "false"}
          aria-controls="navLinks"
          onClick={vlens.cachePartial(menuClicked, menuRef)}
        >
          <span>Menu</span>
          <span className="nav-toggle-icon" aria-hidden="true">
            {vlens.refGet(menuRef) ? "×" : "☰"}
          </span>
        </button>
        <ul className={navLinksClass} id="navLinks">
          <li>
            <a href="/" className={isHome ? "active" : ""}>
              Home
            </a>
          </li>
          <li>
            <a href="/login">Log in</a>
          </li>
          <li>
            <a href="/create-account">Sign up</a>
          </li>
        </ul>
      </nav>
    </header>
  );
};

export const Footer = () => (
  <footer className="site-footer">
    <nav className="footer-links" aria-label="Policies and support">
      <a href="/privacy">Privacy</a>
      <a href="/terms">Terms</a>
      <a href="/support">Support</a>
    </nav>
    <p>© {new Date().getFullYear()} Family Record. All rights reserved.</p>
  </footer>
);

const skipToContent = (event: Event) => {
  event.preventDefault();
  const main = document.getElementById("app");
  if (!main) return;
  main.tabIndex = -1;
  main.focus();
  main.scrollIntoView();
};

let detachMenuDismissal: (() => void) | null = null;

const menuClicked = (menuRef: Ref) => {
  const isOpen = !vlens.refGet(menuRef);

  detachMenuDismissal?.();
  detachMenuDismissal = null;

  if (isOpen) {
    const close = () => {
      detachMenuDismissal?.();
      detachMenuDismissal = null;
      vlens.refSet(menuRef, false);
      vlens.scheduleRedraw();
    };

    const handleClickOutside = (event: MouseEvent) => {
      const nav = document.querySelector(".nav");
      if (event.target instanceof Node && nav && !nav.contains(event.target)) {
        close();
      }
    };

    const handleEscape = (event: KeyboardEvent) => {
      if (event.key !== "Escape") return;
      close();
      document.getElementById("navToggle")?.focus();
    };

    document.addEventListener("mousedown", handleClickOutside);
    document.addEventListener("keydown", handleEscape);
    detachMenuDismissal = () => {
      document.removeEventListener("mousedown", handleClickOutside);
      document.removeEventListener("keydown", handleEscape);
    };
  }

  vlens.refSet(menuRef, isOpen);
  vlens.scheduleRedraw();
};
