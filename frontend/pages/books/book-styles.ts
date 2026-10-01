import { block } from "vlens/css";

block(`
.book-page {
  --paper: #fbf8f2;
  --ink: #2c2721;
  --ink-soft: #7b7064;
  --rule: #e6ddcf;
  --mat: #ffffff;
  --book-serif: "Iowan Old Style", "Palatino Linotype", Palatino, "Book Antiqua", Georgia, serif;
  min-height: 100%;
  background: var(--paper);
  color: var(--ink);
}
`);

block(`
[data-theme="dark"] .book-page {
  --paper: #17181b;
  --ink: #ebe5da;
  --ink-soft: #a59c8e;
  --rule: #2e3036;
  --mat: #22242a;
}
`);

block(`
.book-proto-bar {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: center;
  gap: 10px;
  padding: 10px 16px;
  border-bottom: 1px dashed var(--rule);
  background: var(--bg);
}
`);

block(`
.book-proto-tag {
  font-size: 0.75rem;
  font-weight: 700;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  color: var(--warning);
}
`);

block(`
.book {
  max-width: 680px;
  margin: 0 auto;
  padding: 0 16px 64px;
  font-family: var(--book-serif);
  font-size: 1.125rem;
  line-height: 1.65;
}
`);

block(`
.book h1,
.book h2 {
  font-family: var(--book-serif);
  font-weight: 500;
  letter-spacing: 0.01em;
}
`);

block(`
.book-cover {
  min-height: calc(100vh - 120px);
  display: flex;
  flex-direction: column;
  justify-content: center;
  gap: 28px;
  padding: 32px 0 48px;
  text-align: center;
}
`);

block(`
.book-cover-photo {
  width: min(100%, 560px);
  margin: 0 auto;
  border-radius: 4px;
  box-shadow: 0 18px 50px rgba(40, 30, 20, 0.18);
  max-height: 62vh;
}
`);

block(`
.book-cover-text h1 {
  margin: 0;
  font-size: clamp(2.2rem, 7vw, 3.4rem);
  line-height: 1.1;
}
`);

block(`
.book-cover-text p {
  margin: 10px 0 0;
  color: var(--ink-soft);
  font-style: italic;
}
`);

block(`
.book-contents {
  margin: 0 auto 32px;
  padding: 28px 0;
  border-top: 1px solid var(--rule);
  border-bottom: 1px solid var(--rule);
}
`);

block(`
.book-contents h2 {
  margin: 0 0 12px;
  font-size: 0.8rem;
  text-transform: uppercase;
  letter-spacing: 0.18em;
  color: var(--ink-soft);
  text-align: center;
}
`);

block(`
.book-contents ol {
  list-style: none;
  margin: 0;
  padding: 0;
}
`);

block(`
.book-contents button {
  display: flex;
  justify-content: space-between;
  align-items: baseline;
  gap: 12px;
  width: 100%;
  padding: 8px 4px;
  border: 0;
  border-bottom: 1px dotted var(--rule);
  background: none;
  color: inherit;
  font: inherit;
  text-align: left;
  cursor: pointer;
}
`);

block(`
.book-contents li:last-child button {
  border-bottom: 0;
}
`);

block(`
.book-contents-dates {
  color: var(--ink-soft);
  font-size: 0.85em;
  white-space: nowrap;
}
`);

block(`
.book-chapter {
  padding-top: 56px;
  scroll-margin-top: 16px;
}
`);

block(`
.book-chapter-head {
  margin-bottom: 28px;
  text-align: center;
}
`);

block(`
.book-chapter-head h2 {
  margin: 0;
  font-size: clamp(1.8rem, 5vw, 2.4rem);
  line-height: 1.15;
}
`);

block(`
.book-chapter-dates {
  margin: 0 0 6px;
  font-size: 0.78rem;
  text-transform: uppercase;
  letter-spacing: 0.16em;
  color: var(--ink-soft);
}
`);

block(`
.book .book-chapter > * + * {
  margin-top: 36px;
}
`);

block(`
.book-image {
  position: relative;
  overflow: hidden;
  border-radius: 3px;
  background: var(--rule);
}
`);

block(`
.book-image picture,
.book-image img {
  display: block;
  width: 100%;
  height: 100%;
  object-fit: cover;
}
`);

block(`
.book-placeholder {
  background:
    radial-gradient(circle at 30% 35%, hsl(var(--hue) 55% 88% / 0.9), transparent 55%),
    linear-gradient(135deg, hsl(var(--hue) 40% 74%), hsl(calc(var(--hue) + 50) 35% 58%));
}
`);

block(`
[data-theme="dark"] .book-placeholder {
  filter: brightness(0.7) saturate(0.8);
}
`);

block(`
.book-figure {
  margin: 0;
}
`);

