import { apiFetch, apiFetchVoid } from "@/queries/utils";

export interface HomeNetworkKey {
  id: number;
  keyIdentifier: number;
  scheme: "A" | "B";
  publicKey: string;
}

export interface SliceData {
  name: string;
  sst: number;
  sd?: string | null;
}

export interface OperatorData {
  id: { mcc: string; mnc: string };
  slice: SliceData;
  tracking: { supportedTacs: string[] };
  homeNetworkKeys: HomeNetworkKey[];
  nasSecurity: {
    ciphering: string[];
    integrity: string[];
  };
  spn: { fullName: string; shortName: string };
}

type RawOperatorData = Omit<OperatorData, "slice">;

type ListSlicesResponse = {
  items: SliceData[];
  page: number;
  per_page: number;
  total_count: number;
};

export const getOperator = async (authToken: string): Promise<OperatorData> => {
  const [rawOp, slicesRes] = await Promise.all([
    apiFetch<RawOperatorData>(`/api/v1/operator`, { authToken }),
    apiFetch<ListSlicesResponse>(`/api/v1/slices?page=1&per_page=1`, {
      authToken,
    }),
  ]);

  const firstSlice = slicesRes.items?.[0];
  return {
    ...rawOp,
    slice: firstSlice ?? { name: "default", sst: 1 },
  };
};

export const updateOperatorID = async (
  authToken: string,
  mcc: string,
  mnc: string,
): Promise<void> => {
  await apiFetchVoid(`/api/v1/operator/id`, {
    method: "PUT",
    authToken,
    body: { mcc, mnc },
  });
};

export const updateOperatorTracking = async (
  authToken: string,
  supportedTacs: string[],
): Promise<void> => {
  await apiFetchVoid(`/api/v1/operator/tracking`, {
    method: "PUT",
    authToken,
    body: { supportedTacs },
  });
};

export const updateOperatorSlice = async (
  authToken: string,
  sst: number,
  sd?: string | null,
): Promise<void> => {
  if (typeof sst !== "number") {
    throw new Error("SST must be a number.");
  }

  // Fetch the (single) existing slice to get its name.
  const slicesRes = await apiFetch<{
    items: { name: string }[];
  }>(`/api/v1/slices?page=1&per_page=1`, { authToken });

  const sliceName = slicesRes.items?.[0]?.name;
  if (!sliceName) {
    throw new Error("No network slice configured");
  }

  await apiFetchVoid(`/api/v1/slices/${sliceName}`, {
    method: "PUT",
    authToken,
    body: { sst, ...(sd ? { sd } : {}) },
  });
};

export const updateOperatorCode = async (
  authToken: string,
  operatorCode: string,
): Promise<void> => {
  await apiFetchVoid(`/api/v1/operator/code`, {
    method: "PUT",
    authToken,
    body: { operatorCode },
  });
};

export const createHomeNetworkKey = async (
  authToken: string,
  keyIdentifier: number,
  scheme: string,
  privateKey: string,
): Promise<void> => {
  await apiFetchVoid(`/api/v1/operator/home-network-keys`, {
    method: "POST",
    authToken,
    body: { keyIdentifier, scheme, privateKey },
  });
};

export const deleteHomeNetworkKey = async (
  authToken: string,
  id: number,
): Promise<void> => {
  await apiFetchVoid(`/api/v1/operator/home-network-keys/${id}`, {
    method: "DELETE",
    authToken,
  });
};

export const getHomeNetworkKeyPrivateKey = async (
  authToken: string,
  id: number,
): Promise<{ privateKey: string }> => {
  return apiFetch<{ privateKey: string }>(
    `/api/v1/operator/home-network-keys/${id}/private-key`,
    { authToken },
  );
};

export const updateOperatorNASSecurity = async (
  authToken: string,
  ciphering: string[],
  integrity: string[],
): Promise<void> => {
  await apiFetchVoid(`/api/v1/operator/nas-security`, {
    method: "PUT",
    authToken,
    body: { ciphering, integrity },
  });
};

export const updateOperatorSPN = async (
  authToken: string,
  fullName: string,
  shortName: string,
): Promise<void> => {
  await apiFetchVoid(`/api/v1/operator/spn`, {
    method: "PUT",
    authToken,
    body: { fullName, shortName },
  });
};
