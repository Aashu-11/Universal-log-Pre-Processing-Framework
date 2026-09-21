import type { ApiErrorBody } from "./types";

const CONTROL_PLANE_URL: string = import.meta.env.VITE_CONTROL_PLANE_URL ?? "http://localhost:8000";
const ONBOARDING_URL: string = import.meta.env.VITE_ONBOARDING_URL ?? "http://localhost:8001";

export class ApiError extends Error {
  status: number;
  constructor(status: number, message: string) {
    super(message);
    this.status = status;
  }
}

let authToken: string | null = localStorage.getItem("ulpf_token");

export const AUTH_EXPIRED_EVENT = "ulpf:auth-expired";

export function setAuthToken(token: string | null): void {
  authToken = token;
  if (token) {
    localStorage.setItem("ulpf_token", token);
  } else {
    localStorage.removeItem("ulpf_token");
  }
}

export function getAuthToken(): string | null {
  return authToken;
}

async function request<T>(baseUrl: string, path: string, init?: RequestInit): Promise<T> {
  const headers = new Headers(init?.headers);
  headers.set("Content-Type", "application/json");
  if (authToken) {
    headers.set("Authorization", `Bearer ${authToken}`);
  }

  let resp: Response;
  try {
    resp = await fetch(`${baseUrl}${path}`, { ...init, headers });
  } catch {
    throw new ApiError(0, `could not reach ${baseUrl} — is the service running?`);
  }

  if (!resp.ok) {
    let detail = resp.statusText;
    try {
      const body = (await resp.json()) as ApiErrorBody;
      if (body.detail) detail = body.detail;
    } catch {
      // response wasn't JSON — keep statusText
    }

    // A JWT can become invalid when it expires or when the control-plane is
    // recreated with a different signing secret. Do not leave the UI looking
    // authenticated while every protected request fails with "invalid token".
    if (resp.status === 401 && authToken && path !== "/v1/auth/login") {
      setAuthToken(null);
      window.dispatchEvent(new CustomEvent(AUTH_EXPIRED_EVENT, { detail }));
    }
    throw new ApiError(resp.status, detail);
  }

  if (resp.status === 204) {
    return undefined as T;
  }
  return (await resp.json()) as T;
}

export const controlPlane = {
  get: <T>(path: string) => request<T>(CONTROL_PLANE_URL, path, { method: "GET" }),
  post: <T>(path: string, body?: unknown) =>
    request<T>(CONTROL_PLANE_URL, path, { method: "POST", body: body ? JSON.stringify(body) : undefined }),
  put: <T>(path: string, body?: unknown) =>
    request<T>(CONTROL_PLANE_URL, path, { method: "PUT", body: body ? JSON.stringify(body) : undefined }),
};

export const onboarding = {
  get: <T>(path: string) => request<T>(ONBOARDING_URL, path, { method: "GET" }),
  post: <T>(path: string, body?: unknown) =>
    request<T>(ONBOARDING_URL, path, { method: "POST", body: body ? JSON.stringify(body) : undefined }),
};

export const CONTROL_PLANE_BASE = CONTROL_PLANE_URL;
export const ONBOARDING_BASE = ONBOARDING_URL;
