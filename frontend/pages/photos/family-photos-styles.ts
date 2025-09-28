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
