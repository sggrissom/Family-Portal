import { block } from "vlens/css";

// Import Page Styles
block(`
.import-container {
  max-width: 800px;
  padding: 40px 20px;
  margin: 0 auto;
  min-height: calc(100vh - 200px);
}
`);

block(`
.import-page {
  width: 100%;
}
`);

block(`
.import-form-container {
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 16px;
  padding: 32px;
  margin-bottom: 32px;
}
`);

block(`
.import-form {
  display: flex;
  flex-direction: column;
  gap: 24px;
}
`);

block(`
.form-section {
  display: flex;
  flex-direction: column;
  gap: 12px;
}
`);

block(`
.form-section h3 {
  margin: 0;
  font-size: 1.2rem;
  color: var(--text);
}
`);

block(`
.file-input-container {
  position: relative;
}
`);

block(`
.file-input {
  position: absolute;
  opacity: 0;
  width: 1px;
  height: 1px;
  pointer-events: none;
}
`);

block(`
.file-input-label {
  display: block;
  padding: 12px 16px;
  border: 2px dashed var(--border);
  border-radius: 8px;
  background: var(--bg);
  color: var(--muted);
  text-align: center;
  cursor: pointer;
  transition: all 0.3s ease;
  font-size: 1rem;
}
`);

block(`
.file-input-label:hover {
  border-color: var(--accent);
  color: var(--text);
  background: var(--hover-bg);
}
`);

block(`
.form-divider {
  display: flex;
  align-items: center;
  justify-content: center;
  position: relative;
  margin: 16px 0;
}
`);

block(`
.form-divider::before {
  content: '';
  position: absolute;
  top: 50%;
  left: 0;
  right: 0;
  height: 1px;
  background: var(--border);
  z-index: 1;
}
`);

block(`
.form-divider span {
  background: var(--surface);
  padding: 0 16px;
  color: var(--muted);
  font-size: 0.9rem;
  z-index: 2;
  position: relative;
}
`);

block(`
.json-textarea {
  width: 100%;
  min-height: 200px;
  padding: 12px 16px;
  border: 1px solid var(--border);
  border-radius: 8px;
  background: var(--bg);
  color: var(--text);
  font-family: 'Courier New', monospace;
  font-size: 0.9rem;
  resize: vertical;
  transition: border-color var(--transition-speed) ease;
}
`);

block(`
.json-textarea:focus {
  outline: none;
  border-color: var(--accent);
  box-shadow: 0 0 0 3px rgba(16, 185, 129, 0.1);
}
`);

block(`
.import-success {
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 16px;
  padding: 32px;
  text-align: center;
}
`);

block(`
.import-success h2 {
  margin: 0 0 24px;
  font-size: 1.8rem;
  color: var(--text);
}
`);

block(`
.import-stats {
  display: flex;
  justify-content: center;
  gap: 32px;
  margin: 24px 0;
}
`);

block(`
.stat {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
}
`);

block(`
.stat-number {
  font-size: 2.5rem;
  font-weight: bold;
  color: var(--accent);
}
`);

block(`
.stat-label {
  font-size: 0.9rem;
  color: var(--muted);
  text-transform: uppercase;
  letter-spacing: 0.05em;
}
`);

block(`
.import-warnings {
  background: var(--bg);
  border: 1px solid #f59e0b;
  border-radius: 8px;
  padding: 16px;
  margin: 24px 0;
  text-align: left;
}
`);

block(`
.import-warnings h3 {
  margin: 0 0 12px;
  color: #f59e0b;
  font-size: 1.1rem;
}
`);

block(`
.import-warnings ul {
  margin: 0;
  padding-left: 20px;
  color: var(--text);
}
`);

block(`
.import-warnings li {
  margin-bottom: 4px;
  font-size: 0.9rem;
}
`);

block(`
.success-actions {
  display: flex;
  gap: 16px;
  justify-content: center;
  margin-top: 24px;
}
`);

block(`
.import-help {
  background: var(--bg);
  border: 1px solid var(--border);
  border-radius: 12px;
  padding: 24px;
}
`);

block(`
.import-help h3 {
  margin: 0 0 16px;
  color: var(--text);
  font-size: 1.1rem;
}
`);

block(`
.import-help ul {
  margin: 0;
  padding-left: 20px;
  color: var(--muted);
}
`);

block(`
.import-help li {
  margin-bottom: 8px;
  line-height: 1.4;
}
`);

// Filtering Interface Styles
block(`
.filtering-interface {
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 16px;
  padding: 32px;
  margin: 32px 0;
}
`);

block(`
.filtering-interface h3 {
  margin: 0 0 16px;
  color: var(--text);
  font-size: 1.4rem;
}
`);

block(`
.filtering-interface p {
  margin: 0 0 24px;
  color: var(--muted);
}
`);

block(`
.family-group {
  background: var(--bg);
  border: 1px solid var(--border);
  border-radius: 12px;
  margin-bottom: 24px;
  overflow: hidden;
}
`);

block(`
.family-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 16px 20px;
  background: var(--surface);
  border-bottom: 1px solid var(--border);
}
`);

block(`
.family-header h4 {
  margin: 0;
  font-size: 1.1rem;
}
`);

