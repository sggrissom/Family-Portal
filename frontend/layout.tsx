import * as preact from "preact";
import * as vlens from "vlens";
import * as auth from "./authCache";
import { Ref } from "vlens/refs";

type HeaderData = {
  theme: "light" | "dark";
  isMenuOpen: boolean;
};

const useHeader = vlens.declareHook((): HeaderData => {
  const stored = localStorage.getItem("theme") as HeaderData["theme"] | null;
  const defaultTheme = window.matchMedia("(prefers-color-scheme: dark)").matches
    ? "dark"
    : "light";

  const themeValue: HeaderData["theme"] = stored ?? defaultTheme;

  document.documentElement.setAttribute("data-theme", themeValue);
  localStorage.setItem("theme", themeValue);

  return {
    theme: themeValue,
    isMenuOpen: false,
  };
});

export const Header = ({ isHome }: { isHome: boolean }) => {
  const headerData = useHeader();
  const themeRef = vlens.ref(headerData, "theme");
  const menuRef = vlens.ref(headerData, "isMenuOpen");
  const currentAuth = auth.getAuth();
  const isAuthenticated = currentAuth && currentAuth.id > 0;

  return (
    <header className="site-header">
      <nav className="nav" aria-label="main">
        <a className="brand" href="/">
          Family Portal
        </a>
        <button
          className={vlens.refGet(menuRef) ? "nav-toggle open" : "nav-toggle"}
          id="navToggle"
          aria-label="Toggle navigation"
          aria-expanded={vlens.refGet(menuRef) ? "true" : "false"}
          aria-controls="navLinks"
          onClick={vlens.cachePartial(menuClicked, menuRef)}
        >
          Menu
        </button>
        <ul
          className={vlens.refGet(menuRef) ? "nav-links" : "nav-links hidden"}
          id="navLinks"
        >
          {isAuthenticated ? (
            <>
              <li className="user-info-container">
                <div className="user-info">
                  <span className="user-avatar">👤</span>
                  <span className="user-name">{currentAuth.name}</span>
                </div>
              </li>
            </>
          ) : ""}
          <li>
            <a href="/" className={isHome ? "active" : ""}>
              Home
            </a>
          </li>
          {isAuthenticated ? (
            <>
              <li>
                <button
                  onClick={logoutClicked}
                  className="logout-button"
                >
                  Logout
                </button>
              </li>
            </>
          ) : (
            <>
              <li>
                <a href="/login">Login</a>
              </li>
              <li>
                <a href="/create-account">Sign Up</a>
              </li>
            </>
          )}
          <li>
            <button
              onClick={vlens.cachePartial(themeToggleClicked, themeRef)}
              className="theme-switch"
            >
              {vlens.refGet(themeRef) === "dark" ? "☀️ Light" : "🌙 Dark"}
            </button>
          </li>
        </ul>
      </nav>
    </header>
  );
};

export const Footer = () => (
  <footer className="site-footer">
    <p>
      © <span id="year">2025</span> Family Portal. All rights reserved.
    </p>
  </footer>
);

const logoutClicked = async (event: Event) => {
  event.preventDefault();
  // Close mobile menu first if it's open
  const menuToggle = document.getElementById('navToggle');
  if (menuToggle && menuToggle.getAttribute('aria-expanded') === 'true') {
    menuToggle.click();
  }
  await auth.logout();
};

const themeToggleClicked = (themeRef: Ref) => {
  const html = document.documentElement;
  html.classList.add("theme-transition");

  const next = vlens.refGet(themeRef) === "dark" ? "light" : "dark";
  html.setAttribute("data-theme", next);
  localStorage.setItem("theme", next);
  vlens.refSet(themeRef, next);
  vlens.scheduleRedraw();

  window.setTimeout(() => {
    html.classList.remove("theme-transition");
  }, 600);
};

const menuClicked = (menuRef: Ref) => {
  const handleClickOutside = (event: MouseEvent) => {
    if (event.target instanceof HTMLElement) {
      if (
        event.target.tagName !== "A" &&
        !event.target.classList.contains("nav-toggle") &&
        !event.target.classList.contains("theme-switch") &&
        !event.target.classList.contains("logout-button")
      ) {
        document.removeEventListener("mousedown", handleClickOutside);
        vlens.refSet(menuRef, false);
        vlens.scheduleRedraw();
      }
    }
  };

  const isOpen = !vlens.refGet(menuRef);
  if (isOpen) {
    document.addEventListener("mousedown", handleClickOutside);
  } else {
    document.removeEventListener("mousedown", handleClickOutside);
  }
  vlens.refSet(menuRef, isOpen);
  vlens.scheduleRedraw();
};
