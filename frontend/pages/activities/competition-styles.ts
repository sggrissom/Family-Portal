import { block } from "vlens/css";

block(`
.comp-hero {
  margin-bottom: 2rem;
}
`);

block(`
.comp-hero h1 {
  margin: 0.15rem 0 0.4rem;
}
`);

block(`
.comp-when {
  margin: 0;
  color: var(--text);
  font-weight: 600;
}
`);

block(`
.comp-where {
  margin: 0.15rem 0 0;
  color: var(--muted);
  font-size: 0.9rem;
}
`);

block(`
.comp-status {
  display: inline-block;
  margin-left: 0.6rem;
  padding: 0.1rem 0.5rem;
  border-radius: 999px;
  font-size: 0.7rem;
  letter-spacing: 0.04em;
  vertical-align: 1px;
}
`);

block(`
.comp-status-upcoming {
  background: var(--surface-alt);
  border: 1px solid var(--surface-alt-border);
  color: var(--text);
}
`);

block(`
.comp-status-now {
  background: var(--accent-soft);
  border: 1px solid var(--accent-soft-border);
  color: var(--text);
}
`);

block(`
.comp-progress {
  color: var(--muted);
  font-size: 0.85rem;
}
`);

block(`
.perf-cards {
  display: grid;
  gap: 0.85rem;
}
`);

block(`
.perf-card {
  display: grid;
  gap: 0.75rem;
  padding: 1rem;
  border: 1px solid var(--border);
  border-radius: 14px;
  background: var(--surface);
  min-width: 0;
}
`);

block(`
.perf-card-empty {
  border-style: dashed;
  background: transparent;
}
`);

block(`
.perf-card-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 1rem;
}
`);

block(`
.perf-card-title {
  display: flex;
  flex-direction: column;
  gap: 0.15rem;
  min-width: 0;
}
`);

block(`
.perf-card-title a {
  color: var(--text);
  font-weight: 700;
  font-size: 1.05rem;
  text-decoration: none;
}
`);

block(`
.perf-card-title a:hover {
  text-decoration: underline;
}
`);

block(`
.perf-card-meta {
  color: var(--muted);
  font-size: 0.85rem;
}
`);

block(`
.perf {
  display: grid;
  gap: 0.6rem;
  min-width: 0;
}
`);

block(`
.perf + .perf,
.perf + .perf-editor,
.perf-editor + .perf {
  border-top: 1px solid var(--border);
  padding-top: 0.75rem;
}
`);

block(`
.perf-top {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 1rem;
}
`);

block(`
.perf-top-main {
  display: grid;
  gap: 0.4rem;
  min-width: 0;
}
`);

block(`
.perf-editor {
  border-top: 1px solid var(--border);
  padding-top: 0.75rem;
  min-width: 0;
}
`);

block(`
.perf-editor textarea {
  width: 100%;
  padding: 0.6rem 0.75rem;
  border: 1px solid var(--border);
  border-radius: 6px;
  background: var(--bg);
  color: var(--text);
  font: inherit;
  resize: vertical;
}
`);

block(`
.perf-editor-extra {
  margin-bottom: 0.75rem;
}
`);

block(`
.perf-media-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
  margin: 0.5rem 0;
}
`);

block(`
.draft-media-item {
  position: relative;
}
`);

block(`
.draft-media-remove {
  position: absolute;
  top: 4px;
  right: 4px;
  display: flex;
  padding: 2px;
  border: none;
  border-radius: 999px;
  background: rgba(0, 0, 0, 0.6);
  color: #fff;
  cursor: pointer;
}
`);

block(`
.perf-editor-actions {
  align-items: center;
}
`);

block(`
.perf-delete {
  margin-left: auto;
  background: none;
  border: none;
  color: var(--danger);
  font: inherit;
  font-size: 0.85rem;
  cursor: pointer;
}
`);

block(`
.comp-nav {
  display: flex;
  justify-content: space-between;
  gap: 1rem;
  padding-top: 1.25rem;
  border-top: 1px solid var(--border);
}
`);

block(`
.comp-nav-link {
  display: flex;
  align-items: center;
  gap: 0.4rem;
  color: var(--text);
  text-decoration: none;
  font-weight: 600;
  min-width: 0;
}
`);

block(`
.comp-nav-link span {
  display: flex;
  flex-direction: column;
}
`);

block(`
.comp-nav-link small {
  color: var(--muted);
  font-weight: 400;
  font-size: 0.75rem;
}
`);

block(`
.comp-nav-next {
  text-align: right;
}
`);

block(`
.matchup-fields {
  border: 1px solid var(--border);
  border-radius: 8px;
  padding: 0.75rem 1rem 0;
  margin: 1rem 0;
  min-width: 0;
}
`);

block(`
.matchup-fields legend {
  font-weight: 600;
  font-size: 0.9rem;
  padding: 0 0.25rem;
}
`);

block(`
.matchup-fields .form-row {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(12rem, 1fr));
  align-items: start;
  gap: 0 1rem;
}
`);

block(`
.matchup-fields .form-group {
  margin-bottom: 0.75rem;
  min-width: 0;
}
`);

block(`
.matchup-score {
  display: flex;
  align-items: center;
  gap: 0.5rem;
}
`);

block(`
.matchup-score input {
  width: 5rem;
  min-width: 0;
}
`);

block(`
.perf-editor .form-row {
  align-items: flex-start;
}
`);

block(`
.perf-editor .form-group {
  margin-bottom: 0.9rem;
}
`);

block(`
.perf-card-head > .btn,
.perf-top > .btn {
  flex: 0 0 auto;
  width: auto;
}
`);

block(`
.perf-top-main {
  flex: 1;
}
`);
