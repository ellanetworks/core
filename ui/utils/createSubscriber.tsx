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
    const checkResponse = await fetch(`/api/subscriber/imsi-${imsi}`, {
      method: "GET",
      headers: {
        "Content-Type": "application/json",
      },
    });

    // Workaround for https://github.com/omec-project/webconsole/issues/109
    const existingSubscriberData = await checkResponse.json();
    if (checkResponse.ok && existingSubscriberData["AuthenticationSubscription"]["authenticationMethod"]) {
      throw new Error("Subscriber already exists.");
    }

    const response = await fetch(`/api/subscriber/imsi-${imsi}`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify(subscriberData),
    });

    if (!response.ok) {
      throw new Error(
        `Error creating subscriber. Error code: ${response.status}`,
      );
    }

    const existingDeviceGroupResponse = await fetch(
      `/api/device-group/${deviceGroupName}`,
      {
        method: "GET",
        headers: {
          "Content-Type": "application/json",
        },
      },
    );

    const existingDeviceGroupData = await existingDeviceGroupResponse.json();

    if (!existingDeviceGroupData["imsis"]) {
      existingDeviceGroupData["imsis"] = [];
    }

    existingDeviceGroupData["imsis"].push(imsi);

    const updateDeviceGroupResponse = await fetch(
      `/api/device-group/${deviceGroupName}`,
      {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify(existingDeviceGroupData),
      },
    );

    if (!updateDeviceGroupResponse.ok) {
      throw new Error(
        `Error updating device group. Error code: ${updateDeviceGroupResponse.status}`,
      );
    }

    return response.json();
  } catch (error) {
    console.error(error);
    const details =
      error instanceof Error
        ? error.message
        : "Failed to create the subscriber.";
    throw new Error(details);
  }
};
