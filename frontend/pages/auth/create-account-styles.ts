import { block } from "vlens/css";

block(`
.create-account-container {
  max-width: 480px;
  padding: 40px 20px;
  margin: 0 auto;
  min-height: calc(100vh - 200px);
  display: flex;
  align-items: center;
  justify-content: center;
}
`);

block(`
.create-account-page {
  width: 100%;
}
`);

block(`
.auth-card {
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 16px;
  padding: 40px;
  box-shadow: 0 4px 20px rgba(0, 0, 0, 0.08);
}
`);

block(`
.auth-header {
  text-align: center;
  margin-bottom: 32px;
}
`);

block(`
.auth-header h1 {
  font-size: 2rem;
  margin: 0 0 8px;
  color: var(--text);
}
`);

block(`
.auth-header p {
  color: var(--muted);
  margin: 0;
  font-size: 1rem;
}
`);

block(`
.auth-form {
  display: flex;
  flex-direction: column;
  gap: 20px;
}
`);

block(`
.auth-submit {
  margin-top: 8px;
  width: 100%;
  justify-content: center;
}
`);

block(`
.auth-footer {
  margin-top: 24px;
  text-align: center;
  padding-top: 24px;
  border-top: 1px solid var(--border);
}
`);

block(`
.auth-footer p {
  color: var(--muted);
  margin: 0;
}
`);

block(`
.auth-link {
  color: var(--accent);
  text-decoration: none;
  font-weight: 600;
  margin-left: 8px;
  transition: color var(--transition-speed) ease;
}
`);

block(`
.auth-link:hover {
  color: var(--primary-accent);
  text-decoration: underline;
}
`);

block(`
@media (max-width: 480px) {
  .create-account-container {
    padding: 20px 16px;
  }

  .auth-card {
    padding: 32px 24px;
  }

  .auth-header h1 {
    font-size: 1.8rem;
  }
}
`);

block(`
.form-section {
  border: 1px solid var(--border);
  border-radius: 12px;
  padding: 20px;
  display: flex;
  flex-direction: column;
  gap: 16px;
}
`);

block(`
.form-section legend {
  color: var(--text);
  font-weight: 700;
  padding: 0 8px;
}
`);

block(`
.section-hint {
  color: var(--muted);
  font-size: 0.95rem;
  line-height: 1.5;
  margin: 0;
}
`);

block(`
.auth-consent {
  margin: 16px 0 0;
  font-size: 0.85rem;
  line-height: 1.5;
  color: var(--muted);
  text-align: center;
}
`);
