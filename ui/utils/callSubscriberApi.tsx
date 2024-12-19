import { HTTPStatus } from "@/utils/utils";

function isValidSubscriberName(name: string): boolean {
  return /^[a-zA-Z0-9-_]+$/.test(name);
}

export const apiGetAllSubscribers = async () => {
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

export const apiGetSubscriber = async (imsi: string) => {
  if (!isValidSubscriberName(imsi)) {
    throw new Error(`Error getting subscriber: Invalid name provided.`);
  }
  const response = await fetch(`/api/v1/subscribers/${imsi}`, {
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

export const apiCreateSubscriber = async (imsi: string, subscriberData: any) => {
  if (!isValidSubscriberName(imsi)) {
    throw new Error(`Error updating subscriber: Invalid name provided.`);
  }
  const response = await fetch(`/api/v1/subscribers`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify(subscriberData),
  });
  const respData = await response.json();
  if (!response.ok) {
    throw new Error(`${response.status}: ${HTTPStatus(response.status)}. ${respData.error}`)
  }
  return respData.result
};

export const apiDeleteSubscriber = async (imsi: string) => {
  if (!isValidSubscriberName(imsi)) {
    throw new Error(`Error deleting subscriber: Invalid name provided.`);
  }
  const response = await fetch(`/api/v1/subscribers/imsi-${imsi}`, {
    method: "DELETE",
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