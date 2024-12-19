import { apiGetNetworkSlice, apiCreateNetworkSlice } from "@/utils/callNetworkSliceApi";
import { apiDeleteProfile } from "@/utils/callProfileApi";

interface DeleteProfileArgs {
  name: string;
  networkSliceName: string;
}

export const deleteProfile = async ({
  name,
  networkSliceName,
}: DeleteProfileArgs) => {
  try {
    const existingSliceResponse = await apiGetNetworkSlice(networkSliceName);

    if (existingSliceResponse["profiles"]) {
      const index = existingSliceResponse["profiles"].indexOf(name);
      if (index > -1) {
        existingSliceResponse["profiles"].splice(index, 1);

        const updateSliceResponse = await apiCreateNetworkSlice(networkSliceName, existingSliceResponse);
        if (!updateSliceResponse.ok) {
          throw new Error(
            `Error updating network slice. Error code: ${updateSliceResponse.status}`,
          );
        }
      }
    }

    const deleteResponse = await apiDeleteProfile(name);

    return true;
  } catch (error) {
    console.error(error);
    throw new Error("Failed to delete device group.");
  }
};
