import { block } from "vlens/css";

block(`
.iv-page {
  max-width: 720px;
  margin: 0 auto;
  padding: 2rem 1rem 4rem;
}
`);

block(`
.iv-page input[type="text"],
.iv-page input[type="date"],
.iv-page textarea {
  padding: 10px 12px;
  border: 1px solid var(--control-border);
  border-radius: 8px;
  background: var(--bg);
  color: var(--text);
  font: inherit;
}
`);

block(`
.iv-page input:focus-visible,
.iv-page textarea:focus-visible {
  outline: 2px solid var(--accent);
  outline-offset: 1px;
  border-color: var(--accent);
}
`);

block(`
.iv-hint {
  color: var(--muted);
  font-size: 0.9rem;
}
`);

block(`
.iv-link {
  background: none;
  border: none;
  padding: 0;
  font: inherit;
  font-size: 0.9rem;
  color: var(--accent);
  cursor: pointer;
  text-decoration: none;
}
`);

block(`
.iv-link:hover {
  text-decoration: underline;
}
`);

block(`
.iv-callout {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  padding: 0.9rem 1rem;
  margin: 1rem 0;
  border: 1px solid var(--accent-soft-border);
  background: var(--accent-soft);
  border-radius: 8px;
}
`);

block(`
.iv-callout p {
  margin: 0;
}
`);

block(`
.iv-field {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.5rem 1rem;
  margin: 1.25rem 0 0.75rem;
}
`);

block(`
.iv-field label {
  font-weight: 600;
}
`);

block(`
.iv-toggle {
  display: inline-flex;
  align-items: center;
  gap: 0.5rem;
  font-size: 0.95rem;
  cursor: pointer;
}
`);

block(`
.iv-toggle input {
  accent-color: var(--accent);
  width: 18px;
  height: 18px;
}
`);

block(`
.iv-setup-head {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 1rem;
  margin-top: 1.75rem;
}
`);

block(`
.iv-setup-head h2 {
  margin: 0;
  font-size: 1.2rem;
}
`);

block(`
.iv-setup-section {
  border: none;
  padding: 0;
  margin: 1.25rem 0;
}
`);

block(`
.iv-setup-section legend {
  display: flex;
  align-items: baseline;
  gap: 0.75rem;
  font-weight: 600;
  padding: 0;
  margin-bottom: 0.4rem;
}
`);

block(`
.iv-check {
  display: flex;
  align-items: center;
  gap: 0.6rem;
  padding: 0.45rem 0;
  border-bottom: 1px solid var(--border);
  cursor: pointer;
}
`);

block(`
.iv-check input {
  accent-color: var(--accent);
  width: 18px;
  height: 18px;
  flex: none;
}
`);

block(`
.iv-check span:first-of-type {
  flex: 1;
}
`);

block(`
.iv-inline {
  display: flex;
  gap: 0.5rem;
  margin-top: 0.5rem;
}
`);

block(`
.iv-inline input {
  flex: 1;
  min-width: 0;
}
`);

block(`
.iv-actions {
  display: flex;
  justify-content: flex-end;
  gap: 0.75rem;
  margin-top: 2rem;
}
`);

block(`
.iv-ask-top {
  display: flex;
  align-items: baseline;
  gap: 1rem;
}
`);

block(`
.iv-ask-top a {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
`);

block(`
.iv-progress {
  color: var(--muted);
  font-variant-numeric: tabular-nums;
}
`);

block(`
.iv-bar {
  height: 4px;
  background: var(--border);
  border-radius: 2px;
  margin: 0.6rem 0 1.5rem;
  overflow: hidden;
}
`);

block(`
.iv-bar-fill {
  height: 100%;
  background: var(--accent);
  transition: width 0.2s ease;
}
`);

block(`
.iv-save-status {
  color: var(--warning);
  font-size: 0.9rem;
  margin: -0.75rem 0 1rem;
}
`);

block(`
.iv-section-label {
  text-transform: uppercase;
  letter-spacing: 0.06em;
  font-size: 0.8rem;
  color: var(--muted);
  margin: 0 0 0.4rem;
}
`);

block(`
.iv-question {
  font-size: clamp(1.6rem, 6vw, 2.3rem);
  line-height: 1.25;
  margin: 0 0 1.25rem;
}
`);

