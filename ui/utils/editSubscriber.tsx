import { apiGetProfile, apiPutProfile } from "@/utils/callProfileApi";
import { apiGetSubscriber, apiCreateSubscriber } from "@/utils/callSubscriberApi";

interface EditSubscriberArgs {
  imsi: string;
  opc: string;
  key: string;
  sequenceNumber: string;
  oldProfileName: string;
  newProfileName: string;
}

export const editSubscriber = async ({
  imsi,
  opc,
  key,
  sequenceNumber,
  oldProfileName,
  newProfileName,
}: EditSubscriberArgs) => {
  const subscriberData = {
    UeId: imsi,
    opc: opc,
    key: key,
    sequenceNumber: sequenceNumber,
  };

  try {
    await updateSubscriber(subscriberData);
    if (oldProfileName != newProfileName) {
      var oldProfileData = await getProfileData(oldProfileName);
      const index = oldProfileData["imsis"].indexOf(imsi);
      oldProfileData["imsis"].splice(index, 1);
      await updateProfileData(oldProfileName, oldProfileData);
      var newProfileData = await getProfileData(newProfileName);
      newProfileData["imsis"].push(imsi);
      await updateProfileData(newProfileName, newProfileData);
    }
  } catch (error) {
    console.error(error);
    const details =
      error instanceof Error
        ? error.message
        : `Failed to edit subscriber .`;
    throw new Error(details);
  }
};

const updateSubscriber = async (subscriberData: any) => {
  try {
    const existingSubscriberData = await apiGetSubscriber(subscriberData.UeId);
    existingSubscriberData["AuthenticationSubscription"]["opc"]["opcValue"] = subscriberData.opc;
    existingSubscriberData["AuthenticationSubscription"]["permanentKey"]["permanentKeyValue"] = subscriberData.key;
    existingSubscriberData["AuthenticationSubscription"]["sequenceNumber"] = subscriberData.sequenceNumber;
    await apiCreateSubscriber(subscriberData.UeId, subscriberData);

  } catch (error) {
    console.error(error);
  }
}

const getProfileData = async (deviceGroupName: string) => {
  try {
    const existingProfileData = await apiGetProfile(deviceGroupName);
    if (!existingProfileData["imsis"]) {
      existingProfileData["imsis"] = [];
    }
    return existingProfileData;
  } catch (error) {
    console.error(error);
  }
}

const updateProfileData = async (deviceGroupName: string, deviceGroupData: any) => {
  try {
    await apiPutProfile(deviceGroupName, deviceGroupData);
  } catch (error) {
    console.error(error);
  }
}
