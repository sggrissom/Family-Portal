import * as preact from "preact";
import * as rpc from "vlens/rpc";
import * as vlens from "vlens";
import * as server from "../../server";
import { Header, Footer } from "../../layout";
import { ensureAuthInFetch } from "../../lib/authHelpers";
import { adminView } from "../../components/AdminGuard";
import { logError } from "../../lib/logger";
import "./admin-styles";
import "./seed-styles";

const emptyRuns: server.ListSeedRunsResponse = {
  runs: [],
  defaultDomain: "example.test",
  maxScale: 4,
};

export async function fetch(route: string, prefix: string) {
  if (!(await ensureAuthInFetch())) {
    return rpc.ok<server.ListSeedRunsResponse>(emptyRuns);
  }

  return server.ListSeedRuns({});
}

export function view(
  route: string,
  prefix: string,
  data: server.ListSeedRunsResponse
): preact.ComponentChild {
  return adminView(() => {
    return (
      <div>
        <Header isHome={false} />
        <main id="app" className="admin-container">
          <SeedPage data={data} />
        </main>
        <Footer />
      </div>
    );
  });
}

type SeedPageState = {
  loaded: boolean;
  runs: server.SeedRunInfo[];
  domain: string;
  password: string;
  scale: number;
  creating: boolean;
  createError: string;
  created: server.CreateSeedDataResponse | null;
  createdPassword: string;
  copied: boolean;
  confirms: { [runId: number]: string };
  removing: number;
  removeError: string;
  removeResult: string;
};

const useSeedPageState = vlens.declareHook(
  (): SeedPageState => ({
    loaded: false,
    runs: [],
    domain: "",
    password: "",
    scale: 1,
    creating: false,
    createError: "",
    created: null,
    createdPassword: "",
    copied: false,
    confirms: {},
    removing: 0,
    removeError: "",
    removeResult: "",
  })
);

async function reloadRuns(state: SeedPageState) {
  const [result, error] = await server.ListSeedRuns({});
  if (error) {
    logError("admin", "Failed to load seed runs", error);
    return;
  }
  if (result) {
    state.runs = result.runs;
  }
  vlens.scheduleRedraw();
}

async function createSeedData(state: SeedPageState) {
  state.creating = true;
  state.createError = "";
  state.created = null;
  state.copied = false;
  vlens.scheduleRedraw();

  const password = state.password;
  const [result, error] = await server.CreateSeedData({
    password,
    emailDomain: state.domain,
    scale: state.scale,
  });

  state.creating = false;
  if (error) {
    state.createError = error;
  } else if (result) {
    state.created = result;
    state.createdPassword = password;
    state.password = "";
    state.runs = [result.run, ...state.runs];
  }
  vlens.scheduleRedraw();
}

async function removeSeedData(state: SeedPageState, run: server.SeedRunInfo) {
  const confirmed = confirm(
    `Delete the ${run.accounts} accounts seeded under ${run.domain}?\n\n` +
      "Everything those accounts own — people, milestones, measurements, photos, chat — goes with " +
      "them. Accounts this run did not create are left alone."
  );
  if (!confirmed) return;

  state.removing = run.id;
  state.removeError = "";
  state.removeResult = "";
  vlens.scheduleRedraw();

  const [result, error] = await server.RemoveSeedData({
    runId: run.id,
    confirmValue: state.confirms[run.id] || "",
  });

  state.removing = 0;
  if (error) {
    state.removeError = error;
  } else if (result) {
    const skipped = result.skippedEmails.length
      ? ` Left alone: ${result.skippedEmails.join(", ")}.`
      : "";
    const kept = result.survivingFamilies
      ? ` ${result.survivingFamilies} family group${result.survivingFamilies === 1 ? "" : "s"} still had other members and survived.`
      : "";
    state.removeResult = `Removed ${result.removedEmails.length} account${result.removedEmails.length === 1 ? "" : "s"} and ${result.destroyedFamilies} family group${result.destroyedFamilies === 1 ? "" : "s"}.${kept}${skipped}`;
    state.confirms[run.id] = "";
    if (state.created && state.created.run.id === run.id) {
      state.created = null;
    }
  }
  await reloadRuns(state);
}

function credentialText(created: server.CreateSeedDataResponse, password: string): string {
  const lines = created.accounts.map(account => `${account.email}  ${password}  ${account.name}`);
  return lines.join("\n");
}

async function copyCredentials(state: SeedPageState) {
  if (!state.created) return;
  try {
    await navigator.clipboard.writeText(credentialText(state.created, state.createdPassword));
    state.copied = true;
  } catch (err) {
    logError("ui", "Failed to copy credentials", err);
    state.copied = false;
  }
  vlens.scheduleRedraw();
}

interface SeedPageProps {
  data: server.ListSeedRunsResponse;
}

