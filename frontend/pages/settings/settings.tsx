import * as preact from "preact";
import * as vlens from "vlens";
import * as server from "../../server";
import { Header, Footer } from "../../layout";
import { ensureAuthInFetch, requireAuthInView } from "../../lib/authHelpers";
import { logError } from "../../lib/logger";
import "./settings-styles";

type Data = {
  familyInfo: server.FamilyInfoResponse;
  people: server.Person[];
};

type JoinFamilyForm = {
  inviteCode: string;
  error: string;
  loading: boolean;
  success: boolean;
};

type ExportForm = {
  loading: boolean;
  error: string;
  success: boolean;
};

type MergeForm = {
  sourcePersonId: number;
  targetPersonId: number;
  loading: boolean;
  error: string;
  success: boolean;
  showConfirmation: boolean;
  previewData: {
    sourceName: string;
    targetName: string;
    milestoneCount: number;
    growthCount: number;
    photoCount: number;
  } | null;
};

const useJoinFamilyForm = vlens.declareHook(
  (): JoinFamilyForm => ({
    inviteCode: "",
    error: "",
    loading: false,
    success: false,
  })
);

const useExportForm = vlens.declareHook(
  (): ExportForm => ({
    loading: false,
    error: "",
    success: false,
  })
);

const useMergeForm = vlens.declareHook(
  (): MergeForm => ({
    sourcePersonId: 0,
    targetPersonId: 0,
    loading: false,
    error: "",
    success: false,
    showConfirmation: false,
    previewData: null,
  })
);

export async function fetch(route: string, prefix: string) {
  if (!(await ensureAuthInFetch())) {
    return vlens.rpcOk({ familyInfo: { id: 0, name: "", inviteCode: "" }, people: [] });
  }

  const [familyInfo] = await server.GetFamilyInfo({});
  const [peopleResp] = await server.ListPeople({});

  return vlens.rpcOk({
    familyInfo: familyInfo || { id: 0, name: "", inviteCode: "" },
    people: peopleResp?.people || [],
  });
}

export function view(route: string, prefix: string, data: Data): preact.ComponentChild {
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
    const button = document.querySelector(".copy-button") as HTMLButtonElement;
    if (button) {
      const originalText = button.textContent;
      button.textContent = "Copied!";
      button.classList.add("copied");
      setTimeout(() => {
        button.textContent = originalText;
        button.classList.remove("copied");
      }, 2000);
    }
  } catch (err) {
    // Fallback for browsers that don't support clipboard API
    logError("ui", "Failed to copy to clipboard", err);
    alert("Failed to copy link to clipboard");
  }
}

async function onJoinFamilyClicked(form: JoinFamilyForm, event: Event) {
  event.preventDefault();
  form.loading = true;
  form.error = "";
  form.success = false;

  let [resp, err] = await server.JoinFamily({
    inviteCode: form.inviteCode,
  });

  form.loading = false;

  if (resp && resp.success) {
    form.success = true;
    form.inviteCode = "";
    form.error = "";

    // Update auth cache and reload the page to show new family info
    setTimeout(() => {
      window.location.reload();
    }, 1500);
  } else {
    form.error = resp?.error || err || "Failed to join family";
  }
  vlens.scheduleRedraw();
}

async function onExportDataClicked(form: ExportForm, event: Event) {
  event.preventDefault();
  form.loading = true;
  form.error = "";
  form.success = false;

  try {
    let [resp, err] = await server.ExportData({});

    form.loading = false;

    if (resp && resp.jsonData) {
      // Create downloadable file
      const timestamp = new Date().toISOString().split("T")[0]; // YYYY-MM-DD format
      const filename = `family-data-${timestamp}.json`;

      const blob = new Blob([resp.jsonData], { type: "application/json" });
      const url = URL.createObjectURL(blob);

      // Create download link and trigger download
      const link = document.createElement("a");
      link.href = url;
      link.download = filename;
      document.body.appendChild(link);
      link.click();
      document.body.removeChild(link);
      URL.revokeObjectURL(url);

      form.success = true;
      form.error = "";

      // Clear success message after 3 seconds
      setTimeout(() => {
        form.success = false;
        vlens.scheduleRedraw();
      }, 3000);
    } else {
      form.error = err || "Failed to export data";
    }
  } catch (error) {
    form.loading = false;
    form.error = "Failed to export data";
    logError("ui", "Export failed", error);
  }
  vlens.scheduleRedraw();
}

async function onMergePreview(form: MergeForm, people: server.Person[], event: Event) {
  event.preventDefault();

  if (form.sourcePersonId === 0 || form.targetPersonId === 0) {
    form.error = "Please select both source and target people";
    vlens.scheduleRedraw();
    return;
  }

  if (form.sourcePersonId === form.targetPersonId) {
    form.error = "Cannot merge a person with themselves";
    vlens.scheduleRedraw();
    return;
  }

  form.loading = true;
  form.error = "";

  // Get person details for preview
  const sourcePerson = people.find(p => p.id === form.sourcePersonId);
  const targetPerson = people.find(p => p.id === form.targetPersonId);

  if (!sourcePerson || !targetPerson) {
    form.error = "Selected people not found";
    form.loading = false;
    vlens.scheduleRedraw();
    return;
  }

  // Fetch full person data to get counts
  const [sourceData] = await server.GetPerson({ id: form.sourcePersonId });

  form.loading = false;

  if (sourceData) {
    form.previewData = {
      sourceName: sourcePerson.name,
      targetName: targetPerson.name,
      milestoneCount: sourceData.milestones?.length || 0,
      growthCount: sourceData.growthData?.length || 0,
      photoCount: sourceData.photos?.length || 0,
    };
    form.showConfirmation = true;
  } else {
    form.error = "Failed to load person data";
  }

  vlens.scheduleRedraw();
}

