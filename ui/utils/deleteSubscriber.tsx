export const deleteSubscriber = async (imsi: string) => {
  try {
    const networkSlicesResponse = await fetch(`/api/network-slice`, {
      method: "GET",
      headers: {
        "Content-Type": "application/json",
      },
    });

    if (!networkSlicesResponse.ok) {
      throw new Error(
        `Error fetching network slices. Error code: ${networkSlicesResponse.status}`,
      );
    }

    const sliceNames = await networkSlicesResponse.json();

    for (const sliceName of sliceNames) {
      const networkSliceResponse = await fetch(
        `/api/network-slice/${sliceName}`,
        {
          method: "GET",
          headers: {
            "Content-Type": "application/json",
          },
        },
      );

      if (!networkSliceResponse.ok) {
        throw new Error(
          `Error fetching network slice. Error code: ${networkSlicesResponse.status}`,
        );
      }

      const slice = await networkSliceResponse.json();

      const deviceGroupNames = slice["site-device-group"];
      for (const groupName of deviceGroupNames) {
        const deviceGroupResponse = await fetch(
          `/api/device-group/${groupName}`,
          {
            method: "GET",
            headers: {
              "Content-Type": "application/json",
            },
          },
        );

        if (!deviceGroupResponse.ok) {
          throw new Error(
            `Error fetching device group ${groupName}. Error code: ${deviceGroupResponse.status}`,
          );
        }

        const deviceGroup = await deviceGroupResponse.json();

        if (deviceGroup.imsis?.includes(imsi)) {
          deviceGroup.imsis = deviceGroup.imsis.filter(
            (id: string) => id !== imsi,
          );

          await fetch(`/api/device-group/${groupName}`, {
            method: "POST",
            headers: {
              "Content-Type": "application/json",
            },
            body: JSON.stringify(deviceGroup),
          });
        }
      }
    }
    const response = await fetch(`/api/subscriber/imsi-${imsi}`, {
      method: "DELETE",
    });

    if (!response.ok) {
      throw new Error("Failed to delete subscriber");
    }

    return true;
  } catch (error) {
    console.error(error);
  }
};
