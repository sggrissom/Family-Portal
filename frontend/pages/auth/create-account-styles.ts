import { block } from "vlens/css";

block(`
.create-account-container {
  max-width: 480px;
  padding: 40px 20px;
  margin: 0 auto;
  min-height: calc(100vh - 200px);
  display: flex;
  align-items: center;
  justify-content: center;
}
`);

block(`
.create-account-page {
  width: 100%;
}
`);

block(`
@media (max-width: 480px) {
  .create-account-container {
    padding: 20px 16px;
  }
}
`);