block(`
.iv-page textarea.iv-answer {
  width: 100%;
  font-size: 1.25rem;
  resize: vertical;
  min-height: 5.5rem;
}
`);

block(`
.iv-previous {
  display: block;
  width: 100%;
  margin-top: 0.75rem;
  padding: 0;
  background: none;
  border: none;
  text-align: left;
  font: inherit;
  color: var(--muted);
  cursor: pointer;
}
`);

block(`
.iv-previous ul {
  list-style: none;
  margin: 0;
  padding: 0;
}
`);

block(`
.iv-previous li {
  padding: 0.15rem 0;
}
`);

block(`
.iv-previous-age {
  display: inline-block;
  min-width: 3.5rem;
  font-size: 0.85rem;
}
`);

block(`
.iv-ask-buttons {
  display: flex;
  gap: 0.75rem;
  margin-top: 1.5rem;
}
`);

block(`
.iv-ask-buttons .btn {
  flex: 1;
  width: auto;
}
`);

block(`
.iv-ask-buttons .btn-primary {
  flex: 2;
}
`);

block(`
.iv-ask-extras {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.75rem 1.5rem;
  margin-top: 2rem;
  padding-top: 1rem;
  border-top: 1px solid var(--border);
}
`);

block(`
.iv-portrait-button {
  cursor: pointer;
}
`);

block(`
.iv-followup {
  width: 100%;
  display: flex;
  flex-direction: column;
  gap: 0.6rem;
}
`);

block(`
.iv-followup-options {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem 1.25rem;
}
`);

block(`
.iv-jump {
  list-style: none;
  margin: 0 0 1.5rem;
  padding: 0.5rem;
  max-height: 50vh;
  overflow-y: auto;
  border: 1px solid var(--border);
  border-radius: 8px;
  background: var(--surface);
}
`);

block(`
.iv-jump-item {
  display: flex;
  align-items: center;
  gap: 0.6rem;
  width: 100%;
  padding: 0.45rem 0.5rem;
  background: none;
  border: none;
  border-radius: 6px;
  color: var(--text);
  font: inherit;
  text-align: left;
  cursor: pointer;
}
`);

block(`
.iv-jump-item:hover,
.iv-jump-item.current {
  background: var(--hover-bg);
}
`);

block(`
.iv-dot {
  flex: none;
  width: 10px;
  height: 10px;
  border-radius: 50%;
  border: 2px solid var(--muted);
}
`);

block(`
.iv-dot.answered {
  background: var(--accent);
  border-color: var(--accent);
}
`);

block(`
.iv-dot.skipped {
  border-style: dashed;
}
`);

block(`
.iv-pending ul {
  list-style: none;
  padding: 0;
}
`);

block(`
.iv-pending li {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  padding: 0.4rem 0;
  border-bottom: 1px solid var(--border);
}
`);

block(`
.iv-pending li span {
  flex: 1;
}
`);

block(`
.iv-view-head {
  display: flex;
  align-items: center;
  gap: 1.25rem;
  margin-bottom: 1rem;
}
`);

block(`
.iv-view-head h1 {
  margin: 0 0 0.25rem;
}
`);

block(`
.iv-portrait {
  width: 120px;
  height: 120px;
  object-fit: cover;
  border-radius: 50%;
  flex: none;
}
`);

block(`
.iv-notes {
  font-style: italic;
  color: var(--muted);
}
`);

block(`
.iv-view-section h2 {
  font-size: 1.1rem;
  margin: 2rem 0 0.5rem;
}
`);

block(`
.iv-view-section dl {
  margin: 0;
}
`);

block(`
.iv-qa {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
  gap: 1rem;
  padding: 0.5rem 0;
  border-bottom: 1px solid var(--border);
}
`);

block(`
.iv-qa dt {
  color: var(--muted);
}
`);

block(`
.iv-qa dd {
  margin: 0;
  font-weight: 600;
}
`);

block(`
.iv-qa dd a {
  color: var(--text);
  text-decoration: none;
}
`);

block(`
@media (max-width: 560px) {
  .iv-qa {
    grid-template-columns: 1fr;
    gap: 0.15rem;
  }
}
`);

