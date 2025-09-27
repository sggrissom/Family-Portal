import { block } from "vlens/css";

// Add Photo Page Styles
block(`
.add-photo-container {
  max-width: 580px;
  padding: 40px 20px;
  margin: 0 auto;
  background: var(--color-bg);
  min-height: calc(100vh - 200px);
  display: flex;
  align-items: center;
  justify-content: center;
}
`);

block(`
.add-photo-page {
  width: 100%;
}
`);

// File Upload Area
block(`
.file-upload-area {
  border: 2px dashed var(--color-border);
  border-radius: 12px;
  padding: 40px 20px;
  text-align: center;
  background: var(--color-card-bg);
  transition: all 0.3s ease;
  cursor: pointer;
}
`);

block(`
.file-upload-area:hover {
  border-color: var(--color-primary);
  background: var(--color-surface);
}
`);

block(`
.file-upload-area.drag-active {
  border-color: var(--color-primary);
  background: var(--color-primary-light);
  transform: scale(1.02);
}
`);

block(`
.file-upload-area.has-file {
  border-color: var(--color-success);
  background: var(--color-surface);
  padding: 20px;
}
`);

// Upload Prompt
block(`
.upload-prompt {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12px;
}
`);

block(`
.upload-icon {
  font-size: 48px;
  opacity: 0.7;
}
`);

block(`
.upload-prompt p {
  margin: 0;
  color: var(--color-text);
  font-size: 16px;
}
`);

block(`
.upload-link {
  color: var(--color-primary);
  text-decoration: underline;
  cursor: pointer;
  font-weight: 500;
}
`);

block(`
.upload-link:hover {
  color: var(--color-primary-dark);
}
`);

block(`
.upload-prompt small {
  color: var(--color-text-muted);
  font-size: 14px;
}
`);

// File Preview
block(`
.file-preview {
  display: flex;
  align-items: center;
  gap: 20px;
  text-align: left;
}
`);

block(`
.preview-image {
  width: 100px;
  height: 100px;
  object-fit: cover;
  border-radius: 8px;
  border: 2px solid var(--color-border);
  flex-shrink: 0;
}
`);

block(`
.file-info {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 8px;
}
`);

block(`
.file-name {
  margin: 0;
  font-weight: 500;
  color: var(--color-text-emphasis);
  word-break: break-word;
}
`);

block(`
.file-size {
  margin: 0;
  color: var(--color-text-muted);
  font-size: 14px;
}
`);

block(`
.remove-file {
  background: none;
  border: none;
  color: var(--color-danger);
  text-decoration: underline;
  cursor: pointer;
  font-size: 14px;
  padding: 0;
  align-self: flex-start;
}
`);

block(`
.remove-file:hover {
  color: var(--color-danger-dark);
}
`);

// Photo Preview
block(`
.photo-preview {
  margin-top: 30px;
  padding: 20px;
  background: var(--color-card-bg);
  border-radius: 12px;
  border: 2px solid var(--color-primary);
}
`);

block(`
.photo-preview h3 {
  margin: 0 0 12px 0;
  color: var(--color-primary);
  font-size: 16px;
  font-weight: 600;
}
`);

block(`
.photo-preview p {
  margin: 0 0 8px 0;
  color: var(--color-text);
  line-height: 1.5;
}
`);

block(`
.photo-preview p:last-child {
  margin-bottom: 0;
}
`);

block(`
.photo-preview strong {
  color: var(--color-text-emphasis);
}
`);

block(`
.preview-description {
  font-style: italic;
  color: var(--color-text-muted);
}
`);

// Form Enhancements
block(`
.add-photo-page textarea {
  resize: vertical;
  min-height: 80px;
  font-family: inherit;
}
`);

block(`
.add-photo-page .form-hint {
  display: block;
  margin-top: 6px;
  color: var(--color-text-muted);
  font-size: 14px;
  line-height: 1.4;
}
`);

// Responsive Design
block(`
@media (max-width: 580px) {
  .add-photo-container {
    padding: 30px 16px;
  }

  .file-upload-area {
    padding: 30px 16px;
  }

  .file-preview {
    flex-direction: column;
    text-align: center;
    gap: 16px;
  }

  .preview-image {
    width: 120px;
    height: 120px;
    align-self: center;
  }

  .photo-preview {
    margin-top: 24px;
    padding: 16px;
  }

  .upload-icon {
    font-size: 36px;
  }

  .upload-prompt p {
    font-size: 15px;
  }
}
`);
