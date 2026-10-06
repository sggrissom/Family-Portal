import { block } from "vlens/css";

block(`
:root {
  --bottom-bar-space: 0px;
}
`);

block(`
@media (max-width: 719px) {
  :root {
    --bottom-bar-space: calc(64px + env(safe-area-inset-bottom, 0px));
  }

  body:has(.app-bottombar) {
    padding-bottom: var(--bottom-bar-space);
  }
}
`);

block(`
.app-topbar {
  justify-content: flex-start;
}
`);

block(`
.app-topbar-links {
  list-style: none;
  display: flex;
  gap: 2px;
  margin: 0 0 0 16px;
  padding: 0;
}
`);

block(`
.app-topbar-links a,
.app-topbar-links button {
  display: block;
  border: none;
  background: transparent;
  font: inherit;
  line-height: inherit;
  white-space: nowrap;
  cursor: pointer;
  padding: 8px 12px;
  border-radius: 8px;
  color: var(--muted);
  text-decoration: none;
  font-weight: 600;
}
`);

block(`
.app-topbar-links a:hover,
.app-topbar-links button:hover {
  background: var(--hover-bg);
  color: var(--text);
}
`);

block(`
.app-topbar-links a.active,
.app-topbar-links button.active {
  background: var(--accent-soft);
  color: var(--text);
}
`);

block(`
@media (min-width: 720px) and (max-width: 959px) {
  .app-topbar .brand-name {
    display: none;
  }

  .app-topbar-links {
    margin-left: 4px;
  }

  .app-topbar-links a,
  .app-topbar-links button {
    padding: 8px;
  }

  .app-add-button {
    padding: 8px 10px;
  }
}
`);

block(`
.app-topbar-caret {
  font-size: 0.75em;
  line-height: 1;
}
`);

block(`
.app-topbar-actions {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-left: auto;
}
`);

block(`
.app-add-button {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  min-height: 40px;
  padding: 8px 14px;
  border: none;
  border-radius: 10px;
  background: var(--primary-accent);
  color: var(--button-text);
  font: inherit;
  font-weight: 700;
  cursor: pointer;
}
`);

block(`
.app-add-button:hover {
  background: var(--primary-accent-hover);
}
`);

block(`
.account {
  position: relative;
}
`);

block(`
.account-toggle {
  display: grid;
  place-items: center;
  width: 40px;
  height: 40px;
  border-radius: 50%;
  border: 1px solid var(--border);
  background: var(--accent-soft);
  color: var(--text);
  font: inherit;
  font-weight: 700;
  cursor: pointer;
}
`);

block(`
.account-toggle:hover,
.account-toggle.open {
  border-color: var(--accent);
}
`);

block(`
.account-menu {
  position: absolute;
  top: calc(100% + 8px);
  right: 0;
  z-index: 20;
  width: 240px;
  max-width: calc(100vw - 24px);
  padding: 8px;
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 14px;
  box-shadow: 0 18px 50px rgba(15, 23, 42, 0.18);
}
`);

block(`
.account-menu-who {
  padding: 6px 10px 10px;
  border-bottom: 1px solid var(--border);
  margin-bottom: 6px;
}
`);

block(`
.account-menu-who strong {
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
`);

block(`
.account-menu-eyebrow {
  display: block;
  color: var(--muted);
  font-size: 0.72rem;
  font-weight: 700;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}
`);

block(`
.account-menu ul {
  list-style: none;
  margin: 0;
  padding: 0;
}
`);

block(`
.account-menu a,
.account-menu-logout {
  display: flex;
  align-items: center;
  justify-content: space-between;
  width: 100%;
  padding: 10px;
  border: none;
  border-radius: 8px;
  background: transparent;
  color: var(--text);
  font: inherit;
  text-align: left;
  text-decoration: none;
  cursor: pointer;
}
`);

block(`
.account-menu a:hover,
.account-menu-logout:hover {
  background: var(--hover-bg);
}
`);

block(`
.account-menu-logout {
  margin-top: 6px;
  border-top: 1px solid var(--border);
  border-radius: 0 0 8px 8px;
  color: var(--danger);
}
`);

block(`
.count-badge {
  min-width: 22px;
  padding: 1px 7px;
  border-radius: 999px;
  background: var(--primary-accent);
  color: var(--button-text);
  font-size: 0.78rem;
  font-weight: 700;
  text-align: center;
}
`);

