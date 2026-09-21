import { createContext, useContext, useEffect, useMemo, useState, type ReactNode } from "react";

import { AUTH_EXPIRED_EVENT, controlPlane, setAuthToken, getAuthToken } from "./api";
import type { Role, TokenResponse } from "./types";

interface AuthState {
  isAuthenticated: boolean;
  role: Role | null;
  username: string | null;
  login: (username: string, password: string) => Promise<void>;
  logout: () => void;
}

const AuthContext = createContext<AuthState | null>(null);

function decodeRoleAndUser(token: string): { role: Role; username: string } | null {
  try {
    const payload = JSON.parse(atob(token.split(".")[1] ?? "")) as {
      role?: Role;
      sub?: string;
      exp?: number;
    };
    if (!payload.role || !payload.sub || (payload.exp !== undefined && payload.exp * 1000 <= Date.now())) {
      return null;
    }
    return { role: payload.role as Role, username: payload.sub as string };
  } catch {
    return null;
  }
}

export function AuthProvider({ children }: { children: ReactNode }) {
  const initial = getAuthToken();
  const initialDecoded = initial ? decodeRoleAndUser(initial) : null;

  if (initial && !initialDecoded) {
    setAuthToken(null);
  }

  const [role, setRole] = useState<Role | null>(initialDecoded?.role ?? null);
  const [username, setUsername] = useState<string | null>(initialDecoded?.username ?? null);

  const login = async (usernameInput: string, password: string) => {
    const resp = await controlPlane.post<TokenResponse>("/v1/auth/login", {
      username: usernameInput,
      password,
    });
    setAuthToken(resp.access_token);
    const decoded = decodeRoleAndUser(resp.access_token);
    setRole(decoded?.role ?? resp.role);
    setUsername(decoded?.username ?? usernameInput);
  };

  const logout = () => {
    setAuthToken(null);
    setRole(null);
    setUsername(null);
  };

  useEffect(() => {
    const handleExpiredSession = () => {
      setRole(null);
      setUsername(null);
    };
    window.addEventListener(AUTH_EXPIRED_EVENT, handleExpiredSession);
    return () => window.removeEventListener(AUTH_EXPIRED_EVENT, handleExpiredSession);
  }, []);

  const value = useMemo<AuthState>(
    () => ({ isAuthenticated: role !== null, role, username, login, logout }),
    [role, username],
  );

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth(): AuthState {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error("useAuth must be used within AuthProvider");
  return ctx;
}

export function canWrite(role: Role | null): boolean {
  return role === "admin" || role === "engineer";
}
