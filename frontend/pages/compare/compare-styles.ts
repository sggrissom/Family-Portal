import { block } from "vlens/css";

// Compare page container
block(`
.compare-page {
  max-width: 1400px;
  margin: 0 auto;
  padding: 2rem 1rem;
}
`);

block(`
.compare-header {
  margin-bottom: 2rem;
}
`);

block(`
.compare-header h1 {
  font-size: 2rem;
  margin-bottom: 0.5rem;
  color: var(--text);
}
`);

block(`
.compare-header p {
  color: var(--muted);
  font-size: 1rem;
}
`);

// Person selector section
block(`
.person-selector {
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 8px;
  padding: 1.5rem;
  margin-bottom: 2rem;
}
`);

block(`
.person-selector h2 {
  font-size: 1.25rem;
  margin-bottom: 1rem;
  color: var(--text);
}
`);

block(`
.person-checkboxes {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
  gap: 1rem;
}
`);

block(`
.person-checkbox-item {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  padding: 0.75rem;
  border: 1px solid var(--border);
  border-radius: 6px;
  cursor: pointer;
  transition: all 0.2s ease;
  background: var(--bg);
}
`);

block(`
.person-checkbox-item:hover {
  background: var(--hover-bg);
  border-color: var(--primary-accent);
}
`);

block(`
.person-checkbox-item.selected {
  background: var(--accent);
  border-color: var(--primary-accent);
}
`);

block(`
.person-checkbox-item input[type="checkbox"] {
  cursor: pointer;
}
`);

block(`
.person-checkbox-item label {
  cursor: pointer;
  flex: 1;
  font-weight: 500;
}
`);

// Filter controls
block(`
.compare-filters {
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 8px;
  padding: 1.5rem;
  margin-bottom: 2rem;
}
`);

block(`
.filter-row {
  display: flex;
  flex-wrap: wrap;
  gap: 2rem;
  align-items: center;
}
`);

block(`
.filter-group {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}
`);

block(`
.filter-group label {
  font-size: 0.875rem;
  font-weight: 600;
  color: var(--text);
  text-transform: uppercase;
  letter-spacing: 0.5px;
}
`);

block(`
.filter-buttons {
  display: flex;
  gap: 0.5rem;
  flex-wrap: wrap;
}
`);

block(`
.filter-btn {
  padding: 0.5rem 1rem;
  border: 1px solid var(--border);
  border-radius: 6px;
  background: var(--bg);
  color: var(--text);
  font-size: 0.875rem;
  cursor: pointer;
  transition: all 0.2s ease;
}
`);

block(`
.filter-btn:hover {
  background: var(--hover-bg);
  border-color: var(--primary-accent);
}
`);

block(`
.filter-btn.active {
  background: var(--primary-accent);
  color: white;
  border-color: var(--primary-accent);
}
`);

// Comparison grid
block(`
.comparison-grid {
  display: grid;
  gap: 1.5rem;
}
`);

block(`
.comparison-grid.two-column {
  grid-template-columns: repeat(2, 1fr);
}
`);

block(`
.comparison-grid.three-column {
  grid-template-columns: repeat(3, 1fr);
}
`);

block(`
.comparison-grid.four-column {
  grid-template-columns: repeat(2, 1fr);
}
`);

block(`
.comparison-grid.five-column {
  grid-template-columns: repeat(2, 1fr);
}
`);

// Person comparison column
block(`
.person-column {
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 8px;
  overflow: hidden;
}
`);

block(`
.person-column-header {
  background: var(--accent);
  padding: 1rem;
  border-bottom: 1px solid var(--border);
  display: flex;
  align-items: center;
  gap: 0.75rem;
}
`);

block(`
.person-column-avatar {
  width: 48px;
  height: 48px;
  border-radius: 50%;
  background: var(--bg);
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 1.5rem;
}
`);

block(`
.person-column-info h3 {
  margin: 0;
  font-size: 1.125rem;
  color: var(--text);
}
`);

block(`
.person-column-info p {
  margin: 0.25rem 0 0;
  font-size: 0.875rem;
  color: var(--muted);
}
`);

block(`
.person-column-timeline {
  padding: 1rem;
  max-height: 800px;
  overflow-y: auto;
}
`);

block(`
.timeline-items {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}
`);

// Empty states
block(`
.compare-empty-state {
  text-align: center;
  padding: 3rem 1rem;
  color: var(--muted);
}
`);

block(`
.compare-empty-state h3 {
  font-size: 1.25rem;
  margin-bottom: 0.5rem;
  color: var(--text);
}
`);

block(`
.no-data-message {
  text-align: center;
  padding: 2rem 1rem;
  color: var(--muted);
  font-style: italic;
}
`);

// Mobile responsive
block(`
@media (max-width: 900px) {
  .comparison-grid.two-column,
  .comparison-grid.three-column,
  .comparison-grid.four-column,
  .comparison-grid.five-column {
    grid-template-columns: 1fr;
  }

  .person-column {
    margin-bottom: 1rem;
  }

  .person-column-timeline {
    max-height: 500px;
  }

  .filter-row {
    flex-direction: column;
    align-items: flex-start;
  }
}
`);

block(`
@media (max-width: 600px) {
  .compare-page {
    padding: 1rem 0.5rem;
  }

  .compare-header h1 {
    font-size: 1.5rem;
  }

  .person-checkboxes {
    grid-template-columns: 1fr;
  }

  .filter-buttons {
    width: 100%;
  }

  .filter-btn {
    flex: 1;
    min-width: 80px;
  }
}
`);
