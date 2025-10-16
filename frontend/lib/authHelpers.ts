import * as rpc from "vlens/rpc";
import * as core from "vlens/core";
import * as auth from "./authCache";
import * as server from "../server";

/**
 * Attempts to refresh the authentication token using the refresh token cookie.
 * Returns the new auth context if successful, null otherwise.
 */
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

/**
 * For protected routes' fetch methods - ensures authentication is valid.
 * Data requests will validate auth on backend, so this just ensures we have cached auth.
 * Returns true if auth is available, false if redirected to login.
 */
export async function ensureAuthInFetch(): Promise<boolean> {
  const currentAuth = auth.getAuth();
  if (currentAuth && currentAuth.id > 0) {
    return true;
  }

  // No cached auth, try to get it from server
  try {
    let [authResponse, err] = await server.GetAuthContext({});
    if (authResponse && authResponse.id > 0) {
      // Valid JWT, cache it and continue
      auth.setAuth(authResponse);
      return true;
    }

    // No valid JWT, try refresh token
    const refreshedAuth = await tryRefreshAuth();
    if (refreshedAuth) {
      auth.setAuth(refreshedAuth);
      return true;
    }

    // Refresh failed, clear cache and redirect to login
    auth.clearAuth();
    core.setRoute("/login");
    return false;
  } catch (error) {
    // GetAuthContext failed, try refresh
    const refreshedAuth = await tryRefreshAuth();
    if (refreshedAuth) {
      auth.setAuth(refreshedAuth);
      return true;
    }

    // Refresh failed, clear cache and redirect to login
    auth.clearAuth();
    core.setRoute("/login");
    return false;
  }
}

/**
 * For public routes' fetch methods - redirects authenticated users to dashboard.
 * Returns true if should continue to public page, false if redirected.
 */
export async function ensureNoAuthInFetch(): Promise<boolean> {
  // Check if we already have auth stored locally (quick path)
  const currentAuth = auth.getAuth();
  if (currentAuth && currentAuth.id > 0) {
    // Already authenticated, redirect to dashboard
    core.setRoute("/dashboard");
    return false;
  }

  // No local auth, check server
  try {
    let [authResponse, err] = await server.GetAuthContext({});
    if (authResponse && authResponse.id > 0) {
      // Valid auth on server, cache it and redirect to dashboard
      auth.setAuth(authResponse);
      core.setRoute("/dashboard");
      return false;
    }

    // No valid JWT, try refresh token
    const refreshedAuth = await tryRefreshAuth();
    if (refreshedAuth) {
      auth.setAuth(refreshedAuth);
      core.setRoute("/dashboard");
      return false;
    }
  } catch (error) {
    // GetAuthContext failed, try refresh token
    const refreshedAuth = await tryRefreshAuth();
    if (refreshedAuth) {
      auth.setAuth(refreshedAuth);
      core.setRoute("/dashboard");
      return false;
    }
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
