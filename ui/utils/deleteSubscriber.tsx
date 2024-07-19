import { apiGetNetworkSlice, apiGetAllNetworkSlices } from "@/utils/callNetworkSliceApi";
import { apiGetDeviceGroup, apiPostDeviceGroup } from "@/utils/callDeviceGroupApi";
import { apiDeleteSubscriber } from "@/utils/callSubscriberApi";

export const deleteSubscriber = async (imsi: string) => {
  try {
    const networkSlicesResponse = await apiGetAllNetworkSlices();

    if (!networkSlicesResponse.ok) {
      throw new Error(
        `Error fetching network slices. Error code: ${networkSlicesResponse.status}`,
      );
    }

    const sliceNames = await networkSlicesResponse.json();

    for (const sliceName of sliceNames) {
      const networkSliceResponse = await apiGetNetworkSlice(sliceName);

      if (!networkSliceResponse.ok) {
        throw new Error(
          `Error fetching network slice. Error code: ${networkSlicesResponse.status}`,
        );
      }

      const sliceData = await networkSliceResponse.json();
      const deviceGroupNames = sliceData["site-device-group"];
      for (const groupName of deviceGroupNames) {
        const deviceGroupResponse = await apiGetDeviceGroup(groupName);

        if (!deviceGroupResponse.ok) {
          throw new Error(
            `Error fetching device group ${groupName}. Error code: ${deviceGroupResponse.status}`,
          );
        }

        var deviceGroupData = await deviceGroupResponse.json();

        if (deviceGroupData.imsis?.includes(imsi)) {
          deviceGroupData.imsis = deviceGroupData.imsis.filter(
            (id: string) => id !== imsi,
          );

          await apiPostDeviceGroup(groupName, deviceGroupData);
        }
      }
    }
    const deleteSubscriberResponse = await apiDeleteSubscriber(imsi);
    if (!deleteSubscriberResponse.ok) {
      throw new Error("Failed to delete subscriber");
    }

    return true;
  } catch (error) {
    console.error(error);
  }
};