block(`
.iv-toolbar {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
  margin: 1rem 0;
}
`);

block(`
.iv-page .iv-toolbar .btn,
.iv-page .iv-inline .btn {
  width: auto;
}
`);

block(`
.iv-details {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
  padding: 1rem;
  margin: 1rem 0;
  border: 1px solid var(--border);
  border-radius: 8px;
  background: var(--surface);
}
`);

block(`
.iv-details label {
  display: flex;
  flex-direction: column;
  gap: 0.3rem;
  font-weight: 600;
  font-size: 0.9rem;
}
`);

block(`
.iv-changes {
  margin: 1.5rem 0;
  padding: 1rem 1.25rem;
  border-radius: 10px;
  background: var(--accent-soft);
  border: 1px solid var(--accent-soft-border);
}
`);

block(`
.iv-changes h2 {
  font-size: 1rem;
  margin: 0 0 0.5rem;
}
`);

block(`
.iv-changes ul {
  list-style: none;
  margin: 0;
  padding: 0;
}
`);

block(`
.iv-changes li {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
  gap: 1rem;
  padding: 0.3rem 0;
}
`);

block(`
.iv-changes-q {
  color: var(--muted);
}
`);

block(`
.iv-changes s {
  color: var(--muted);
}
`);

block(`
.iv-qa input {
  width: 100%;
}
`);

block(`
.iv-empty {
  text-align: center;
  padding: 2rem 1rem;
  color: var(--muted);
}
`);

block(`
.iv-tab {
  display: grid;
  grid-template-columns: minmax(0, 1fr);
}
`);

block(`
.iv-tab-head {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  margin-bottom: 1rem;
}
`);

block(`
.iv-tab-head .btn {
  width: auto;
}
`);

block(`
.iv-interview-list {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
  list-style: none;
  padding: 0 0 0.5rem;
  margin: 0 0 1rem;
}
`);

block(`
.iv-interview-list a {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  padding: 0.4rem 0.75rem;
  border: 1px solid var(--border);
  border-radius: 999px;
  white-space: nowrap;
  color: var(--text);
  text-decoration: none;
  font-size: 0.9rem;
}
`);

block(`
.iv-interview-list small {
  color: var(--muted);
}
`);

block(`
.iv-thumb {
  width: 28px;
  height: 28px;
  border-radius: 50%;
  object-fit: cover;
}
`);

block(`
.iv-grid-wrap {
  overflow-x: auto;
  border: 1px solid var(--border);
  border-radius: 8px;
}
`);

block(`
.iv-grid {
  border-collapse: separate;
  border-spacing: 0;
  min-width: 100%;
  font-size: 0.92rem;
}
`);

block(`
.iv-grid th,
.iv-grid td {
  padding: 0.5rem 0.75rem;
  border-bottom: 1px solid var(--border);
  text-align: left;
  vertical-align: top;
  min-width: 8rem;
}
`);

block(`
.iv-grid thead th {
  position: sticky;
  top: 0;
  background: var(--surface);
  white-space: nowrap;
  font-weight: 600;
}
`);

block(`
.iv-grid thead a {
  display: inline-flex;
  align-items: center;
  gap: 0.4rem;
  color: var(--text);
  text-decoration: none;
}
`);

block(`
.iv-grid .iv-grid-q {
  position: sticky;
  left: 0;
  z-index: 1;
  background: var(--bg);
  font-weight: 400;
  color: var(--muted);
  min-width: 12rem;
  max-width: 16rem;
}
`);

block(`
.iv-grid thead .iv-grid-q {
  z-index: 2;
  background: var(--surface);
}
`);

block(`
.iv-grid-section th {
  position: sticky;
  left: 0;
  background: var(--surface);
  font-weight: 700;
  font-size: 0.8rem;
  text-transform: uppercase;
  letter-spacing: 0.05em;
}
`);

block(`
.iv-cell a {
  color: var(--text);
  text-decoration: none;
}
`);

block(`
.iv-cell.changed a {
  font-weight: 700;
  color: var(--accent);
}
`);

block(`
.iv-cell.same a {
  color: var(--muted);
}
`);

block(`
.iv-cell.skipped {
  color: var(--muted);
}
`);

