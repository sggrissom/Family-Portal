import * as preact from "preact";
import * as vlens from "vlens";
import * as rpc from "vlens/rpc";
import * as server from "../../server";
import { Header, Footer } from "../../layout";
import { ensureAuthInFetch, requireAuthInView } from "../../lib/authHelpers";
import "./admin-styles";
import "./app-versions-styles";

const emptyVersions: server.AdminGetMobileVersionsResponse = { platforms: [] };

export async function fetch(route: string, prefix: string) {
  if (!(await ensureAuthInFetch())) {
    return rpc.ok<server.AdminGetMobileVersionsResponse>(emptyVersions);
  }

  return server.AdminGetMobileVersions({});
}

export function view(
  route: string,
  prefix: string,
  data: server.AdminGetMobileVersionsResponse
): preact.ComponentChild {
  const currentAuth = requireAuthInView();
  if (!currentAuth) {
    return;
  }

  if (!currentAuth.isAdmin) {
    return (
      <div>
        <Header isHome={false} />
        <main id="app" className="page-container">
          <div className="error-page">
            <h1>Access Denied</h1>
            <p>You do not have permission to access this page.</p>
            <a href="/admin" className="btn btn-primary">
              Return to Admin Dashboard
            </a>
          </div>
        </main>
        <Footer />
      </div>
    );
  }

  return (
    <div>
      <Header isHome={false} />
      <main id="app" className="admin-container">
        <div className="admin-page">
          <div className="admin-breadcrumb">
            <a href="/admin">Admin Dashboard</a>
            <span className="breadcrumb-separator">›</span>
            <span>App Versions</span>
          </div>

          <div className="admin-header">
            <div className="admin-badge">
              <span className="admin-icon">📱</span>
              <span>App Versions</span>
            </div>
            <h1>App Versions</h1>
            <p>
              What the companion app is told about itself before anybody signs in: the oldest build
              still allowed to run, the build to suggest, and where to send someone to update.
            </p>
          </div>

          <div className="version-explainer">
            <dl>
              <div>
                <dt>Minimum version</dt>
                <dd>
                  Anything below it is refused outright — the app shows the message and the store
                  link, and will not continue. Never raise it past a build that is actually
                  available.
                </dd>
              </div>
              <div>
                <dt>Latest version</dt>
                <dd>Anything below it is offered an update it can dismiss.</dd>
              </div>
              <div>
                <dt>Leave both blank</dt>
                <dd>Every build is accepted. This is the state a platform starts in.</dd>
              </div>
            </dl>
          </div>

          {data.platforms.map(platform => (
            <PlatformForm key={platform.platform} platform={platform} />
          ))}
        </div>
      </main>
      <Footer />
    </div>
  );
}

type VersionForm = {
  minimumVersion: string;
  latestVersion: string;
  updateUrl: string;
  updateMessage: string;
  saving: boolean;
  error: string;
  saved: boolean;
};

const useVersionForm = vlens.declareHook(
  (
    platform: string,
    minimumVersion: string,
    latestVersion: string,
    updateUrl: string,
    updateMessage: string
  ): VersionForm => ({
    minimumVersion,
    latestVersion,
    updateUrl,
    updateMessage,
    saving: false,
    error: "",
    saved: false,
  })
);

async function onSave(platform: string, form: VersionForm, event: Event) {
  event.preventDefault();

  form.saving = true;
  form.error = "";
  form.saved = false;
  vlens.scheduleRedraw();

  const [resp, err] = await server.AdminSetMobileVersion({
    platform,
    minimumVersion: form.minimumVersion,
    latestVersion: form.latestVersion,
    updateUrl: form.updateUrl,
    updateMessage: form.updateMessage,
  });

  form.saving = false;
  if (resp) {
    form.saved = true;
  } else {
    form.error = err || "Could not save the version policy";
  }
  vlens.scheduleRedraw();
}

const platformLabels: Record<string, string> = {
  ios: "iOS",
  android: "Android",
};

function hostList(hosts: string[]): string {
  if (hosts.length < 2) {
    return hosts[0] || "";
  }
  if (hosts.length === 2) {
    return hosts.join(" or ");
  }
  return hosts.slice(0, -1).join(", ") + ", or " + hosts[hosts.length - 1];
}

const PlatformForm = ({ platform }: { platform: server.AdminMobileVersionPlatform }) => {
  const form = useVersionForm(
    platform.platform,
    platform.minimumVersion,
    platform.latestVersion,
    platform.updateUrl,
    platform.updateMessage
  );
  const label = platformLabels[platform.platform] || platform.platform;
  const configured = platform.configured || form.saved;

  return (
    <div className="admin-card">
      <div className="card-header">
        <div className="card-icon">{platform.platform === "ios" ? "🍎" : "🤖"}</div>
        <h3>{label}</h3>
        <span className={configured ? "version-state-set" : "version-state-unset"}>
          {configured ? "Policy set" : "No policy — every build accepted"}
        </span>
      </div>
      <div className="card-content">
        {platform.warnings.length > 0 && (
          <ul className="version-warnings">
            {platform.warnings.map((warning, index) => (
              <li key={index}>{warning}</li>
            ))}
          </ul>
        )}

        <form
          className="version-form"
          onSubmit={vlens.cachePartial(onSave, platform.platform, form)}
        >
          <div className="version-grid">
            <div className="form-group">
              <label htmlFor={`${platform.platform}-minimum`}>Minimum version</label>
              <input
                type="text"
                id={`${platform.platform}-minimum`}
                placeholder="1.0.0"
                {...vlens.attrsBindInput(vlens.ref(form, "minimumVersion"))}
                disabled={form.saving}
              />
            </div>

            <div className="form-group">
              <label htmlFor={`${platform.platform}-latest`}>Latest version</label>
              <input
                type="text"
                id={`${platform.platform}-latest`}
                placeholder="1.2.0"
                {...vlens.attrsBindInput(vlens.ref(form, "latestVersion"))}
                disabled={form.saving}
              />
            </div>
          </div>

          <div className="form-group">
            <label htmlFor={`${platform.platform}-url`}>Update URL</label>
            <input
              type="url"
              id={`${platform.platform}-url`}
              placeholder={`https://${platform.allowedHosts[0] || "example.com"}/...`}
              aria-describedby={`${platform.platform}-url-hint`}
              {...vlens.attrsBindInput(vlens.ref(form, "updateUrl"))}
              disabled={form.saving}
            />
            <span className="version-hint" id={`${platform.platform}-url-hint`}>
              Must be an https link to {hostList(platform.allowedHosts)}. Required once either
              version above is set.
            </span>
          </div>

          <div className="form-group">
            <label htmlFor={`${platform.platform}-message`}>Update message</label>
            <input
              type="text"
              id={`${platform.platform}-message`}
              maxLength={200}
              placeholder="This version is no longer supported."
              aria-describedby={`${platform.platform}-message-hint`}
              {...vlens.attrsBindInput(vlens.ref(form, "updateMessage"))}
              disabled={form.saving}
            />
            <span className="version-hint" id={`${platform.platform}-message-hint`}>
              One line, shown to somebody who is not signed in. No links — the store link above is
              the only one the app will follow.
            </span>
          </div>

          <div className="version-actions">
            <button className="admin-btn admin-btn-primary" type="submit" disabled={form.saving}>
              {form.saving ? "Saving..." : `Save ${label} policy`}
            </button>
            {form.saved && <span className="version-result-ok">Saved.</span>}
            {form.error && <span className="version-result-bad">{form.error}</span>}
          </div>
        </form>
      </div>
    </div>
  );
};
