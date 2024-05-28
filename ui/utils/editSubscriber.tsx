interface EditSubscriberArgs {
  imsi: string;
  opc: string;
  key: string;
  sequenceNumber: string;
  oldDeviceGroupName: string;
  newDeviceGroupName: string;
}

export const editSubscriber = async ({
  imsi,
  opc,
  key,
  sequenceNumber,
  oldDeviceGroupName,
  newDeviceGroupName,
}: EditSubscriberArgs) => {
  const subscriberData = {
    UeId: imsi,
    opc: opc,
    key: key,
    sequenceNumber: sequenceNumber,
  };

  try {
    await updateSubscriber(subscriberData);
    if (oldDeviceGroupName != newDeviceGroupName) {
      var oldDeviceGroupData = await getDeviceGroupData(oldDeviceGroupName);
      const index = oldDeviceGroupData["imsis"].indexOf(imsi);
      oldDeviceGroupData["imsis"].splice(index, 1);
      await updateDeviceGroupData(oldDeviceGroupName, oldDeviceGroupData);
      var newDeviceGroupData = await getDeviceGroupData(newDeviceGroupName);
      newDeviceGroupData["imsis"].push(imsi);
      await updateDeviceGroupData(newDeviceGroupName, newDeviceGroupData);
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
    const checkResponse = await fetch(`/api/subscriber/imsi-${subscriberData.UeId}`, {
      method: "GET",
      headers: {
      "Content-Type": "application/json",
      },
    });

    // Workaround for https://github.com/omec-project/webconsole/issues/109
    var existingSubscriberData = await checkResponse.json();
    if (!checkResponse.ok || !existingSubscriberData["AuthenticationSubscription"]["authenticationMethod"]) {
      throw new Error("Subscriber does not exist.");
    }

    existingSubscriberData["AuthenticationSubscription"]["opc"]["opcValue"] = subscriberData.opc;
    existingSubscriberData["AuthenticationSubscription"]["permanentKey"]["permanentKeyValue"] = subscriberData.key;
    existingSubscriberData["AuthenticationSubscription"]["sequenceNumber"] = subscriberData.sequenceNumber;

    const response = await fetch(`/api/subscriber/imsi-${subscriberData.UeId}`, {
      method: "POST",
      headers: {
      "Content-Type": "application/json",
      },
      body: JSON.stringify(subscriberData),
    });
    
    if (!response.ok) {
      throw new Error(
      `Error editing subscriber. Error code: ${response.status}`,
      );
    }
  } catch (error) {
    console.error(error);
  }
}

const getDeviceGroupData = async (deviceGroupName: string) => {
  try {
    const existingDeviceGroupResponse = await fetch(`/api/device-group/${deviceGroupName}`, {
      method: "GET",
      headers: {
      "Content-Type": "application/json",
      },
    });

    var existingDeviceGroupData = await existingDeviceGroupResponse.json();

    if (!existingDeviceGroupData["imsis"]) {
      existingDeviceGroupData["imsis"] = [];
    }
    return existingDeviceGroupData;
  } catch (error) {
    console.error(error);
  }
}

const updateDeviceGroupData = async (deviceGroupName:string, deviceGroupData: any) => {
  try {
    const updateDeviceGroupResponse = await fetch(`/api/device-group/${deviceGroupName}`, {
      method: "POST",
      headers: {
      "Content-Type": "application/json",
      },
      body: JSON.stringify(deviceGroupData),
    });

    if (!updateDeviceGroupResponse.ok) {
      throw new Error(
      `Error updating device group. Error code: ${updateDeviceGroupResponse.status}`,
      );
    }
  } catch (error) {
    console.error(error);
  }
}
