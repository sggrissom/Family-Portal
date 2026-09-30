import { block } from "vlens/css";

import "./photos-styles";

block(`
.family-photos-container {
  padding: 28px 16px 72px;
  max-width: 1200px;
  margin: 0 auto;
}
`);

block(`
.family-photos-page {
  min-height: 70vh;
}
`);

block(`
.family-photos-container .header-content {
  display: flex;
  justify-content: space-between;
  align-items: flex-end;
  gap: 1rem;
}
`);

block(`
.family-photos-container .photos-count {
  color: var(--muted);
  font-size: 0.875rem;
  font-weight: 500;
}
`);

block(`
.filter-panel {
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 8px;
  padding: 1.5rem;
  margin-bottom: 2rem;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.1);
}
`);

block(`
.family-photos-container .filter-section {
  margin-bottom: 1.5rem;
}
`);

block(`
.family-photos-container .filter-section:last-of-type {
  margin-bottom: 0;
}
`);

block(`
.filter-section h3 {
  margin: 0 0 1rem 0;
  font-size: 1rem;
  font-weight: 600;
  color: var(--text);
}
`);

block(`
.people-filter {
  display: flex;
  flex-wrap: wrap;
  gap: 0.75rem;
}
`);

block(`
.family-photos-container .person-checkbox {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  padding: 0.5rem 0.75rem;
  background: var(--bg);
  border: 1px solid var(--border);
  border-radius: 6px;
  cursor: pointer;
  transition: all 0.2s ease;
  user-select: none;
}
`);

block(`
.family-photos-container .person-checkbox:hover {
  background: var(--hover-bg);
  border-color: var(--primary-accent);
}
`);

block(`
.person-checkbox input[type="checkbox"] {
  margin: 0;
  cursor: pointer;
}
`);

block(`
.person-label {
  font-size: 0.875rem;
  font-weight: 500;
  color: var(--text);
  cursor: pointer;
}
`);

block(`
.date-filter {
  display: flex;
  gap: 1rem;
  flex-wrap: wrap;
}
`);

block(`
.date-input-group {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  min-width: 150px;
}
`);

block(`
.date-input-group label {
  font-size: 0.875rem;
  font-weight: 500;
  color: var(--text);
}
`);

block(`
.date-input-group input[type="date"] {
  padding: 0.5rem;
  border: 1px solid var(--border);
  border-radius: 4px;
  background: var(--bg);
  color: var(--text);
  font-size: 0.875rem;
  transition: border-color 0.2s ease;
}
`);

block(`
.date-input-group input[type="date"]:focus {
  outline: none;
  border-color: var(--primary-accent);
  box-shadow: 0 0 0 3px rgba(76, 175, 80, 0.1);
}
`);

block(`
.filter-actions {
  display: flex;
  gap: 0.75rem;
  justify-content: flex-end;
  align-items: center;
  margin-top: 1.5rem;
  padding-top: 1.5rem;
  border-top: 1px solid var(--border);
}
`);

block(`
.family-photos-container .filter-toggle {
  white-space: nowrap;
}
`);

block(`
.family-photos-container .loading-state {
  padding: 1rem;
  text-align: center;
  color: var(--muted);
  font-style: italic;
}
`);

block(`
.people-badges {
  position: absolute;
  bottom: 8px;
  left: 8px;
  right: 8px;
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
  z-index: 1;
}
`);

block(`
.person-badge {
  background: rgba(0, 0, 0, 0.8);
  color: white;
  padding: 4px 8px;
  border-radius: 12px;
  font-size: 11px;
  font-weight: 600;
  box-shadow: 0 2px 4px rgba(0, 0, 0, 0.3);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  flex-shrink: 0;
  max-width: 100%;
}
`);

block(`
.photo-search {
  display: flex;
  gap: 0.5rem;
  margin-bottom: 1rem;
}
`);

block(`
.photo-search input {
  flex: 1;
  min-width: 0;
  padding: 0.625rem 0.875rem;
  border: 1px solid var(--border);
  border-radius: 8px;
  background: var(--bg);
  color: var(--text);
  font-size: 1rem;
}
`);

block(`
.photo-search-summary {
  display: flex;
  flex-wrap: wrap;
  align-items: baseline;
  gap: 0.25rem 1rem;
  margin-bottom: 1rem;
  color: var(--text);
}
`);

block(`
.photo-search-clear {
  padding: 0 4px;
  border: none;
  background: none;
  color: var(--primary-accent);
  font: inherit;
  cursor: pointer;
}
`);

block(`
.photo-search-note {
  color: var(--muted);
  font-size: 0.875rem;
}
`);

block(`
.similar-stack-badge {
  position: absolute;
  top: 8px;
  left: 8px;
  z-index: 2;
  background: rgba(0, 0, 0, 0.75);
  color: #fff;
  border: 1px solid rgba(255, 255, 255, 0.4);
  border-radius: 12px;
  padding: 3px 10px;
  font-size: 0.75rem;
  font-weight: 600;
  cursor: pointer;
  box-shadow: 3px 3px 0 rgba(255, 255, 255, 0.35), 6px 6px 0 rgba(0, 0, 0, 0.25);
}
`);

block(`
.similar-stack-badge:hover {
  background: rgba(0, 0, 0, 0.9);
}
`);

block(`
.places-filter {
  display: flex;
  flex-wrap: wrap;
  gap: 0.75rem;
}
`);

block(`
.places-filter .place-count {
  color: var(--muted);
  font-weight: 400;
}
`);

block(`
.tags-filter {
  display: flex;
  flex-wrap: wrap;
  gap: 0.75rem;
}
`);

