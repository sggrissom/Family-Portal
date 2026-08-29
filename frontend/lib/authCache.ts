import { logWarn } from "./logger";
import type { FamilyRef } from "@app/server";

export interface AuthCache {
  id: number;
  name: string;
  email: string;
  isAdmin: boolean;
  emailVerified?: boolean;
  familyId: number;
  personId?: number;
  families?: FamilyRef[];
}

let _auth: AuthCache | null = (() => {
  try {
    return JSON.parse(localStorage.getItem("auth-cache")!) as AuthCache;
  } catch {
    return null;
  }
})();

export function getAuth(): AuthCache | null {
  return _auth;
}

export function setAuth(a: AuthCache) {
  _auth = a;
  localStorage.setItem("auth-cache", JSON.stringify(a));
}

export function getFamilies(): FamilyRef[] {
  if (!_auth) {
    return [];
  }
  if (_auth.families && _auth.families.length > 0) {
    return _auth.families;
  }
  if (!_auth.familyId) {
    return [];
  }
  return [{ id: _auth.familyId, name: "", role: 3, isPrimary: true }];
}

export function clearAuth() {
  _auth = null;
  localStorage.removeItem("auth-cache");
}

export async function logout() {
  try {
    await fetch("/api/logout", {
      method: "POST",
      credentials: "include",
    });
  } catch (error) {
    logWarn("auth", "Failed to logout from server", error);
  }

  clearAuth();
  if (typeof window !== "undefined") {
    window.location.href = "/";
  }
}
