import * as preact from "preact";
import * as auth from "../lib/authCache";
import { requireAuthInView } from "../lib/authHelpers";
import { ErrorPage } from "./ErrorPage";

export function adminView(
  render: (currentAuth: auth.AuthCache) => preact.ComponentChild
): preact.ComponentChild {
  const currentAuth = requireAuthInView();
  if (!currentAuth) {
    return;
  }

  if (!currentAuth.isAdmin) {
    return (
      <ErrorPage
        title="Access Denied"
        message="You do not have permission to access this page."
        backLink="/dashboard"
        backLabel="Return to Dashboard"
        containerClass="page-container"
      />
    );
  }

  return render(currentAuth);
}
