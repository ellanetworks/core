import { apiFetch, apiFetchVoid } from "@/queries/utils";

export type AuthTokenResponse = {
  token: string;
};

export const login = async (
  email: string,
  password: string,
): Promise<AuthTokenResponse> => {
  return apiFetch<AuthTokenResponse>("/api/v1/auth/login", {
    method: "POST",
    body: { email, password },
    credentials: "include",
  });
};

export const logout = async (): Promise<void> => {
  await apiFetchVoid("/api/v1/auth/logout", {
    method: "POST",
    credentials: "include",
  });
};

export const refresh = async (): Promise<AuthTokenResponse> => {
  return apiFetch<AuthTokenResponse>("/api/v1/auth/refresh", {
    method: "POST",
    credentials: "include",
  });
};

export const lookupToken = async (authToken: string): Promise<void> => {
  await apiFetchVoid("/api/v1/auth/lookup-token", {
    method: "POST",
    authToken,
  });
};
