import { HTTPStatus } from "@/queries/utils";

export const getMetrics = async () => {
  const response = await fetch(`/api/v1/metrics`, {
    method: "GET",
  });

  if (!response.ok) {
    throw new Error(
      `${response.status}: ${HTTPStatus(response.status)}. ${response.statusText}`,
    );
  }

  return response.text();
};
