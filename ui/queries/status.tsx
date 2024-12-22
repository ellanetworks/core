import { HTTPStatus } from "@/queries/utils";

export const getStatus = async () => {
  const response = await fetch(`/api/v1/status`, {
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
