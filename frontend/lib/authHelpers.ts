import * as rpc from "vlens/rpc";
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

export async function ensureAuthInFetch(): Promise<boolean> {
  const currentAuth = auth.getAuth();
  if (currentAuth && currentAuth.id > 0) {
    return true;
  }

  try {
    let [authResponse, err] = await server.GetAuthContext({});
    if (authResponse && authResponse.id > 0) {
      auth.setAuth(authResponse);
      return true;
    }

    const refreshedAuth = await tryRefreshAuth();
    if (refreshedAuth) {
      auth.setAuth(refreshedAuth);
      return true;
    }

    auth.clearAuth();
    core.setRoute("/login");
    return false;
  } catch (error) {
    const refreshedAuth = await tryRefreshAuth();
    if (refreshedAuth) {
      auth.setAuth(refreshedAuth);
      return true;
    }

    auth.clearAuth();
    core.setRoute("/login");
    return false;
  }
}

export async function ensureNoAuthInFetch(): Promise<boolean> {
  const currentAuth = auth.getAuth();
  if (currentAuth && currentAuth.id > 0) {
    core.setRoute("/dashboard");
    return false;
  }

  try {
    let [authResponse, err] = await server.GetAuthContext({});
    if (authResponse && authResponse.id > 0) {
      auth.setAuth(authResponse);
      core.setRoute("/dashboard");
      return false;
    }

    const refreshedAuth = await tryRefreshAuth();
    if (refreshedAuth) {
      auth.setAuth(refreshedAuth);
      core.setRoute("/dashboard");
      return false;
    }
  } catch (error) {
    const refreshedAuth = await tryRefreshAuth();
    if (refreshedAuth) {
      auth.setAuth(refreshedAuth);
      core.setRoute("/dashboard");
      return false;
    }
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