block(`
.tag-filter-label {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  padding: 0.5rem 0.75rem;
  background: var(--bg);
  border: 1px solid var(--border);
  border-radius: 6px;
  cursor: pointer;
  transition: all 0.2s ease;
  user-select: none;
  font-size: 0.875rem;
  font-weight: 500;
  color: var(--text);
}
`);

block(`
.tag-filter-label:hover {
  background: var(--hover-bg);
  border-color: var(--primary-accent);
}
`);

block(`
.tag-filter-label input[type="checkbox"] {
  margin: 0;
  cursor: pointer;
}
`);

block(`
.tag-color-dot {
  width: 10px;
  height: 10px;
  border-radius: 50%;
  flex-shrink: 0;
}
`);

block(`
.empty-state-inline {
  padding: 0.5rem 0;
  color: var(--muted);
  font-style: italic;
  font-size: 0.875rem;
}
`);

block(`
.tag-badges {
  position: absolute;
  top: 8px;
  right: 8px;
  display: flex;
  gap: 4px;
  z-index: 1;
}
`);

block(`
.tag-badge {
  width: 10px;
  height: 10px;
  border-radius: 50%;
  display: inline-block;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.4);
  flex-shrink: 0;
}
`);

block(`
.family-photos-container .empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  text-align: center;
  padding: 44px 20px;
  min-height: 0;
}
`);

block(`
.empty-icon {
  font-size: 4rem;
  margin-bottom: 1rem;
  opacity: 0.6;
}
`);

block(`
.empty-state h2 {
  margin: 0 0 0.75rem 0;
  font-size: 1.5rem;
  font-weight: 600;
  color: var(--text);
}
`);

block(`
.empty-state p {
  margin: 0 0 2rem 0;
  color: var(--muted);
  font-size: 1rem;
  max-width: 400px;
  line-height: 1.5;
}
`);

block(`
.family-photos-container .page-header {
  margin-bottom: 20px;
  padding-bottom: 0;
  border-bottom: 0;
}
`);

block(`
.family-photos-container .header-content {
  align-items: center;
}
`);

block(`
.family-photos-container .page-header h1 {
  margin-bottom: 4px;
  font-size: clamp(1.5rem, 4vw, 2rem);
}
`);

block(`
.family-photos-container .btn {
  width: auto;
  flex: 0 0 auto;
  white-space: nowrap;
}
`);

block(`
.family-photos-container .photo-toolbar {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 12px;
  margin-bottom: 20px;
}
`);

block(`
.family-photos-container .photo-search {
  flex: 1;
  min-width: 0;
  margin: 0;
}
`);

block(`
.family-photos-container .photo-search input {
  width: 0;
  background: var(--surface);
}
`);

block(`
.family-photos-container .photo-tools {
  display: flex;
  gap: 16px;
  font-size: 0.875rem;
}
`);

block(`
.family-photos-container .photo-tools a {
  color: var(--muted);
  text-decoration: none;
  padding: 8px 0;
}
`);

block(`
.family-photos-container .photo-tools a:hover {
  color: var(--primary-accent);
  text-decoration: underline;
}
`);

block(`
.family-photos-container .photo-active-filters {
  flex-basis: 100%;
  display: flex;
  flex-wrap: wrap;
  gap: 8px 16px;
  color: var(--muted);
  font-size: 0.875rem;
}
`);

block(`
.family-photos-container .similar-photos-option {
  display: flex;
  align-items: center;
  gap: 8px;
  cursor: pointer;
}
`);

block(`
.family-photos-container .similar-photos-help {
  margin: 8px 0 0;
  color: var(--muted);
  font-size: 0.875rem;
  line-height: 1.5;
}
`);

block(`
.family-photos-container .photos-gallery {
  grid-template-columns: repeat(auto-fill, minmax(220px, 1fr));
  gap: 16px;
}
`);

block(`
.family-photos-container .photo-card {
  position: relative;
  box-shadow: none;
  border: 1px solid var(--border);
}
`);

block(`
.family-photos-container .photo-open {
  display: block;
  color: inherit;
  text-decoration: none;
}
`);

block(`
.family-photos-container .photo-open:focus-visible {
  outline: 3px solid var(--primary-accent);
  outline-offset: -3px;
}
`);

block(`
.family-photos-container .photo-image-container {
  height: auto;
  aspect-ratio: 4 / 5;
  overflow: hidden;
}
`);

block(`
.family-photos-container .photo-image-container picture,
.family-photos-container .processing-image-wrapper {
  display: block;
  width: 100%;
  height: 100%;
}
`);

block(`
.family-photos-container .photo-image {
  object-fit: cover;
}
`);

block(`
.family-photos-container .photo-info {
  padding: 10px 12px;
}
`);

block(`
.family-photos-container .photo-date {
  margin: 0;
}
`);

block(`
.family-photos-container .photo-description {
  margin-top: 6px;
}
`);

block(`
.family-photos-container .person-badge {
  background: rgba(0, 0, 0, 0.65);
  box-shadow: none;
}
`);

block(`
@media (max-width: 768px) {
  .family-photos-container {
    padding: 20px 16px 88px;
  }
  .family-photos-container .photo-tools {
    flex-basis: 100%;
    justify-content: flex-end;
  }
  .family-photos-container .photo-toolbar {
    gap: 8px;
  }
  .family-photos-container .photos-gallery {
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 10px;
  }
  .family-photos-container .filter-panel {
    padding: 16px;
  }
  .family-photos-container .date-input-group {
    flex: 1;
    min-width: 0;
  }
  .family-photos-container .date-input-group input {
    min-width: 0;
    width: 100%;
  }
  .family-photos-container .photo-title {
    font-size: 0.8125rem;
  }
  .family-photos-container .photo-add,
  .family-photos-container .photo-toolbar .btn {
    padding: 10px 12px;
    font-size: 0.875rem;
  }
}
`);
