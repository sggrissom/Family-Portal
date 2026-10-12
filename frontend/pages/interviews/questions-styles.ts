import { block } from "vlens/css";

block(`
.iq-page {
  max-width: 760px;
  margin: 0 auto;
  padding: 2rem 1rem 6rem;
}
`);

block(`
.iq-page input[type="text"],
.iq-page input[type="number"],
.iq-page select {
  padding: 8px 10px;
  border: 1px solid var(--control-border);
  border-radius: 8px;
  background: var(--bg);
  color: var(--text);
  font-size: 0.95rem;
}
`);

block(`
.iq-page input:focus-visible,
.iq-page select:focus-visible {
  outline: 2px solid var(--accent);
  outline-offset: 1px;
  border-color: var(--accent);
}
`);

block(`
.iq-intro {
  color: var(--muted);
  margin: 0 0 0.5rem;
}
`);

block(`
.iq-suggested {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.5rem 1rem;
  margin: 1.5rem 0;
  padding: 0.75rem 1rem;
  border: 1px solid var(--accent-soft-border);
  background: var(--accent-soft);
  border-radius: 8px;
}
`);

block(`
.iq-suggested-toggle {
  background: none;
  border: none;
  padding: 0;
  font: inherit;
  font-weight: 600;
  color: var(--text);
  cursor: pointer;
}
`);

block(`
.iq-suggested ul,
.iq-retired ul {
  list-style: none;
  margin: 0;
  padding: 0;
  width: 100%;
}
`);

block(`
.iq-suggested li,
.iq-retired li {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  padding: 0.4rem 0;
}
`);

block(`
.iq-suggested li span,
.iq-retired li span:first-child {
  flex: 1;
}
`);

block(`
.iq-section {
  margin: 2rem 0;
}
`);

block(`
.iq-page input.iq-section-name {
  font-size: 1.15rem;
  font-weight: 600;
  border: 1px solid transparent;
  background: transparent;
  color: var(--text);
  padding: 0.25rem 0.4rem;
  margin-left: -0.4rem;
  border-radius: 6px;
  width: 100%;
}
`);

block(`
.iq-page input.iq-section-name:hover,
.iq-page input.iq-section-name:focus {
  border-color: var(--control-border);
}
`);

block(`
.iq-list {
  list-style: none;
  margin: 0.5rem 0;
  padding: 0;
}
`);

block(`
.iq-question {
  display: flex;
  align-items: flex-start;
  gap: 0.5rem;
  padding: 0.6rem 0;
  border-bottom: 1px solid var(--border);
}
`);

block(`
.iq-question.dragging {
  opacity: 0.4;
}
`);

block(`
.iq-handle {
  cursor: grab;
  color: var(--muted);
  padding-top: 0.45rem;
  user-select: none;
}
`);

block(`
.iq-question-body {
  flex: 1;
  min-width: 0;
}
`);

block(`
.iq-page input.iq-text {
  width: 100%;
  border-color: transparent;
  background: transparent;
  font-size: 1rem;
  padding: 6px 8px;
  margin-left: -8px;
}
`);

block(`
.iq-page input.iq-text:hover {
  border-color: var(--border);
}
`);

block(`
.iq-more {
  display: inline;
}
`);

block(`
.iq-more summary {
  display: inline;
  cursor: pointer;
  font-size: 0.85rem;
  color: var(--muted);
  text-decoration: underline;
}
`);

block(`
.iq-more-body {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.5rem 0.75rem;
  margin-top: 0.5rem;
  font-size: 0.9rem;
}
`);

block(`
.iq-meta {
  display: inline-flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.4rem 0.9rem;
  margin-right: 0.9rem;
  font-size: 0.85rem;
  color: var(--muted);
}
`);

block(`
.iq-more-body select {
  max-width: 14rem;
}
`);

block(`
.iq-age {
  display: inline-flex;
  align-items: center;
  gap: 0.3rem;
}
`);

block(`
.iq-page .iq-age input {
  width: 4rem;
}
`);

block(`
.iq-actions {
  display: flex;
  gap: 0.3rem;
}
`);

block(`
.iq-icon {
  background: none;
  border: 1px solid var(--border);
  border-radius: 6px;
  color: var(--text);
  width: 2rem;
  height: 2rem;
  cursor: pointer;
}
`);

block(`
.iq-add,
.iq-add-section {
  display: flex;
  gap: 0.5rem;
  margin-top: 0.5rem;
}
`);

block(`
.iq-add input,
.iq-add-section input {
  flex: 1;
  min-width: 0;
}
`);

block(`
.iq-empty {
  color: var(--muted);
  font-size: 0.9rem;
}
`);

block(`
.iq-retired {
  margin: 2rem 0;
}
`);

block(`
.iq-retired summary {
  cursor: pointer;
  font-weight: 600;
}
`);

block(`
.iq-footer {
  position: sticky;
  bottom: var(--bottom-bar-space);
  display: flex;
  align-items: center;
  gap: 1rem;
  padding: 0.75rem 0;
  background: var(--bg);
  border-top: 1px solid var(--border);
}
`);

block(`
.iq-status {
  flex: 1;
  text-align: right;
  color: var(--muted);
  font-size: 0.9rem;
}
`);

block(`
@media (max-width: 560px) {
  .iq-handle {
    display: none;
  }
  .iq-page .btn {
    width: auto;
  }
}
`);
