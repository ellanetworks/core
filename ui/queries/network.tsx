import { HTTPStatus } from "@/queries/utils";

export const getNetwork = async () => {
  const response = await fetch(`/api/v1/network`, {
    method: "GET",
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
};

export const updateNetwork = async (mcc: string, mnc: string) => {
  const getResponse = await fetch(`/api/v1/network`, {
    method: "GET",
    headers: {
      "Content-Type": "application/json",
    },
  });
  const getRespData = await getResponse.json();
  if (!getResponse.ok) {
    throw new Error(`${getResponse.status}: ${HTTPStatus(getResponse.status)}. ${getRespData.error}`)
  }
  const networkData = getRespData.result
  networkData.mcc = mcc
  networkData.mnc = mnc
  const response = await fetch(`/api/v1/network`, {
    method: "PUT",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify(networkData),
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
