import { HTTPStatus } from "@/queries/utils";

export const getStatus = async () => {
  const response = await fetch(`/api/v1/status`, {
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
