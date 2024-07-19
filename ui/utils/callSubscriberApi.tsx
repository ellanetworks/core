function isValidSubscriberName(name: string): boolean {
  return /^[a-zA-Z0-9-_]+$/.test(name);
}

export const apiGetAllSubscribers = async () => {
  try {
    var response = await fetch(`/api/subscriber`, {
        method: "GET",
        headers: {
        "Content-Type": "application/json",
        },
    });
    return response
  } catch (error) {
    console.error(error);
    throw error;
  }
};

export const apiGetSubscriber = async (imsi: string) => {
  if (isValidSubscriberName(imsi)){
    throw new Error(`Error getting subscriber: Invalid name provided.`);
  }
  try {
    const response = await fetch(`/api/subscriber/imsi-${imsi}`, {
      method: "GET",
      headers: {
        "Content-Type": "application/json",
      },
    });
    return response
  } catch (error) {
    console.error(error);
    throw error;
  }
};

export const apiPostSubscriber = async (imsi: string, subscriberData: any) => {
  if (isValidSubscriberName(imsi)){
    throw new Error(`Error updating subscriber: Invalid name provided.`);
  }
  try {
    const response = await fetch(`/api/subscriber/imsi-${imsi}`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify(subscriberData),
      });
    return response
  } catch (error) {
    console.error(error);
    throw error;
  }
};

export const apiDeleteSubscriber = async (imsi: string) => {
  if (isValidSubscriberName(imsi)){
    throw new Error(`Error deleting subscriber: Invalid name provided.`);
  }
  try {
    const response = await fetch(`/api/subscriber/imsi-${imsi}`, {
        method: "DELETE",
        headers: {
          "Content-Type": "application/json",
        },
    });
    return response
  } catch (error) {
    console.error(error);
    throw error;
  }
};