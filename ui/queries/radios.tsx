import { HTTPStatus } from "@/queries/utils";

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
  const response = await fetch(
    `/api/v1/ran/radios?page=${page}&per_page=${perPage}`,
    {
      method: "GET",
      headers: {
        "Content-Type": "application/json",
        Authorization: "Bearer " + authToken,
      },
    },
  );
  let json: { result: ListRadiosResponse; error?: string };
  try {
    json = await response.json();
  } catch {
    throw new Error(
      `${response.status}: ${HTTPStatus(response.status)}. ${response.statusText}`,
    );
  }

  if (!response.ok) {
    throw new Error(
      `${response.status}: ${HTTPStatus(response.status)}. ${json?.error || "Unknown error"}`,
    );
  }

  return json.result;
}