async function onMergeConfirm(form: MergeForm, event: Event) {
  event.preventDefault();
  form.loading = true;
  form.error = "";

  const [resp, err] = await server.MergePeople({
    sourcePersonId: form.sourcePersonId,
    targetPersonId: form.targetPersonId,
  });

  form.loading = false;

  if (resp && resp.success) {
    form.success = true;
    form.showConfirmation = false;
    form.sourcePersonId = 0;
    form.targetPersonId = 0;
    form.previewData = null;

    // Reload page after 2 seconds to show updated people list
    setTimeout(() => {
      window.location.reload();
    }, 2000);
  } else {
    form.error = err || "Failed to merge people";
  }

  vlens.scheduleRedraw();
}

function onMergeCancel(form: MergeForm) {
  form.showConfirmation = false;
  form.error = "";
  vlens.scheduleRedraw();
}

const SettingsPage = ({ data }: SettingsPageProps) => {
  const baseUrl = typeof window !== "undefined" ? window.location.origin : "";
  const inviteLink = `${baseUrl}/create-account?code=${data.familyInfo.inviteCode}`;
  const joinForm = useJoinFamilyForm();
  const exportForm = useExportForm();
  const mergeForm = useMergeForm();

  return (
    <div className="settings-page">
      <div className="settings-header">
        <h1>Family Settings</h1>
        <p>Manage your family portal</p>
      </div>

      <div className="settings-sections">
        {/* Join Another Family (only show if user has a family but wants to join another) */}
        <div className="settings-section">
          <h2>Join Another Family</h2>
          <div className="settings-card">
            <p className="section-description">
              Have a family invite code? Enter it below to join another family.
            </p>

            {joinForm.success && (
              <div className="success-message">Successfully joined family! Reloading page...</div>
            )}

            {joinForm.error && <div className="error-message">{joinForm.error}</div>}

            <form onSubmit={vlens.cachePartial(onJoinFamilyClicked, joinForm)}>
              <div className="form-group">
                <label htmlFor="joinInviteCode">Family Invite Code</label>
                <input
                  type="text"
                  id="joinInviteCode"
                  placeholder="Enter family invite code"
                  {...vlens.attrsBindInput(vlens.ref(joinForm, "inviteCode"))}
                  disabled={joinForm.loading}
                  required
                />
              </div>

              <button
                type="submit"
                className="btn btn-primary"
                disabled={joinForm.loading || !joinForm.inviteCode}
              >
                {joinForm.loading ? "Joining..." : "Join Family"}
              </button>
            </form>
          </div>
        </div>

        {/* Family Information - only show if user is in a family */}
        {data.familyInfo.id > 0 && (
          <div className="settings-section">
            <h2>Family Information</h2>
            <div className="settings-card">
              <div className="form-group">
                <label>Family Name</label>
                <div className="readonly-field">{data.familyInfo.name}</div>
              </div>
            </div>
          </div>
        )}

        {/* Data Management - only show if user is in a family */}
        {data.familyInfo.id > 0 && (
          <div className="settings-section">
            <h2>Data Management</h2>
            <div className="settings-card">
              <p className="section-description">
                Export or import your family's data for backup or migration purposes.
              </p>

              <div className="data-management-actions">
                <div className="data-action">
                  <h4>Export Data</h4>
                  <p>Download your family's data including people, milestones, and measurements.</p>

                  {exportForm.success && (
                    <div className="success-message">Data exported successfully!</div>
                  )}

                  {exportForm.error && <div className="error-message">{exportForm.error}</div>}

                  <button
                    type="button"
                    className="btn btn-primary"
                    onClick={vlens.cachePartial(onExportDataClicked, exportForm)}
                    disabled={exportForm.loading}
                  >
                    {exportForm.loading ? "Exporting..." : "📥 Export Data"}
                  </button>
                </div>

                <div className="data-action">
                  <h4>Import Data</h4>
                  <p>Upload data from a previous export or another family portal.</p>

                  <a href="/import" className="btn btn-secondary">
                    📤 Import Data
                  </a>
                </div>
              </div>
            </div>
          </div>
        )}

        {/* Advanced Data Management - only show if user is in a family and has people */}
        {data.familyInfo.id > 0 && data.people.length > 1 && (
          <div className="settings-section">
            <h2>Advanced Data Management</h2>
            <div className="settings-card merge-card">
              <div className="warning-banner">
                <strong>⚠️ Warning:</strong> This is a destructive operation that cannot be undone.
              </div>

              <h4>Merge People</h4>
              <p className="section-description">
                Combine two person records into one. All data (milestones, growth records, photos)
                from the source person will be moved to the target person, and the source person
                will be permanently deleted.
              </p>

              {mergeForm.success && (
                <div className="success-message">People merged successfully! Reloading page...</div>
              )}

              {mergeForm.error && <div className="error-message">{mergeForm.error}</div>}

              {!mergeForm.showConfirmation && (
                <form onSubmit={vlens.cachePartial(onMergePreview, mergeForm, data.people)}>
                  <div className="merge-selectors">
                    <div className="form-group">
                      <label htmlFor="sourcePerson">Merge From (will be deleted)</label>
                      <select
                        id="sourcePerson"
                        value={mergeForm.sourcePersonId}
                        onChange={e => {
                          mergeForm.sourcePersonId = parseInt(
                            (e.target as HTMLSelectElement).value
                          );
                          vlens.scheduleRedraw();
                        }}
                        disabled={mergeForm.loading}
                        required
                      >
                        <option value="0">Select person to merge from...</option>
                        {data.people.map(person => (
                          <option key={person.id} value={person.id}>
                            {person.name}
                          </option>
                        ))}
                      </select>
                    </div>

                    <div className="merge-arrow">→</div>

                    <div className="form-group">
                      <label htmlFor="targetPerson">Merge Into (will keep)</label>
                      <select
                        id="targetPerson"
                        value={mergeForm.targetPersonId}
                        onChange={e => {
                          mergeForm.targetPersonId = parseInt(
                            (e.target as HTMLSelectElement).value
                          );
                          vlens.scheduleRedraw();
                        }}
                        disabled={mergeForm.loading}
                        required
                      >
                        <option value="0">Select person to merge into...</option>
                        {data.people.map(person => (
                          <option key={person.id} value={person.id}>
                            {person.name}
                          </option>
                        ))}
                      </select>
                    </div>
                  </div>

                  <button
                    type="submit"
                    className="btn btn-warning"
                    disabled={
                      mergeForm.loading ||
                      mergeForm.sourcePersonId === 0 ||
                      mergeForm.targetPersonId === 0
                    }
                  >
                    {mergeForm.loading ? "Loading..." : "Preview Merge"}
                  </button>
                </form>
              )}

              {mergeForm.showConfirmation && mergeForm.previewData && (
                <div className="merge-confirmation">
                  <h4>Confirm Merge</h4>
                  <div className="merge-preview">
                    <p>
                      <strong>Source:</strong> {mergeForm.previewData.sourceName} (will be deleted)
                    </p>
                    <p>
                      <strong>Target:</strong> {mergeForm.previewData.targetName} (will keep all
                      data)
                    </p>
                    <div className="merge-stats">
                      <p>The following will be moved from source to target:</p>
                      <ul>
                        <li>{mergeForm.previewData.milestoneCount} milestone(s)</li>
                        <li>{mergeForm.previewData.growthCount} growth record(s)</li>
                        <li>{mergeForm.previewData.photoCount} photo association(s)</li>
                      </ul>
                    </div>
                  </div>
                  <div className="confirmation-actions">
                    <button
                      type="button"
                      className="btn btn-danger"
                      onClick={vlens.cachePartial(onMergeConfirm, mergeForm)}
                      disabled={mergeForm.loading}
                    >
                      {mergeForm.loading ? "Merging..." : "Confirm Merge"}
                    </button>
                    <button
                      type="button"
                      className="btn btn-secondary"
                      onClick={() => onMergeCancel(mergeForm)}
                      disabled={mergeForm.loading}
                    >
                      Cancel
                    </button>
                  </div>
                </div>
              )}
            </div>
          </div>
        )}

        {/* Invite Members - only show if user is in a family */}
        {data.familyInfo.id > 0 && (
          <div className="settings-section">
            <h2>Invite Family Members</h2>
            <div className="settings-card">
              <p className="section-description">
                Share this link with family members to invite them to join your family portal.
              </p>

              <div className="form-group">
                <label>Family Invite Code</label>
                <div className="invite-code-display">
                  <span className="invite-code">{data.familyInfo.inviteCode}</span>
                </div>
              </div>

              <div className="form-group">
                <label>Invite Link</label>
                <div className="invite-link-display">
                  <input type="text" value={inviteLink} readOnly className="invite-link-input" />
                  <button
                    type="button"
                    className="btn btn-primary copy-button"
                    onClick={() => copyInviteLink(data.familyInfo.inviteCode)}
                  >
                    Copy Link
                  </button>
                </div>
              </div>

              <div className="invite-instructions">
                <h4>How to invite family members:</h4>
                <ol>
                  <li>Click "Copy Link" above to copy the invite link</li>
                  <li>
                    Share the link with your family member via text, email, or any messaging app
                  </li>
                  <li>
                    When they click the link, it will take them to the account creation page with
                    your family code pre-filled
                  </li>
                  <li>
                    They just need to fill out their information and they'll automatically join your
                    family
                  </li>
                </ol>
              </div>
            </div>
          </div>
        )}
      </div>
    </div>
  );
};
