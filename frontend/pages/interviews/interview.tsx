import * as preact from "preact";
import * as server from "../../server";
import { Header, Footer } from "../../layout";
import { requireAuthInView } from "../../lib/authHelpers";
import { getIdFromRoute } from "../../lib/routeHelpers";
import { AskView } from "./ask";
import { InterviewPage } from "./view";
import "./interview-styles";

export async function fetch(route: string, prefix: string) {
  const id = getIdFromRoute(route);
  if (!id) return [null, "Interview ID is required"] as [null, string];
  return server.GetInterview({ id });
}

export function view(
  route: string,
  prefix: string,
  data: server.GetInterviewResponse
): preact.ComponentChild {
  if (!requireAuthInView()) return;
  const asking = route.split("?")[0].replace(/\/$/, "").endsWith("/ask");
  return (
    <div>
      <Header isHome={false} />
      <main id="app" className="iv-page">
        {asking ? <AskView data={data} /> : <InterviewPage data={data} />}
      </main>
      <Footer />
    </div>
  );
}
