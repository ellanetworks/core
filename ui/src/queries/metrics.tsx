import { HTTPStatus } from "@/queries/utils";

/**
 * Fetches Prometheus-format metrics as plain text.
 * Cannot use apiFetch because the response is text, not JSON.
 */
export const getMetrics = async (): Promise<string> => {
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
