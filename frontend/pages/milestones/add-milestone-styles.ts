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
}
`);
