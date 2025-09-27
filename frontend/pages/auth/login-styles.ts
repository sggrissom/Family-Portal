import { block } from "vlens/css";

// Login Page Styles
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
.auth-methods {
  display: flex;
  flex-direction: column;
  gap: 20px;
}
`);

block(`
.btn-google {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 12px;
  padding: 12px 24px;
  border: 1px solid var(--border);
  border-radius: 8px;
  background: var(--surface);
  color: var(--text);
  font-size: 1rem;
  font-weight: 500;
  cursor: pointer;
  transition: all var(--transition-speed) ease;
  min-height: 48px;
}
`);

block(`
.btn-google:hover {
  background: var(--hover-bg);
  border-color: var(--muted);
  transform: translateY(-1px);
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.1);
}
`);

block(`
.auth-divider {
  position: relative;
  text-align: center;
  margin: 8px 0;
}
`);

block(`
.auth-divider::before {
  content: '';
  position: absolute;
  top: 50%;
  left: 0;
  right: 0;
  height: 1px;
  background: var(--border);
}
`);

block(`
.auth-divider span {
  background: var(--surface);
  color: var(--muted);
  padding: 0 16px;
  font-size: 0.9rem;
  position: relative;
  z-index: 1;
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
.checkbox-label {
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

  .btn-google {
    font-size: 0.95rem;
    padding: 14px 24px;
  }
}
`);
