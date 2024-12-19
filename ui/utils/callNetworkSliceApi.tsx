import { HTTPStatus } from "@/utils/utils";

function isValidNetworkSliceName(name: string): boolean {
  return /^[a-zA-Z0-9-_]+$/.test(name);
}

export const apiGetAllNetworkSlices = async () => {
  const networkSlicesResponse = await fetch(`/api/v1/network-slices`, {
    method: "GET",
    headers: {
      "Content-Type": "application/json",
    },
  });
  const respData = await networkSlicesResponse.json();
  if (!networkSlicesResponse.ok) {
    throw new Error(`${networkSlicesResponse.status}: ${HTTPStatus(networkSlicesResponse.status)}. ${respData.error}`)
  }
  return respData.result
};

export const apiGetNetworkSlice = async (name: string) => {
  if (!isValidNetworkSliceName(name)) {
    throw new Error(`Error getting network slice: Invalid name provided: ${name}`);
  }
  const response = await fetch(`/api/v1/network-slices/${name}`, {
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

export const apiCreateNetworkSlice = async (name: string, sliceData: any) => {
  if (!isValidNetworkSliceName(name)) {
    throw new Error(`Error updating network slice: Invalid name provided.`);
  }
  const response = await fetch(`/api/v1/network-slices`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify(sliceData),
  });
  const respData = await response.json();
  if (!response.ok) {
    throw new Error(`${response.status}: ${HTTPStatus(response.status)}. ${respData.error}`)
  }
  return respData.result
};

export const apiDeleteNetworkSlice = async (name: string) => {
  if (!isValidNetworkSliceName(name)) {
    throw new Error(`Error deleting network slice: Invalid name provided.`);
  }
  const response = await fetch(`/api/v1/network-slices/${name}`, {
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