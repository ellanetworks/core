import { apiFetch } from "@/queries/utils";

export type InitializeResponse = {
  message: string;
  token: string;
};

export const initialize = async (
  email: string,
  password: string,
): Promise<InitializeResponse> => {
  return apiFetch<InitializeResponse>(`/api/v1/init`, {
    method: "POST",
    body: { email, password },
    credentials: "include",
  });
};
