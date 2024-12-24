import { HTTPStatus } from "@/queries/utils";

export const getNetwork = async () => {
  const response = await fetch(`/api/v1/network`, {
    method: "GET",
    headers: {
      "Content-Type": "application/json",
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
