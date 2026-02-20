import { block } from "vlens/css";

// Add Milestone Page Styles
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
.milestone-photo-picker {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  padding: 8px;
  border: 1px solid var(--color-border);
  border-radius: 12px;
  background: var(--color-card-bg);
  max-height: 240px;
  overflow-y: auto;
}
`);

block(`
.milestone-photo-picker-empty {
  color: var(--color-text-muted);
  font-size: 14px;
  padding: 4px;
  margin: 0;
}
`);

block(`
.milestone-photo-picker-item {
  position: relative;
  cursor: pointer;
  border-radius: 6px;
  overflow: hidden;
  border: 2px solid transparent;
  width: 72px;
  height: 72px;
  flex-shrink: 0;
}
`);

block(`
.milestone-photo-picker-item.selected {
  border-color: var(--color-primary);
}
`);

block(`
.milestone-photo-picker-img {
  width: 72px;
  height: 72px;
  object-fit: cover;
  display: block;
}
`);

block(`
.milestone-photo-picker-check {
  position: absolute;
  top: 4px;
  right: 4px;
  width: 18px;
  height: 18px;
  background: var(--color-primary);
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  color: white;
  font-size: 11px;
  font-weight: bold;
  line-height: 1;
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
