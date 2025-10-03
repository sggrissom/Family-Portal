import { block } from "vlens/css";

block(`
.family-chart-page {
  max-width: 1400px;
  margin: 0 auto;
  padding: 2rem;
}
`);

block(`
.family-chart-container {
  display: flex;
  flex-direction: column;
  gap: 2rem;
}
`);

block(`
.family-chart-header {
  text-align: center;
}
`);

block(`
.family-chart-header h1 {
  font-size: 2.5rem;
  color: var(--text);
  margin: 0 0 0.5rem;
}
`);

block(`
.family-chart-header .subtitle {
  font-size: 1.1rem;
  color: var(--muted);
  margin: 0;
}
`);

block(`
.chart-controls {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 2rem;
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 12px;
  padding: 1.5rem;
}
`);

block(`
.person-selection h3 {
  font-size: 1.1rem;
  color: var(--text);
  margin: 0 0 1rem;
}
`);

block(`
.measurement-filters h3 {
  font-size: 1.1rem;
  color: var(--text);
  margin: 0 0 1rem;
}
`);

block(`
.person-checkboxes {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
  margin-bottom: 1rem;
}
`);

block(`
.person-checkbox {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  cursor: pointer;
  padding: 0.5rem;
  border-radius: 6px;
  transition: background-color 0.2s ease;
}
`);

block(`
.person-checkbox:hover {
  background: var(--hover-bg);
}
`);

block(`
.person-checkbox input[type="checkbox"] {
  width: 18px;
  height: 18px;
  cursor: pointer;
  accent-color: var(--accent);
}
`);

block(`
.checkbox-label {
  font-size: 1rem;
  color: var(--text);
  user-select: none;
}
`);

block(`
.btn-clear {
  padding: 0.5rem 1rem;
  background: var(--accent);
  color: white;
  border: none;
  border-radius: 6px;
  font-size: 0.9rem;
  font-weight: 600;
  cursor: pointer;
  transition: background-color 0.2s ease;
}
`);

block(`
.btn-clear:hover {
  background: var(--primary-accent);
}
`);

block(`
.filter-toggles {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
}
`);

block(`
.filter-toggle {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  cursor: pointer;
  padding: 0.5rem;
  border-radius: 6px;
  transition: background-color 0.2s ease;
}
`);

block(`
.filter-toggle:hover {
  background: var(--hover-bg);
}
`);

block(`
.filter-toggle input[type="checkbox"] {
  width: 18px;
  height: 18px;
  cursor: pointer;
  accent-color: var(--accent);
}
`);

block(`
.toggle-label {
  font-size: 1rem;
  color: var(--text);
  user-select: none;
}
`);

block(`
.chart-display {
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 12px;
  padding: 2rem;
  min-height: 500px;
  display: flex;
  align-items: center;
  justify-content: center;
}
`);

block(`
.chart-wrapper {
  width: 100%;
}
`);

block(`
.loading-state {
  text-align: center;
  color: var(--muted);
  font-size: 1.1rem;
}
`);

block(`
.error-state {
  text-align: center;
  color: #ef4444;
  font-size: 1.1rem;
}
`);

block(`
.empty-state {
  text-align: center;
  color: var(--muted);
  font-size: 1.1rem;
}
`);

block(`
.chart-info {
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 12px;
  padding: 1.5rem;
}
`);

block(`
.chart-info h3 {
  font-size: 1.1rem;
  color: var(--text);
  margin: 0 0 1rem;
}
`);

block(`
.chart-info ul {
  margin: 0;
  padding-left: 1.5rem;
  color: var(--text);
  line-height: 1.8;
}
`);

block(`
.chart-info li {
  margin-bottom: 0.5rem;
}
`);

block(`
@media (max-width: 768px) {
  .family-chart-page {
    padding: 1rem;
  }

  .family-chart-header h1 {
    font-size: 1.8rem;
  }

  .family-chart-header .subtitle {
    font-size: 0.95rem;
  }

  .chart-controls {
    grid-template-columns: 1fr;
    gap: 1.5rem;
    padding: 1rem;
  }

  .chart-display {
    padding: 1rem;
    min-height: 400px;
  }

  .chart-info {
    padding: 1rem;
  }

  .chart-info ul {
    font-size: 0.9rem;
  }
}
`);
