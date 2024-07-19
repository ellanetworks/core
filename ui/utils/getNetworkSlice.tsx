import { NetworkSlice } from "@/components/types";
import { apiGetNetworkSlice } from "@/utils/callNetworkSliceApi";

export const getNetworkSlice = async (sliceName: string): Promise<NetworkSlice> => {
  try {
    const response = await apiGetNetworkSlice(sliceName);
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
