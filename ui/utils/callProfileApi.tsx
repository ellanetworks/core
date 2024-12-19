import { HTTPStatus } from "@/utils/utils";

function isValidProfileName(name: string): boolean {
  return /^[a-zA-Z0-9-_]+$/.test(name);
}

export const apiGetAllProfiles = async () => {
  const response = await fetch(`/api/v1/profiles/`, {
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

export const apiGetProfile = async (name: string) => {
  if (!isValidProfileName(name)) {
    throw new Error(`Error getting device group: Invalid name provided.`);
  }
  const response = await fetch(`/api/v1/profiles/${name}`, {
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

export const apiPostProfile = async (name: string, deviceGroupData: any) => {
  if (!isValidProfileName(name)) {
    throw new Error(`Error updating device group: Invalid name provided.`);
  }
  const response = await fetch(`/api/v1/profiles`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify(deviceGroupData),
  });
  const respData = await response.json();
  if (!response.ok) {
    throw new Error(`${response.status}: ${HTTPStatus(response.status)}. ${respData.error}`)
  }
  return respData.result
};

export const apiPutProfile = async (name: string, deviceGroupData: any) => {
  if (!isValidProfileName(name)) {
    throw new Error(`Error updating device group: Invalid name provided.`);
  }
  const response = await fetch(`/api/v1/profiles/${name}`, {
    method: "PUT",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify(deviceGroupData),
  });
  const respData = await response.json();
  if (!response.ok) {
    throw new Error(`${response.status}: ${HTTPStatus(response.status)}. ${respData.error}`)
  }
}

export const apiDeleteProfile = async (name: string) => {
  if (!isValidProfileName(name)) {
    throw new Error(`Error deleting device group: Invalid name provided.`);
  }
  const response = await fetch(`/api/v1/profiles/${name}`, {
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
