import * as core from "vlens/core";
import * as auth from "./authCache";
import * as server from "../server";

async function tryRefreshAuth(): Promise<auth.AuthCache | null> {
  try {
    const response = await fetch("/api/refresh", {
      method: "POST",
      credentials: "include",
    });

    if (response.ok) {
      const data = await response.json();
      if (data.success && data.auth) {
        return data.auth;
      }
    }
    return null;
  } catch (error) {
    return null;
  }
}

// Cached profile data does not prove the HttpOnly access cookie is still valid.
// Share concurrent checks so token rotation happens once within this tab.
let pendingAuth: Promise<auth.AuthCache | null> | null = null;

async function loadSession(): Promise<auth.AuthCache | null> {
  try {
    const [current] = await server.GetAuthContext({});
    if (current && current.id > 0) return current;
  } catch {
    // An expired access token can still have a valid refresh cookie.
  }
  return tryRefreshAuth();
}

async function restoreSession(): Promise<boolean> {
  if (!pendingAuth) {
    pendingAuth = loadSession().finally(() => {
      pendingAuth = null;
    });
  }
  const current = await pendingAuth;
  if (current) {
    auth.setAuth(current);
    return true;
  }
  auth.clearAuth();
  return false;
}

export async function ensureAuthInFetch(): Promise<boolean> {
  if (await restoreSession()) return true;
  core.setRoute("/login");
  return false;
}

export async function ensureNoAuthInFetch(): Promise<boolean> {
  if (await restoreSession()) {
    core.setRoute("/dashboard");
    return false;
  }
  return true;
}

export function requireAuthInView(): auth.AuthCache | null {
  const currentAuth = auth.getAuth();
  if (!currentAuth || currentAuth.id <= 0) {
    auth.clearAuth();
    core.setRoute("/login");
    return null;
  }
  return currentAuth;
}
