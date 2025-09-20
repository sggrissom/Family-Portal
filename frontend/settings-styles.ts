import { block } from "vlens/css";

block(`
.settings-container {
  max-width: 800px;
  margin: 0 auto;
  padding: 2rem 1rem;
}
`);

block(`
.settings-page {
  display: flex;
  flex-direction: column;
  gap: 2rem;
}
`);

block(`
.settings-header {
  text-align: center;
  margin-bottom: 2rem;
}
`);

block(`
.settings-header h1 {
  font-size: 2.5rem;
  font-weight: 700;
  color: var(--text);
  margin-bottom: 0.5rem;
}
`);

block(`
.settings-header p {
  font-size: 1.125rem;
  color: var(--muted);
}
`);

block(`
.settings-sections {
  display: flex;
  flex-direction: column;
  gap: 2rem;
}
`);

block(`
.settings-section h2 {
  font-size: 1.5rem;
  font-weight: 600;
  color: var(--text);
  margin-bottom: 1rem;
}
`);

block(`
.settings-card {
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 8px;
  padding: 1.5rem;
}
`);

block(`
.section-description {
  color: var(--muted);
  margin-bottom: 1.5rem;
  line-height: 1.5;
}
`);

block(`
.readonly-field {
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 4px;
  padding: 0.75rem;
  color: var(--text);
  font-size: 1rem;
}
`);

block(`
.invite-code-display {
  display: flex;
  align-items: center;
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 4px;
  padding: 0.75rem;
}
`);

block(`
.invite-code {
  font-family: 'Monaco', 'Menlo', 'Ubuntu Mono', monospace;
  font-size: 1.125rem;
  font-weight: 600;
  color: var(--primary-accent);
  letter-spacing: 0.05em;
}
`);

block(`
.invite-link-display {
  display: flex;
  gap: 0.5rem;
}
`);

block(`
.invite-link-input {
  flex: 1;
  padding: 0.75rem;
  border: 1px solid var(--border);
  border-radius: 4px;
  background: var(--surface);
  color: var(--text);
  font-size: 0.875rem;
  font-family: 'Monaco', 'Menlo', 'Ubuntu Mono', monospace;
}
`);

block(`
.copy-button {
  min-width: 100px;
  transition: all 0.2s ease;
}
`);

block(`
.copy-button.copied {
  background: var(--accent);
  border-color: var(--accent);
  color: var(--button-text);
}
`);

block(`
.invite-instructions {
  margin-top: 1.5rem;
  padding-top: 1.5rem;
  border-top: 1px solid var(--border);
}
`);

block(`
.invite-instructions h4 {
  color: var(--text);
  margin-bottom: 0.75rem;
  font-size: 1rem;
  font-weight: 600;
}
`);

block(`
.invite-instructions ol {
  color: var(--muted);
  padding-left: 1.25rem;
  line-height: 1.6;
}
`);

block(`
.invite-instructions li {
  margin-bottom: 0.5rem;
}
`);

block(`
@media (max-width: 600px) {
  .settings-container {
    padding: 1rem 0.75rem;
  }

  .settings-header h1 {
    font-size: 2rem;
  }

  .invite-link-display {
    flex-direction: column;
  }

  .copy-button {
    min-width: auto;
  }
}
`);