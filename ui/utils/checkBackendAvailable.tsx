import { apiGetAllNetworkSlices } from "@/utils/callNetworkSliceApi";

export const checkBackendAvailable = async () => {
  try {
    const response = await apiGetAllNetworkSlices();
    return response.status === 200;
  } catch (error) {
    return false;
  }
};
