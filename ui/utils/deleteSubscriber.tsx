import { apiGetNetworkSlice, apiGetAllNetworkSlices } from "@/utils/callNetworkSliceApi";
import { apiGetProfile, apiPutProfile } from "@/utils/callProfileApi";
import { apiDeleteSubscriber } from "@/utils/callSubscriberApi";

export const deleteSubscriber = async (imsi: string) => {
  try {
    const networkSlicesResponse = await apiGetAllNetworkSlices();

    for (const slice of networkSlicesResponse) {
      const networkSliceResponse = await apiGetNetworkSlice(slice.name);
      const deviceGroupNames = networkSliceResponse["profiles"];
      for (const groupName of deviceGroupNames) {
        const deviceGroupResponse = await apiGetProfile(groupName);

        if (deviceGroupResponse.imsis?.includes(imsi)) {
          deviceGroupResponse.imsis = deviceGroupResponse.imsis.filter(
            (id: string) => id !== imsi,
          );

          await apiPutProfile(groupName, deviceGroupResponse);
        }
      }
    }
    await apiDeleteSubscriber(imsi);

    return true;
  } catch (error) {
    console.error(error);
  }
};
