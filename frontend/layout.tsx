import * as preact from "preact";
import * as vlens from "vlens";
import * as auth from "./lib/authCache";
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

export const Header = ({ isHome }: { isHome: boolean }) => {
  const headerData = useHeader();
  const menuRef = vlens.ref(headerData, "isMenuOpen");
  const currentAuth = auth.getAuth();
  const isAuthenticated = currentAuth && currentAuth.id > 0;

  return (
    <header className="site-header">
      <nav className="nav" aria-label="Main navigation">
        <a className="brand" href={isAuthenticated ? "/dashboard" : "/"}>
          Family Portal
        </a>
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
        <ul className={vlens.refGet(menuRef) ? "nav-links" : "nav-links hidden"} id="navLinks">
          {isAuthenticated ? (
            <>
              <li className="menu-account">
                <span className="user-avatar" aria-hidden="true">
                  👤
                </span>
                <span>
                  <span className="menu-eyebrow">Signed in as</span>
                  <strong className="user-name">{currentAuth.name}</strong>
                </span>
              </li>
              <li className="menu-section" aria-label="Quick add">
                <span className="menu-section-title">Quick add</span>
                <div className="menu-action-grid">
                  <a href="/add-milestone" className="menu-action menu-action-featured">
                    <span className="menu-action-icon" aria-hidden="true">
                      ⭐
                    </span>
                    <span>
                      <strong>Milestone</strong>
                      <small>Capture a special moment</small>
                    </span>
                  </a>
                  <a href="/add-growth" className="menu-action">
                    <span className="menu-action-icon" aria-hidden="true">
                      📏
                    </span>
                    <span>
                      <strong>Growth</strong>
                      <small>Record height or weight</small>
                    </span>
                  </a>
                  <a href="/add-photo" className="menu-action">
                    <span className="menu-action-icon" aria-hidden="true">
                      📷
                    </span>
                    <span>
                      <strong>Photo</strong>
                      <small>Save a family memory</small>
                    </span>
                  </a>
                </div>
              </li>
              <li className="menu-section" aria-label="Go to">
                <span className="menu-section-title">Go to</span>
                <div className="menu-destination-grid">
                  <a href="/dashboard">🏠 Dashboard</a>
                  <a href="/family-timeline">📅 Timeline</a>
                  <a href="/photos">🖼️ Photos</a>
                  <a href="/family-chart">📈 Growth chart</a>
                  <a href="/chat">💬 Family chat</a>
                  <a href="/settings">⚙️ Settings</a>
                </div>
              </li>
              <li className="menu-footer">
                {currentAuth.isAdmin ? (
                  <a href="/admin" className="admin-link">
                    <span aria-hidden="true">⚡</span> Admin
                  </a>
                ) : (
                  <span />
                )}
                <button onClick={logoutClicked} className="logout-button">
                  Log out
                </button>
              </li>
            </>
          ) : (
            <>
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
            </>
          )}
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
  const menuToggle = document.getElementById("navToggle");
  if (menuToggle && menuToggle.getAttribute("aria-expanded") === "true") {
    menuToggle.click();
  }
  await auth.logout();
};

const menuClicked = (menuRef: Ref) => {
  const handleClickOutside = (event: MouseEvent) => {
    const nav = document.querySelector(".nav");
    if (event.target instanceof Node && nav && !nav.contains(event.target)) {
      document.removeEventListener("mousedown", handleClickOutside);
      vlens.refSet(menuRef, false);
      vlens.scheduleRedraw();
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
