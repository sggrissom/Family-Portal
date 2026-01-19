import { block } from "vlens/css";

block(`
.processing-image-wrapper {
  position: relative;
  display: inline-block;
}
`);

block(`
.processing-overlay {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background-color: rgba(255, 255, 255, 0.8);
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  z-index: 10;
}
`);

block(`
[data-theme="dark"] .processing-overlay {
  background-color: rgba(0, 0, 0, 0.8);
}
`);

block(`
.processing-spinner {
  width: 32px;
  height: 32px;
  border: 3px solid var(--border);
  border-top: 3px solid var(--primary-accent);
  border-radius: 50%;
  animation: spin 1s linear infinite;
}
`);

block(`
.processing-text {
  margin-top: 12px;
  font-size: 14px;
  color: var(--text);
  font-weight: 500;
  text-align: center;
}
`);

block(`
@keyframes spin {
  0% { transform: rotate(0deg); }
  100% { transform: rotate(360deg); }
}
`);

block(`
.processing-image {
  opacity: 0.6;
}
`);

// Profile image crop container styles
block(`
.profile-image-crop-container {
  position: relative;
  overflow: hidden;
  width: 100%;
  height: 100%;
}
`);

block(`
.profile-image-inner {
  width: 100%;
  height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
}
`);

block(`
.profile-image-inner picture {
  width: 100%;
  height: 100%;
  display: block;
}
`);

block(`
.profile-image-cropped {
  width: 100%;
  height: 100%;
  object-fit: cover;
}
`);