block(`
.iv-legend-changed {
  color: var(--accent);
  font-weight: 700;
}
`);

block(`
.iv-grid-narrow {
  display: none;
}
`);

block(`
@media (max-width: 640px) {
  .iv-grid-wrap,
  .iv-grid-legend {
    display: none;
  }
  .iv-grid-narrow {
    display: block;
  }
}
`);

block(`
.iv-questions-tools {
  display: flex;
  flex-wrap: wrap;
  gap: 0.75rem;
  align-items: center;
  margin-bottom: 1rem;
}
`);

block(`
.iv-questions-tools input {
  flex: 1;
  min-width: 12rem;
  padding: 8px 10px;
  border: 1px solid var(--control-border);
  border-radius: 8px;
  background: var(--bg);
  color: var(--text);
  font: inherit;
}
`);

block(`
.iv-cards {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(16rem, 1fr));
  gap: 0.75rem;
}
`);

block(`
.iv-card {
  padding: 0.9rem 1rem;
  border: 1px solid var(--border);
  border-radius: 10px;
  background: var(--surface);
}
`);

block(`
.iv-card h3 {
  font-size: 0.95rem;
  font-weight: 600;
  color: var(--muted);
  margin: 0 0 0.5rem;
}
`);

block(`
.iv-timeline {
  list-style: none;
  margin: 0;
  padding: 0 0 0 0.75rem;
  border-left: 2px solid var(--border);
}
`);

block(`
.iv-timeline li {
  display: flex;
  gap: 0.6rem;
  padding: 0.15rem 0;
}
`);

block(`
.iv-timeline a {
  color: var(--text);
  text-decoration: none;
  font-weight: 600;
}
`);

block(`
.iv-timeline li.same a {
  color: var(--muted);
  font-weight: 400;
}
`);

block(`
.iv-timeline-age {
  min-width: 1.5rem;
  color: var(--muted);
  font-variant-numeric: tabular-nums;
}
`);

block(`
.iv-family-head {
  display: flex;
  flex-wrap: wrap;
  justify-content: space-between;
  align-items: flex-end;
  gap: 0.75rem;
}
`);

block(`
.iv-family-head h1 {
  margin: 0;
}
`);

block(`
.iv-family-links {
  display: flex;
  gap: 1rem;
}
`);

block(`
.iv-family {
  list-style: none;
  padding: 0;
  margin: 1.25rem 0;
}
`);

block(`
.iv-family-row {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  padding: 0.75rem 0;
  border-bottom: 1px solid var(--border);
}
`);

block(`
.iv-family-who {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  color: var(--text);
  text-decoration: none;
  min-width: 0;
}
`);

block(`
.iv-family-who > span:last-child {
  display: flex;
  flex-direction: column;
}
`);

block(`
.iv-family-avatar {
  width: 44px;
  height: 44px;
  border-radius: 50%;
  overflow: hidden;
  flex: none;
  display: grid;
  place-items: center;
  background: var(--accent-soft);
  font-weight: 700;
}
`);

block(`
.iv-family-photo {
  width: 100%;
  height: 100%;
  object-fit: cover;
}
`);

block(`
.iv-family-actions {
  display: flex;
  align-items: center;
  gap: 0.6rem;
}
`);

block(`
.iv-family-actions .btn {
  width: auto;
}
`);

block(`
.iv-badge {
  padding: 0.15rem 0.6rem;
  border-radius: 999px;
  background: var(--surface-alt);
  border: 1px solid var(--surface-alt-border);
  color: var(--warning);
  font-size: 0.8rem;
  font-weight: 600;
}
`);

block(`
.iv-others summary {
  cursor: pointer;
  color: var(--muted);
}
`);

block(`
.iv-tab-links {
  display: flex;
  flex-wrap: wrap;
  justify-content: center;
  gap: 0.5rem 1.25rem;
  margin: 0.75rem 0 1rem;
}
`);

block(`
.iv-tab .iv-tab-links {
  justify-content: flex-start;
  margin-top: -0.5rem;
}
`);

block(`
.iv-past-row {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
  align-items: center;
  gap: 0.75rem;
  padding: 0.35rem 0;
  border-bottom: 1px solid var(--border);
}
`);

