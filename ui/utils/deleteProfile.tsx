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
    if (!existingSliceResponse.ok) {
      throw new Error(
        `Error fetching network slice. Error code: ${existingSliceResponse.status}`,
      );
    }

    var existingSliceData = await existingSliceResponse.json();

    if (existingSliceData["profiles"]) {
      const index = existingSliceData["profiles"].indexOf(name);
      if (index > -1) {
        existingSliceData["profiles"].splice(index, 1);

        const updateSliceResponse = await apiCreateNetworkSlice(networkSliceName, existingSliceData);
        if (!updateSliceResponse.ok) {
          throw new Error(
            `Error updating network slice. Error code: ${updateSliceResponse.status}`,
          );
        }
      }
    }

    const deleteResponse = await apiDeleteProfile(name);
    if (!deleteResponse.ok) {
      throw new Error(
        `Error deleting device group. Error code: ${deleteResponse.status}`,
      );
    }

    return true;
  } catch (error) {
    console.error(error);
    throw new Error("Failed to delete device group.");
  }
};
