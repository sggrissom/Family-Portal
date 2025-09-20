import { block } from "vlens/css";

// Photos Tab Styles
block(`
.photos-gallery {
  min-height: 300px;
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
  gap: 20px;
  padding: 0;
}
`);

block(`
.photos-gallery.has-photos {
  min-height: auto;
}
`);

block(`
.photo-card {
  background: var(--surface);
  border-radius: 12px;
  overflow: hidden;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.1);
  transition: transform 0.2s ease, box-shadow 0.2s ease;
  cursor: pointer;
}
`);

block(`
.photo-card:hover {
  transform: translateY(-2px);
  box-shadow: 0 4px 16px rgba(0, 0, 0, 0.15);
}
`);

block(`
.photo-image {
  width: 100%;
  height: 200px;
  object-fit: cover;
  display: block;
  background: var(--color-border);
}
`);

block(`
.photo-info {
  padding: 12px 16px;
}
`);

block(`
.photo-title {
  font-weight: 600;
  font-size: 14px;
  color: var(--color-text-emphasis);
  margin: 0 0 6px 0;
  line-height: 1.3;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
`);

block(`
.photo-date {
  font-size: 12px;
  color: var(--color-text-muted);
  margin: 0 0 4px 0;
}
`);

block(`
.photo-description {
  font-size: 12px;
  color: var(--color-text);
  margin: 0;
  line-height: 1.4;
  overflow: hidden;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
}
`);

block(`
.photos-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 24px;
}
`);

block(`
.photos-count {
  font-size: 14px;
  color: var(--color-text-muted);
}
`);

block(`
.photos-actions {
  display: flex;
  gap: 12px;
}
`);

// Responsive Design
block(`
@media (max-width: 768px) {
  .photos-gallery {
    grid-template-columns: repeat(auto-fill, minmax(150px, 1fr));
    gap: 16px;
  }

  .photo-image {
    height: 150px;
  }

  .photo-info {
    padding: 10px 12px;
  }

  .photos-header {
    flex-direction: column;
    gap: 16px;
    align-items: stretch;
  }

  .photos-actions {
    justify-content: center;
  }
}
`);

block(`
@media (max-width: 480px) {
  .photos-gallery {
    grid-template-columns: repeat(auto-fill, minmax(120px, 1fr));
    gap: 12px;
  }

  .photo-image {
    height: 120px;
  }

  .photo-info {
    padding: 8px 10px;
  }

  .photo-title {
    font-size: 13px;
  }

  .photo-date,
  .photo-description {
    font-size: 11px;
  }
}
`);