import { NetworkSlice } from "@/components/types";
import { apiGetNetworkSlice } from "@/utils/callNetworkSliceApi";

export const getNetworkSlice = async (sliceName: string): Promise<NetworkSlice> => {
  console.log("Getting network slice: " + sliceName);
  try {
    const response = await apiGetNetworkSlice(sliceName);
    if (!response.ok) {
      throw new Error("Failed to fetch network slice: " + sliceName);
    }
    const slice = await response.json();
    return slice;
  } catch (error) {
    console.error(error);
    throw error;
  }
};
