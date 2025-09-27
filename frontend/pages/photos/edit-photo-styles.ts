import { block } from "vlens/css";

// Edit Photo Page Container
block(`
.edit-photo-container {
  max-width: 600px;
  margin: 0 auto;
  padding: 2rem;
  min-height: 100vh;
  overflow-y: auto;
}
`);

block(`
.edit-photo-page {
  display: flex;
  flex-direction: column;
  gap: 2rem;
}
`);

// Ensure auth-card doesn't restrict content
block(`
.edit-photo-page .auth-card {
  max-height: none;
  overflow: visible;
  padding-bottom: 2rem;
}
`);

// Photo preview section
block(`
.photo-preview {
  display: flex;
  gap: 1.5rem;
  align-items: flex-start;
  background: var(--surface);
  padding: 1.5rem;
  border-radius: 8px;
  margin-bottom: 1rem;
}
`);

block(`
.photo-preview .preview-image {
  width: 120px;
  height: 120px;
  object-fit: cover;
  border-radius: 8px;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.1);
}
`);

block(`
.photo-info {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  font-size: 0.875rem;
  color: var(--muted);
}
`);

block(`
.photo-info strong {
  color: var(--text);
}
`);

// Form actions
block(`
.form-actions {
  display: flex;
  gap: 1rem;
  justify-content: flex-end;
  padding: 1rem 0;
  margin-top: 1rem;
  border-top: 1px solid var(--border);
  background: var(--surface);
  position: relative;
  z-index: 1;
}
`);

// Remove duplicate button styles - use global styles from styles.ts instead

// Error page
block(`
.error-page {
  text-align: center;
  padding: 3rem 1rem;
}
`);

block(`
.error-page h1 {
  color: var(--text);
  margin-bottom: 1rem;
}
`);

block(`
.error-page p {
  color: var(--muted);
  margin-bottom: 2rem;
}
`);

// Responsive design
block(`
@media (max-width: 768px) {
  .edit-photo-container {
    padding: 1rem;
  }

  .photo-preview {
    flex-direction: column;
    align-items: center;
    text-align: center;
  }

  .form-actions {
    flex-direction: column-reverse;
  }

  .form-actions .btn {
    width: 100%;
  }
}
`);

block(`
@media (max-width: 480px) {
  .edit-photo-container {
    padding: 0.5rem;
  }

  .photo-preview {
    padding: 1rem;
  }

  .photo-preview .preview-image {
    width: 100px;
    height: 100px;
  }
}
`);