block(`
.family-checkbox, .person-checkbox {
  display: flex;
  align-items: center;
  gap: 12px;
  cursor: pointer;
  font-weight: 500;
  color: var(--text);
}
`);

block(`
.family-checkbox input, .person-checkbox input {
  width: 18px;
  height: 18px;
  accent-color: var(--accent);
}
`);

block(`
.btn-small {
  padding: 6px 12px;
  font-size: 0.85rem;
}
`);

block(`
.filtering-interface .people-list {
  max-height: 300px;
  overflow-y: auto;
}
`);

block(`
.person-item {
  padding: 12px 20px;
  border-bottom: 1px solid var(--border);
}
`);

block(`
.person-item:last-child {
  border-bottom: none;
}
`);

block(`
.person-item:hover {
  background: var(--hover-bg);
}
`);

block(`
.person-checkbox {
  font-weight: normal;
  width: 100%;
}
`);

block(`
.person-details {
  display: flex;
  flex-direction: column;
  gap: 4px;
  flex: 1;
}
`);

block(`
.person-name {
  font-weight: 600;
  color: var(--text);
  font-size: 1rem;
}
`);

block(`
.person-meta {
  font-size: 0.85rem;
  color: var(--muted);
}
`);

block(`
.person-measurements {
  font-size: 0.8rem;
  color: var(--accent);
}
`);

block(`
.filter-summary {
  margin-top: 24px;
  padding: 16px;
  background: var(--bg);
  border-radius: 8px;
  border: 1px solid var(--border);
}
`);

block(`
.filter-summary p {
  margin: 0;
  color: var(--text);
}
`);

block(`
.filter-summary .warning {
  color: #f59e0b;
  font-weight: 500;
  margin-top: 8px;
}
`);

// Mobile responsive styles for import page
block(`
@media (max-width: 768px) {
  .import-container {
    padding: 30px 16px;
  }

  .import-form-container {
    padding: 24px;
  }

  .import-stats {
    flex-direction: column;
    gap: 16px;
  }

  .stat-number {
    font-size: 2rem;
  }

  .success-actions {
    flex-direction: column;
  }

  .success-actions .btn {
    width: 100%;
  }

  .json-textarea {
    min-height: 150px;
    font-size: 0.85rem;
  }

  .import-help {
    padding: 20px;
  }

  .filtering-interface {
    padding: 24px;
    margin: 24px 0;
  }

  .family-header {
    flex-direction: column;
    gap: 12px;
    align-items: stretch;
  }

  .family-header h4 {
    text-align: center;
  }

  .person-details {
    gap: 6px;
  }

  .person-name {
    font-size: 0.95rem;
  }

  .person-meta, .person-measurements {
    font-size: 0.8rem;
  }
}
`);

// Import Options Styles
block(`
.import-options {
  display: flex;
  flex-direction: column;
  gap: 24px;
}
`);

block(`
.option-group {
  display: flex;
  flex-direction: column;
  gap: 12px;
}
`);

block(`
.option-label {
  color: var(--text);
  font-size: 1rem;
  margin: 0;
}
`);

block(`
.radio-group {
  display: flex;
  flex-direction: column;
  gap: 12px;
  margin-left: 16px;
}
`);

block(`
.radio-option {
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding: 12px;
  border: 1px solid var(--border);
  border-radius: 8px;
  background: var(--bg);
  cursor: pointer;
  transition: all 0.2s ease;
}
`);

block(`
.radio-option:hover {
  background: var(--hover-bg);
  border-color: var(--accent);
}
`);

block(`
.radio-option input[type="radio"] {
  width: 16px;
  height: 16px;
  accent-color: var(--accent);
  margin-right: 8px;
}
`);

block(`
.radio-option span {
  font-weight: 500;
  color: var(--text);
  display: flex;
  align-items: center;
}
`);

block(`
.radio-option small {
  color: var(--muted);
  font-size: 0.85rem;
  margin-left: 24px;
  line-height: 1.3;
}
`);

block(`
.checkbox-option {
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding: 12px;
  border: 1px solid var(--border);
  border-radius: 8px;
  background: var(--bg);
  cursor: pointer;
  transition: all 0.2s ease;
}
`);

block(`
.checkbox-option:hover {
  background: var(--hover-bg);
  border-color: var(--accent);
}
`);

block(`
.checkbox-option input[type="checkbox"] {
  width: 16px;
  height: 16px;
  accent-color: var(--accent);
  margin-right: 8px;
}
`);

block(`
.checkbox-option span {
  font-weight: 500;
  color: var(--text);
  display: flex;
  align-items: center;
}
`);

block(`
.checkbox-option small {
  color: var(--muted);
  font-size: 0.85rem;
  margin-left: 24px;
  line-height: 1.3;
}
`);

// Error styles (in addition to warnings)
block(`
.import-errors {
  background: var(--bg);
  border: 1px solid #ef4444;
  border-radius: 8px;
  padding: 16px;
  margin: 24px 0;
  text-align: left;
}
`);

block(`
.import-errors h3 {
  margin: 0 0 12px;
  color: #ef4444;
  font-size: 1.1rem;
}
`);

block(`
.import-errors ul {
  margin: 0;
  padding-left: 20px;
  color: var(--text);
}
`);

block(`
.import-errors li {
  margin-bottom: 4px;
  font-size: 0.9rem;
}
`);
