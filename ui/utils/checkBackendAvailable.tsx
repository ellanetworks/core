import { apiGetStatus } from "@/utils/callStatusApi";

export const checkBackendAvailable = async () => {
  try {
    const response = await apiGetStatus();
    return response.version !== "";
  } catch (error) {
    return false;
  }
};
