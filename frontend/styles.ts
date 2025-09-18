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

// Landing Page Styles
block(`
.landing-container {
  max-width: 1200px;
  padding: 0;
  margin: 0 auto;
}
`);

block(`
.landing-page {
  width: 100%;
}
`);

block(`
.landing-hero {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 60px;
  align-items: center;
  padding: 80px 20px;
  min-height: 60vh;
  background: linear-gradient(135deg, var(--surface) 0%, var(--bg) 100%);
}
`);

block(`
.hero-content {
  max-width: 600px;
}
`);

block(`
.hero-title {
  font-size: clamp(2rem, 5vw, 3.5rem);
  font-weight: 800;
  margin: 0 0 20px;
  background: linear-gradient(135deg, var(--text) 0%, var(--accent) 100%);
  background-clip: text;
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  line-height: 1.2;
}
`);

block(`
.hero-subtitle {
  font-size: clamp(1.1rem, 2vw, 1.4rem);
  color: var(--muted);
  margin: 0 0 40px;
  line-height: 1.5;
}
`);

block(`
.hero-actions {
  display: flex;
  gap: 16px;
  flex-wrap: wrap;
}
`);

block(`
.btn-large {
  padding: 16px 32px;
  font-size: 1.1rem;
  font-weight: 600;
  border-radius: 12px;
  transition: all 0.3s ease;
  cursor: pointer;
  border: none;
  min-width: 140px;
}
`);

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
  transform: translateY(-2px);
}
`);

block(`
.btn-primary.btn-large:hover {
  transform: translateY(-2px);
  box-shadow: 0 10px 30px rgba(105, 219, 124, 0.3);
}
`);

block(`
.hero-visual {
  position: relative;
  height: 400px;
  display: flex;
  align-items: center;
  justify-content: center;
}
`);

block(`
.floating-card {
  position: absolute;
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 16px;
  padding: 24px;
  box-shadow: 0 10px 40px rgba(0, 0, 0, 0.1);
  transition: all 0.3s ease;
  display: flex;
  align-items: center;
  gap: 12px;
}
`);

block(`
.floating-card:hover {
  transform: translateY(-5px);
  box-shadow: 0 15px 50px rgba(0, 0, 0, 0.15);
}
`);

block(`
.card-1 {
  top: 20px;
  left: 20px;
  animation: float 6s ease-in-out infinite;
}
`);

block(`
.card-2 {
  top: 140px;
  right: 40px;
  animation: float 6s ease-in-out infinite 2s;
}
`);

block(`
.card-3 {
  bottom: 40px;
  left: 60px;
  animation: float 6s ease-in-out infinite 4s;
}
`);

block(`
@keyframes float {
  0%, 100% {
    transform: translateY(0px);
  }
  50% {
    transform: translateY(-20px);
  }
}
`);

block(`
.card-icon {
  font-size: 2rem;
}
`);

block(`
.card-text {
  font-weight: 600;
  color: var(--text);
}
`);

block(`
.features-section {
  padding: 80px 20px;
  background: var(--bg);
}
`);

block(`
.section-title {
  text-align: center;
  font-size: clamp(1.8rem, 4vw, 2.5rem);
  font-weight: 700;
  margin: 0 0 60px;
  color: var(--text);
}
`);

block(`
.features-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(250px, 1fr));
  gap: 30px;
  max-width: 1000px;
  margin: 0 auto;
}
`);

block(`
.feature-card {
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 16px;
  padding: 32px;
  text-align: center;
  transition: all 0.3s ease;
}
`);

block(`
.feature-card:hover {
  transform: translateY(-5px);
  border-color: var(--accent);
  box-shadow: 0 10px 40px rgba(0, 0, 0, 0.1);
}
`);

block(`
.feature-icon {
  font-size: 3rem;
  margin-bottom: 20px;
}
`);

block(`
.feature-card h3 {
  font-size: 1.3rem;
  margin: 0 0 12px;
  color: var(--text);
}
`);

block(`
.feature-card p {
  color: var(--muted);
  line-height: 1.6;
  margin: 0;
}
`);

block(`
.cta-section {
  padding: 100px 20px;
  background: linear-gradient(135deg, var(--surface) 0%, var(--bg) 100%);
  text-align: center;
}
`);

block(`
.cta-content h2 {
  font-size: clamp(1.8rem, 4vw, 2.8rem);
  margin: 0 0 16px;
  color: var(--text);
}
`);

block(`
.cta-content p {
  font-size: 1.2rem;
  color: var(--muted);
  margin: 0 0 40px;
}
`);

block(`
.cta-actions {
  display: flex;
  justify-content: center;
  gap: 20px;
}
`);

block(`
@media (max-width: 768px) {
  .landing-hero {
    grid-template-columns: 1fr;
    gap: 40px;
    padding: 60px 20px;
    text-align: center;
  }

  .hero-visual {
    display: none;
  }

  .hero-actions {
    justify-content: center;
  }

  .btn-large {
    width: 100%;
    max-width: 300px;
  }

  .features-grid {
    grid-template-columns: 1fr;
    gap: 20px;
  }

  .features-section {
    padding: 60px 20px;
  }

  .cta-section {
    padding: 60px 20px;
  }

  .cta-actions {
    flex-direction: column;
    align-items: center;
  }
}
`);

block(`
@media (max-width: 480px) {
  .hero-title {
    font-size: 2rem;
  }

  .hero-subtitle {
    font-size: 1rem;
  }

  .section-title {
    font-size: 1.5rem;
    margin: 0 0 40px;
  }

  .feature-card {
    padding: 24px;
  }

  .feature-icon {
    font-size: 2.5rem;
  }

  .cta-content h2 {
    font-size: 1.8rem;
  }

  .cta-content p {
    font-size: 1rem;
  }
}
`);

// Create Account Page Styles
block(`
.create-account-container {
  max-width: 480px;
  padding: 40px 20px;
  margin: 0 auto;
  min-height: calc(100vh - 200px);
  display: flex;
  align-items: center;
  justify-content: center;
}
`);

block(`
.create-account-page {
  width: 100%;
}
`);

block(`
.auth-card {
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 16px;
  padding: 40px;
  box-shadow: 0 4px 20px rgba(0, 0, 0, 0.08);
}
`);

block(`
.auth-header {
  text-align: center;
  margin-bottom: 32px;
}
`);

block(`
.auth-header h1 {
  font-size: 2rem;
  margin: 0 0 8px;
  color: var(--text);
}
`);

block(`
.auth-header p {
  color: var(--muted);
  margin: 0;
  font-size: 1rem;
}
`);

block(`
.auth-form {
  display: flex;
  flex-direction: column;
  gap: 20px;
}
`);

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
.form-hint {
  color: var(--muted);
  font-size: 0.85rem;
  margin-top: 4px;
}
`);

