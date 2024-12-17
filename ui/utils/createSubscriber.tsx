import { apiGetProfile, apiPostProfile } from "@/utils/callProfileApi";
import { apiGetSubscriber, apiCreateSubscriber } from "@/utils/callSubscriberApi";

interface CreateSubscriberArgs {
  imsi: string;
  opc: string;
  key: string;
  sequenceNumber: string;
  deviceGroupName: string;
}

export const createSubscriber = async ({
  imsi,
  opc,
  key,
  sequenceNumber,
  deviceGroupName,
}: CreateSubscriberArgs) => {
  const subscriberData = {
    UeId: imsi,
    opc: opc,
    key: key,
    sequenceNumber: sequenceNumber,
  };

  try {
    const getSubscriberResponse = await apiGetSubscriber(imsi);

    // Workaround for https://github.com/omec-project/webconsole/issues/109
    const existingSubscriberData = await getSubscriberResponse.json();
    if (getSubscriberResponse.ok && existingSubscriberData["AuthenticationSubscription"]["authenticationMethod"]) {
      throw new Error("Subscriber already exists.");
    }

    const updateSubscriberResponse = await apiCreateSubscriber(imsi, subscriberData);
    if (!updateSubscriberResponse.ok) {
      throw new Error(
        `Error creating subscriber. Error code: ${updateSubscriberResponse.status}`,
      );
    }

    const existingProfileResponse = await apiGetProfile(deviceGroupName);
    var existingProfileData = await existingProfileResponse.json();

    if (!existingProfileData["imsis"]) {
      existingProfileData["imsis"] = [];
    }
    existingProfileData["imsis"].push(imsi);

    const updateProfileResponse = await apiPostProfile(deviceGroupName, existingProfileData);
    if (!updateProfileResponse.ok) {
      throw new Error(
        `Error updating device group. Error code: ${updateProfileResponse.status}`,
      );
    }

    return updateSubscriberResponse.json();
  } catch (error) {
    console.error(error);
    const details =
      error instanceof Error
        ? error.message
        : "Failed to create the subscriber.";
    throw new Error(details);
  }
};
