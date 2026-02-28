import { block } from "vlens/css";

block(`
.family-timeline-container {
  max-width: 1000px;
  margin: 0 auto;
  padding: 2rem 1rem;
}
`);

block(`
.family-timeline-page {
  width: 100%;
}
`);

block(`
.timeline-header {
  text-align: center;
  margin-bottom: 2rem;
}
`);

block(`
.timeline-header h1 {
  font-size: 2rem;
  font-weight: 700;
  color: var(--text);
  margin-bottom: 0.5rem;
}
`);

block(`
.timeline-header p {
  color: var(--muted);
  font-size: 1rem;
}
`);

block(`
.search-container {
  display: flex;
  gap: 0.75rem;
  margin-bottom: 1.5rem;
  padding: 1.5rem;
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 8px;
  flex-wrap: wrap;
}
`);

block(`
.search-input {
  flex: 1;
  min-width: 250px;
  padding: 0.75rem 1rem;
  border: 1px solid var(--border);
  border-radius: 6px;
  background: var(--bg);
  color: var(--text);
  font-size: 1rem;
}
`);

block(`
.search-input:focus {
  outline: 2px solid var(--accent);
  outline-offset: 2px;
}
`);

block(`
.search-btn {
  padding: 0.75rem 1.5rem;
  white-space: nowrap;
}
`);

block(`
.search-results {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}
`);

block(`
.milestone-category {
  display: inline-block;
  padding: 0.125rem 0.5rem;
  border-radius: 4px;
  font-size: 0.75rem;
  font-weight: 600;
  text-transform: uppercase;
  background: rgba(139, 92, 246, 0.1);
  color: rgb(139, 92, 246);
  margin-bottom: 0.25rem;
}
`);

block(`
.milestone-description {
  color: var(--text);
  font-size: 1rem;
  line-height: 1.5;
  word-wrap: break-word;
}
`);

block(`
.milestone-item .timeline-item-header {
  display: flex;
  gap: 0.75rem;
  align-items: flex-start;
  margin-bottom: 0.5rem;
}
`);

block(`
.milestone-item .timeline-item-meta {
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
  flex: 1;
}
`);

block(`
.timeline-filters {
  display: flex;
  gap: 1.5rem;
  flex-wrap: wrap;
  margin-bottom: 1.5rem;
  padding: 1.5rem;
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 8px;
}
`);

block(`
.filter-group {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  min-width: 200px;
  flex: 1;
}
`);

block(`
.filter-group label {
  font-size: 0.875rem;
  font-weight: 600;
  color: var(--text);
}
`);

block(`
.filter-group select {
  padding: 0.5rem;
  border: 1px solid var(--border);
  border-radius: 6px;
  background: var(--bg);
  color: var(--text);
  font-size: 0.875rem;
  cursor: pointer;
}
`);

block(`
.filter-group select:focus {
  outline: 2px solid var(--accent);
  outline-offset: 2px;
}
`);

block(`
.timeline-stats {
  text-align: center;
  color: var(--muted);
  font-size: 0.875rem;
  margin-bottom: 1rem;
}
`);

/* ── Year jump navigation (Phase 3) ── */

block(`
.year-jump-nav {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
  margin-bottom: 1.25rem;
}
`);

block(`
.year-pill {
  display: inline-block;
  padding: 0.25rem 0.75rem;
  border-radius: 999px;
  font-size: 0.8125rem;
  font-weight: 600;
  background: var(--surface);
  border: 1px solid var(--border);
  color: var(--text);
  text-decoration: none;
  transition: background 0.15s ease, border-color 0.15s ease, color 0.15s ease;
}
`);

block(`
.year-pill:hover {
  background: var(--primary-accent);
  border-color: var(--primary-accent);
  color: #fff;
}
`);

/* ── Timeline spine + year groups (Phase 1) ── */

block(`
.timeline-items {
  display: flex;
  flex-direction: column;
  position: relative;
  padding-left: 2rem;
}
`);

block(`
.timeline-items::before {
  content: '';
  position: absolute;
  left: 0.75rem;
  top: 0;
  bottom: 0;
  width: 2px;
  background: var(--border);
  border-radius: 1px;
}
`);

/* Year banner */

block(`
.year-banner {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  margin-left: -2rem;
  margin-bottom: 0.875rem;
}
`);

block(`
.timeline-item + .year-banner {
  margin-top: 2rem;
}
`);

block(`
.year-banner-label {
  font-size: 1.625rem;
  font-weight: 700;
  color: var(--primary-accent);
  white-space: nowrap;
  min-width: 5rem;
  text-align: left;
  line-height: 1;
}
`);

block(`
.year-banner-line {
  flex: 1;
  height: 2px;
  background: var(--border);
  border-radius: 1px;
  opacity: 0.5;
}
`);

/* Timeline items */

block(`
.timeline-item {
  display: grid;
  grid-template-columns: auto 1fr auto;
  gap: 1rem;
  padding: 1rem;
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 8px;
  transition: box-shadow 0.2s ease, border-color 0.2s ease;
  position: relative;
  margin-bottom: 0.875rem;
}
`);

block(`
.timeline-item::before {
  content: '';
  position: absolute;
  left: -1.25rem;
  top: 1.25rem;
  width: 10px;
  height: 10px;
  border-radius: 50%;
  background: var(--accent);
  border: 2px solid var(--bg);
  transform: translateX(-50%);
}
`);

block(`
.timeline-item:hover {
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.1);
  border-color: var(--accent);
}
`);

