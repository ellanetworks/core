import { NetworkSlice } from "@/components/types";
import { apiGetNetworkSlice, apiGetAllNetworkSlices } from "@/utils/callNetworkSliceApi";

export const getNetworkSlices = async (): Promise<NetworkSlice[]> => {
  try {
    const slices = await apiGetAllNetworkSlices();
    return slices;
  } catch (error) {
    console.error(error);
    throw error;
  }
};