block(`
.auth-submit {
  margin-top: 8px;
  width: 100%;
  justify-content: center;
}
`);

block(`
.auth-footer {
  margin-top: 24px;
  text-align: center;
  padding-top: 24px;
  border-top: 1px solid var(--border);
}
`);

block(`
.auth-footer p {
  color: var(--muted);
  margin: 0;
}
`);

block(`
.auth-link {
  color: var(--accent);
  text-decoration: none;
  font-weight: 600;
  margin-left: 8px;
  transition: color var(--transition-speed) ease;
}
`);

block(`
.auth-link:hover {
  color: var(--primary-accent);
  text-decoration: underline;
}
`);

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
.form-group input:disabled,
.auth-submit:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}
`);

block(`
@media (max-width: 480px) {
  .create-account-container {
    padding: 20px 16px;
  }

  .auth-card {
    padding: 32px 24px;
  }

  .auth-header h1 {
    font-size: 1.8rem;
  }

  .form-group input {
    padding: 14px 16px;
  }
}
`);

// Login Page Styles
block(`
.login-container {
  max-width: 420px;
  padding: 40px 20px;
  margin: 0 auto;
  min-height: calc(100vh - 200px);
  display: flex;
  align-items: center;
  justify-content: center;
}
`);

block(`
.login-page {
  width: 100%;
}
`);

block(`
.auth-methods {
  display: flex;
  flex-direction: column;
  gap: 20px;
}
`);

block(`
.btn-google {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 12px;
  padding: 12px 24px;
  border: 1px solid var(--border);
  border-radius: 8px;
  background: var(--surface);
  color: var(--text);
  font-size: 1rem;
  font-weight: 500;
  cursor: pointer;
  transition: all var(--transition-speed) ease;
  min-height: 48px;
}
`);

block(`
.btn-google:hover {
  background: var(--hover-bg);
  border-color: var(--muted);
  transform: translateY(-1px);
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.1);
}
`);

block(`
.auth-divider {
  position: relative;
  text-align: center;
  margin: 8px 0;
}
`);

block(`
.auth-divider::before {
  content: '';
  position: absolute;
  top: 50%;
  left: 0;
  right: 0;
  height: 1px;
  background: var(--border);
}
`);

block(`
.auth-divider span {
  background: var(--surface);
  color: var(--muted);
  padding: 0 16px;
  font-size: 0.9rem;
  position: relative;
  z-index: 1;
}
`);

block(`
.form-options {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-top: 4px;
}
`);

block(`
.checkbox-label {
  display: flex;
  align-items: center;
  gap: 8px;
  cursor: pointer;
  font-size: 0.9rem;
}
`);

block(`
.checkbox-label input[type="checkbox"] {
  width: 16px;
  height: 16px;
  accent-color: var(--accent);
}
`);

block(`
.checkbox-text {
  color: var(--text);
  user-select: none;
}
`);

block(`
@media (max-width: 480px) {
  .login-container {
    padding: 20px 16px;
  }

  .form-options {
    flex-direction: column;
    gap: 12px;
    align-items: flex-start;
  }

  .btn-google {
    font-size: 0.95rem;
    padding: 14px 24px;
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

// Dashboard Page Styles
block(`
.dashboard-container {
  max-width: 1200px;
  padding: 40px 20px;
  margin: 0 auto;
}
`);

block(`
.dashboard-page {
  width: 100%;
}
`);

block(`
.dashboard-header {
  text-align: center;
  margin-bottom: 50px;
}
`);

block(`
.dashboard-header h1 {
  font-size: clamp(2rem, 4vw, 2.5rem);
  margin: 0 0 8px;
  color: var(--text);
}
`);

block(`
.dashboard-header p {
  font-size: 1.2rem;
  color: var(--muted);
  margin: 0;
}
`);

block(`
.dashboard-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
  gap: 24px;
  max-width: 800px;
  margin: 0 auto;
}
`);

block(`
.dashboard-card {
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 16px;
  padding: 32px;
  text-align: center;
  transition: all 0.3s ease;
}
`);

block(`
.dashboard-card:hover {
  transform: translateY(-4px);
  border-color: var(--accent);
  box-shadow: 0 8px 30px rgba(0, 0, 0, 0.1);
}
`);

block(`
.dashboard-card .card-icon {
  font-size: 3rem;
  margin-bottom: 16px;
  display: block;
}
`);

block(`
.dashboard-card h3 {
  font-size: 1.3rem;
  margin: 0 0 8px;
  color: var(--text);
}
`);

block(`
.dashboard-card p {
  color: var(--muted);
  margin: 0 0 20px;
  line-height: 1.5;
}
`);

block(`
@media (max-width: 768px) {
  .dashboard-container {
    padding: 30px 16px;
  }

  .dashboard-grid {
    grid-template-columns: 1fr;
    gap: 20px;
  }

  .dashboard-card {
    padding: 24px;
  }

  .dashboard-header {
    margin-bottom: 30px;
  }
}
`);

// Family Members Section Styles
block(`
.family-section {
  margin-bottom: 50px;
}
`);

block(`
.section-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 30px;
}
`);

block(`
.section-header h2 {
  font-size: 1.8rem;
  margin: 0;
  color: var(--text);
}
`);

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

block(`
.people-grid {
  display: flex;
  flex-direction: column;
  gap: 30px;
}
`);

block(`
.people-group h3 {
  font-size: 1.3rem;
  margin: 0 0 16px;
  color: var(--text);
  border-bottom: 2px solid var(--accent);
  padding-bottom: 8px;
  display: inline-block;
}
`);

block(`
.people-list {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(250px, 1fr));
  gap: 16px;
}
`);

block(`
.person-card {
  display: flex;
  align-items: center;
  gap: 16px;
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 12px;
  padding: 20px;
  transition: all 0.3s ease;
  cursor: pointer;
}
`);

block(`
.person-card:hover {
  transform: translateY(-2px);
  border-color: var(--accent);
  box-shadow: 0 4px 20px rgba(0, 0, 0, 0.1);
}
`);

block(`
.person-avatar {
  font-size: 2.5rem;
  display: flex;
  align-items: center;
  justify-content: center;
  width: 60px;
  height: 60px;
  background: var(--bg);
  border-radius: 50%;
  border: 2px solid var(--border);
}
`);

block(`
.person-info h4 {
  font-size: 1.1rem;
  margin: 0 0 4px;
  color: var(--text);
}
`);

block(`
.person-details {
  font-size: 0.9rem;
  color: var(--muted);
  margin: 0;
}
`);

// Add Person Page Styles
block(`
.add-person-container {
  max-width: 500px;
  padding: 40px 20px;
  margin: 0 auto;
  min-height: calc(100vh - 200px);
  display: flex;
  align-items: center;
  justify-content: center;
}
`);

block(`
.add-person-page {
  width: 100%;
}
`);

block(`
.form-actions {
  display: flex;
  gap: 16px;
  margin-top: 8px;
}
`);

block(`
.form-actions .btn {
  flex: 1;
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

// Mobile responsive styles for person management
block(`
@media (max-width: 768px) {
  .section-header {
    flex-direction: column;
    gap: 16px;
    align-items: stretch;
  }

  .people-list {
    grid-template-columns: 1fr;
  }

  .person-card {
    padding: 16px;
  }

  .person-avatar {
    width: 50px;
    height: 50px;
    font-size: 2rem;
  }

  .add-person-container {
    padding: 30px 16px;
  }

  .form-actions {
    flex-direction: column;
  }
}
`);

// Clickable person card styles
block(`
.person-card.clickable {
  text-decoration: none;
  color: inherit;
}
`);

block(`
.person-card.clickable:hover {
  text-decoration: none;
  color: inherit;
}
`);

// Profile Page Styles
block(`
.profile-container {
  max-width: 1200px;
  padding: 40px 20px;
  margin: 0 auto;
}
`);

block(`
.profile-page {
  width: 100%;
}
`);

block(`
.profile-header {
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 16px;
  padding: 32px;
  margin-bottom: 30px;
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 20px;
}
`);

block(`
.profile-header-main {
  display: flex;
  align-items: center;
  gap: 20px;
}
`);

block(`
.profile-avatar {
  font-size: 4rem;
  display: flex;
  align-items: center;
  justify-content: center;
  width: 80px;
  height: 80px;
  background: var(--bg);
  border-radius: 50%;
  border: 3px solid var(--border);
}
`);

block(`
.profile-info h1 {
  font-size: 2.2rem;
  margin: 0 0 8px;
  color: var(--text);
}
`);

block(`
.profile-details {
  font-size: 1.1rem;
  color: var(--muted);
  margin: 0;
}
`);

block(`
.profile-actions {
  display: flex;
  gap: 12px;
  flex-wrap: wrap;
}
`);

block(`
.profile-tabs {
  display: flex;
  gap: 4px;
  margin-bottom: 30px;
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 12px;
  padding: 6px;
}
`);

block(`
.tab {
  flex: 1;
  padding: 12px 20px;
  border: none;
  background: transparent;
  color: var(--muted);
  border-radius: 8px;
  cursor: pointer;
  transition: all 0.3s ease;
  font-size: 1rem;
  font-weight: 500;
}
`);

block(`
.tab:hover {
  background: var(--hover-bg);
  color: var(--text);
}
`);

block(`
.tab.active {
  background: var(--accent);
  color: var(--button-text);
  font-weight: 600;
}
`);

block(`
.profile-content {
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 16px;
  padding: 32px;
  min-height: 400px;
}
`);

block(`
.timeline-tab h2,
.growth-tab h2,
.photos-tab h2 {
  font-size: 1.5rem;
  margin: 0 0 24px;
  color: var(--text);
}
`);

block(`
.timeline-content,
.growth-content,
.photos-content {
  display: flex;
  flex-direction: column;
  gap: 20px;
}
`);

block(`
.growth-chart-placeholder {
  background: var(--bg);
  border: 1px solid var(--border);
  border-radius: 12px;
  padding: 24px;
  text-align: center;
}
`);

block(`
.growth-chart-placeholder h3 {
  margin: 0 0 16px;
  color: var(--text);
}
`);

block(`
.chart-placeholder {
  height: 200px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--surface);
  border-radius: 8px;
  color: var(--muted);
  font-size: 1.1rem;
}
`);

block(`
.growth-table h3 {
  margin: 0 0 16px;
  color: var(--text);
}
`);

block(`
.growth-records {
  background: var(--bg);
  border: 1px solid var(--border);
  border-radius: 12px;
  overflow: hidden;
}
`);

block(`
.data-table {
  width: 100%;
  border-collapse: collapse;
  background: transparent;
}
`);

block(`
.data-table th,
.data-table td {
  padding: 12px 16px;
  text-align: left;
  border-bottom: 1px solid var(--border);
}
`);

block(`
.data-table th {
  background: var(--surface);
  color: var(--muted);
  font-weight: 600;
  font-size: 0.9rem;
  text-transform: uppercase;
  letter-spacing: 0.05em;
}
`);

block(`
.data-table td {
  color: var(--text);
  font-size: 0.95rem;
}
`);

block(`
.data-table tbody tr:hover {
  background: var(--hover-bg);
}
`);

block(`
.data-table tbody tr:last-child td {
  border-bottom: none;
}
`);

block(`
.table-actions {
  padding: 16px;
  background: var(--surface);
  border-top: 1px solid var(--border);
  text-align: center;
}
`);

block(`
.photos-gallery {
  min-height: 300px;
  display: flex;
  align-items: center;
  justify-content: center;
}
`);

block(`
.profile-content .empty-state {
  background: var(--bg);
  border: 1px dashed var(--border);
  border-radius: 12px;
  padding: 40px 20px;
  text-align: center;
}
`);

block(`
.profile-content .empty-state p {
  font-size: 1rem;
  color: var(--muted);
  margin: 0 0 16px;
}
`);

block(`
.error-page {
  text-align: center;
  padding: 80px 20px;
}
`);

block(`
.error-page h1 {
  font-size: 2rem;
  margin: 0 0 16px;
  color: var(--text);
}
`);

block(`
.error-page p {
  font-size: 1.1rem;
  color: var(--muted);
  margin: 0 0 24px;
}
`);

// Mobile responsive styles for profile page
block(`
@media (max-width: 768px) {
  .profile-container {
    padding: 30px 16px;
  }

  .profile-header {
    flex-direction: column;
    text-align: center;
    gap: 24px;
  }

  .profile-header-main {
    flex-direction: column;
    text-align: center;
    gap: 16px;
  }

  .profile-avatar {
    width: 70px;
    height: 70px;
    font-size: 3rem;
  }

  .profile-info h1 {
    font-size: 1.8rem;
  }

  .profile-actions {
    justify-content: center;
    width: 100%;
  }

  .profile-actions .btn {
    flex: 1;
    min-width: 0;
    font-size: 0.9rem;
    padding: 10px 8px;
  }

  .profile-tabs {
    flex-direction: column;
    gap: 2px;
  }

  .tab {
    padding: 14px 20px;
    font-size: 0.95rem;
  }

  .profile-content {
    padding: 24px 20px;
  }

  .chart-placeholder {
    height: 150px;
    font-size: 1rem;
  }
}
`);

// Custom Error Page Styles
block(`
.error-container {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--bg);
  padding: 20px;
}
`);

block(`
.error-page {
  text-align: center;
  max-width: 500px;
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 16px;
  padding: 60px 40px;
}
`);

block(`
.error-page h1 {
  font-size: 2.2rem;
  margin: 0 0 20px;
  color: var(--text);
}
`);

block(`
.error-message {
  font-size: 1.1rem;
  color: var(--muted);
  margin: 0 0 30px;
  line-height: 1.5;
}
`);

block(`
.error-actions {
  display: flex;
  gap: 16px;
  justify-content: center;
  flex-wrap: wrap;
}
`);

block(`
.auth-failure-redirect {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--bg);
  font-size: 1.2rem;
  color: var(--muted);
}
`);

// Mobile responsive styles for error pages
block(`
@media (max-width: 480px) {
  .error-page {
    padding: 40px 24px;
  }

  .error-page h1 {
    font-size: 1.8rem;
  }

  .error-actions {
    flex-direction: column;
    gap: 12px;
  }

  .error-actions .btn {
    width: 100%;
  }
}
`);

// Add Growth Page Styles
block(`
.add-growth-container {
  max-width: 580px;
  padding: 40px 20px;
  margin: 0 auto;
  min-height: calc(100vh - 200px);
  display: flex;
  align-items: center;
  justify-content: center;
}
`);

block(`
.add-growth-page {
  width: 100%;
}
`);

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

block(`
.measurement-preview {
  margin-top: 24px;
  padding: 20px;
  background: var(--bg);
  border: 1px solid var(--border);
  border-radius: 12px;
}
`);

block(`
.measurement-preview h3 {
  margin: 0 0 12px;
  color: var(--text);
  font-size: 1.1rem;
}
`);

block(`
.measurement-preview p {
  margin: 0;
  color: var(--muted);
  line-height: 1.5;
}
`);

block(`
.measurement-preview strong {
  color: var(--text);
}
`);

// Mobile responsive styles for add growth page
block(`
@media (max-width: 580px) {
  .add-growth-container {
    padding: 30px 16px;
  }

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

  .measurement-preview {
    padding: 16px;
  }
}
`);
