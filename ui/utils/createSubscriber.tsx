import { apiGetProfile, apiPutProfile } from "@/utils/callProfileApi";
import { apiCreateSubscriber } from "@/utils/callSubscriberApi";

interface CreateSubscriberArgs {
  ueId: string;  // UEID (Should start with `imsi-`)
  plmnID: string;
  opc: string;
  key: string;
  sequenceNumber: string;
  deviceGroupName: string;
}

export const createSubscriber = async ({
  ueId,
  plmnID,
  opc,
  key,
  sequenceNumber,
  deviceGroupName,
}: CreateSubscriberArgs) => {
  const subscriberData = {
    UeId: ueId,
    plmnID: plmnID,
    opc: opc,
    key: key,
    sequenceNumber: sequenceNumber,
  };

  try {
    await apiCreateSubscriber(ueId, subscriberData);
    const existingProfileData = await apiGetProfile(deviceGroupName);
    if (!existingProfileData["imsis"]) {
      existingProfileData["imsis"] = [];
    }
    const imsi = ueId.replace("imsi-", "");
    existingProfileData["imsis"].push(imsi);
    const updateProfileResponse = await apiPutProfile(deviceGroupName, existingProfileData);
    return updateProfileResponse;
  } catch (error) {
    console.error(error);
    const details =
      error instanceof Error
        ? error.message
        : "Failed to create the subscriber.";
    throw new Error(details);
  }
};
