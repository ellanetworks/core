
export const apiListSubscribers = async () => {
  try {
    var response = await fetch(`/api/v1/subscribers`, {
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

export const apiGetSubscriber = async (id: string) => {
  try {
    const response = await fetch(`/api/v1/subscribers/${id}`, {
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

export const apiPostSubscriber = async (subscriberData: any) => {
  try {
    const response = await fetch(`/api/v1/subscribers`, {
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

export const apiDeleteSubscriber = async (id: string) => {
  try {
    const response = await fetch(`/api/v1/subscribers/${id}`, {
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

export const deleteSubscriber = async (id: string) => {
  try {
    const deleteSubscriberResponse = await apiDeleteSubscriber(id);
    if (!deleteSubscriberResponse.ok) {
      throw new Error("Failed to delete subscriber");
    }

    return true;
  } catch (error) {
    console.error(error);
  }
};



export const listSubscribers = async () => {
  try {
    const response = await apiListSubscribers();
    if (!response.ok) {
      throw new Error(
        `Failed to fetch subscribers. Status: ${response.status}`,
      );
    }

    var jsonResponse = await response.json();
    // Response format:
    // {"result":[1,2,3]}
    if (!jsonResponse["result"]) {
      throw new Error("Failed to fetch subscribers.");
    }

    // Fetch all subscribers
    // Save as an array of Subscriber objects
    var allSubscribers = await Promise.all(
      jsonResponse["result"].map(async (id: string) => {
        return await getSubscriber(id);
      }),
    );

    return allSubscribers.filter((item) => item !== undefined);
  } catch (error) {
    console.error(error);
    throw error;
  }
};

const getSubscriber = async (id: string) => {
  try {
    const response = await apiGetSubscriber(id);
    if (!response.ok)
      throw new Error(
        `Failed to fetch subscriber. Status: ${response.status}`,
      );
    const responseJson = await response.json();
    // Response format:
    // {"result":{"id":1,"imsi":"208930100007411","plmn_id":"20893","opc":"981d464c7c52eb6e5036234984ad0bcf","key":"5122250214c33e723a5dd523fc145fc0","sequence_number":"16f3b3f70fc2"}}
    if (!responseJson["result"]) {
      throw new Error("Failed to fetch subscriber.");
    }

    return responseJson["result"];
  } catch (error) {
    console.error(error);
  }
};


interface CreateSubscriberArgs {
  imsi: string;
  plmn_id: string;
  opc: string;
  key: string;
  sequence_number: string;
}

export const createSubscriber = async ({
  imsi,
  plmn_id,
  opc,
  key,
  sequence_number,
}: CreateSubscriberArgs) => {
  const subscriberData = {
    imsi: imsi,
    plmn_id: plmn_id,
    opc: opc,
    key: key,
    sequence_number: sequence_number,
  };

  try {

    const updateSubscriberResponse = await apiPostSubscriber(subscriberData);
    if (!updateSubscriberResponse.ok) {
      throw new Error(
        `Error creating subscriber. Error code: ${updateSubscriberResponse.status}`,
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
