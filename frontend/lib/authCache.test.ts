import { afterEach, describe, expect, it } from "vitest";
import {
  canAdmin,
  canContribute,
  canContributeAnywhere,
  clearAuth,
  contributableFamilies,
  roleIn,
  setAuth,
} from "./authCache";

function signInWith(families: { id: number; role: number; isPrimary?: boolean }[]) {
  setAuth({
    id: 7,
    name: "Theo",
    email: "theo@example.com",
    isAdmin: false,
    familyId: families.find(f => f.isPrimary)?.id ?? 0,
    families: families.map(f => ({ name: "", isPrimary: false, ...f })),
  });
}

afterEach(() => clearAuth());

describe("family roles", () => {
  it("separates a view-only family from one the user runs", () => {
    signInWith([
      { id: 1, role: 3, isPrimary: true },
      { id: 2, role: 1 },
    ]);
    expect(canContribute(1)).toBe(true);
    expect(canAdmin(1)).toBe(true);
    expect(canContribute(2)).toBe(false);
    expect(canAdmin(2)).toBe(false);
    expect(contributableFamilies().map(f => f.id)).toEqual([1]);
  });

  it("lets a contributor add but not administer", () => {
    signInWith([{ id: 2, role: 2 }]);
    expect(canContribute(2)).toBe(true);
    expect(canAdmin(2)).toBe(false);
  });

  it("treats a family the user is not in as no access", () => {
    signInWith([{ id: 1, role: 3, isPrimary: true }]);
    expect(roleIn(99)).toBe(0);
    expect(canContribute(99)).toBe(false);
  });

  it("has nowhere to contribute when every family is view-only", () => {
    signInWith([{ id: 2, role: 1 }]);
    expect(canContributeAnywhere()).toBe(false);
  });
});
