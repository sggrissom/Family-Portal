import { logWarn } from "./logger";
import type { FamilyRef } from "@app/server";

export interface AuthCache {
  id: number;
  name: string;
  email: string;
  isAdmin: boolean;
  familyId: number;
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

// getFamilies returns the families the user belongs to, falling back to the
// primary family alone for caches written before memberships were returned.
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
  // Call backend to clear JWT cookie
  try {
    await fetch("/api/logout", {
      method: "POST",
      credentials: "include",
    });
  } catch (error) {
    // Continue with logout even if backend call fails
    logWarn("auth", "Failed to logout from server", error);
  }

  clearAuth();
  // Clear any auth headers for future requests
  if (typeof window !== "undefined") {
    // Redirect to home page after logout
    window.location.href = "/";
  }
}
