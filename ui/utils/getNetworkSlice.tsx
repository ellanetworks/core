import { NetworkSlice } from "@/components/types";
import { apiGetNetworkSlice } from "@/utils/callNetworkSliceApi";

export const getNetworkSlice = async (sliceName: string): Promise<NetworkSlice> => {
  console.log("Getting network slice: " + sliceName);
  try {
    const slice = await apiGetNetworkSlice(sliceName);
    return slice;
  } catch (error) {
    console.error(error);
    throw error;
  }
};
