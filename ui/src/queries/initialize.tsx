import { apiFetchVoid } from "@/queries/utils";

export const initialize = async (
  email: string,
  password: string,
): Promise<void> => {
  await apiFetchVoid(`/api/v1/init`, {
    method: "POST",
    body: { email, password },
  });
};
