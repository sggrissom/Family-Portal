import { block } from "vlens/css";

// Growth Form Page Styles (Add Growth & Edit Growth)
block(`
.add-growth-container {
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
.add-growth-page {
  width: 100%;
}
`);

block(`
.measurement-preview {
  margin-top: 24px;
  padding: 20px;
  background: var(--bg);
  border: 1px solid var(--border);
  border-radius: 12px;
}
`);

block(`
.measurement-preview h3 {
  margin: 0 0 12px;
  color: var(--text);
  font-size: 1.1rem;
  font-weight: 600;
}
`);

block(`
.measurement-preview p {
  margin: 0;
  color: var(--muted);
  line-height: 1.5;
}
`);

block(`
.measurement-preview strong {
  color: var(--text);
}
`);

// Mobile responsive styles for growth pages
block(`
@media (max-width: 580px) {
  .add-growth-container {
    padding: 30px 16px;
  }

  .measurement-preview {
    padding: 16px;
  }
}
`);

block(`
.growth-form {
  gap: 18px;
}
`);

block(`
.growth-choice-group {
  border: 0;
  margin: 0;
  padding: 0;
}
`);

block(`
.growth-choice-group legend {
  color: var(--text);
  font-size: 0.9rem;
  font-weight: 600;
  margin: 0 0 10px;
  padding: 0;
}
`);

block(`
.growth-form .form-row {
  align-items: start;
}
`);

block(`
@media (max-width: 580px) {
  .add-growth-container {
    padding: 24px 16px 32px;
    align-items: flex-start;
  }

  .growth-form {
    gap: 16px;
  }
}
`);
