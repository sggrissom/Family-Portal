import * as rpc from "vlens/rpc";
import * as core from "vlens/core";
import * as auth from "./authCache";
import * as server from "../server";

/**
 * For protected routes' fetch methods - ensures authentication or tries to restore from cookies.
 * Returns true if auth is available, false if redirected to login.
 */
export async function ensureAuthInFetch(): Promise<boolean> {
  // Check if we have auth in localStorage, if not try to fetch from server
  const currentAuth = auth.getAuth();
  if (!currentAuth || currentAuth.id <= 0) {
    try {
      let [authResponse, err] = await server.GetAuthContext({});
      if (authResponse) {
        // Auth context exists on server, cache it locally
        auth.setAuth(authResponse);
        return true;
      } else {
        // No valid auth context, redirect to login
        core.setRoute("/login");
        return false;
      }
    } catch (error) {
      // Failed to get auth context, redirect to login
      core.setRoute("/login");
      return false;
    }
  }
  return true;
}

/**
 * For public routes' fetch methods - redirects authenticated users to dashboard.
 * Returns true if should continue to public page, false if redirected.
 */
export async function ensureNoAuthInFetch(): Promise<boolean> {
  // Check if we already have auth stored locally
  const currentAuth = auth.getAuth();
  if (currentAuth && currentAuth.id > 0) {
    // Already authenticated, redirect to dashboard
    core.setRoute("/dashboard");
    return false;
  }

  // No local auth, but maybe we have a JWT cookie from OAuth
  // Try to fetch auth context from server
  try {
    let [authResponse, err] = await server.GetAuthContext({});
    if (authResponse) {
      // We have valid auth on server, cache it locally and redirect
      auth.setAuth(authResponse);
      core.setRoute("/dashboard");
      return false;
    }
  } catch (error) {
    // No server auth either, continue to public page
  }

  return true;
}

/**
 * For protected routes' view methods - verifies auth is still valid.
 * Returns the current auth if valid, null if redirected to login.
 */
export function requireAuthInView(): auth.AuthCache | null {
  const currentAuth = auth.getAuth();
  if (!currentAuth || currentAuth.id <= 0) {
    auth.clearAuth();
    core.setRoute("/login");
    return null;
  }
  return currentAuth;
}
