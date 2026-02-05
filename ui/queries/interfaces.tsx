import { apiFetch, apiFetchVoid } from "@/queries/utils";

export type VlanInfo = {
  master_interface?: string;
  vlan_id?: number;
};

export type InterfacesInfo = {
  n2?: { address?: string; port?: number };
  n3?: {
    name?: string;
    address?: string;
    external_address?: string;
    vlan?: VlanInfo;
  };
  n6?: { name?: string; vlan?: VlanInfo };
  api?: { address?: string; port?: number };
};

export const getInterfaces = async (
  authToken: string,
): Promise<InterfacesInfo> => {
  return apiFetch<InterfacesInfo>(`/api/v1/networking/interfaces`, {
    authToken,
  });
};

export const updateN3Settings = async (
  authToken: string,
  externalAddress: string,
): Promise<void> => {
  await apiFetchVoid(`/api/v1/networking/interfaces/n3`, {
    method: "PUT",
    authToken,
    body: { external_address: externalAddress },
  });
};
