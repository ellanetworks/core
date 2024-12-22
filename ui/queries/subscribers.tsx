import { HTTPStatus } from "@/queries/utils";

export const listSubscribers = async () => {
  const response = await fetch(`/api/v1/subscribers`, {
    method: "GET",
    headers: {
      "Content-Type": "application/json",
    },
  });
  const respData = await response.json();
  if (!response.ok) {
    throw new Error(`${response.status}: ${HTTPStatus(response.status)}. ${respData.error}`)
  }
  return respData.result
};

export const createSubscriber = async (imsi: string, opc: string, key: string, sequenceNumber: string, profileName: string) => {
  const subscriberData = {
    "imsi": imsi,
    "opc": opc,
    "key": key,
    "sequenceNumber": sequenceNumber,
    "profileName": profileName
  }

  const response = await fetch(`/api/v1/subscribers`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify(subscriberData),
  });
  try {
    const respData = await response.json();
    if (!response.ok) {
      throw new Error(`${response.status}: ${HTTPStatus(response.status)}. ${respData.error}`)
    }
    return respData.result
  } catch (error) {
    throw new Error(`${response.status}: ${HTTPStatus(response.status)}. ${response.statusText}`)
  }
};

export const updateSubscriber = async (imsi: string, opc: string, key: string, sequenceNumber: string, profileName: string) => {
  const subscriberData = {
    "imsi": imsi,
    "opc": opc,
    "key": key,
    "sequenceNumber": sequenceNumber,
    "profileName": profileName
  }

  const response = await fetch(`/api/v1/subscribers/${imsi}`, {
    method: "PUT",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify(subscriberData),
  });
  try {
    const respData = await response.json();
    if (!response.ok) {
      throw new Error(`${response.status}: ${HTTPStatus(response.status)}. ${respData.error}`)
    }
    return respData.result
  } catch (error) {
    throw new Error(`${response.status}: ${HTTPStatus(response.status)}. ${response.statusText}`)
  }
}

export const deleteSubscriber = async (name: string) => {
  const response = await fetch(`/api/v1/subscribers/${name}`, {
    method: "DELETE",
    headers: {
      "Content-Type": "application/json",
    },
  });
  try {
    const respData = await response.json();
    if (!response.ok) {
      throw new Error(`${response.status}: ${HTTPStatus(response.status)}. ${respData.error}`)
    }
    return respData.result
  } catch (error) {
    throw new Error(`${response.status}: ${HTTPStatus(response.status)}. ${response.statusText}`)
  }
}