const SeedPage = ({ data }: SeedPageProps) => {
  const state = useSeedPageState();
  if (!state.loaded) {
    state.loaded = true;
    state.runs = data.runs;
    state.domain = data.defaultDomain;
  }

  const scales = [];
  for (let i = 1; i <= data.maxScale; i++) scales.push(i);

  return (
    <div className="admin-page">
      <div className="admin-breadcrumb">
        <a href="/admin">Admin Dashboard</a>
        <span className="breadcrumb-separator">›</span>
        <span>Test Accounts</span>
      </div>

      <div className="admin-header">
        <div className="admin-badge">
          <span className="admin-icon">🌱</span>
          <span>Test Accounts</span>
        </div>
        <h1>Seeded Test Accounts</h1>
        <p>Create the demo family on this server so a reviewer has something to sign in to.</p>
      </div>

      <div className="admin-card">
        <div className="card-header">
          <div className="admin-card-icon">📋</div>
          <h3>What this writes</h3>
        </div>
        <div className="card-content">
          <p>
            Ten accounts across five family groups: one household with five children and a
            pregnancy, two sets of grandparents linked to it with different sharing scopes, an aunt
            whose invitation is still pending, and an unrelated family that can see none of it.
            Every account gets the password you choose below, and every address is created under the
            domain you choose, so a second set never collides with the first.
          </p>
          <p className="seed-note">
            This only ever adds. It refuses outright if any of the addresses it would create already
            exists, and it never touches an account it did not create.
          </p>
        </div>
      </div>

      <div className="admin-card">
        <div className="card-header">
          <div className="admin-card-icon">🌱</div>
          <h3>Create a Set</h3>
        </div>
        <div className="card-content">
          <div className="seed-form">
            <label className="seed-field">
              <span className="seed-label">Email domain</span>
              <input
                type="text"
                className="seed-input"
                value={state.domain}
                placeholder={data.defaultDomain}
                disabled={state.creating}
                onInput={(e: any) => {
                  state.domain = e.currentTarget.value;
                }}
              />
              <span className="seed-hint">
                Addresses look like dad@{state.domain || data.defaultDomain}
              </span>
            </label>

            <label className="seed-field">
              <span className="seed-label">Password for every account</span>
              <input
                type="text"
                className="seed-input"
                value={state.password}
                placeholder="At least 8 characters"
                disabled={state.creating}
                onInput={(e: any) => {
                  state.password = e.currentTarget.value;
                }}
              />
              <span className="seed-hint">Shown once, here. Nothing stores it in the clear.</span>
            </label>

            <label className="seed-field seed-field-narrow">
              <span className="seed-label">Measurement density</span>
              <select
                className="seed-input"
                value={String(state.scale)}
                disabled={state.creating}
                onChange={(e: any) => {
                  state.scale = parseInt(e.currentTarget.value, 10) || 1;
                }}
              >
                {scales.map(value => (
                  <option key={value} value={String(value)}>
                    {value}×
                  </option>
                ))}
              </select>
              <span className="seed-hint">Higher means more growth rows per child.</span>
            </label>
          </div>

          <div className="seed-actions">
            <button
              className="admin-btn admin-btn-primary"
              onClick={() => createSeedData(state)}
              disabled={state.creating || state.password.length < 8}
            >
              {state.creating ? "Creating…" : "Create Accounts"}
            </button>
          </div>

          {state.createError && (
            <div className="seed-result seed-result-bad">{state.createError}</div>
          )}
        </div>
      </div>

      {state.created && (
        <div className="admin-card">
          <div className="card-header">
            <div className="admin-card-icon">🔑</div>
            <h3>Credentials</h3>
          </div>
          <div className="card-content">
            <p>
              Password for every account below:{" "}
              <code className="seed-mono">{state.createdPassword}</code>. It is not recoverable from
              here once you leave the page.
            </p>
            <div className="seed-actions">
              <button
                className="admin-btn admin-btn-secondary"
                onClick={() => copyCredentials(state)}
              >
                {state.copied ? "Copied" : "Copy Credentials"}
              </button>
            </div>
            <div className="table-wrapper">
              <table className="users-table">
                <thead>
                  <tr>
                    <th>Email</th>
                    <th>Name</th>
                    <th>Family</th>
                    <th>Access</th>
                  </tr>
                </thead>
                <tbody>
                  {state.created.accounts.map(account => (
                    <tr key={account.email}>
                      <td className="seed-mono">{account.email}</td>
                      <td>{account.name}</td>
                      <td>{account.family}</td>
                      <td>{account.access}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
            <p className="seed-note">
              {state.created.people} people, {state.created.milestones} milestones,{" "}
              {state.created.measurements} measurements, {state.created.chatMessages} chat messages.
            </p>
          </div>
        </div>
      )}

      <div className="admin-section">
        <h2>Seeded Sets ({state.runs.length})</h2>

        {state.removeError && (
          <div className="seed-result seed-result-bad">{state.removeError}</div>
        )}
        {state.removeResult && (
          <div className="seed-result seed-result-ok">{state.removeResult}</div>
        )}

        {state.runs.length === 0 ? (
          <div className="admin-empty-state">
            <p>Nothing has been seeded on this server.</p>
          </div>
        ) : (
          <div className="users-table-container">
            <div className="table-wrapper">
              <table className="users-table">
                <thead>
                  <tr>
                    <th>Domain</th>
                    <th>Created</th>
                    <th>Accounts</th>
                    <th>Families</th>
                    <th>Remove</th>
                  </tr>
                </thead>
                <tbody>
                  {state.runs.map(run => (
                    <tr key={run.id} className="admin-row">
                      <td className="seed-mono">{run.domain}</td>
                      <td className="user-created">{run.createdAt}</td>
                      <td>
                        {run.existing} of {run.accounts} still present
                      </td>
                      <td>{run.families}</td>
                      <td>
                        <div className="seed-remove">
                          <input
                            type="text"
                            className="seed-input seed-input-small"
                            aria-label={`Type ${run.domain} to confirm`}
                            placeholder={`Type ${run.domain}`}
                            value={state.confirms[run.id] || ""}
                            disabled={state.removing === run.id}
                            onInput={(e: any) => {
                              state.confirms[run.id] = e.currentTarget.value;
                              vlens.scheduleRedraw();
                            }}
                          />
                          <button
                            className="admin-btn admin-btn-small seed-btn-danger"
                            onClick={() => removeSeedData(state, run)}
                            disabled={
                              state.removing === run.id ||
                              (state.confirms[run.id] || "").trim().toLowerCase() !== run.domain
                            }
                          >
                            {state.removing === run.id ? "Removing…" : "Remove"}
                          </button>
                        </div>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </div>
        )}
      </div>
    </div>
  );
};
