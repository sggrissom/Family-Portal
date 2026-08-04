import { block } from "vlens/css";

block(`
.view-growth-container {
  max-width: 800px;
  margin: 0 auto;
  padding: 40px 20px;
}
`);

block(`
.view-growth-page {
  display: flex;
  flex-direction: column;
  gap: 24px;
}
`);

block(`
.view-growth-header {
  display: flex;
  align-items: center;
}
`);

block(`
.back-link {
  color: var(--accent);
  text-decoration: none;
  font-weight: 500;
}
`);

block(`
.back-link:hover {
  text-decoration: underline;
}
`);

block(`
.growth-detail-card {
  display: flex;
  align-items: center;
  gap: 20px;
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 12px;
  padding: 24px;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.1);
}
`);

block(`
.growth-detail-icon {
  font-size: 2.5rem;
}
`);

block(`
.growth-detail-main {
  flex: 1;
}
`);

block(`
.growth-detail-type {
  color: var(--muted);
  font-size: 0.9rem;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.05em;
}
`);

block(`
.growth-detail-value {
  color: var(--text);
  font-size: 2rem;
  font-weight: 700;
  margin: 4px 0;
}
`);

block(`
.growth-detail-meta {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
  color: var(--muted);
  font-size: 0.95rem;
}
`);

block(`
.growth-detail-actions {
  display: flex;
  flex-direction: column;
  gap: 8px;
}
`);

block(`
.percentile-badge {
  display: inline-block;
  background: var(--hover-bg);
  color: var(--accent);
  padding: 2px 10px;
  border-radius: 999px;
  font-size: 0.8rem;
  font-weight: 600;
}
`);

block(`
.family-comparison h2 {
  color: var(--text);
  margin: 0 0 16px;
}
`);

block(`
.comparison-group {
  margin-bottom: 24px;
}
`);

block(`
.comparison-group h3 {
  color: var(--muted);
  font-size: 0.9rem;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  margin: 0 0 12px;
}
`);

block(`
.comparison-cards {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(260px, 1fr));
  gap: 16px;
}
`);

block(`
.comparison-card {
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 12px;
  padding: 16px;
}
`);

block(`
.comparison-person-name {
  display: block;
  color: var(--text);
  font-weight: 700;
  text-decoration: none;
  margin-bottom: 10px;
}
`);

block(`
.comparison-person-name:hover {
  color: var(--accent);
}
`);

block(`
.comparison-empty {
  color: var(--muted);
  font-size: 0.9rem;
  margin: 0;
}
`);

block(`
.comparison-rows {
  display: flex;
  flex-direction: column;
  gap: 10px;
}
`);

block(`
.comparison-row {
  display: flex;
  align-items: flex-start;
  gap: 8px;
}
`);

block(`
.comparison-row-icon {
  flex-shrink: 0;
}
`);

block(`
.comparison-row-text {
  color: var(--text);
  font-size: 0.9rem;
  line-height: 1.4;
}
`);

block(`
@media (max-width: 580px) {
  .view-growth-container {
    padding: 24px 16px;
  }

  .growth-detail-card {
    flex-direction: column;
    align-items: flex-start;
  }

  .growth-detail-actions {
    flex-direction: row;
    width: 100%;
  }
}
`);
