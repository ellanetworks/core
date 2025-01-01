export const backup = async (authToken: string): Promise<Blob> => {
  const response = await fetch(`/api/v1/backup`, {
    method: "POST",
    headers: {
      "Authorization": "Bearer " + authToken,
    },
  });

  if (!response.ok) {
    let respData;
    try {
      respData = await response.json();
    } catch {
      throw new Error(`${response.status}: ${response.statusText}`);
    }
    throw new Error(respData?.error || "Unknown error");
  }

  return await response.blob();
};

export const restore = async (authToken: string, backupFile: Blob): Promise<void> => {
  const formData = new FormData();
  formData.append("backup", backupFile);

  const response = await fetch(`/api/v1/restore`, {
    method: "POST",
    headers: {
      "Authorization": "Bearer " + authToken,
    },
    body: formData,
  });

  if (!response.ok) {
    let respData;
    try {
      respData = await response.json();
    } catch {
      throw new Error(`${response.status}: ${response.statusText}`);
    }
    throw new Error(respData?.error || "Unknown error");
  }
}