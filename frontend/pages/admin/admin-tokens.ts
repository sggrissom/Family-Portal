import { block } from "vlens/css";

block(`
:root {
  --admin-accent: #6366f1;
  --admin-accent-hover: #4f46e5;
  --admin-danger: #dc2626;
  --admin-danger-hover: #b91c1c;
  --admin-success: #059669;
  --admin-border: #d1d5db;
  --admin-surface: #f9fafb;
  --admin-surface-elevated: #ffffff;
  --admin-text-on-accent: #ffffff;
}
`);

block(`
[data-theme="dark"] {
  --admin-accent: #818cf8;
  --admin-accent-hover: #6366f1;
  --admin-danger: #f87171;
  --admin-danger-hover: #ef4444;
  --admin-success: #34d399;
  --admin-border: #4b5563;
  --admin-surface: #111827;
  --admin-surface-elevated: #1f2937;
  --admin-text-on-accent: #ffffff;
}
`);
