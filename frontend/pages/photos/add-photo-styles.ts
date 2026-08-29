import { block } from "vlens/css";

block(`
.add-photo-container {
  max-width: 580px;
  padding: 40px 20px;
  margin: 0 auto;
  background: var(--bg);
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

block(`
.file-upload-area {
  border: 2px dashed var(--border);
  border-radius: 12px;
  padding: 40px 20px;
  text-align: center;
  background: var(--surface);
  transition: all 0.3s ease;
  cursor: pointer;
}
`);

block(`
.file-upload-area:hover {
  border-color: var(--accent);
  background: var(--surface);
}
`);

block(`
.file-upload-area.drag-active {
  border-color: var(--accent);
  background: var(--hover-bg);
  transform: scale(1.02);
}
`);

block(`
.file-upload-area.has-file {
  border-color: var(--success);
  background: var(--surface);
  padding: 20px;
}
`);

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
  color: var(--text);
  font-size: 16px;
}
`);

block(`
.upload-link {
  color: var(--accent);
  text-decoration: underline;
  cursor: pointer;
  font-weight: 500;
}
`);

block(`
.upload-link:hover {
  color: var(--primary-accent);
}
`);

block(`
.upload-prompt small {
  color: var(--muted);
  font-size: 14px;
}
`);

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
  border: 2px solid var(--border);
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
  color: var(--text);
  word-break: break-word;
}
`);

block(`
.file-size {
  margin: 0;
  color: var(--muted);
  font-size: 14px;
}
`);

block(`
.remove-file {
  background: none;
  border: none;
  color: var(--danger);
  text-decoration: underline;
  cursor: pointer;
  font-size: 14px;
  padding: 0;
  align-self: flex-start;
}
`);

block(`
.remove-file:hover {
  color: var(--danger-hover);
}
`);

block(`
.add-photo-container .photo-preview {
  margin-top: 30px;
  padding: 20px;
  background: var(--surface);
  border-radius: 12px;
  border: 2px solid var(--accent);
}
`);

block(`
.photo-preview h3 {
  margin: 0 0 12px 0;
  color: var(--accent);
  font-size: 16px;
  font-weight: 600;
}
`);

block(`
.photo-preview p {
  margin: 0 0 8px 0;
  color: var(--text);
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
  color: var(--text);
}
`);

block(`
.preview-description {
  font-style: italic;
  color: var(--muted);
}
`);

block(`
.tag-picker {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
}
`);

block(`
.tag-pill {
  display: inline-flex;
  font-family: inherit;
  color: inherit;
  align-items: center;
  gap: 0.4rem;
  padding: 0.3rem 0.75rem;
  border-radius: 999px;
  border: 2px solid transparent;
  background: var(--surface);
  cursor: pointer;
  font-size: 0.85rem;
  user-select: none;
  transition: background 0.15s;
}
`);

block(`
.tag-pill.selected {
  background: color-mix(in srgb, var(--surface) 60%, currentColor 40%);
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
.photo-person-group {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}
`);

block(`
.photo-person-option {
  display: inline-flex;
  align-items: center;
  gap: 10px;
  padding: 10px 14px;
  border: 1px solid var(--control-border);
  border-radius: 10px;
  background: var(--bg);
  color: var(--text);
  font-weight: 500;
  cursor: pointer;
  min-height: 44px;
}
`);

block(`
.photo-person-option:hover {
  border-color: var(--accent);
}
`);

block(`
.photo-person-option input {
  width: 18px;
  height: 18px;
  flex-shrink: 0;
}
`);

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
  color: var(--muted);
  font-size: 14px;
  line-height: 1.4;
}
`);

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
