import { block } from "vlens/css";

block(`
.login-container {
  max-width: 420px;
  padding: 40px 20px;
  margin: 0 auto;
  min-height: calc(100vh - 200px);
  display: flex;
  align-items: center;
  justify-content: center;
}
`);

block(`
.login-page {
  width: 100%;
}
`);

block(`
.form-options {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-top: 4px;
}
`);

block(`
.login-container .checkbox-label {
  display: flex;
  align-items: center;
  gap: 8px;
  cursor: pointer;
  font-size: 0.9rem;
}
`);

block(`
.checkbox-label input[type="checkbox"] {
  width: 16px;
  height: 16px;
  accent-color: var(--accent);
}
`);

block(`
.checkbox-text {
  color: var(--text);
  user-select: none;
}
`);

block(`
@media (max-width: 480px) {
  .login-container {
    padding: 20px 16px;
  }

  .form-options {
    flex-direction: column;
    gap: 12px;
    align-items: flex-start;
  }
}
`);
