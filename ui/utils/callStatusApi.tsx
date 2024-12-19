import { HTTPStatus } from "@/utils/utils";

export const apiGetStatus = async () => {
  const statusResponse = await fetch(`/api/v1/status`, {
    method: "GET",
    headers: {
      "Content-Type": "application/json",
    },
  });
  const respData = await statusResponse.json();
  if (!statusResponse.ok) {
    throw new Error(`${statusResponse.status}: ${HTTPStatus(statusResponse.status)}. ${respData.error}`)
  }
  return respData.result
};
