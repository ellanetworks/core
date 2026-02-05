import { apiFetch } from "@/queries/utils";

export interface OperatorData {
  id: { mcc: string; mnc: string };
  slice: { sst: number; sd?: string | null };
  tracking: { supportedTacs: string[] };
  homeNetwork: { publicKey: string };
}

export const getOperator = async (authToken: string): Promise<OperatorData> => {
  return apiFetch<OperatorData>(`/api/v1/operator`, { authToken });
};

export const updateOperatorID = async (
  authToken: string,
  mcc: string,
  mnc: string,
) => {
  return apiFetch(`/api/v1/operator/id`, {
    method: "PUT",
    authToken,
    body: { mcc, mnc },
  });
};

export const updateOperatorTracking = async (
  authToken: string,
  supportedTacs: string[],
) => {
  return apiFetch(`/api/v1/operator/tracking`, {
    method: "PUT",
    authToken,
    body: { supportedTacs },
  });
};

export const updateOperatorSlice = async (
  authToken: string,
  sst: number,
  sd?: string | null,
) => {
  if (typeof sst !== "number") {
    throw new Error("SST must be a number.");
  }
  return apiFetch(`/api/v1/operator/slice`, {
    method: "PUT",
    authToken,
    body: { sd, sst },
  });
};

export const updateOperatorCode = async (
  authToken: string,
  operatorCode: string,
) => {
  return apiFetch(`/api/v1/operator/code`, {
    method: "PUT",
    authToken,
    body: { operatorCode },
  });
};

export const updateOperatorHomeNetwork = async (
  authToken: string,
  privateKey: string,
) => {
  return apiFetch(`/api/v1/operator/home-network`, {
    method: "PUT",
    authToken,
    body: { privateKey },
  });
};
