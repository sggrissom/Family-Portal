import { block } from "vlens/css";

block(`
.add-milestone-container {
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
.add-milestone-page {
  width: 100%;
}
`);

block(`
.milestone-preview {
  margin-top: 30px;
  padding: 20px;
  background: var(--color-card-bg);
  border-radius: 12px;
  border: 2px solid var(--color-primary);
}
`);
block(`
.milestone-preview h3 {
  margin: 0 0 12px 0;
  color: var(--color-primary);
  font-size: 16px;
  font-weight: 600;
}
`);
block(`
.milestone-preview p {
  margin: 0;
  color: var(--color-text);
  line-height: 1.5;
}
`);
block(`
.milestone-preview strong {
  color: var(--color-text-emphasis);
}
`);

block(`
.add-milestone-page textarea {
  resize: vertical;
  min-height: 80px;
  font-family: inherit;
}
`);
block(`
.add-milestone-page .form-hint {
  display: block;
  margin-top: 6px;
  color: var(--color-text-muted);
  font-size: 14px;
  line-height: 1.4;
}
`);

block(`
.photo-upload {
  padding: 12px;
  border: 1px dashed var(--color-border);
  border-radius: 12px;
  background: var(--color-card-bg);
}
`);

block(`
.photo-select {
  padding: 12px;
  margin-bottom: 12px;
  border: 1px solid var(--color-border);
  border-radius: 12px;
  background: var(--color-card-bg);
}
`);

block(`
.photo-select-label {
  display: block;
  margin-bottom: 6px;
  font-weight: 600;
  color: var(--color-text);
}
`);

block(`
.photo-select-input {
  display: block;
  width: 100%;
  padding: 8px;
  border-radius: 8px;
  border: 1px solid var(--color-border);
  background: var(--color-bg);
  color: var(--color-text);
}
`);

block(`
.photo-upload-input {
  display: block;
  width: 100%;
  padding: 8px;
  border-radius: 8px;
  border: 1px solid var(--color-border);
  background: var(--color-bg);
}
`);

block(`
.photo-upload-hint {
  margin: 8px 0 0;
  color: var(--color-text-muted);
  font-size: 14px;
  line-height: 1.4;
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
@media (max-width: 580px) {
  .add-milestone-container {
    padding: 30px 16px;
  }

  .milestone-preview {
    margin-top: 24px;
    padding: 16px;
  }

  .add-milestone-page .form-hint {
    font-size: 13px;
  }

  .photo-upload-hint {
    font-size: 13px;
  }
}
`);
