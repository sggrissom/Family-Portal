import { block } from "vlens/css";

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
.profile-photo {
  width: 100%;
  height: 100%;
  object-fit: cover;
  border-radius: 50%;
}
`);

block(`
.profile-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 100%;
  height: 100%;
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

// Filter Controls (replacing tabs)
block(`
.profile-filters {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 20px;
  margin-bottom: 30px;
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 12px;
  padding: 16px 20px;
  flex-wrap: wrap;
}
`);

block(`
.filter-section {
  display: flex;
  align-items: center;
  gap: 12px;
}
`);

block(`
.filter-label {
  font-size: 0.9rem;
  font-weight: 600;
  color: var(--muted);
  text-transform: uppercase;
  letter-spacing: 0.05em;
  margin: 0;
}
`);

block(`
.type-filters,
.sort-controls {
  display: flex;
  gap: 8px;
}
`);

block(`
.filter-toggle {
  padding: 8px 16px;
  border: 1px solid var(--border);
  background: var(--bg);
  color: var(--muted);
  border-radius: 8px;
  cursor: pointer;
  transition: all 0.2s ease;
  font-size: 0.9rem;
  font-weight: 500;
}
`);

block(`
.filter-toggle:hover {
  background: var(--hover-bg);
  color: var(--text);
  border-color: var(--accent);
}
`);

block(`
.filter-toggle.active {
  background: var(--accent);
  color: var(--button-text);
  border-color: var(--accent);
  font-weight: 600;
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
  overflow-x: auto;
}
`);

block(`
.data-table {
  width: 100%;
  min-width: 480px;
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
  display: flex;
  gap: 8px;
  align-items: center;
  justify-content: flex-start;
}
`);

block(`
.btn-action {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  border: none;
  background: transparent;
  color: var(--muted);
  cursor: pointer;
  border-radius: 4px;
  text-decoration: none;
  font-size: 14px;
  transition: all 0.2s ease;
}
`);

block(`
.btn-action:hover {
  background: var(--hover-bg);
  color: var(--text);
  transform: scale(1.1);
}
`);

block(`
.btn-action.btn-edit:hover {
  color: var(--accent);
}
`);

block(`
.btn-action.btn-delete:hover {
  color: #dc3545;
  background: rgba(220, 53, 69, 0.1);
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
.empty-state-actions {
  display: flex;
  gap: 12px;
  justify-content: center;
  flex-wrap: wrap;
}
`);

// Unified Timeline Styles
block(`
.unified-timeline {
  display: flex;
  flex-direction: column;
  gap: 20px;
}
`);

block(`
.timeline-items {
  display: flex;
  flex-direction: column;
  gap: 16px;
}
`);

block(`
.timeline-item {
  display: flex;
  align-items: flex-start;
  gap: 16px;
  background: var(--bg);
  border: 1px solid var(--border);
  border-radius: 12px;
  padding: 20px;
  transition: all 0.2s ease;
}
`);

block(`
.timeline-item:hover {
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.08);
  border-color: var(--accent);
}
`);

block(`
.timeline-item-icon {
  font-size: 2rem;
  display: flex;
  align-items: center;
  justify-content: center;
  width: 48px;
  height: 48px;
  border-radius: 50%;
  flex-shrink: 0;
}
`);

block(`
.milestone-item .timeline-item-icon {
  background: rgba(99, 102, 241, 0.1);
}
`);

block(`
.measurement-item .timeline-item-icon {
  background: rgba(16, 185, 129, 0.1);
}
`);

block(`
.photo-item .timeline-item-icon {
  background: rgba(236, 72, 153, 0.1);
}
`);

block(`
.timeline-item-content {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 8px;
}
`);

block(`
.timeline-item-header {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}
`);

block(`
.timeline-item-type {
  font-size: 0.85rem;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  padding: 4px 10px;
  border-radius: 6px;
}
`);

block(`
.milestone-type {
  background: rgba(99, 102, 241, 0.15);
  color: rgb(99, 102, 241);
}
`);

block(`
.measurement-type {
  background: rgba(16, 185, 129, 0.15);
  color: rgb(16, 185, 129);
}
`);

block(`
.photo-type {
  background: rgba(236, 72, 153, 0.15);
  color: rgb(236, 72, 153);
}
`);

block(`
.timeline-item-age {
  font-size: 0.9rem;
  color: var(--muted);
  font-weight: 500;
}
`);

block(`
.timeline-item-date {
  font-size: 0.85rem;
  color: var(--muted);
  margin-left: auto;
}
`);

block(`
.timeline-item-description {
  font-size: 1rem;
  color: var(--text);
  line-height: 1.6;
  white-space: pre-wrap;
}
`);

block(`
.measurement-value {
  font-size: 1.2rem;
  font-weight: 600;
  color: rgb(16, 185, 129);
}
`);

block(`
.timeline-item-actions {
  display: flex;
  gap: 8px;
  flex-shrink: 0;
}
`);

block(`
.btn-action.btn-view:hover {
  color: var(--accent);
}
`);

// Photo item specific styles
block(`
.photo-item-details {
  display: flex;
  gap: 16px;
  align-items: flex-start;
}
`);

block(`
.photo-thumbnail {
  width: 120px;
  height: 120px;
  border-radius: 8px;
  overflow: hidden;
  cursor: pointer;
  transition: transform 0.2s ease;
  flex-shrink: 0;
  position: relative;
}
`);

block(`
.photo-thumbnail:hover {
  transform: scale(1.05);
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
  gap: 4px;
}
`);

block(`
.photo-title {
  font-size: 1rem;
  font-weight: 600;
  color: var(--text);
}
`);

block(`
.photo-description {
  font-size: 0.9rem;
  color: var(--muted);
  line-height: 1.5;
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

  .profile-filters {
    flex-direction: column;
    gap: 16px;
    align-items: stretch;
  }

  .filter-section {
    flex-direction: column;
    align-items: stretch;
    gap: 8px;
  }

  .type-filters,
  .sort-controls {
    flex-wrap: wrap;
  }

  .filter-toggle {
    flex: 1;
    min-width: 0;
  }

  .profile-content {
    padding: 24px 20px;
  }

  .timeline-item {
    flex-direction: column;
    gap: 12px;
  }

  .timeline-item-icon {
    width: 40px;
    height: 40px;
    font-size: 1.5rem;
  }

  .timeline-item-header {
    flex-direction: column;
    align-items: flex-start;
    gap: 6px;
  }

  .timeline-item-date {
    margin-left: 0;
  }

  .photo-item-details {
    flex-direction: column;
    gap: 12px;
  }

  .photo-thumbnail {
    width: 100%;
    height: 200px;
  }
}
`);
