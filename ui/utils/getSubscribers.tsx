import { Subscriber } from "@/app/(network)/subscribers/page";
import { apiGetSubscriber, apiGetAllSubscribers } from "@/utils/callSubscriberApi";

export const getSubscribers = async () => {
  try {
    const response = await apiGetAllSubscribers();
    if (!response.ok) {
      throw new Error(
        `Failed to fetch subscribers. Status: ${response.status}`,
      );
    }

    var subscriberNames = await response.json();
    if (!Array.isArray(subscriberNames) || subscriberNames.length === 0) {
      subscriberNames = [];
    }

    const allSubscribers = await Promise.all(
      subscriberNames.map(async (subscriber: Subscriber) =>
        await getSubscriber(subscriber.ueId),
      ),
    );

    return allSubscribers.filter((item) => item !== undefined);
  } catch (error) {
    console.error(error);
    throw error;
  }
};

const getSubscriber = async (imsi: string) => {
  try {
    const response = await apiGetSubscriber(imsi);
    if (!response.ok)
      throw new Error(
        `Failed to fetch subscriber. Status: ${response.status}`,
      );
    const subscriber = await response.json();
    return subscriber;
  } catch (error) {
    console.error(error);
  }
};
