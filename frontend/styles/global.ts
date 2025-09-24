import { block } from "vlens/css";

block(`
:root {
  --transition-speed: 0.3s;
}
`);

block(`
[data-theme="dark"] {
  --bg: #0f1216;
  --surface: #161a20;
  --text: #e6edf3;
  --muted: #94a3b8;
  --accent: #69db7c;
  --primary-accent: #38d9a9;
  --button-text: #0b141a;
  --border: #263041;
  --hero: #c9d4e0;
  --hover-bg: rgba(255, 255, 255, 0.1);
  --active-bg: rgba(255, 255, 255, 0.1);
  --active-text: #fff;
}
`);

block(`
html.theme-transition *,
html.theme-transition *::before,
html.theme-transition *::after {
  transition:
    background-color var(--transition-speed) ease,
    color            var(--transition-speed) ease,
    border-color     var(--transition-speed) ease,
    background-image var(--transition-speed) ease,
    filter           var(--transition-speed) ease,
    backdrop-filter  var(--transition-speed) ease !important;
}
`);

block(`
.btn-primary {
  transition:
    background-image var(--transition-speed) ease,
    filter var(--transition-speed) ease;
}
`);

block(`
@media (prefers-reduced-motion: reduce) {
  *, *::before, *::after {
    transition: none !important;
  }
}
`);

block(`
* {
  box-sizing: border-box;
}
`);

block(`
html,
body {
  height: 100%;
}
`);

block(`
body {
  margin: 0;
  font-family:
    ui-sans-serif,
    system-ui,
    -apple-system,
    Segoe UI,
    Roboto,
    Helvetica,
    Arial,
    "Apple Color Emoji",
    "Segoe UI Emoji";
  background: var(--bg);
  color: var(--text);
  font-size: var(--fs-base);
  -webkit-font-smoothing: antialiased;
  -moz-osx-font-smoothing: grayscale;
  text-rendering: optimizeLegibility;
}
`);

block(`
img {
  max-width: 100%;
  height: auto;
  display: block;
}
`);

block(`
.site-header {
  position: sticky;
  top: 0;
  z-index: 10;
  background: var(--surface);
  backdrop-filter: blur(8px);
  border-bottom: 1px solid var(--border);
}
`);

block(`
.nav {
  max-width: 1000px;
  margin: 0 auto;
  padding: 12px 16px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}
`);

block(`
.brand {
  color: var(--text);
  text-decoration: none;
  font-weight: 700;
  letter-spacing: 0.3px;
  white-space: nowrap;
}
`);

block(`
.nav-links {
  list-style: none;
  display: flex;
  gap: 10px;
  margin: 0;
  padding: 0;
  align-items: center;
}
`);

block(`
.nav-links a, .nav-links button {
  color: var(--text);
  text-decoration: none;
  padding: 10px 12px;
  border-radius: 8px;
  transition:
    background var(--transition-speed) ease,
    color var(--transition-speed) ease;
}
`);

block(`
.nav-links a.active {
  background-color: var(--active-bg);
  color: var(--active-text);
  font-weight: bold;
  pointer-events: none;
  user-select: none;
  cursor: default;
  border-radius: 8px;
}
`);

block(`
.nav-links a:hover, .nav-links button:hover {
  background: var(--hover-bg);
  color: var(--muted);
}
`);

block(`
.muted {
  background: var(--card-bg);
  color: var(--muted);
}
`);

block(`
.nav-toggle {
  display: none;
  background: transparent;
  border: 1px solid var(--border);
  color: var(--text);
  padding: 8px 10px;
  border-radius: 8px;
}
`);

block(`
.app {
  max-width: 1000px;
  padding: 22px 16px 72px;
  margin: 0 auto;
}
`);

block(`
.hero {
  display: grid;
  grid-template-columns: 170px 1fr;
  gap: 20px;
  align-items: center;
  padding: 22px 18px;
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 16px;
}
`);

block(`
.hero img {
  width: 170px;
  height: 170px;
  border-radius: 16px;
  object-fit: cover;
  border: 1px solid var(--border);
}
`);

block(`
.hero h1 {
  margin: 0 0 6px;
  font-size: var(--fs-1);
}
`);

block(`
.hero h2 {
  margin: 0 0 12px;
  font-size: var(--fs-2);
  font-weight: 500;
  color: var(--muted);
}
`);

block(`
.hero p {
  margin: 0 0 10px;
  line-height: 1.6;
  color: var(--hero);
}
`);

block(`
.cta-row {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-top: 14px;
  flex-wrap: wrap;
}
`);

block(`
.btn {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  padding: 12px 14px;
  border-radius: 10px;
  text-decoration: none;
  color: var(--text);
  border: 1px solid var(--border);
  background: var(--bg);
  transition:
    transform 0.06s ease,
    background var(--transition-speed) ease;
  min-height: 44px; /* touch target */
}
`);