block(`
.app-bottombar {
  display: none;
}
`);

block(`
@media (max-width: 719px) {
  .app-topbar-links,
  .app-add-button {
    display: none;
  }

  .app-bottombar {
    position: fixed;
    left: 0;
    right: 0;
    bottom: 0;
    z-index: 30;
    display: grid;
    grid-template-columns: repeat(5, 1fr);
    align-items: center;
    height: var(--bottom-bar-space);
    padding-bottom: env(safe-area-inset-bottom, 0px);
    background: var(--surface);
    border-top: 1px solid var(--border);
  }
}
`);

block(`
.app-bottombar-item {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 2px;
  padding: 6px 0;
  border: none;
  background: transparent;
  color: var(--muted);
  font: inherit;
  font-size: 0.72rem;
  font-weight: 600;
  text-decoration: none;
  cursor: pointer;
}
`);

block(`
.app-bottombar-item.active {
  color: var(--text);
}
`);

block(`
.app-bottombar-icon {
  font-size: 1.25rem;
  line-height: 1;
  filter: grayscale(1);
  opacity: 0.7;
}
`);

block(`
.app-bottombar-item.active .app-bottombar-icon {
  filter: none;
  opacity: 1;
}
`);

block(`
.app-bottombar-add {
  justify-self: center;
  display: grid;
  place-items: center;
  width: 52px;
  height: 52px;
  border: none;
  border-radius: 50%;
  background: var(--primary-accent);
  color: var(--button-text);
  font-size: 1.9rem;
  line-height: 1;
  cursor: pointer;
  box-shadow: 0 4px 14px rgba(15, 23, 42, 0.25);
}
`);

block(`
.add-sheet-backdrop {
  position: fixed;
  inset: 0;
  z-index: 50;
  display: flex;
  align-items: flex-start;
  justify-content: center;
  padding: 80px 16px 16px;
  background: rgba(15, 23, 42, 0.45);
}
`);

block(`
.add-sheet {
  width: min(460px, 100%);
  max-height: calc(100vh - 96px);
  overflow-y: auto;
  padding: 16px;
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 16px;
  box-shadow: 0 18px 50px rgba(15, 23, 42, 0.25);
}
`);

block(`
@media (max-width: 719px) {
  .add-sheet-backdrop {
    align-items: flex-end;
    padding: 0;
  }

  .add-sheet {
    width: 100%;
    max-height: 85vh;
    border-radius: 18px 18px 0 0;
    padding-bottom: calc(16px + env(safe-area-inset-bottom, 0px));
  }
}
`);

block(`
.add-sheet-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 12px;
}
`);

block(`
.add-sheet-header h2 {
  margin: 0;
  font-size: 1.15rem;
}
`);

block(`
.add-sheet-close {
  width: 36px;
  height: 36px;
  border: none;
  border-radius: 50%;
  background: var(--hover-bg);
  color: var(--text);
  font-size: 1.3rem;
  line-height: 1;
  cursor: pointer;
}
`);

block(`
.add-sheet-options {
  display: grid;
  gap: 6px;
}
`);

block(`
.add-sheet-options a,
.add-sheet-options button {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px;
  border: 1px solid var(--border);
  border-radius: 12px;
  background: transparent;
  color: var(--text);
  font: inherit;
  font-weight: 600;
  text-align: left;
  text-decoration: none;
  cursor: pointer;
}
`);

block(`
.add-sheet-options a:hover,
.add-sheet-options button:hover {
  border-color: var(--accent);
  background: var(--hover-bg);
}
`);

block(`
.add-sheet-icon {
  display: grid;
  place-items: center;
  width: 40px;
  height: 40px;
  border-radius: 10px;
  background: var(--bg);
  font-size: 1.2rem;
}
`);

block(`
.add-sheet-when {
  margin-left: auto;
  color: var(--muted);
  font-size: 0.85rem;
  font-weight: 500;
}
`);

block(`
.more-sheet-label {
  display: flex;
  flex-direction: column;
  gap: 2px;
}
`);

block(`
.more-sheet-label span {
  color: var(--muted);
  font-size: 0.85rem;
  font-weight: 500;
}
`);

block(`
.add-sheet-options a[aria-current="page"] {
  border-color: var(--accent);
  background: var(--accent-soft);
}
`);

block(`
@media (min-width: 720px) {
  .more-sheet-mobile-only {
    display: none !important;
  }
}
`);
