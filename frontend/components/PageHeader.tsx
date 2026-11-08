import * as preact from "preact";
import "./page-header-styles";

export const PageHeader = ({
  title,
  lede,
  children,
}: {
  title: preact.ComponentChild;
  lede?: preact.ComponentChild;
  children?: preact.ComponentChildren;
}) => (
  <header className="page-head">
    <div className="page-head-text">
      <h1>{title}</h1>
      {lede && <p className="page-head-lede">{lede}</p>}
    </div>
    {children && <div className="page-head-actions">{children}</div>}
  </header>
);
