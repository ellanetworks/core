import { apiDeleteNetworkSlice } from "@/utils/callNetworkSliceApi";

export const deleteNetworkSlice = async (sliceName: string) => {
  try {
    await apiDeleteNetworkSlice(sliceName);
    return true;
  } catch (error) {
    console.error(error);
    return false;
  }
};
