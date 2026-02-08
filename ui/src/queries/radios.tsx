import { apiFetch } from "@/queries/utils";

export type SupportedTAI = {
  plmn_id: string;
  tac: string;
};

export type APIRadio = {
  name: string;
  id: string;
  address: string;
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
