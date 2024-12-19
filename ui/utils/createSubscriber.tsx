import { apiGetProfile, apiPostProfile } from "@/utils/callProfileApi";
import { apiGetSubscriber, apiCreateSubscriber } from "@/utils/callSubscriberApi";

interface CreateSubscriberArgs {
  imsi: string;
  plmnID: string;
  opc: string;
  key: string;
  sequenceNumber: string;
  deviceGroupName: string;
}

export const createSubscriber = async ({
  imsi,
  plmnID,
  opc,
  key,
  sequenceNumber,
  deviceGroupName,
}: CreateSubscriberArgs) => {
  const subscriberData = {
    UeId: imsi,
    plmnID: plmnID,
    opc: opc,
    key: key,
    sequenceNumber: sequenceNumber,
  };

  try {
    await apiCreateSubscriber(imsi, subscriberData);
    const existingProfileData = await apiGetProfile(deviceGroupName);
    if (!existingProfileData["imsis"]) {
      existingProfileData["imsis"] = [];
    }
    existingProfileData["imsis"].push(imsi);
    const updateProfileResponse = await apiPostProfile(deviceGroupName, existingProfileData);
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
