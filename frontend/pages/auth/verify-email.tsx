import * as preact from "preact";
import * as rpc from "vlens/rpc";
import * as server from "../../server";
import * as auth from "../../lib/authCache";
import { Header, Footer } from "../../layout";
import "./login-styles";

type Data = {
  success: boolean;
  error: string;
};

function tokenFromLocation(): string {
  if (typeof window === "undefined") return "";
  return new URLSearchParams(window.location.search).get("token") ?? "";
}

export async function fetch(route: string, prefix: string) {
  const token = tokenFromLocation();
  if (!token) {
    return rpc.ok<Data>({ success: false, error: "This confirmation link is missing its code." });
  }

  let [resp, err] = await server.VerifyEmail({ token });
  if (err || !resp) {
    return rpc.ok<Data>({
      success: false,
      error: "That confirmation could not be completed. Try the link again.",
    });
  }

  if (resp.success) {
    // Refresh the cached auth so the banner stops asking in this browser.
    const [authResp, authErr] = await server.GetAuthContext({});
    if (!authErr && authResp && authResp.id > 0) {
      auth.setAuth(authResp);
    }
  }

  return rpc.ok<Data>({ success: resp.success, error: resp.error ?? "" });
}

export function view(route: string, prefix: string, data: Data): preact.ComponentChild {
  return (
    <div>
      <Header isHome={false} />
      <main id="app" className="login-container">
        <div className="login-card">
          <h1>{data.success ? "Email confirmed" : "Confirmation failed"}</h1>
          <p>
            {data.success
              ? "Thanks — we know we can reach you at this address now."
              : data.error || "This confirmation link is no longer valid."}
          </p>
          <a href="/dashboard" className="btn btn-primary">
            Go to dashboard
          </a>
        </div>
      </main>
      <Footer />
    </div>
  );
}