block(`
.iv-past-extra {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
  align-items: center;
  margin-bottom: 0.5rem;
}
`);

block(`
.iv-past-extra input[type="text"] {
  flex: 1;
  min-width: 10rem;
}
`);

block(`
.iv-more-details summary {
  cursor: pointer;
  color: var(--muted);
}
`);

block(`
.iv-page .iv-select,
.iv-review-match select {
  padding: 8px 10px;
  border: 1px solid var(--control-border);
  border-radius: 8px;
  background: var(--bg);
  color: var(--text);
  font: inherit;
  max-width: 100%;
}
`);

block(`
.iv-formats {
  margin: 0.25rem 0 1rem;
  padding-left: 1.25rem;
}
`);

block(`
.iv-drop {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  margin: 1rem 0;
}
`);

block(`
.iv-page textarea.iv-paste {
  width: 100%;
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 0.9rem;
}
`);

block(`
.iv-import {
  max-width: 980px;
}
`);

block(`
.iv-review {
  margin: 1.5rem 0;
  padding: 1rem;
  border: 1px solid var(--border);
  border-radius: 10px;
  background: var(--surface);
}
`);

block(`
.iv-review.skipped {
  opacity: 0.6;
}
`);

block(`
.iv-review-head {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.5rem 1rem;
  margin-bottom: 0.75rem;
}
`);

block(`
.iv-review-head h2 {
  margin: 0;
  font-size: 1.1rem;
  flex: 1;
}
`);

block(`
.iv-review-head label:not(.iv-toggle) {
  display: inline-flex;
  align-items: center;
  gap: 0.4rem;
}
`);

block(`
.iv-review-row {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(0, 1fr) minmax(0, 1.3fr);
  gap: 0.5rem 0.75rem;
  align-items: center;
  padding: 0.4rem 0;
  border-top: 1px solid var(--border);
}
`);

block(`
.iv-review-row.ignored .iv-review-raw,
.iv-review-row.ignored input {
  opacity: 0.45;
}
`);

block(`
.iv-review-raw {
  color: var(--muted);
}
`);

block(`
.iv-review-match {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.4rem;
}
`);

block(`
.iv-review-match select {
  flex: 1;
  min-width: 0;
}
`);

block(`
.iv-done-list {
  padding-left: 1.25rem;
}
`);

block(`
@media (max-width: 640px) {
  .iv-review-row,
  .iv-past-row {
    grid-template-columns: 1fr;
    gap: 0.3rem;
  }
}
`);

block(`
@media print {
  .site-header,
  .app-bottombar,
  .site-footer,
  .iv-no-print {
    display: none !important;
  }
  .iv-page {
    padding: 0;
    max-width: none;
    color: #000;
  }
  .iv-view,
  .iv-view * {
    color: #000 !important;
    background: none !important;
  }
  .iv-view-section { break-inside: auto; }
 .iv-view-section h2 { break-after: avoid; }
 .iv-qa { break-inside: avoid; }
  .iv-qa {
    padding: 0.2rem 0;
    gap: 0.5rem;
  }
  .iv-view-section h2 {
    margin: 0.75rem 0 0.25rem;
  }
}
`);

block(`
.iv-card h3 a {
  color: inherit;
  text-decoration: none;
}
`);

block(`
.iv-compare-people {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem 1.25rem;
  margin: 0.5rem 0 1.25rem;
}
`);

block(`
.iv-compare {
  list-style: none;
  padding: 0;
  margin: 0;
}
`);

block(`
.iv-compare-age {
  padding: 0.75rem 1rem;
  margin-bottom: 0.6rem;
  border: 1px solid var(--border);
  border-radius: 10px;
  background: var(--surface);
}
`);

block(`
.iv-compare-age.current {
  border-color: var(--accent);
}
`);

block(`
.iv-compare-age h2 {
  font-size: 0.95rem;
  margin: 0 0 0.35rem;
  color: var(--muted);
}
`);

block(`
.iv-compare-age ul {
  list-style: none;
  margin: 0;
  padding: 0;
}
`);

block(`
.iv-compare-age li {
  padding: 0.15rem 0;
}
`);

block(`
.iv-compare-age a {
  color: var(--text);
}
`);
