import { apiFetch } from "@/queries/utils";

export const initialize = async (email: string, password: string) => {
  return apiFetch(`/api/v1/init`, {
    method: "POST",
    body: { email, password },
  });
};
