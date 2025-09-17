export interface AuthCache {
  id: number;
  name: string;
  email: string;
  isAdmin: boolean;
  familyId: number;
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

export function clearAuth() {
  _auth = null;
  localStorage.removeItem("auth-cache");
}

export function logout() {
  clearAuth();
  // Clear any auth headers for future requests
  if (typeof window !== 'undefined') {
    // Redirect to home page after logout
    window.location.href = '/';
  }
}