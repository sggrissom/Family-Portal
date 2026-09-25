import { block } from "vlens/css";

block(`
.auth-methods {
  display: flex;
  flex-direction: column;
  gap: 20px;
}
`);

block(`
.auth-methods + .auth-form {
  margin-top: 20px;
}
`);

block(`
.btn-oauth {
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
.btn-oauth:hover {
  background: var(--hover-bg);
  border-color: var(--muted);
  transform: translateY(-1px);
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.1);
}
`);

block(`
.btn-apple {
  background: #000;
  border-color: #000;
  color: #fff;
}
`);

block(`
.btn-apple:hover {
  background: #1a1a1a;
  border-color: #1a1a1a;
  color: #fff;
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
@media (max-width: 480px) {
  .btn-oauth {
    font-size: 0.95rem;
    padding: 14px 24px;
  }
}
`);
