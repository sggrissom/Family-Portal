import * as preact from "preact";
import * as vlens from "vlens";
import * as rpc from "vlens/rpc";
import * as auth from "../../lib/authCache";
import * as server from "../../server";
import { Header, Footer } from "../../layout";
import { ensureAuthInFetch, requireAuthInView } from "../../lib/authHelpers";
import { logError } from "../../lib/logger";
import { FamilySelect } from "../../components/FamilySelect";
import { FamilyLinksSection } from "../../components/FamilyLinks";
import { FamilyMembersSection } from "../../components/FamilyMembers";
import "./settings-styles";

type Data = {
  familyInfo: server.FamilyInfoResponse;
  people: server.Person[];
  links: server.FamilyLinkView[];
  members: server.FamilyMemberView[];
  callerIsOwner: boolean;
  notifications: server.NotificationPreferencesResponse;
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
  exportMode: "data_only" | "with_photos";
  familyId: number;
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
    exportMode: "data_only",
    familyId: 0,
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

type ChangePasswordForm = {
  currentPassword: string;
  newPassword: string;
  confirmPassword: string;
  error: string;
  success: string;
  loading: boolean;
};

const useChangePasswordForm = vlens.declareHook(
  (): ChangePasswordForm => ({
    currentPassword: "",
    newPassword: "",
    confirmPassword: "",
    error: "",
    success: "",
    loading: false,
  })
);

type DeleteAccountForm = {
  password: string;
  confirmEmail: string;
  error: string;
  loading: boolean;
};

const useDeleteAccountForm = vlens.declareHook(
  (): DeleteAccountForm => ({
    password: "",
    confirmEmail: "",
    error: "",
    loading: false,
  })
);

type NotificationForm = {
  chatEnabled: boolean;
  showMessageText: boolean;
  saving: boolean;
  error: string;
  saved: boolean;
};

const notificationDefaults: server.NotificationPreferencesResponse = {
  chatEnabled: true,
  showMessageText: false,
};

const useNotificationForm = vlens.declareHook(
  (chatEnabled: boolean, showMessageText: boolean): NotificationForm => ({
    chatEnabled,
    showMessageText,
    saving: false,
    error: "",
    saved: false,
  })
);

async function onNotificationPreferenceChanged(
  form: NotificationForm,
  field: "chatEnabled" | "showMessageText",
  event: Event
) {
  const checked = (event.target as HTMLInputElement).checked;
  const previous = form[field];
  form[field] = checked;
  form.saving = true;
  form.error = "";
  form.saved = false;
  vlens.scheduleRedraw();

  const [resp, err] = await server.UpdateNotificationPreferences({
    chatEnabled: form.chatEnabled,
    showMessageText: form.showMessageText,
  });

  form.saving = false;
  if (resp) {
    form.chatEnabled = resp.preferences.chatEnabled;
    form.showMessageText = resp.preferences.showMessageText;
    form.saved = true;
  } else {
    form[field] = previous;
    form.error = err || "Could not save your notification settings";
  }
  vlens.scheduleRedraw();
}

type AppearanceSettings = {
  theme: "light" | "dark";
};

const useAppearanceSettings = vlens.declareHook(
  (): AppearanceSettings => ({
    theme:
      (localStorage.getItem("theme") as AppearanceSettings["theme"] | null) ??
      (window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light"),
  })
);

function setTheme(settings: AppearanceSettings, theme: AppearanceSettings["theme"]) {
  const html = document.documentElement;
  html.classList.add("theme-transition");
  html.setAttribute("data-theme", theme);
  localStorage.setItem("theme", theme);
  settings.theme = theme;
  vlens.scheduleRedraw();

  window.setTimeout(() => html.classList.remove("theme-transition"), 600);
}

export async function fetch(route: string, prefix: string) {
  if (!(await ensureAuthInFetch())) {
    return vlens.rpcOk({
      familyInfo: { id: 0, name: "", inviteCode: "", families: [] },
      people: [],
      links: [],
      members: [],
      callerIsOwner: false,
      notifications: notificationDefaults,
    });
  }

  const [familyInfo] = await server.GetFamilyInfo({});
  const [peopleResp] = await server.ListPeople({});
  const [linksResp] = await server.ListFamilyLinks({ familyId: 0 });
  const [membersResp] = await server.ListFamilyMembers({ familyId: 0 });
  const [notificationsResp] = await server.GetNotificationPreferences({});

  return vlens.rpcOk({
    familyInfo: familyInfo || { id: 0, name: "", inviteCode: "", families: [] },
    people: peopleResp?.people || [],
    links: linksResp?.links || [],
    members: membersResp?.members || [],
    callerIsOwner: membersResp?.callerIsOwner || false,
    notifications: notificationsResp || notificationDefaults,
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

    setTimeout(() => {
      window.location.reload();
    }, 1500);
  } else {
    form.error = resp?.error || err || "Failed to join family";
  }
  vlens.scheduleRedraw();
}

async function onChangePasswordClicked(form: ChangePasswordForm, event: Event) {
  event.preventDefault();
  form.loading = true;
  form.error = "";
  form.success = "";
  vlens.scheduleRedraw();

  const nativeFetch = window.fetch.bind(window);
  try {
    const res = await nativeFetch("/api/change-password", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      credentials: "include",
      body: JSON.stringify({
        currentPassword: form.currentPassword,
        newPassword: form.newPassword,
        confirmPassword: form.confirmPassword,
      }),
    });
    const result = await res.json();

    if (result.success) {
      form.currentPassword = "";
      form.newPassword = "";
      form.confirmPassword = "";
      if (result.token) {
        rpc.setAuthHeaders({ "x-auth-token": result.token });
        form.success = "Password changed. Other devices have been signed out.";
      } else {
        form.success = result.error || "Password changed. Please sign in again.";
      }
    } else {
      form.error = result.error || "Failed to change password";
    }
  } catch (e) {
    logError("ui", "Password change failed", e);
    form.error = "Network error. Please try again.";
  }

  form.loading = false;
  vlens.scheduleRedraw();
}

async function onDeleteAccountClicked(form: DeleteAccountForm, event: Event) {
  event.preventDefault();

  if (
    !confirm(
      "Delete your account? This cannot be undone. Any family you are the only member of is deleted with everything in it — people, photos, measurements and milestones."
    )
  ) {
    return;
  }

  form.loading = true;
  form.error = "";
  vlens.scheduleRedraw();

  const nativeFetch = window.fetch.bind(window);
  try {
    const res = await nativeFetch("/api/delete-account", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      credentials: "include",
      body: JSON.stringify({
        password: form.password,
        confirmEmail: form.confirmEmail,
      }),
    });
    const result = await res.json();

    if (result.success) {
      auth.clearAuth();
      window.location.href = "/";
      return;
    }
    form.error = result.error || "Could not delete your account";
  } catch (e) {
    logError("ui", "Account deletion failed", e);
    form.error = "Network error. Please try again.";
  }

  form.loading = false;
  vlens.scheduleRedraw();
}

async function onRotateInviteCode(familyId: number, familyName: string) {
  if (
    !confirm(
      `Generate a new invite code for ${familyName}? Any link or code you have already shared stops working, and anyone still waiting to join will need the new one.`
    )
  ) {
    return;
  }

  const [resp, err] = await server.RotateInviteCode({ familyId });
  if (resp && resp.success) {
    window.location.reload();
    return;
  }
  alert(resp?.error || err || "Could not generate a new invite code");
}

async function onExportDataClicked(exportForm: ExportForm) {
  exportForm.loading = true;
  exportForm.error = "";
  exportForm.success = false;
  vlens.scheduleRedraw();
  try {
    const timestamp = new Date().toISOString().split("T")[0];

    if (exportForm.exportMode === "data_only") {
      const [resp, err] = await server.ExportData({ familyId: exportForm.familyId });
      if (!resp || !resp.jsonData) {
        throw new Error(err || "Failed to export data");
      }
      const blob = new Blob([resp.jsonData], { type: "application/json" });
      const url = URL.createObjectURL(blob);
      const link = document.createElement("a");
      link.href = url;
      link.download = `family-data-${timestamp}.json`;
      document.body.appendChild(link);
      link.click();
      document.body.removeChild(link);
      URL.revokeObjectURL(url);
    } else {
      const familyQuery = exportForm.familyId ? `&familyId=${exportForm.familyId}` : "";
      const resp = await window.fetch(`/api/export-bundle?mode=with_photos${familyQuery}`, {
        credentials: "include",
      });
      if (!resp.ok) {
        throw new Error(`Export failed: ${resp.statusText}`);
      }
      const blob = await resp.blob();
      const url = URL.createObjectURL(blob);
      const link = document.createElement("a");
      link.href = url;
      link.download = `family-export-${timestamp}.zip`;
      document.body.appendChild(link);
      link.click();
      document.body.removeChild(link);
      URL.revokeObjectURL(url);
    }

    exportForm.success = true;
    vlens.scheduleRedraw();
    setTimeout(() => {
      exportForm.success = false;
      vlens.scheduleRedraw();
    }, 3000);
  } catch (e: any) {
    exportForm.error = e?.message ?? "Export failed";
    logError("ui", "Export failed", e);
  } finally {
    exportForm.loading = false;
    vlens.scheduleRedraw();
  }
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

  const sourcePerson = people.find(p => p.id === form.sourcePersonId);
  const targetPerson = people.find(p => p.id === form.targetPersonId);

  if (!sourcePerson || !targetPerson) {
    form.error = "Selected people not found";
    form.loading = false;
    vlens.scheduleRedraw();
    return;
  }

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
  const families =
    data.familyInfo.families && data.familyInfo.families.length > 0
      ? data.familyInfo.families
      : data.familyInfo.id > 0
        ? [
            {
              id: data.familyInfo.id,
              name: data.familyInfo.name,
              inviteCode: data.familyInfo.inviteCode,
              role: 3,
              isPrimary: true,
            },
          ]
        : [];
  const joinForm = useJoinFamilyForm();
  const exportForm = useExportForm();
  const mergeForm = useMergeForm();
  const passwordForm = useChangePasswordForm();
  const deleteForm = useDeleteAccountForm();
  const appearance = useAppearanceSettings();
  const notifications = useNotificationForm(
    data.notifications.chatEnabled,
    data.notifications.showMessageText
  );
  const accountEmail = auth.getAuth()?.email ?? "";

  return (
    <div className="settings-page">
      <div className="settings-header">
        <h1>Family Settings</h1>
        <p>Manage your family portal</p>
      </div>

      <div className="settings-sections">
        <div className="settings-section">
          <h2>Appearance</h2>
          <div className="settings-card appearance-card">
            <div>
              <h3>Color theme</h3>
              <p className="section-description">
                Choose the look that is most comfortable for you. This preference is saved on this
                device.
              </p>
            </div>
            <div className="theme-options" role="group" aria-label="Color theme">
              <button
                type="button"
                className={appearance.theme === "light" ? "theme-option selected" : "theme-option"}
                aria-pressed={appearance.theme === "light"}
                onClick={() => setTheme(appearance, "light")}
              >
                <span className="theme-preview theme-preview-light" aria-hidden="true">
                  <span />
                </span>
                <span>
                  <strong>Light</strong>
                  <small>Bright and clear</small>
                </span>
              </button>
              <button
                type="button"
                className={appearance.theme === "dark" ? "theme-option selected" : "theme-option"}
                aria-pressed={appearance.theme === "dark"}
                onClick={() => setTheme(appearance, "dark")}
              >
                <span className="theme-preview theme-preview-dark" aria-hidden="true">
                  <span />
                </span>
                <span>
                  <strong>Dark</strong>
                  <small>Easy on the eyes</small>
                </span>
              </button>
            </div>
          </div>
        </div>

        <div className="settings-section">
          <h2>Notifications</h2>
          <div className="settings-card">
            <p className="section-description">
              These control push notifications on the Family Portal app for iPhone. They follow your
              account, not the phone, so they apply to every device you sign in on.
            </p>

            {notifications.saved && (
              <div className="success-message">Notification settings saved.</div>
            )}
            {notifications.error && (
              <div className="error-message" role="alert">
                {notifications.error}
              </div>
            )}

            <div className="notification-options">
              <label className="notification-option">
                <input
                  type="checkbox"
                  checked={notifications.chatEnabled}
                  disabled={notifications.saving}
                  onChange={vlens.cachePartial(
                    onNotificationPreferenceChanged,
                    notifications,
                    "chatEnabled"
                  )}
                />
                <span>
                  <strong>Chat messages</strong>
                  <small>
                    Notify me when someone in my family sends a message while I am away from the
                    app.
                  </small>
                </span>
              </label>

              <label className="notification-option">
                <input
                  type="checkbox"
                  checked={notifications.showMessageText}
                  disabled={notifications.saving}
                  onChange={vlens.cachePartial(
                    onNotificationPreferenceChanged,
                    notifications,
                    "showMessageText"
                  )}
                />
                <span>
                  <strong>Show message text on the lock screen</strong>
                  <small>
                    Off by default: a notification says only that a message arrived, and you see who
                    sent it and what it says after unlocking. Turn this on to show the sender and
                    the message itself on the lock screen.
                  </small>
                </span>
              </label>
            </div>
          </div>
        </div>

        <div className="settings-section">
          <h2>Security</h2>
          <div className="settings-card">
            <h3>Change password</h3>
            <p className="section-description">
              Changing your password signs out every other device you are signed in on. This device
              stays signed in.
            </p>

            {passwordForm.success && <div className="success-message">{passwordForm.success}</div>}
            {passwordForm.error && (
              <div className="error-message" role="alert">
                {passwordForm.error}
              </div>
            )}

            <form onSubmit={vlens.cachePartial(onChangePasswordClicked, passwordForm)}>
              <div className="form-group">
                <label htmlFor="currentPassword">Current password</label>
                <input
                  type="password"
                  id="currentPassword"
                  autoComplete="current-password"
                  {...vlens.attrsBindInput(vlens.ref(passwordForm, "currentPassword"))}
                  disabled={passwordForm.loading}
                  required
                />
              </div>

              <div className="form-group">
                <label htmlFor="newPassword">New password</label>
                <input
                  type="password"
                  id="newPassword"
                  autoComplete="new-password"
                  minLength={8}
                  {...vlens.attrsBindInput(vlens.ref(passwordForm, "newPassword"))}
                  disabled={passwordForm.loading}
                  required
                />
              </div>

              <div className="form-group">
                <label htmlFor="confirmNewPassword">Confirm new password</label>
                <input
                  type="password"
                  id="confirmNewPassword"
                  autoComplete="new-password"
                  minLength={8}
                  {...vlens.attrsBindInput(vlens.ref(passwordForm, "confirmPassword"))}
                  disabled={passwordForm.loading}
                  required
                />
              </div>

              <button
                type="submit"
                className="btn btn-primary"
                disabled={
                  passwordForm.loading ||
                  !passwordForm.currentPassword ||
                  !passwordForm.newPassword ||
                  !passwordForm.confirmPassword
                }
              >
                {passwordForm.loading ? "Changing..." : "Change Password"}
              </button>
            </form>
          </div>
        </div>

        <div className="settings-section">
          <h2>Join Another Family</h2>
          <div className="settings-card">
            <p className="section-description">
              Have a family invite code? Enter it below to join another family.
            </p>

            {joinForm.success && (
              <div className="success-message">Successfully joined family! Reloading page...</div>
            )}

            {joinForm.error && (
              <div className="error-message" role="alert">
                {joinForm.error}
              </div>
            )}

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

        {families.length > 0 && (
          <div className="settings-section">
            <h2>{families.length > 1 ? "Your Families" : "Family Information"}</h2>
            <div className="settings-card">
              <div className="form-group">
                <label>{families.length > 1 ? "Families" : "Family Name"}</label>
                {families.map(family => (
                  <div key={family.id} className="readonly-field">
                    {family.name}
                    {families.length > 1 && family.isPrimary ? " (primary)" : ""}
                  </div>
                ))}
              </div>
            </div>
          </div>
        )}

        {families.length > 0 && (
          <FamilyMembersSection
            initialMembers={data.members}
            initialCallerIsOwner={data.callerIsOwner}
            familyName={families[0].name}
          />
        )}

        {families.length > 0 && <FamilyLinksSection initialLinks={data.links} />}

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

                  {exportForm.error && (
                    <div className="error-message" role="alert">
                      {exportForm.error}
                    </div>
                  )}

                  <FamilySelect
                    id="exportFamilyId"
                    label="Family to export"
                    value={exportForm.familyId}
                    onChange={familyId => {
                      exportForm.familyId = familyId;
                    }}
                    disabled={exportForm.loading}
                  />

                  <div className="export-mode-group">
                    <label className="export-mode-option">
                      <input
                        type="radio"
                        name="exportMode"
                        checked={exportForm.exportMode === "data_only"}
                        onChange={() => {
                          exportForm.exportMode = "data_only";
                          vlens.scheduleRedraw();
                        }}
                      />
                      <div className="export-mode-label">
                        <span>Data only</span>
                        <span className="export-mode-desc">
                          JSON file — people, heights, weights, milestones, tags
                        </span>
                      </div>
                    </label>
                    <label className="export-mode-option">
                      <input
                        type="radio"
                        name="exportMode"
                        checked={exportForm.exportMode === "with_photos"}
                        onChange={() => {
                          exportForm.exportMode = "with_photos";
                          vlens.scheduleRedraw();
                        }}
                      />
                      <div className="export-mode-label">
                        <span>With photos</span>
                        <span className="export-mode-desc">
                          ZIP — all data + original photo files
                        </span>
                      </div>
                    </label>
                  </div>
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

              {mergeForm.error && (
                <div className="error-message" role="alert">
                  {mergeForm.error}
                </div>
              )}

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

        {families.length > 0 && (
          <div className="settings-section">
            <h2>Invite Family Members</h2>
            <div className="settings-card">
              <p className="section-description">
                Share this link with family members to invite them to join your family portal.
                {families.length > 1 && " Each family has its own code."}
              </p>

              {families.map(family => (
                <div key={family.id}>
                  <div className="form-group">
                    <label>
                      {families.length > 1 ? `${family.name} Invite Code` : "Family Invite Code"}
                    </label>
                    <div className="invite-code-display">
                      <span className="invite-code">{family.inviteCode}</span>
                      <button
                        type="button"
                        className="btn btn-secondary btn-small"
                        onClick={() => onRotateInviteCode(family.id, family.name)}
                      >
                        Generate new code
                      </button>
                    </div>
                  </div>

                  <div className="form-group">
                    <label htmlFor={`inviteLink-${family.id}`}>Invite Link</label>
                    <div className="invite-link-display">
                      <input
                        type="text"
                        id={`inviteLink-${family.id}`}
                        value={`${baseUrl}/create-account?code=${family.inviteCode}`}
                        readOnly
                        className="invite-link-input"
                      />
                      <button
                        type="button"
                        className="btn btn-primary copy-button"
                        onClick={() => copyInviteLink(family.inviteCode)}
                      >
                        Copy Link
                      </button>
                    </div>
                  </div>
                </div>
              ))}

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

        <div className="settings-section">
          <h2>Policies &amp; Support</h2>
          <div className="settings-card">
            <p className="section-description">
              What is stored, what leaves the server, how long it is kept, and where to write when
              something is wrong.
            </p>
            <div className="policy-links">
              <a href="/privacy">Privacy</a>
              <a href="/terms">Terms of use</a>
              <a href="/support">Support</a>
            </div>
          </div>
        </div>

        <div className="settings-section">
          <h2>Delete Account</h2>
          <div className="settings-card">
            <div className="warning-banner">
              <strong>⚠️ Warning:</strong> This cannot be undone.
            </div>

            <p className="section-description">
              Deleting your account removes your sign-in, your sessions, your registered devices and
              your chat messages. Records stay with any family that still has another member in it.
              A family you are the only member of is deleted along with everything in it — people,
              photos, measurements and milestones. Export your data first if you want to keep it.{" "}
              <a href="/privacy">The privacy page</a> lists exactly what goes and what stays.
            </p>

            {deleteForm.error && (
              <div className="error-message" role="alert">
                {deleteForm.error}
              </div>
            )}

            <form onSubmit={vlens.cachePartial(onDeleteAccountClicked, deleteForm)}>
              <div className="form-group">
                <label htmlFor="deletePassword">Password</label>
                <input
                  type="password"
                  id="deletePassword"
                  autoComplete="current-password"
                  {...vlens.attrsBindInput(vlens.ref(deleteForm, "password"))}
                  disabled={deleteForm.loading}
                />
                <small>Leave blank if you sign in with Google or Apple.</small>
              </div>

              <div className="form-group">
                <label htmlFor="deleteConfirmEmail">
                  Type <strong>{accountEmail}</strong> to confirm
                </label>
                <input
                  type="email"
                  id="deleteConfirmEmail"
                  autoComplete="off"
                  {...vlens.attrsBindInput(vlens.ref(deleteForm, "confirmEmail"))}
                  disabled={deleteForm.loading}
                  required
                />
              </div>

              <button
                type="submit"
                className="btn btn-danger"
                disabled={deleteForm.loading || !deleteForm.confirmEmail}
              >
                {deleteForm.loading ? "Deleting..." : "Delete My Account"}
              </button>
            </form>
          </div>
        </div>
      </div>
    </div>
  );
};
