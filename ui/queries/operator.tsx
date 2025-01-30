import { HTTPStatus } from "@/queries/utils";

export const getOperator = async (authToken: string) => {
  const response = await fetch(`/api/v1/operator`, {
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

export const updateOperatorId = async (authToken: string, mcc: string, mnc: string) => {
  const operatorIdData = {
    mcc: mcc,
    mnc: mnc,
  };
  const response = await fetch(`/api/v1/operator/id`, {
    method: "PUT",
    headers: {
      "Content-Type": "application/json",
      "Authorization": "Bearer " + authToken
    },
    body: JSON.stringify(operatorIdData),
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

export const updateOperatorTracking = async (authToken: string, supportedTacs: string[]) => {
  const operatorTrackingData = {
    supportedTacs: supportedTacs,
  };
  const response = await fetch(`/api/v1/operator/tracking`, {
    method: "PUT",
    headers: {
      "Content-Type": "application/json",
      "Authorization": "Bearer " + authToken
    },
    body: JSON.stringify(operatorTrackingData),
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

export const updateOperatorSlice = async (authToken: string, sd: number, sst: number) => {
  if (typeof sd !== "number" || typeof sst !== "number") {
    throw new Error("Both sd and sst must be numbers.");
  }
  const operatorSliceData = {
    sd: sd,
    sst: sst,
  };

  const data = JSON.stringify(operatorSliceData)
  console.log(data)
  const response = await fetch(`/api/v1/operator/slice`, {
    method: "PUT",
    headers: {
      "Content-Type": "application/json",
      "Authorization": "Bearer " + authToken
    },
    body: JSON.stringify(operatorSliceData),
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

export const updateOperatorCode = async (authToken: string, operatorCode: string) => {
  const operatorCodeData = {
    operatorCode: operatorCode,
  };
  const response = await fetch(`/api/v1/operator/code`, {
    method: "PUT",
    headers: {
      "Content-Type": "application/json",
      "Authorization": "Bearer " + authToken
    },
    body: JSON.stringify(operatorCodeData),
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

export const updateOperatorHomeNetwork = async (authToken: string, privateKey: string) => {
  const operatorHomeNetworkData = {
    privateKey: privateKey,
  };
  const response = await fetch(`/api/v1/operator/home-network`, {
    method: "PUT",
    headers: {
      "Content-Type": "application/json",
      "Authorization": "Bearer " + authToken
    },
    body: JSON.stringify(operatorHomeNetworkData),
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