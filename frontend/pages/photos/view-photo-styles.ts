import { block } from "vlens/css";

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

block(`
.photo-header {
  display: flex;
  align-items: center;
  margin-bottom: 1rem;
}
`);

block(`
.view-photo-container .back-link {
  color: var(--accent);
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
.view-photo-container .back-link:hover {
  background-color: var(--hover-bg);
  text-decoration: none;
}
`);

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
.view-photo-title {
  font-size: 2rem;
  font-weight: 700;
  color: var(--text);
  margin: 0;
  line-height: 1.2;
}
`);

block(`
.view-photo-date {
  font-size: 1.1rem;
  color: var(--muted);
  display: flex;
  align-items: center;
  gap: 0.5rem;
}
`);

block(`
.view-photo-description {
  font-size: 1rem;
  color: var(--text);
  line-height: 1.6;
  background: var(--hover-bg);
  padding: 1rem;
  border-radius: 8px;
  border-left: 4px solid var(--accent);
}
`);

block(`
.photo-details {
  font-size: 0.875rem;
  color: var(--muted);
  padding-top: 1rem;
  border-top: 1px solid var(--border);
}
`);

block(`
.photo-actions {
  display: flex;
  flex-direction: column;
  gap: 1rem;
  min-width: 220px;
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
  width: 100%;
}
`);

block(`
.profile-photo-actions {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
}
`);

block(`
.profile-photo-actions h4 {
  margin: 0;
  font-size: 1rem;
  color: var(--text);
}
`);

block(`
.profile-action {
  width: 100%;
}
`);

block(`
.photo-actions .btn-sm {
  min-height: 40px;
}
`);

block(`
.btn-success {
  background: #198754;
  color: white;
  border: 1px solid #198754;
}
`);

block(`
.btn-success:hover {
  background: #157347;
  border-color: #146c43;
}
`);

block(`.photo-tags { margin-top: 1rem; }`);
block(`.tag-list { display: flex; flex-wrap: wrap; gap: 0.5rem; margin-top: 0.5rem; }`);
block(`
.tag-pill-view {
  display: inline-flex;
  align-items: center;
  gap: 0.4rem;
  padding: 0.25rem 0.65rem;
  border-radius: 999px;
  border: 2px solid transparent;
  background: var(--surface);
  font-size: 0.8rem;
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
@media (max-width: 768px) {
  .view-photo-container {
    padding: 1rem;
  }

  .photo-info-panel {
    grid-template-columns: 1fr;
    gap: 1.5rem;
    padding: 1.5rem;
  }

  .view-photo-title {
    font-size: 1.5rem;
  }

  .photo-actions {
    min-width: auto;
    width: 100%;
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

  .photo-actions {
    gap: 0.75rem;
  }
}
`);
