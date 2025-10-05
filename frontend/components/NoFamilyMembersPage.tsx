import * as preact from "preact";
import { Header, Footer } from "../layout";

interface NoFamilyMembersPageProps {
  message?: string;
  containerClass?: string;
}

export const NoFamilyMembersPage = ({
  message = "Please add family members first",
  containerClass = "add-milestone-container",
}: NoFamilyMembersPageProps) => {
  return (
    <div>
      <Header isHome={false} />
      <main id="app" className={containerClass}>
        <div className="error-page">
          <h1>No Family Members</h1>
          <p>{message}</p>
          <a href="/add-person" className="btn btn-primary">
            Add Family Member
          </a>
          <a href="/dashboard" className="btn btn-secondary">
            Back to Dashboard
          </a>
        </div>
      </main>
      <Footer />
    </div>
  );
};
