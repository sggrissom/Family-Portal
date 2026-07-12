import { block } from "vlens/css";

// Add Person Page Styles
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
.form-actions {
  display: flex;
  gap: 16px;
  margin-top: 8px;
}
`);

block(`
.form-actions .btn {
  flex: 1;
}
`);

block(`
@media (max-width: 768px) {
  .add-person-container {
    padding: 30px 16px;
  }

  .form-actions {
    flex-direction: column;
  }
}
`);

block(`
.checkbox-option {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 12px 14px;
  border: 1px solid rgba(102, 126, 234, 0.25);
  border-radius: 12px;
  background: rgba(102, 126, 234, 0.08);
  color: #4a5568;
  font-weight: 600;
}
`);

block(`
.checkbox-option input {
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
  border-radius: 14px;
  background: linear-gradient(135deg, rgba(72, 187, 120, 0.14), rgba(56, 178, 172, 0.12));
  border: 1px solid rgba(56, 178, 172, 0.25);
}
`);

block(`
.born-now-card p {
  margin: 4px 0 0;
  color: #4a5568;
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
