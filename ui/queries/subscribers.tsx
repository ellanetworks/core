import { HTTPStatus } from "@/queries/utils";
import { Subscriber } from "@/types/types";

export const listSubscribers = async (
  authToken: string,
): Promise<Subscriber[]> => {
  const response = await fetch(`/api/v1/subscribers`, {
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

  const transformed: Subscriber[] = respData.result.map((p: any) => ({
    imsi: p.imsi,
    ipAddress: p.ipAddress,
    opc: p.opc,
    sequenceNumber: p.sequenceNumber,
    key: p.key,
    policyName: p.policyName,
  }));

  return transformed;
};

export const getSubscriber = async (authToken: string, imsi: string) => {
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

  return respData.result;
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
