import { block } from "vlens/css";

// Profile Page Styles
block(`
.profile-container {
  max-width: 1200px;
  padding: 40px 20px;
  margin: 0 auto;
}
`);

block(`
.profile-page {
  width: 100%;
}
`);

block(`
.profile-header {
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 16px;
  padding: 32px;
  margin-bottom: 30px;
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 20px;
}
`);

block(`
.profile-header-main {
  display: flex;
  align-items: center;
  gap: 20px;
}
`);

block(`
.profile-avatar {
  font-size: 4rem;
  display: flex;
  align-items: center;
  justify-content: center;
  width: 80px;
  height: 80px;
  background: var(--bg);
  border-radius: 50%;
  border: 3px solid var(--border);
}
`);

block(`
.profile-photo {
  width: 100%;
  height: 100%;
  object-fit: cover;
  border-radius: 50%;
}
`);

block(`
.profile-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 100%;
  height: 100%;
}
`);

block(`
.profile-info h1 {
  font-size: 2.2rem;
  margin: 0 0 8px;
  color: var(--text);
}
`);

block(`
.profile-details {
  font-size: 1.1rem;
  color: var(--muted);
  margin: 0;
}
`);

block(`
.profile-actions {
  display: flex;
  gap: 12px;
  flex-wrap: wrap;
}
`);

block(`
.profile-tabs {
  display: flex;
  gap: 4px;
  margin-bottom: 30px;
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 12px;
  padding: 6px;
}
`);

block(`
.tab {
  flex: 1;
  padding: 12px 20px;
  border: none;
  background: transparent;
  color: var(--muted);
  border-radius: 8px;
  cursor: pointer;
  transition: all 0.3s ease;
  font-size: 1rem;
  font-weight: 500;
}
`);

block(`
.tab:hover {
  background: var(--hover-bg);
  color: var(--text);
}
`);

block(`
.tab.active {
  background: var(--accent);
  color: var(--button-text);
  font-weight: 600;
}
`);

block(`
.profile-content {
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 16px;
  padding: 32px;
  min-height: 400px;
}
`);

block(`
.timeline-tab h2,
.growth-tab h2,
.photos-tab h2 {
  font-size: 1.5rem;
  margin: 0 0 24px;
  color: var(--text);
}
`);

block(`
.timeline-content,
.growth-content,
.photos-content {
  display: flex;
  flex-direction: column;
  gap: 20px;
}
`);

block(`
.growth-table h3 {
  margin: 0 0 16px;
  color: var(--text);
}
`);

block(`
.growth-records {
  background: var(--bg);
  border: 1px solid var(--border);
  border-radius: 12px;
  overflow: hidden;
  overflow-x: auto;
}
`);

block(`
.data-table {
  width: 100%;
  min-width: 480px;
  border-collapse: collapse;
  background: transparent;
}
`);

block(`
.data-table th,
.data-table td {
  padding: 12px 16px;
  text-align: left;
  border-bottom: 1px solid var(--border);
}
`);

block(`
.data-table th {
  background: var(--surface);
  color: var(--muted);
  font-weight: 600;
  font-size: 0.9rem;
  text-transform: uppercase;
  letter-spacing: 0.05em;
}
`);

block(`
.data-table td {
  color: var(--text);
  font-size: 0.95rem;
}
`);

block(`
.data-table tbody tr:hover {
  background: var(--hover-bg);
}
`);

block(`
.data-table tbody tr:last-child td {
  border-bottom: none;
}
`);

block(`
.table-actions {
  display: flex;
  gap: 8px;
  align-items: center;
  justify-content: flex-start;
}
`);

block(`
.btn-action {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  border: none;
  background: transparent;
  color: var(--muted);
  cursor: pointer;
  border-radius: 4px;
  text-decoration: none;
  font-size: 14px;
  transition: all 0.2s ease;
}
`);

block(`
.btn-action:hover {
  background: var(--hover-bg);
  color: var(--text);
  transform: scale(1.1);
}
`);

block(`
.btn-action.btn-edit:hover {
  color: var(--accent);
}
`);

block(`
.btn-action.btn-delete:hover {
  color: #dc3545;
  background: rgba(220, 53, 69, 0.1);
}
`);

block(`
.photos-gallery {
  min-height: 300px;
  display: flex;
  align-items: center;
  justify-content: center;
}
`);

block(`
.profile-content .empty-state {
  background: var(--bg);
  border: 1px dashed var(--border);
  border-radius: 12px;
  padding: 40px 20px;
  text-align: center;
}
`);

block(`
.profile-content .empty-state p {
  font-size: 1rem;
  color: var(--muted);
  margin: 0 0 16px;
}
`);

block(`
.error-page {
  text-align: center;
  padding: 80px 20px;
}
`);

block(`
.error-page h1 {
  font-size: 2rem;
  margin: 0 0 16px;
  color: var(--text);
}
`);

block(`
.error-page p {
  font-size: 1.1rem;
  color: var(--muted);
  margin: 0 0 24px;
}
`);

// Mobile responsive styles for profile page
block(`
@media (max-width: 768px) {
  .profile-container {
    padding: 30px 16px;
  }

  .profile-header {
    flex-direction: column;
    text-align: center;
    gap: 24px;
  }

  .profile-header-main {
    flex-direction: column;
    text-align: center;
    gap: 16px;
  }

  .profile-avatar {
    width: 70px;
    height: 70px;
    font-size: 3rem;
  }

  .profile-info h1 {
    font-size: 1.8rem;
  }

  .profile-actions {
    justify-content: center;
    width: 100%;
  }

  .profile-actions .btn {
    flex: 1;
    min-width: 0;
    font-size: 0.9rem;
    padding: 10px 8px;
  }

  .profile-tabs {
    flex-direction: column;
    gap: 2px;
  }

  .tab {
    padding: 14px 20px;
    font-size: 0.95rem;
  }

  .profile-content {
    padding: 24px 20px;
  }
}
`);
