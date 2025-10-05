import * as preact from "preact";
import { Header, Footer } from "../layout";

interface ErrorPageProps {
  title: string;
  message: string;
  backLink?: string;
  backLabel?: string;
  containerClass?: string;
}

export const ErrorPage = ({
  title,
  message,
  backLink = "/dashboard",
  backLabel = "Back to Dashboard",
  containerClass = "add-milestone-container",
}: ErrorPageProps) => {
  return (
    <div>
      <Header isHome={false} />
      <main id="app" className={containerClass}>
        <div className="error-page">
          <h1>{title}</h1>
          <p>{message}</p>
          <a href={backLink} className="btn btn-primary">
            {backLabel}
          </a>
        </div>
      </main>
      <Footer />
    </div>
  );
};
