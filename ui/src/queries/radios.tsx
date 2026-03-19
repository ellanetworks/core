import { apiFetch } from "@/queries/utils";

export type PlmnID = {
  mcc: string;
  mnc: string;
};

export type Tai = {
  plmnID: PlmnID;
  tac: string;
};

export type Snssai = {
  sst: number;
  sd: string;
};

export type SupportedTAI = {
  tai: Tai;
  snssais: Snssai[];
};

export type APIRadio = {
  name: string;
  id: string;
  address: string;
  ran_node_type: string;
  supported_tais: SupportedTAI[];
};

export type ListRadiosResponse = {
  items: APIRadio[];
  page: number;
  per_page: number;
  total_count: number;
};

export async function listRadios(
  authToken: string,
  page: number,
  perPage: number,
): Promise<ListRadiosResponse> {
  return apiFetch<ListRadiosResponse>(
    `/api/v1/ran/radios?page=${page}&per_page=${perPage}`,
    { authToken },
  );
}

export type APIRadioDetail = {
  name: string;
  id: string;
  address: string;
  connected_at: string;
  last_seen_at: string;
  ran_node_type: string;
  supported_tais: SupportedTAI[];
};

export async function getRadio(
  authToken: string,
  name: string,
): Promise<APIRadioDetail> {
  return apiFetch<APIRadioDetail>(
    `/api/v1/ran/radios/${encodeURIComponent(name)}`,
    { authToken },
  );
}
