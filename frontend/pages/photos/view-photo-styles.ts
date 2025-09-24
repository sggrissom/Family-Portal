import { block } from "vlens/css";

// View Photo Page Container
block(`
.view-photo-container {
  max-width: 1200px;
  margin: 0 auto;
  padding: 2rem;
}
`);

block(`
.view-photo-page {
  display: flex;
  flex-direction: column;
  gap: 2rem;
}
`);

// Header with back navigation
block(`
.photo-header {
  display: flex;
  align-items: center;
  margin-bottom: 1rem;
}
`);

block(`
.back-link {
  color: var(--color-primary);
  text-decoration: none;
  font-weight: 500;
  display: flex;
  align-items: center;
  gap: 0.5rem;
  padding: 0.5rem 1rem;
  border-radius: 6px;
  transition: background-color 0.2s ease;
}
`);

block(`
.back-link:hover {
  background-color: var(--color-background-subtle);
  text-decoration: none;
}
`);

// Main photo display
block(`
.photo-display {
  display: flex;
  justify-content: center;
  background: var(--surface);
  border-radius: 12px;
  padding: 1rem;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.1);
}
`);

block(`
.photo-main-image {
  max-width: 100%;
  max-height: 70vh;
  object-fit: contain;
  border-radius: 8px;
  box-shadow: 0 4px 16px rgba(0, 0, 0, 0.15);
}
`);

// Photo information panel
block(`
.photo-info-panel {
  background: var(--surface);
  border-radius: 12px;
  padding: 2rem;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.1);
  display: grid;
  grid-template-columns: 1fr auto;
  gap: 2rem;
  align-items: start;
}
`);

block(`
.photo-metadata {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}
`);

block(`
.photo-title {
  font-size: 2rem;
  font-weight: 700;
  color: var(--color-text-emphasis);
  margin: 0;
  line-height: 1.2;
}
`);

block(`
.photo-date {
  font-size: 1.1rem;
  color: var(--color-text-muted);
  display: flex;
  align-items: center;
  gap: 0.5rem;
}
`);

block(`
.photo-description {
  font-size: 1rem;
  color: var(--color-text);
  line-height: 1.6;
  background: var(--color-background-subtle);
  padding: 1rem;
  border-radius: 8px;
  border-left: 4px solid var(--color-primary);
}
`);

block(`
.photo-details {
  font-size: 0.875rem;
  color: var(--color-text-muted);
  padding-top: 1rem;
  border-top: 1px solid var(--color-border);
}
`);

// Action buttons
block(`
.photo-actions {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
  min-width: 120px;
}
`);

block(`
.photo-actions .btn {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 0.5rem;
  padding: 0.75rem 1rem;
  font-size: 0.875rem;
  border-radius: 6px;
  text-decoration: none;
  border: none;
  cursor: pointer;
  transition: all 0.2s ease;
}
`);

block(`
.btn-secondary {
  background: var(--color-background-subtle);
  color: var(--color-text);
  border: 1px solid var(--color-border);
}
`);

block(`
.btn-secondary:hover {
  background: var(--color-background);
  border-color: var(--color-primary);
  text-decoration: none;
}
`);

block(`
.btn-danger {
  background: #dc3545;
  color: white;
  border: 1px solid #dc3545;
}
`);

block(`
.btn-danger:hover {
  background: #c82333;
  border-color: #c82333;
}
`);

// Error page
block(`
.error-page {
  text-align: center;
  padding: 3rem 1rem;
}
`);

block(`
.error-page h1 {
  color: var(--color-text-emphasis);
  margin-bottom: 1rem;
}
`);

block(`
.error-page p {
  color: var(--color-text-muted);
  margin-bottom: 2rem;
}
`);

// Responsive design
block(`
@media (max-width: 768px) {
  .view-photo-container {
    padding: 1rem;
  }

  .photo-info-panel {
    grid-template-columns: 1fr;
    gap: 1.5rem;
    padding: 1.5rem;
  }

  .photo-title {
    font-size: 1.5rem;
  }

  .photo-actions {
    flex-direction: row;
    min-width: auto;
  }

  .photo-main-image {
    max-height: 50vh;
  }
}
`);

block(`
@media (max-width: 480px) {
  .view-photo-container {
    padding: 0.5rem;
  }

  .photo-info-panel {
    padding: 1rem;
  }

  .photo-actions .btn {
    font-size: 0.8rem;
    padding: 0.5rem 0.75rem;
  }
}
`);