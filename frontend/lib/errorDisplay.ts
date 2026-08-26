const REFERENCE_PATTERN = /Reference:\s*([0-9a-f]{12})/;

export type ErrorKind =
  | "auth"
  | "permission"
  | "not-found"
  | "rate-limited"
  | "unavailable"
  | "server"
  | "unknown";

export interface DisplayableError {
  kind: ErrorKind;
  title: string;
  message: string;
  reference: string | null;
  retryable: boolean;
}

export function classifyError(raw: string): DisplayableError {
  const reference = extractReference(raw);
  const message = stripReference(raw).trim();
  const lower = message.toLowerCase();

  if (lower.includes("authfailure") || lower.includes("not signed in")) {
    return {
      kind: "auth",
      title: "You have been signed out",
      message: "Sign in again to pick up where you left off.",
      reference,
      retryable: false,
    };
  }

  if (lower.includes("access denied") || lower.includes("permission")) {
    return {
      kind: "permission",
      title: "That is not yours to see",
      message: message || "This record belongs to another family.",
      reference,
      retryable: false,
    };
  }

  if (lower.includes("not found")) {
    return {
      kind: "not-found",
      title: "Not found",
      message: message || "That record no longer exists, or the link was wrong.",
      reference,
      retryable: false,
    };
  }

  if (lower.includes("too many requests")) {
    return {
      kind: "rate-limited",
      title: "Too many requests",
      message: "Wait a moment and try again.",
      reference,
      retryable: true,
    };
  }

  if (lower.includes("not available") || lower.includes("could not be reached")) {
    return {
      kind: "unavailable",
      title: "Temporarily unavailable",
      message: message,
      reference,
      retryable: true,
    };
  }

  if (lower.includes("something went wrong")) {
    return {
      kind: "server",
      title: "Something went wrong on our end",
      message:
        "This one is not your fault, and nothing you entered was lost. Trying again often works.",
      reference,
      retryable: true,
    };
  }

  return {
    kind: "unknown",
    title: "Something went wrong",
    message: message || "The page could not be loaded.",
    reference,
    retryable: true,
  };
}

export function extractReference(raw: string): string | null {
  const match = REFERENCE_PATTERN.exec(raw);
  return match ? match[1] : null;
}

export function stripReference(raw: string): string {
  return raw.replace(REFERENCE_PATTERN, "").replace(/\s{2,}/g, " ");
}
