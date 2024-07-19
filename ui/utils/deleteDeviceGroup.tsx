import { apiGetNetworkSlice, apiPostNetworkSlice } from "@/utils/callNetworkSliceApi";
import { apiDeleteDeviceGroup } from "@/utils/callDeviceGroupApi";

interface DeleteDeviceGroupArgs {
  name: string;
  networkSliceName: string;
}

export const deleteDeviceGroup = async ({
  name,
  networkSliceName,
}: DeleteDeviceGroupArgs) => {
  try {
    const existingSliceResponse = await apiGetNetworkSlice(networkSliceName);
    if (!existingSliceResponse.ok) {
      throw new Error(
        `Error fetching network slice. Error code: ${existingSliceResponse.status}`,
      );
    }

    var existingSliceData = await existingSliceResponse.json();

    if (existingSliceData["site-device-group"]) {
      const index = existingSliceData["site-device-group"].indexOf(name);
      if (index > -1) {
        existingSliceData["site-device-group"].splice(index, 1);

        const updateSliceResponse = await apiPostNetworkSlice(networkSliceName, existingSliceData);
        if (!updateSliceResponse.ok) {
          throw new Error(
            `Error updating network slice. Error code: ${updateSliceResponse.status}`,
          );
        }
      }
    }

    const deleteResponse = await apiDeleteDeviceGroup(name);
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
