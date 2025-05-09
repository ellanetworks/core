import { HTTPStatus } from "@/queries/utils";

export const listSubscribers = async (authToken: string) => {
  const response = await fetch(`/api/v1/subscribers`, {
    method: "GET",
    headers: {
      "Content-Type": "application/json",
      "Authorization": "Bearer " + authToken
    },
  });
  let respData;
  try {
    respData = await response.json();
  } catch {
    throw new Error(`${response.status}: ${HTTPStatus(response.status)}. ${response.statusText}`);
  }

  if (!response.ok) {
    throw new Error(`${response.status}: ${HTTPStatus(response.status)}. ${respData?.error || "Unknown error"}`);
  }

  return respData.result;
};

export const getSubscriber = async (authToken: string, imsi: string) => {
  const response = await fetch(`/api/v1/subscribers/${imsi}`, {
    method: "GET",
    headers: {
      "Content-Type": "application/json",
      "Authorization": "Bearer " + authToken
    },
  });
  let respData;
  try {
    respData = await response.json();
  } catch {
    throw new Error(`${response.status}: ${HTTPStatus(response.status)}. ${response.statusText}`);
  }

  if (!response.ok) {
    throw new Error(`${response.status}: ${HTTPStatus(response.status)}. ${respData?.error || "Unknown error"}`);
  }

  return respData.result;
};

export const createSubscriber = async (authToken: string, imsi: string, key: string, sequenceNumber: string, profileName: string, opc: string) => {
  const subscriberData = {
    imsi,
    key,
    sequenceNumber,
    profileName,
    opc
  };

  const response = await fetch(`/api/v1/subscribers`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      "Authorization": "Bearer " + authToken
    },
    body: JSON.stringify(subscriberData),
  });

  let respData;
  try {
    respData = await response.json();
  } catch {
    throw new Error(`${response.status}: ${HTTPStatus(response.status)}. ${response.statusText}`);
  }

  if (!response.ok) {
    throw new Error(`${response.status}: ${HTTPStatus(response.status)}. ${respData?.error || "Unknown error"}`);
  }

  return respData.result;
};

export const updateSubscriber = async (authToken: string, imsi: string, profileName: string) => {
  const subscriberData = {
    "imsi": imsi,
    "profileName": profileName
  }

  const response = await fetch(`/api/v1/subscribers/${imsi}`, {
    method: "PUT",
    headers: {
      "Content-Type": "application/json",
      "Authorization": "Bearer " + authToken
    },
    body: JSON.stringify(subscriberData),
  });
  let respData;
  try {
    respData = await response.json();
  } catch {
    throw new Error(`${response.status}: ${HTTPStatus(response.status)}. ${response.statusText}`);
  }

  if (!response.ok) {
    throw new Error(`${response.status}: ${HTTPStatus(response.status)}. ${respData?.error || "Unknown error"}`);
  }

  return respData.result;
}

export const deleteSubscriber = async (authToken: string, name: string) => {
  const response = await fetch(`/api/v1/subscribers/${name}`, {
    method: "DELETE",
    headers: {
      "Content-Type": "application/json",
      "Authorization": "Bearer " + authToken
    },
  });
  let respData;
  try {
    respData = await response.json();
  } catch {
    throw new Error(`${response.status}: ${HTTPStatus(response.status)}. ${response.statusText}`);
  }

  if (!response.ok) {
    throw new Error(`${response.status}: ${HTTPStatus(response.status)}. ${respData?.error || "Unknown error"}`);
  }

  return respData.result;
}