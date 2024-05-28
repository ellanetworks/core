import { NetworkSlice } from "@/components/types";

export const getNetworkSlice = async (sliceName: string): Promise<NetworkSlice> => {
  try {
    const response = await fetch(`/api/network-slice/${sliceName}`, {
      method: "GET",
    });
    if (!response.ok) {
      throw new Error("Failed to fetch network slice");
    }
    const slice = await response.json();
    return slice;
  } catch (error) {
    console.error(error);
    throw error;
  }
};
