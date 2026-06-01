import { createContext, useCallback, useContext, useEffect, useMemo, useState } from "react";
import { api, ApiError, setUnauthorizedHandler } from "./api/client";

const STORAGE_KEY = "ctf-evals.creds";

type AuthState = {
  authenticated: boolean;
  login: (password: string) => Promise<void>;
  logout: () => void;
};

const AuthCtx = createContext<AuthState | null>(null);

function readStored(): boolean {
  return localStorage.getItem(STORAGE_KEY) !== null;
}

export function useAuthValue(): AuthState {
  const [authenticated, setAuthenticated] = useState<boolean>(() => readStored());

  const logout = useCallback(() => {
    localStorage.removeItem(STORAGE_KEY);
    setAuthenticated(false);
  }, []);

  useEffect(() => {
    setUnauthorizedHandler(() => logout());
    return () => setUnauthorizedHandler(null);
  }, [logout]);

  const login = useCallback(async (password: string) => {
    const creds = btoa(`admin:${password}`);
    localStorage.setItem(STORAGE_KEY, creds);
    try {
      await api.me();
      setAuthenticated(true);
    } catch (err) {
      localStorage.removeItem(STORAGE_KEY);
      if (err instanceof ApiError && err.status === 401) {
        throw new Error("Wrong password");
      }
      throw err;
    }
  }, []);

  return useMemo(() => ({ authenticated, login, logout }), [authenticated, login, logout]);
}

export const AuthProvider = AuthCtx.Provider;

export function useAuth(): AuthState {
  const ctx = useContext(AuthCtx);
  if (!ctx) throw new Error("useAuth must be used inside <AuthProvider>");
  return ctx;
}
