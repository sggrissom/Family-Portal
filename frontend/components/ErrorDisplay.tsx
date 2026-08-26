import * as preact from "preact";
import type { DisplayableError } from "../lib/errorDisplay";
import "./error-display-styles";

interface ErrorDisplayProps {
  error: DisplayableError;
  onRetry?: () => void;
}

export const ErrorDisplay = ({ error, onRetry }: ErrorDisplayProps) => (
  <div className="error-display" role="alert">
    <h1 className="error-display-title">{error.title}</h1>
    <p className="error-display-message">{error.message}</p>

    {error.reference && <ReferenceCode code={error.reference} />}

    <div className="error-display-actions">
      {error.kind === "auth" ? (
        <a href="/login" className="btn btn-primary">
          Sign in
        </a>
      ) : (
        <a href="/dashboard" className="btn btn-primary">
          Back to dashboard
        </a>
      )}
      {error.retryable && (
        <button
          type="button"
          className="btn btn-secondary"
          onClick={onRetry ?? (() => window.location.reload())}
        >
          Try again
        </button>
      )}
    </div>

    {error.reference && (
      <p className="error-display-support">
        If it keeps happening, <a href="/support">send me the code above</a> and I can find this
        exact failure in the logs.
      </p>
    )}
  </div>
);

const ReferenceCode = ({ code }: { code: string }) => (
  <div className="error-reference">
    <span className="error-reference-label">Reference</span>
    <code className="error-reference-code">{code}</code>
    <button
      type="button"
      className="error-reference-copy"
      aria-label={`Copy reference code ${code}`}
      onClick={event => copyReference(event, code)}
    >
      Copy
    </button>
  </div>
);

async function copyReference(event: Event, code: string) {
  const button = event.currentTarget as HTMLButtonElement;
  try {
    await navigator.clipboard.writeText(code);
    button.textContent = "Copied";
  } catch {
    button.textContent = "Select it";
  }
  window.setTimeout(() => {
    button.textContent = "Copy";
  }, 2000);
}
