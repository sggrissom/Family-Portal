import { block } from "vlens/css";

// Import shared photo styles
import "./photos-styles";

// Family Photos Page Styles
block(`
.family-photos-container {
  padding: 2rem;
  max-width: 1200px;
  margin: 0 auto;
}
`);

block(`
.family-photos-page {
  min-height: 70vh;
}
`);

// Page Header
block(`
.page-header {
  margin-bottom: 2rem;
  border-bottom: 1px solid var(--border);
  padding-bottom: 1.5rem;
}
`);

block(`
.header-content {
  display: flex;
  justify-content: space-between;
  align-items: flex-end;
  gap: 1rem;
}
`);

block(`
.page-header h1 {
  margin: 0 0 0.5rem 0;
  font-size: 2rem;
  font-weight: 700;
  color: var(--text);
}
`);

block(`
.photos-count {
  color: var(--muted);
  font-size: 0.875rem;
  font-weight: 500;
}
`);

block(`
.header-actions {
  flex-shrink: 0;
  display: flex;
  gap: 0.75rem;
  align-items: center;
}
`);

// Filter Panel
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
.filter-section {
  margin-bottom: 1.5rem;
}
`);

block(`
.filter-section:last-of-type {
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
.person-checkbox {
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
.person-checkbox:hover {
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
.filter-toggle {
  white-space: nowrap;
}
`);

block(`
.loading-state {
  padding: 1rem;
  text-align: center;
  color: var(--muted);
  font-style: italic;
}
`);

// People Badges Container
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

// Person Badge on Photos
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

// Responsive styles
block(`
@media (max-width: 768px) {
  .header-content {
    flex-direction: column;
    align-items: flex-start;
    gap: 1rem;
  }

  .header-actions {
    width: 100%;
    justify-content: space-between;
  }

  .filter-panel {
    padding: 1rem;
  }

  .people-filter {
    gap: 0.5rem;
  }

  .person-checkbox {
    padding: 0.375rem 0.5rem;
    font-size: 0.8125rem;
  }

  .date-filter {
    flex-direction: column;
    gap: 0.75rem;
  }

  .date-input-group {
    min-width: auto;
  }

  .filter-actions {
    flex-direction: column-reverse;
    gap: 0.5rem;
  }

  .filter-actions button {
    width: 100%;
  }
}
`);

// Empty State
block(`
.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  text-align: center;
  padding: 4rem 2rem;
  min-height: 400px;
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

// Responsive adjustments
block(`
@media (max-width: 768px) {
  .family-photos-container {
    padding: 1rem;
  }

  .page-header h1 {
    font-size: 1.75rem;
  }

  .header-content {
    flex-direction: column;
    align-items: flex-start;
    gap: 1rem;
  }

  .header-actions {
    align-self: stretch;
  }

  .header-actions .btn {
    width: 100%;
    text-align: center;
  }

  .photos-gallery {
    grid-template-columns: repeat(auto-fill, minmax(150px, 1fr));
    gap: 15px;
  }

  .photo-image-container {
    height: 150px;
  }

  .empty-state {
    padding: 2rem 1rem;
    min-height: 300px;
  }

  .empty-icon {
    font-size: 3rem;
  }

  .empty-state h2 {
    font-size: 1.25rem;
  }
}
`);

// Small mobile screens
block(`
@media (max-width: 480px) {
  .photos-gallery {
    grid-template-columns: repeat(auto-fill, minmax(120px, 1fr));
    gap: 10px;
  }

  .photo-image-container {
    height: 120px;
  }

  .photo-info {
    padding: 8px 12px;
  }

  .photo-title {
    font-size: 12px;
  }

  .photo-date,
  .photo-description {
    font-size: 11px;
  }

  .person-badge,
  .profile-photo-badge {
    font-size: 10px;
    padding: 2px 6px;
  }
}
`);
