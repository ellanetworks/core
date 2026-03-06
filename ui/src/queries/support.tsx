/**
 * Downloads a support bundle as a binary blob.
 * Mirrors the backup API behavior but calls the support-bundle endpoint.
 */
import { HTTPStatus } from "@/queries/utils";

export const generateSupportBundle = async (
  authToken: string,
): Promise<Blob> => {
  const response = await fetch(`/api/v1/support-bundle`, {
    method: "POST",
    headers: {
      Authorization: "Bearer " + authToken,
    },
  });

  if (!response.ok) {
    let respData: { error?: string } | undefined;
    try {
      respData = await response.json();
    } catch {
      throw new Error(
        `${response.status}: ${HTTPStatus(response.status)}. ${response.statusText}`,
      );
    }
    throw new Error(
      `${response.status}: ${HTTPStatus(response.status)}. ${respData?.error || "Unknown error"}`,
    );
  }

  return await response.blob();
};