block(`
.btn:hover {
  background: var(--hover-bg);
  transform: none;
}
`);

block(`
.btn-primary {
  background: linear-gradient(90deg, var(--accent), var(--primary-accent));
  color: var(--button-text);
  border: none;
  font-weight: 700;
  transition: filter var(--transition-speed) ease;
}
`);

block(`
.btn-primary:hover {
  background: linear-gradient(
    90deg,
    var(--accent-hover),
    var(--primary-accent-hover)
  );
  filter: brightness(1.02);
  transform: none;
}
`);

block(`
.section {
  margin-top: 22px;
  padding: 18px;
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 16px;
}
`);

block(`
.section h3 {
  margin: 0 0 12px;
  font-size: var(--fs-2);
  letter-spacing: 0.2px;
}
`);

block(`
.kv {
  display: grid;
  grid-template-columns: 160px 1fr;
  gap: 8px 14px;
}
`);

block(`
.kv div {
  padding: 8px 0;
  border-bottom: 1px dotted var(--border);
  color: var(--hero);
}
`);

block(`
.kv div:nth-child(odd) {
  color: var(--muted);
}
`);

block(`
.resume h1 {
  margin: 0 0 4px;
  font-size: 24px;
}
`);

block(`
.resume h2 {
  margin: 0 0 16px;
  font-size: 16px;
  color: var(--muted);
  font-weight: 500;
}
`);

block(`
.resume .meta {
  display: flex;
  flex-wrap: wrap;
  gap: 10px 16px;
  color: var(--hero);
  margin-bottom: 12px;
}
`);

block(`
.resume .download {
  margin-left: auto;
}
`);

block(`
.resume h3 {
  margin: 18px 0 8px;
  font-size: var(--fs-2);
}
`);

block(`
.resume ul {
  margin: 8px 0 0 18px;
  line-height: 1.6;
  color: var(--hero);
}
`);

block(`
.badges {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-top: 8px;
}
`);

block(`
.badge {
  background: var(--card-bg);
  border: 1px solid var(--border);
  color: var(--text);
  padding: 6px 10px;
  border-radius: 999px;
  font-size: 12px;
}
`);

block(`
.split {
  display: grid;
  grid-template-columns: 1fr;
  gap: 12px;
}
`);

block(`
@media (min-width: 840px) {
  .split {
    grid-template-columns: 1.3fr 0.8fr;
    gap: 16px;
  }
}
`);

block(`
.card {
  margin-top: 6px;
  padding: 16px;
  background: var(--card-bg);
  border: 1px solid var(--border);
  border-radius: 12px;
}
`);

block(`
@media (max-width: 640px) {
  .nav {
    padding: 10px 12px;
  }

  .nav-links {
    position: absolute;
    top: 56px;
    right: 12px;
    left: 12px;
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: 12px;
    padding: 8px;
    flex-direction: column;
    gap: 8px;
  }

  .hidden {
    display: none;
  }

  .nav-links a {
    display: block;
    width: 100%;
    padding: 10px 12px;
    border-radius: 8px;
    text-decoration: none;
    color: inherit;
  }

  .nav-links.open {
    display: flex;
  }

  .nav-toggle {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    gap: 6px;
    min-height: 40px;
    padding: 8px 12px;
  }

  .brand {
    font-size: 16px;
  }

  .hero {
    grid-template-columns: 1fr;
    padding: 16px;
    gap: 14px;
    text-align: left;
  }

  .hero img {
    width: 120px;
    height: 120px;
    border-radius: 12px;
  }

  .hero h1 {
    font-size: 22px;
  }

  .hero h2 {
    font-size: 15px;
  }

  .app {
    padding: 18px 12px 64px;
  }

  .section {
    padding: 14px;
    border-radius: 12px;
  }

  .kv {
    grid-template-columns: 1fr;
    gap: 0;
  }

  .kv div {
    border-bottom: 1px dotted var(--border);
    padding: 10px 0;
  }

  .resume .meta {
    gap: 8px 12px;
  }

  .btn {
    width: 100%;
    justify-content: center;
  }
}
`);

block(`
.site-footer {
  border-top: 1px solid var(--border);
  color: var(--muted);
  text-align: center;
  padding: 18px;
  background: var(--surface);
}
`);

// Shared Form Styles
block(`
.form-group {
  display: flex;
  flex-direction: column;
  gap: 6px;
}
`);

block(`
.form-group label {
  font-weight: 600;
  color: var(--text);
  font-size: 0.9rem;
}
`);

block(`
.form-group input {
  padding: 12px 16px;
  border: 1px solid var(--border);
  border-radius: 8px;
  background: var(--bg);
  color: var(--text);
  font-size: 1rem;
  transition: border-color var(--transition-speed) ease;
}
`);

block(`
.form-group input:focus {
  outline: none;
  border-color: var(--accent);
  box-shadow: 0 0 0 3px rgba(16, 185, 129, 0.1);
}
`);

