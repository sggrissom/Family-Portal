import * as preact from "preact";
import * as vlens from "vlens";
import * as server from "../server";
import * as auth from "../lib/authCache";
import "./verify-banner-styles";

const DISMISS_KEY = "verify-banner-dismissed";

type BannerState = {
  dismissed: boolean;
  sending: boolean;
  sent: boolean;
  error: string;
};

const useBanner = vlens.declareHook(
  (): BannerState => ({
    dismissed: readDismissed(),
    sending: false,
    sent: false,
    error: "",
  })
);

function readDismissed(): boolean {
  try {
    return sessionStorage.getItem(DISMISS_KEY) === "1";
  } catch {
    return false;
  }
}

async function resendClicked(state: BannerState, event: Event) {
  event.preventDefault();
  if (state.sending) return;

  state.sending = true;
  state.error = "";
  vlens.scheduleRedraw();

  const [resp, err] = await server.ResendVerificationEmail({});
  state.sending = false;
  if (err || !resp?.success) {
    state.error = resp?.error || "Could not send that right now. Try again in a minute.";
  } else {
    state.sent = true;
  }
  vlens.scheduleRedraw();
}

function dismissClicked(state: BannerState, event: Event) {
  event.preventDefault();
  state.dismissed = true;
  try {
    sessionStorage.setItem(DISMISS_KEY, "1");
  } catch {
    // Dismissal is a convenience; losing it is fine.
  }
  vlens.scheduleRedraw();
}

// Advisory only — nothing in the app is gated on confirming. It stays out of the
// way after one dismissal per browser session.
export const VerifyEmailBanner = (): preact.ComponentChild => {
  const currentAuth = auth.getAuth();
  if (!currentAuth || currentAuth.id <= 0 || currentAuth.emailVerified !== false) {
    return null;
  }

  const state = useBanner();
  if (state.dismissed) {
    return null;
  }

  return (
    <div className="verify-banner" role="status">
      <p className="verify-banner-text">
        {state.sent
          ? `Sent — check ${currentAuth.email} for the confirmation link.`
          : state.error
            ? state.error
            : `Confirm your email address so you can reset your password if you ever get locked out.`}
      </p>
      <div className="verify-banner-actions">
        {!state.sent && (
          <button
            type="button"
            className="verify-banner-button"
            disabled={state.sending}
            onClick={vlens.cachePartial(resendClicked, state)}
          >
            {state.sending ? "Sending…" : "Send the link"}
          </button>
        )}
        <button
          type="button"
          className="verify-banner-dismiss"
          aria-label="Dismiss"
          onClick={vlens.cachePartial(dismissClicked, state)}
        >
          ×
        </button>
      </div>
    </div>
  );
};
