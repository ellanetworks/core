import { Subscriber } from "@/app/(network)/subscribers/page";
import { apiGetSubscriber, apiGetAllSubscribers } from "@/utils/callSubscriberApi";

export const getSubscribers = async () => {
  try {
    const subscribers = await apiGetAllSubscribers();

    const allSubscribers = await Promise.all(
      subscribers.map(async (subscriber: Subscriber) =>
        await apiGetSubscriber(subscriber.ueId),
      ),
    );

    return allSubscribers.filter((item) => item !== undefined);
  } catch (error) {
    console.error(error);
    throw error;
  }
};