block(`
.form-group input::placeholder {
  color: var(--muted);
}
`);

block(`
.form-group select {
  padding: 12px 16px;
  border: 1px solid var(--border);
  border-radius: 8px;
  background: var(--bg);
  color: var(--text);
  font-size: 1rem;
  transition: border-color var(--transition-speed) ease;
  width: 100%;
}
`);

block(`
.form-group select:focus {
  outline: none;
  border-color: var(--accent);
  box-shadow: 0 0 0 3px rgba(16, 185, 129, 0.1);
}
`);

block(`
.form-hint {
  color: var(--muted);
  font-size: 0.85rem;
  margin-top: 4px;
}
`);

block(`
.form-group input:disabled,
input:disabled,
select:disabled,
textarea:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}
`);

// Shared Message Styles
block(`
.error-message {
  background: #fee2e2;
  border: 1px solid #fecaca;
  color: #dc2626;
  padding: 12px 16px;
  border-radius: 8px;
  margin-bottom: 20px;
  font-size: 0.9rem;
}
`);

block(`
[data-theme="dark"] .error-message {
  background: #450a0a;
  border-color: #7f1d1d;
  color: #fca5a5;
}
`);

block(`
.success-message {
  background: #d1fae5;
  border: 1px solid #a7f3d0;
  color: #065f46;
  padding: 12px 16px;
  border-radius: 8px;
  margin-bottom: 20px;
  font-size: 0.9rem;
}
`);

block(`
[data-theme="dark"] .success-message {
  background: #064e3b;
  border-color: #047857;
  color: #6ee7b7;
}
`);

// Shared Utility Classes
block(`
.empty-state {
  text-align: center;
  padding: 60px 20px;
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 16px;
}
`);

block(`
.empty-state p {
  font-size: 1.1rem;
  color: var(--muted);
  margin: 0 0 20px;
}
`);

// Shared Button Variants
block(`
.btn-secondary {
  background: transparent;
  color: var(--text);
  border: 2px solid var(--border);
}
`);

block(`
.btn-secondary:hover {
  background: var(--surface);
  border-color: var(--accent);
  transform: translateY(-1px);
}
`);

// Radio Button and Checkbox Shared Styles
block(`
.radio-group {
  display: flex;
  gap: 20px;
  margin-top: 8px;
}
`);

block(`
.radio-option {
  display: flex;
  align-items: center;
  gap: 8px;
  cursor: pointer;
  font-size: 1rem;
  color: var(--text);
}
`);

block(`
.radio-option input[type="radio"] {
  width: 18px;
  height: 18px;
  accent-color: var(--accent);
}
`);

block(`
.radio-option span {
  user-select: none;
}
`);

// Form Layout Utilities
block(`
.form-row {
  display: flex;
  gap: 16px;
  align-items: end;
}
`);

block(`
.form-row .form-group {
  margin: 0;
}
`);

block(`
.form-row .flex-1 {
  flex: 1;
}
`);

block(`
.form-row .flex-2 {
  flex: 2;
}
`);

// Mobile responsive styles for shared components
block(`
@media (max-width: 580px) {
  .radio-group {
    flex-direction: column;
    gap: 12px;
  }

  .form-row {
    flex-direction: column;
    gap: 12px;
  }

  .form-row .form-group {
    width: 100%;
  }
}
`);




// Header authentication styles
block(`
.user-info-container {
  padding: 0;
}
`);

block(`
.user-info {
  display: flex;
  align-items: center;
  gap: 8px;
  color: var(--text) !important;
  font-weight: 600;
  padding: 8px 12px;
  border-radius: 20px;
  background: linear-gradient(90deg, var(--accent), var(--primary-accent));
  color: var(--button-text) !important;
  pointer-events: none;
  user-select: none;
  font-size: 0.9rem;
  box-shadow: 0 2px 8px rgba(105, 219, 124, 0.2);
  margin: 4px 0;
}
`);

block(`
.user-avatar {
  font-size: 1rem;
}
`);

block(`
.user-name {
  color: var(--button-text) !important;
  font-weight: 600;
}
`);

block(`
.logout-button {
  background: transparent;
  border: 1px solid var(--border);
  color: var(--text);
  padding: 8px 12px;
  border-radius: 8px;
  cursor: pointer;
  transition: all var(--transition-speed) ease;
}
`);

block(`
.logout-button:hover {
  background: var(--hover-bg);
  border-color: #dc2626;
  color: #dc2626;
}
`);

block(`
.nav-links .logout-button {
  padding: 10px 12px;
  width: auto;
}
`);

// Mobile-specific user info styling
block(`
@media (max-width: 640px) {
  .user-info-container {
    order: -1;
    margin-bottom: 8px;
  }

  .user-info {
    justify-content: center;
    margin: 0 0 8px 0;
    padding: 10px 16px;
  }

  .logout-button {
    width: 100%;
    justify-content: center;
    margin-top: 4px;
  }
}
`);



