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

export const updateOperatorID = async (authToken: string, mcc: string, mnc: string) => {
  const getResponse = await fetch(`/api/v1/operator`, {
    method: "GET",
    headers: {
      "Content-Type": "application/json",
      "Authorization": "Bearer " + authToken
    },
  });
  const getRespData = await getResponse.json();
  if (!getResponse.ok) {
    throw new Error(`${getResponse.status}: ${HTTPStatus(getResponse.status)}. ${getRespData.error}`)
  }
  const OperatorIDData = getRespData.result
  OperatorIDData.mcc = mcc
  OperatorIDData.mnc = mnc
  const response = await fetch(`/api/v1/operator`, {
    method: "PUT",
    headers: {
      "Content-Type": "application/json",
      "Authorization": "Bearer " + authToken
    },
    body: JSON.stringify(OperatorIDData),
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

export const updateSupportedTacs = async (authToken: string, supportedTacs: string[]) => {
  const getResponse = await fetch(`/api/v1/operator`, {
    method: "GET",
    headers: {
      "Content-Type": "application/json",
      "Authorization": "Bearer " + authToken
    },
  });
  const getRespData = await getResponse.json();
  if (!getResponse.ok) {
    throw new Error(`${getResponse.status}: ${HTTPStatus(getResponse.status)}. ${getRespData.error}`)
  }
  const OperatorIDData = getRespData.result
  OperatorIDData.supportedTacs = supportedTacs
  const response = await fetch(`/api/v1/operator`, {
    method: "PUT",
    headers: {
      "Content-Type": "application/json",
      "Authorization": "Bearer " + authToken
    },
    body: JSON.stringify(OperatorIDData),
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