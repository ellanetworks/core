import { HTTPStatus } from "@/queries/utils";

export type SubscriberSession = {
  ipAddress: string;
};

export type SubscriberStatus = {
  registered?: boolean;
  sessions?: SubscriberSession[];
};

export type APISubscriber = {
  imsi: string;
  opc: string;
  sequenceNumber: string;
  key: string;
  policyName: string;
  status: SubscriberStatus;
};

export type ListSubscribersResponse = {
  items: APISubscriber[];
  page: number;
  per_page: number;
  total_count: number;
};

export async function listSubscribers(
  authToken: string,
  page: number,
  perPage: number,
): Promise<ListSubscribersResponse> {
  const response = await fetch(
    `/api/v1/subscribers?page=${page}&per_page=${perPage}`,
    {
      method: "GET",
      headers: {
        "Content-Type": "application/json",
        Authorization: "Bearer " + authToken,
      },
    },
  );

  let json: { result: ListSubscribersResponse; error?: string };
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

export const getSubscriber = async (
  authToken: string,
  imsi: string,
): Promise<APISubscriber> => {
  const response = await fetch(`/api/v1/subscribers/${imsi}`, {
    method: "GET",
    headers: {
      "Content-Type": "application/json",
      Authorization: "Bearer " + authToken,
    },
  });
  let respData;
  try {
    respData = await response.json();
  } catch {
    throw new Error(
      `${response.status}: ${HTTPStatus(response.status)}. ${response.statusText}`,
    );
  }

  if (!response.ok) {
    throw new Error(
      `${response.status}: ${HTTPStatus(response.status)}. ${respData?.error || "Unknown error"}`,
    );
  }

  return {
    imsi: respData.result.imsi,
    opc: respData.result.opc,
    sequenceNumber: respData.result.sequenceNumber,
    key: respData.result.key,
    policyName: respData.result.policyName,
    status: respData.result.status,
  };
};

export const createSubscriber = async (
  authToken: string,
  imsi: string,
  key: string,
  sequenceNumber: string,
  policyName: string,
  opc: string,
) => {
  const subscriberData = {
    imsi,
    key,
    sequenceNumber,
    policyName,
    opc,
  };

  const response = await fetch(`/api/v1/subscribers`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      Authorization: "Bearer " + authToken,
    },
    body: JSON.stringify(subscriberData),
  });

  let respData;
  try {
    respData = await response.json();
  } catch {
    throw new Error(
      `${response.status}: ${HTTPStatus(response.status)}. ${response.statusText}`,
    );
  }

  if (!response.ok) {
    throw new Error(
      `${response.status}: ${HTTPStatus(response.status)}. ${respData?.error || "Unknown error"}`,
    );
  }

  return respData.result;
};

export const updateSubscriber = async (
  authToken: string,
  imsi: string,
  policyName: string,
) => {
  const subscriberData = {
    imsi: imsi,
    policyName: policyName,
  };

  const response = await fetch(`/api/v1/subscribers/${imsi}`, {
    method: "PUT",
    headers: {
      "Content-Type": "application/json",
      Authorization: "Bearer " + authToken,
    },
    body: JSON.stringify(subscriberData),
  });
  let respData;
  try {
    respData = await response.json();
  } catch {
    throw new Error(
      `${response.status}: ${HTTPStatus(response.status)}. ${response.statusText}`,
    );
  }

  if (!response.ok) {
    throw new Error(
      `${response.status}: ${HTTPStatus(response.status)}. ${respData?.error || "Unknown error"}`,
    );
  }

  return respData.result;
};

export const deleteSubscriber = async (authToken: string, name: string) => {
  const response = await fetch(`/api/v1/subscribers/${name}`, {
    method: "DELETE",
    headers: {
      "Content-Type": "application/json",
      Authorization: "Bearer " + authToken,
    },
  });
  let respData;
  try {
    respData = await response.json();
  } catch {
    throw new Error(
      `${response.status}: ${HTTPStatus(response.status)}. ${response.statusText}`,
    );
  }

  if (!response.ok) {
    throw new Error(
      `${response.status}: ${HTTPStatus(response.status)}. ${respData?.error || "Unknown error"}`,
    );
  }

  return respData.result;
};
