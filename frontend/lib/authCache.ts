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

export const ROLE_VIEW = 1;
export const ROLE_CONTRIBUTE = 2;
export const ROLE_ADMIN = 3;

export function roleIn(familyId: number): number {
  return getFamilies().find(family => family.id === familyId)?.role ?? 0;
}

export function canContribute(familyId: number): boolean {
  return roleIn(familyId) >= ROLE_CONTRIBUTE;
}

export function canAdmin(familyId: number): boolean {
  return roleIn(familyId) >= ROLE_ADMIN;
}

export function contributableFamilies(): FamilyRef[] {
  return getFamilies().filter(family => family.role >= ROLE_CONTRIBUTE);
}

export function canContributeAnywhere(): boolean {
  return contributableFamilies().length > 0;
}

export function clearAuth() {
  _auth = null;
  localStorage.removeItem("auth-cache");
}

export async function logout() {
  const prefix = `interview-draft:${_auth?.id ?? 0}:`;
  const keys = Object.keys(localStorage).filter(k => k.startsWith(prefix));
  if (
    keys.length &&
    !confirm(
      "This device has unsaved interview answers. Signing out will remove them. Sign out anyway?"
    )
  )
    return;
  for (const key of keys) localStorage.removeItem(key);
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
