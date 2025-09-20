import * as preact from "preact";
import * as vlens from "vlens";
import * as server from "./server";
import { Header, Footer } from "./layout";
import { ensureAuthInFetch, requireAuthInView } from "./authHelpers";
import "./settings-styles";

type Data = server.FamilyInfoResponse;

export async function fetch(route: string, prefix: string) {
  if (!await ensureAuthInFetch()) {
    return vlens.rpcOk({ id: 0, name: "", inviteCode: "" });
  }

  return server.GetFamilyInfo({});
}

export function view(
  route: string,
  prefix: string,
  data: Data,
): preact.ComponentChild {
  const currentAuth = requireAuthInView();
  if (!currentAuth) {
    return;
  }

  return (
    <div>
      <Header isHome={false} />
      <main id="app" className="settings-container">
        <SettingsPage data={data} />
      </main>
      <Footer />
    </div>
  );
}

interface SettingsPageProps {
  data: Data;
}

async function copyInviteLink(inviteCode: string) {
  const baseUrl = window.location.origin;
  const inviteLink = `${baseUrl}/create-account?code=${inviteCode}`;

  try {
    await navigator.clipboard.writeText(inviteLink);

    // Show temporary success message
    const button = document.querySelector('.copy-button') as HTMLButtonElement;
    if (button) {
      const originalText = button.textContent;
      button.textContent = "Copied!";
      button.classList.add('copied');
      setTimeout(() => {
        button.textContent = originalText;
        button.classList.remove('copied');
      }, 2000);
    }
  } catch (err) {
    // Fallback for browsers that don't support clipboard API
    console.error('Failed to copy: ', err);
    alert('Failed to copy link to clipboard');
  }
}

const SettingsPage = ({ data }: SettingsPageProps) => {
  const baseUrl = typeof window !== 'undefined' ? window.location.origin : '';
  const inviteLink = `${baseUrl}/create-account?code=${data.inviteCode}`;

  return (
    <div className="settings-page">
      <div className="settings-header">
        <h1>Family Settings</h1>
        <p>Manage your family portal</p>
      </div>

      <div className="settings-sections">
        {/* Family Information */}
        <div className="settings-section">
          <h2>Family Information</h2>
          <div className="settings-card">
            <div className="form-group">
              <label>Family Name</label>
              <div className="readonly-field">{data.name}</div>
            </div>
          </div>
        </div>

        {/* Invite Members */}
        <div className="settings-section">
          <h2>Invite Family Members</h2>
          <div className="settings-card">
            <p className="section-description">
              Share this link with family members to invite them to join your family portal.
            </p>

            <div className="form-group">
              <label>Family Invite Code</label>
              <div className="invite-code-display">
                <span className="invite-code">{data.inviteCode}</span>
              </div>
            </div>

            <div className="form-group">
              <label>Invite Link</label>
              <div className="invite-link-display">
                <input
                  type="text"
                  value={inviteLink}
                  readOnly
                  className="invite-link-input"
                />
                <button
                  type="button"
                  className="btn btn-primary copy-button"
                  onClick={() => copyInviteLink(data.inviteCode)}
                >
                  Copy Link
                </button>
              </div>
            </div>

            <div className="invite-instructions">
              <h4>How to invite family members:</h4>
              <ol>
                <li>Click "Copy Link" above to copy the invite link</li>
                <li>Share the link with your family member via text, email, or any messaging app</li>
                <li>When they click the link, it will take them to the account creation page with your family code pre-filled</li>
                <li>They just need to fill out their information and they'll automatically join your family</li>
              </ol>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
};