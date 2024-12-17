import { apiGetProfile, apiPostProfile } from "@/utils/callProfileApi";
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
    const getSubscriberResponse = await apiGetSubscriber(subscriberData.UeId);

    // Workaround for https://github.com/omec-project/webconsole/issues/109
    var existingSubscriberData = await getSubscriberResponse.json();
    if (!getSubscriberResponse.ok || !existingSubscriberData["AuthenticationSubscription"]["authenticationMethod"]) {
      throw new Error("Subscriber does not exist.");
    }

    existingSubscriberData["AuthenticationSubscription"]["opc"]["opcValue"] = subscriberData.opc;
    existingSubscriberData["AuthenticationSubscription"]["permanentKey"]["permanentKeyValue"] = subscriberData.key;
    existingSubscriberData["AuthenticationSubscription"]["sequenceNumber"] = subscriberData.sequenceNumber;

    const updateSubscriberResponse = await apiCreateSubscriber(subscriberData.UeId, subscriberData);
    if (!updateSubscriberResponse.ok) {
      throw new Error(
        `Error editing subscriber. Error code: ${updateSubscriberResponse.status}`,
      );
    }
  } catch (error) {
    console.error(error);
  }
}

const getProfileData = async (deviceGroupName: string) => {
  try {
    const existingProfileResponse = await apiGetProfile(deviceGroupName);
    var existingProfileData = await existingProfileResponse.json();

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
    const updateProfileResponse = await apiPostProfile(deviceGroupName, deviceGroupData);
    if (!updateProfileResponse.ok) {
      throw new Error(
        `Error updating device group. Error code: ${updateProfileResponse.status}`,
      );
    }
  } catch (error) {
    console.error(error);
  }
}