block(`
.book-figure figcaption {
  display: flex;
  flex-wrap: wrap;
  justify-content: space-between;
  gap: 2px 12px;
  margin-top: 8px;
  font-size: 0.85rem;
  line-height: 1.4;
}
`);

block(`
.book-caption {
  font-style: italic;
}
`);

block(`
.book-caption-when {
  color: var(--ink-soft);
  font-family: ui-sans-serif, system-ui, sans-serif;
  font-size: 0.75rem;
}
`);

block(`
.book-hero {
  width: min(100vw - 32px, 960px);
  margin-left: 50%;
  transform: translateX(-50%);
}
`);

block(`
.book-hero .book-image {
  width: 100%;
  max-height: 80vh;
  aspect-ratio: 3 / 2 !important;
}
`);

block(`
.book-photos {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px;
  align-items: start;
}
`);

block(`
.book-photos-1 {
  grid-template-columns: 1fr;
  width: 78%;
  margin-left: auto;
  margin-right: auto;
}
`);

block(`
.book-photos-3 > :first-child {
  grid-column: 1 / -1;
}
`);

block(`
.book-photos .book-image {
  max-height: 520px;
}
`);

block(`
.book-photos-2 .book-image,
.book-photos-4 .book-image,
.book-photos-3 > :not(:first-child) .book-image {
  aspect-ratio: 4 / 5 !important;
}
`);

block(`
.book-moment {
  margin-left: auto;
  margin-right: auto;
}
`);

block(`
.book-moment.has-photo {
  display: grid;
  gap: 20px;
}
`);

block(`
.book-moment-words {
  margin: 0;
  font-size: 1.3rem;
  line-height: 1.5;
}
`);

block(`
.book-moment:not(.has-photo) .book-moment-text {
  padding: 8px 0 8px 22px;
  border-left: 3px solid var(--rule);
}
`);

block(`
.book-moment-label {
  margin: 0 0 6px;
  font-family: ui-sans-serif, system-ui, sans-serif;
  font-size: 0.7rem;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.16em;
  color: var(--ink-soft);
}
`);

block(`
.book-moment-context {
  margin: 8px 0 0;
  color: var(--ink-soft);
}
`);

block(`
.book-when {
  margin: 8px 0 0;
  font-family: ui-sans-serif, system-ui, sans-serif;
  font-size: 0.75rem;
  color: var(--ink-soft);
}
`);

block(`
.book-quote {
  margin: 0 auto;
  padding: 12px 0;
  text-align: center;
}
`);

block(`
.book-quote > p:first-child {
  margin: 0;
  font-size: clamp(2rem, 7vw, 2.8rem);
  font-style: italic;
  line-height: 1.2;
}
`);

block(`
.book-quote > p:first-child::before {
  content: "“";
}
`);

block(`
.book-quote > p:first-child::after {
  content: "”";
}
`);

block(`
.book-quote-context {
  max-width: 30em;
  margin: 12px auto 0;
  color: var(--ink-soft);
}
`);

block(`
.book-artwork {
  margin: 0 auto;
  max-width: 520px;
  text-align: center;
}
`);

block(`
.book-artwork-mat {
  padding: 22px;
  background: var(--mat);
  box-shadow:
    0 1px 2px rgba(0, 0, 0, 0.08),
    0 10px 30px rgba(40, 30, 20, 0.12);
}
`);

block(`
.book-artwork figcaption {
  display: grid;
  gap: 4px;
  margin-top: 16px;
}
`);

block(`
.book-notes {
  list-style: none;
  margin: 0;
  padding: 0;
  display: grid;
  gap: 18px;
}
`);

block(`
.book-notes li {
  display: grid;
  grid-template-columns: 9.5em 1fr;
  gap: 16px;
  align-items: baseline;
}
`);

block(`
.book-notes .book-when {
  margin: 0;
  text-align: right;
}
`);

block(`
.book-notes p {
  margin: 0;
}
`);

block(`
.book-facts {
  list-style: none;
  margin: 0;
  padding: 0;
  text-align: center;
  font-variant: small-caps;
  letter-spacing: 0.05em;
  color: var(--ink-soft);
}
`);

block(`
.book-letter {
  margin: 0 auto;
  max-width: 34em;
  font-style: italic;
  font-size: 1.2rem;
}
`);

block(`
.book-letter p {
  margin: 0;
}
`);

block(`
.book-letter-signature {
  margin-top: 14px !important;
  text-align: right;
}
`);

block(`
.book-growth {
  display: grid;
  gap: 28px;
}
`);

block(`
.book-growth-chart {
  margin: 0;
}
`);

block(`
.book-growth-chart svg {
  display: block;
  width: 100%;
  height: auto;
  overflow: visible;
}
`);

block(`
.book-growth-chart figcaption {
  display: flex;
  flex-wrap: wrap;
  justify-content: space-between;
  gap: 4px 12px;
  margin-top: 6px;
  font-size: 0.9rem;
}
`);

