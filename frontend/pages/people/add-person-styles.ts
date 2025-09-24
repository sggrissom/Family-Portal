import { block } from "vlens/css";

// Add Person Page Styles
block(`
.add-person-container {
  max-width: 500px;
  padding: 40px 20px;
  margin: 0 auto;
  min-height: calc(100vh - 200px);
  display: flex;
  align-items: center;
  justify-content: center;
}
`);

block(`
.add-person-page {
  width: 100%;
}
`);

block(`
.form-actions {
  display: flex;
  gap: 16px;
  margin-top: 8px;
}
`);

block(`
.form-actions .btn {
  flex: 1;
}
`);

block(`
@media (max-width: 768px) {
  .add-person-container {
    padding: 30px 16px;
  }

  .form-actions {
    flex-direction: column;
  }
}
`);