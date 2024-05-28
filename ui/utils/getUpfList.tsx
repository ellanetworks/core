export interface UpfItem {
  hostname: string;
  port: string;
}

export const getUpfList = async (): Promise<UpfItem[]> => {
  try {
    const response = await fetch("/api/upf", {
      method: "GET",
    });
    if (!response.ok) {
      throw new Error("Failed to fetch UPF list");
    }
    const upfList = await response.json();
    return upfList;
  } catch (error) {
    console.error(error);
    throw error;
  }
};
