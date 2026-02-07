import { HTTPStatus } from "@/queries/utils";

/**
 * Downloads a backup as a binary blob.
 * Cannot use apiFetch because the response is a Blob, not JSON.
 */
export const backup = async (authToken: string): Promise<Blob> => {
  const response = await fetch(`/api/v1/backup`, {
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

/**
 * Uploads a backup file for restore.
 * Cannot use apiFetch because the request body is FormData, not JSON.
 */
export const restore = async (
  authToken: string,
  backupFile: Blob,
): Promise<void> => {
  const formData = new FormData();
  formData.append("backup", backupFile);

  const response = await fetch(`/api/v1/restore`, {
    method: "POST",
    headers: {
      Authorization: "Bearer " + authToken,
    },
    body: formData,
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
};
