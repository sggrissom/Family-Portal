import * as preact from "preact";
import * as core from "vlens/core";
import * as server from "../../server";
import { Header, Footer } from "../../layout";
import { requireAuthInView } from "../../lib/authHelpers";
import { getIdFromRoute } from "../../lib/routeHelpers";
import { ErrorPage } from "../../components/ErrorPage";
import { GrowthForm } from "./GrowthForm";
import "./growth-styles";

export async function fetch(route: string, prefix: string) {
  // Extract growth record ID from URL (e.g., /edit-growth/123)
  const growthId = getIdFromRoute(route);

  if (!growthId) {
    throw new Error("Growth record ID is required");
  }

  return server.GetGrowthData({ id: growthId });
}

export function view(
  route: string,
  prefix: string,
  data: server.GetGrowthDataResponse
): preact.ComponentChild {
  const currentAuth = requireAuthInView();
  if (!currentAuth) {
    return;
  }

  if (!data.growthData) {
    return (
      <ErrorPage
        title="Growth Record Not Found"
        message="The growth record you're trying to edit could not be found"
        containerClass="add-growth-container"
      />
    );
  }

  return (
    <div>
      <Header isHome={false} />
      <main id="app" className="add-growth-container">
        <GrowthForm
          mode="edit"
          growthData={data.growthData}
          onCancel={() => core.setRoute(`/profile/${data.growthData!.personId}`)}
          onSuccess={personId => core.setRoute(`/profile/${personId}`)}
        />
      </main>
      <Footer />
    </div>
  );
}
