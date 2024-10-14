import { apiGetSubscriber, apiPostSubscriber } from "@/queries/subscribers";

interface EditSubscriberArgs {
  imsi: string;
  opc: string;
  key: string;
  sequence_number: string;
}

export const editSubscriber = async ({
  imsi,
  opc,
  key,
  sequence_number,
}: EditSubscriberArgs) => {
  const subscriberData = {
    imsi: imsi,
    opc: opc,
    key: key,
    sequence_number: sequence_number,
  };

  try {
    await updateSubscriber(subscriberData);
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
    const getSubscriberResponse = await apiGetSubscriber(subscriberData.imsi);

    // Workaround for https://github.com/omec-project/webconsole/issues/109
    var existingSubscriberData = await getSubscriberResponse.json();
    if (!getSubscriberResponse.ok || !existingSubscriberData["AuthenticationSubscription"]["authenticationMethod"]) {
      throw new Error("Subscriber does not exist.");
    }

    existingSubscriberData["AuthenticationSubscription"]["opc"]["opcValue"] = subscriberData.opc;
    existingSubscriberData["AuthenticationSubscription"]["permanentKey"]["permanentKeyValue"] = subscriberData.key;
    existingSubscriberData["AuthenticationSubscription"]["sequence_number"] = subscriberData.sequence_number;

    const updateSubscriberResponse = await apiPostSubscriber(subscriberData);
    if (!updateSubscriberResponse.ok) {
      throw new Error(
        `Error editing subscriber. Error code: ${updateSubscriberResponse.status}`,
      );
    }
  } catch (error) {
    console.error(error);
  }
}
