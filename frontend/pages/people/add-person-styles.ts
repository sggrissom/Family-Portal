import { block } from "vlens/css";

block(`
.add-person-container {
  max-width: 500px;
  padding: 40px 20px;
  margin: 0 auto;
  min-height: calc(100vh - 200px);
  display: flex;
  align-items: center;
  justify-content: center;
}
`);

block(`
.add-person-page {
  width: 100%;
}
`);

block(`
.add-person-container .form-actions {
  display: flex;
  gap: 16px;
  margin-top: 8px;
}
`);

block(`
@media (max-width: 768px) {
  .add-person-container {
    padding: 24px 16px;
  }

  .form-actions {
    flex-direction: column;
  }
}
`);

block(`
.add-person-page .checkbox-option {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 12px 14px;
  border: 1px solid var(--border);
  border-radius: 8px;
  background: var(--bg);
  color: var(--text);
  font-weight: 600;
}
`);

block(`
.add-person-page .checkbox-option input {
  width: 18px;
  height: 18px;
}
`);

block(`
.born-now-card {
  display: flex;
  justify-content: space-between;
  gap: 16px;
  align-items: center;
  padding: 16px;
  border-radius: 10px;
  background: var(--accent-soft);
  border: 1px solid var(--accent-soft-border);
}
`);

block(`
.born-now-card p {
  margin: 4px 0 0;
  color: var(--muted);
  font-size: 0.9rem;
}
`);

block(`
@media (max-width: 520px) {
  .born-now-card {
    align-items: stretch;
    flex-direction: column;
  }
}
`);
