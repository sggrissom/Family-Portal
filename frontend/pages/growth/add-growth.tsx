import * as preact from "preact";
import * as core from "vlens/core";
import * as server from "../../server";
import { Header, Footer } from "../../layout";
import { requireAuthInView } from "../../lib/authHelpers";
import { getIdFromRoute } from "../../lib/routeHelpers";
import { NoFamilyMembersPage } from "../../components/NoFamilyMembersPage";
import { GrowthForm } from "./GrowthForm";
import "./growth-styles";

export async function fetch(route: string, prefix: string) {
  // Fetch people list to populate the person selector
  return server.ListPeople({});
}

export function view(
  route: string,
  prefix: string,
  data: server.ListPeopleResponse
): preact.ComponentChild {
  const currentAuth = requireAuthInView();
  if (!currentAuth) {
    return;
  }

  if (!data.people || data.people.length === 0) {
    return (
      <NoFamilyMembersPage
        message="Please add family members before tracking growth data"
        containerClass="add-growth-container"
      />
    );
  }

  // Extract person ID from URL if present (e.g., /add-growth/123)
  const personId = getIdFromRoute(route);
  const personIdFromUrl = personId ? personId.toString() : undefined;

  return (
    <div>
      <Header isHome={false} />
      <main id="app" className="add-growth-container">
        <GrowthForm
          mode="add"
          personId={personIdFromUrl}
          people={data.people}
          onCancel={() => core.setRoute("/dashboard")}
          onSuccess={personId => core.setRoute(`/profile/${personId}`)}
        />
      </main>
      <Footer />
    </div>
  );
}
