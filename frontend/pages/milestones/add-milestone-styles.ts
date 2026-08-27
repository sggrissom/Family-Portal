import { block } from "vlens/css";

block(`
.add-milestone-container {
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
.add-milestone-page {
  width: 100%;
}
`);

block(`
.milestone-preview {
  margin-top: 30px;
  padding: 20px;
  background: var(--surface);
  border-radius: 12px;
  border: 2px solid var(--accent);
}
`);
block(`
.milestone-preview h3 {
  margin: 0 0 12px 0;
  color: var(--accent);
  font-size: 16px;
  font-weight: 600;
}
`);
block(`
.milestone-preview p {
  margin: 0;
  color: var(--text);
  line-height: 1.5;
}
`);
block(`
.milestone-preview strong {
  color: var(--text);
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
  color: var(--muted);
  font-size: 14px;
  line-height: 1.4;
}
`);

block(`
.photo-upload {
  padding: 12px;
  border: 1px dashed var(--border);
  border-radius: 12px;
  background: var(--surface);
}
`);

block(`
.photo-select {
  padding: 12px;
  margin-bottom: 12px;
  border: 1px solid var(--border);
  border-radius: 12px;
  background: var(--surface);
}
`);

block(`
.photo-select-label {
  display: block;
  margin-bottom: 6px;
  font-weight: 600;
  color: var(--text);
}
`);

block(`
.photo-select-input {
  display: block;
  width: 100%;
  padding: 8px;
  border-radius: 8px;
  border: 1px solid var(--border);
  background: var(--bg);
  color: var(--text);
}
`);

block(`
.photo-upload-input {
  display: block;
  width: 100%;
  padding: 8px;
  border-radius: 8px;
  border: 1px solid var(--border);
  background: var(--bg);
}
`);

block(`
.photo-upload-hint {
  margin: 8px 0 0;
  color: var(--muted);
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