block(`
.timeline-item-icon {
  font-size: 1.5rem;
  display: flex;
  align-items: flex-start;
  padding-top: 0.25rem;
}
`);

block(`
.timeline-item-content {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  min-width: 0;
}
`);

block(`
.timeline-item-header {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
  align-items: center;
  font-size: 0.875rem;
}
`);

block(`
.timeline-item-person {
  font-weight: 700;
  color: var(--primary-accent);
  font-size: 0.875rem;
}
`);

block(`
.timeline-item-type {
  padding: 0.125rem 0.5rem;
  border-radius: 4px;
  font-size: 0.75rem;
  font-weight: 600;
  text-transform: uppercase;
}
`);

block(`
.milestone-type {
  background: rgba(139, 92, 246, 0.1);
  color: rgb(139, 92, 246);
}
`);

block(`
.measurement-type {
  background: rgba(34, 197, 94, 0.1);
  color: rgb(34, 197, 94);
}
`);

block(`
.photo-type {
  background: rgba(59, 130, 246, 0.1);
  color: rgb(59, 130, 246);
}
`);

/* Birthday item (Phase 2) */

block(`
.birthday-item {
  background: rgba(251, 191, 36, 0.06);
  border-color: rgba(251, 191, 36, 0.35);
}
`);

block(`
.birthday-item::before {
  background: rgb(251, 191, 36);
}
`);

block(`
.birthday-type {
  background: rgba(251, 191, 36, 0.15);
  color: rgb(180, 120, 0);
}
`);

block(`
.timeline-item-age {
  color: var(--muted);
  font-size: 0.875rem;
}
`);

block(`
.timeline-item-date {
  color: var(--muted);
  font-size: 0.875rem;
  margin-left: auto;
}
`);

block(`
.timeline-item-description {
  color: var(--text);
  font-size: 1rem;
  line-height: 1.5;
  word-wrap: break-word;
}
`);

block(`
.measurement-value {
  font-weight: 600;
  font-size: 1.125rem;
}
`);

block(`
.photo-item-details {
  display: flex;
  gap: 1rem;
  margin-top: 0.5rem;
}
`);

block(`
.photo-thumbnail {
  flex-shrink: 0;
  width: 120px;
  height: 120px;
  cursor: pointer;
  border-radius: 6px;
  overflow: hidden;
  border: 1px solid var(--border);
}
`);

block(`
.timeline-photo-image {
  width: 100%;
  height: 100%;
  object-fit: cover;
}
`);

block(`
.photo-info {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
  min-width: 0;
}
`);

block(`
.photo-title {
  font-weight: 600;
  color: var(--text);
  font-size: 0.9375rem;
}
`);

block(`
.photo-description {
  color: var(--muted);
  font-size: 0.875rem;
  line-height: 1.5;
}
`);

block(`
.milestone-photos {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-top: 12px;
}
`);

block(`
.milestone-photo-thumb {
  width: 72px;
  height: 72px;
  object-fit: cover;
  border-radius: 8px;
  cursor: pointer;
}
`);

block(`
.milestone-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
  margin-top: 4px;
}
`);

block(`
.milestone-tag-badge {
  font-size: 11px;
  padding: 1px 7px;
  border-radius: 10px;
  border: 1.5px solid;
  background: transparent;
}
`);

block(`
.timeline-item-actions {
  display: flex;
  gap: 0.5rem;
  align-items: flex-start;
}
`);

block(`
.btn-action {
  background: none;
  border: none;
  font-size: 1.25rem;
  cursor: pointer;
  padding: 0.25rem;
  text-decoration: none;
  transition: transform 0.2s ease;
}
`);

block(`
.btn-action:hover {
  transform: scale(1.1);
}
`);

block(`
.empty-state {
  text-align: center;
  padding: 3rem 1rem;
  color: var(--muted);
}
`);

block(`
.empty-state h3 {
  font-size: 1.5rem;
  margin-bottom: 0.5rem;
  color: var(--text);
}
`);

block(`
.empty-state p {
  margin-bottom: 1.5rem;
  font-size: 1rem;
}
`);

block(`
.empty-state-actions {
  display: flex;
  gap: 1rem;
  justify-content: center;
  flex-wrap: wrap;
}
`);

block(`
@media (max-width: 600px) {
  .family-timeline-container {
    padding: 1rem 0.5rem;
  }

  .search-container {
    flex-direction: column;
    padding: 1rem;
  }

  .search-input {
    min-width: 100%;
  }

  .search-btn {
    width: 100%;
  }

  .timeline-filters {
    flex-direction: column;
    gap: 1rem;
    padding: 1rem;
  }

  .filter-group {
    min-width: 100%;
  }

  /* Collapse spine on mobile */
  .timeline-items {
    padding-left: 0;
  }

  .timeline-items::before {
    display: none;
  }

  .timeline-item::before {
    display: none;
  }

  .year-banner {
    margin-left: 0;
  }

  .timeline-item {
    grid-template-columns: auto 1fr;
    gap: 0.75rem;
    margin-bottom: 0.75rem;
  }

  .timeline-item-actions {
    grid-column: 1 / -1;
    justify-content: flex-end;
    padding-top: 0.5rem;
    border-top: 1px solid var(--border);
  }

  .timeline-item-header {
    font-size: 0.8125rem;
  }

  .timeline-item-date {
    margin-left: 0;
    width: 100%;
  }

  .photo-item-details {
    flex-direction: column;
  }

  .photo-thumbnail {
    width: 100%;
    height: 200px;
  }
}
`);