block(`
.book-growth-chart figcaption span {
  color: var(--ink-soft);
}
`);

block(`
.book-growth-axis {
  stroke: var(--rule);
  stroke-width: 1;
}
`);

block(`
.book-growth-label {
  fill: var(--ink);
  font-family: ui-sans-serif, system-ui, sans-serif;
  font-size: 11px;
}
`);

block(`
.book-growth-tick {
  fill: var(--ink-soft);
  font-family: ui-sans-serif, system-ui, sans-serif;
  font-size: 10px;
}
`);

block(`
.book-end {
  margin-top: 72px;
  text-align: center;
  color: var(--ink-soft);
  font-style: italic;
}
`);

block(`
.book-end span {
  font-size: 1.6rem;
}
`);

block(`
.book-editor-notes {
  max-width: 680px;
  margin: 0 auto 48px;
  padding: 12px 16px;
  border: 1px dashed var(--rule);
  border-radius: 8px;
  font-size: 0.9rem;
  color: var(--ink-soft);
}
`);

block(`
.book-editor-notes summary {
  cursor: pointer;
}
`);

block(`
.book-missing {
  max-width: 680px;
  margin: 0 auto;
  padding: 40px 16px;
}
`);

block(`
@media (min-width: 720px) {
  .book-moment.has-photo {
    grid-template-columns: 1.1fr 1fr;
    align-items: center;
    width: calc(100% + 120px);
    margin-left: -60px;
  }

  .book-chapter > .book-moment.has-photo:nth-of-type(even) .book-moment-photo {
    order: 2;
  }

  .book-photos:not(.book-photos-1) {
    width: calc(100% + 120px);
    margin-left: -60px;
  }

  .book-growth {
    grid-template-columns: 1fr 1fr;
  }
}
`);

block(`
@media (max-width: 480px) {
  .book {
    font-size: 1.05rem;
  }

  .book-notes li {
    grid-template-columns: 1fr;
    gap: 2px;
  }

  .book-notes .book-when {
    text-align: left;
  }

  .book-photos-1 {
    width: 100%;
  }
}
`);

block(`
.books-index {
  max-width: 680px;
  margin: 0 auto;
  padding: 24px 16px 48px;
}
`);

block(`
.books-index h1 {
  margin: 0 0 8px;
}
`);

block(`
.books-index h2 {
  margin: 28px 0 10px;
  font-size: 1.1rem;
}
`);

block(`
.books-lede,
.books-empty {
  color: var(--muted);
}
`);

block(`
.books-list {
  list-style: none;
  margin: 0;
  padding: 0;
  display: grid;
  gap: 10px;
}
`);

block(`
.books-list a,
.books-list button {
  display: grid;
  width: 100%;
  font: inherit;
  text-align: left;
  cursor: pointer;
  gap: 2px;
  padding: 14px 16px;
  border: 1px solid var(--border);
  border-radius: 10px;
  background: var(--surface);
  color: var(--text);
  text-decoration: none;
}
`);

block(`
.books-list a:hover,
.books-list button:hover:not(:disabled) {
  border-color: var(--accent);
}
`);

block(`
.books-list span {
  color: var(--muted);
  font-size: 0.9rem;
}
`);

block(`
.books-list button:disabled {
  cursor: progress;
  opacity: 0.7;
}
`);

block(`
.books-shelf {
  list-style: none;
  margin: 20px 0 8px;
  padding: 0;
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(150px, 1fr));
  gap: 16px;
}
`);

block(`
.books-shelf a {
  display: grid;
  gap: 4px;
  color: var(--text);
  text-decoration: none;
}
`);

block(`
.books-shelf-cover {
  aspect-ratio: 3 / 4;
  overflow: hidden;
  border-radius: 4px 10px 10px 4px;
  background: linear-gradient(135deg, var(--accent-soft, var(--surface)), var(--border));
  box-shadow: inset 6px 0 0 rgba(0, 0, 0, 0.08), 0 6px 18px rgba(0, 0, 0, 0.12);
}
`);

block(`
.books-shelf-cover picture,
.books-shelf-cover img {
  display: block;
  width: 100%;
  height: 100%;
  object-fit: cover;
}
`);

block(`
.books-shelf span {
  color: var(--muted);
  font-size: 0.85rem;
}
`);

block(`
.books-samples {
  margin-top: 36px;
  color: var(--muted);
  font-size: 0.9rem;
}
`);

block(`
.book-reader-bar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  max-width: 960px;
  margin: 0 auto;
  padding: 12px 16px;
  font-family: ui-sans-serif, system-ui, sans-serif;
  font-size: 0.9rem;
}
`);

block(`
.book-reader-bar a {
  color: var(--ink-soft);
}
`);
