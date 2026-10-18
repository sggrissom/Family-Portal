import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("vlens/core", () => ({ setRoute: vi.fn() }));
vi.mock("../server", () => ({ GetAuthContext: vi.fn() }));
import { setRoute } from "vlens/core";
import { GetAuthContext } from "../server";
import { clearAuth, getAuth, setAuth } from "./authCache";
import { ensureAuthInFetch, ensureNoAuthInFetch } from "./authHelpers";

const user = {
  id: 7,
  name: "Theo",
  email: "theo@example.com",
  isAdmin: false,
  emailVerified: true,
  familyId: 1,
  personId: 0,
  families: [],
};
const fetchMock = vi.fn();
beforeEach(() => {
  clearAuth();
  vi.clearAllMocks();
  vi.stubGlobal("fetch", fetchMock);
  vi.mocked(GetAuthContext).mockResolvedValue([null, "AuthFailure"]);
  fetchMock.mockResolvedValue({ ok: true, json: async () => ({ success: true, auth: user }) });
});
afterEach(() => {
  clearAuth();
  vi.unstubAllGlobals();
});

describe("browser session restoration", () => {
  it("refreshes an expired access cookie before loading a protected page, despite cached profile data", async () => {
    setAuth(user);
    expect(await ensureAuthInFetch()).toBe(true);
    expect(fetchMock).toHaveBeenCalledWith("/api/refresh", {
      method: "POST",
      credentials: "include",
    });
    expect(getAuth()).toEqual(user);
    expect(setRoute).not.toHaveBeenCalled();
  });

  it("restores a refresh-only session from the homepage or login page", async () => {
    // GetAuthContext normally returns an anonymous profile, rather than throwing.
    vi.mocked(GetAuthContext).mockResolvedValue([{ ...user, id: 0 }, ""]);
    expect(await ensureNoAuthInFetch()).toBe(false);
    expect(getAuth()).toEqual(user);
    expect(setRoute).toHaveBeenCalledWith("/dashboard");
  });

  it("does not refresh a valid access cookie", async () => {
    vi.mocked(GetAuthContext).mockResolvedValue([user, ""]);
    expect(await ensureAuthInFetch()).toBe(true);
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it("shares concurrent session checks and token rotation", async () => {
    expect(await Promise.all([ensureAuthInFetch(), ensureAuthInFetch()])).toEqual([true, true]);
    expect(GetAuthContext).toHaveBeenCalledTimes(1);
    expect(fetchMock).toHaveBeenCalledTimes(1);
  });

  it("redirects to login only when session restoration fails", async () => {
    setAuth(user);
    fetchMock.mockResolvedValue({ ok: false });
    expect(await ensureAuthInFetch()).toBe(false);
    expect(getAuth()).toBeNull();
    expect(setRoute).toHaveBeenCalledWith("/login");
  });

  it("allows an anonymous visitor to stay on the public page", async () => {
    fetchMock.mockResolvedValue({ ok: false });
    expect(await ensureNoAuthInFetch()).toBe(true);
    expect(setRoute).not.toHaveBeenCalled();
  });
});
